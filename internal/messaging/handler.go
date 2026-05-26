package messaging

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/attachment"
	"github.com/bhyland-usda/job-portal/internal/middleware"
	"github.com/bhyland-usda/job-portal/internal/notification"
)

type Handler struct {
	db     *sql.DB
	pages  map[string]*template.Template
	notif  *notification.Handler
	broker *Broker
}

func NewHandler(db *sql.DB, pages map[string]*template.Template, notif *notification.Handler) *Handler {
	return &Handler{
		db:     db,
		pages:  pages,
		notif:  notif,
		broker: NewBroker(),
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	// API routes for chat drawer
	mux.Handle("GET /messages/search-users", requireAuth(http.HandlerFunc(h.searchUsers)))
	mux.Handle("POST /messages/start", requireAuth(http.HandlerFunc(h.startGroupConversation)))
	mux.Handle("GET /messages/recent", requireAuth(http.HandlerFunc(h.recentConversations)))
	mux.Handle("GET /messages/events", requireAuth(http.HandlerFunc(h.streamMessages)))
	mux.Handle("GET /messages/attachment/{id}", requireAuth(http.HandlerFunc(h.serveAttachment)))
	mux.Handle("GET /messages/chat/{id}", requireAuth(http.HandlerFunc(h.chatMessages)))
	mux.Handle("POST /messages/chat/{id}", requireAuth(http.HandlerFunc(h.chatSend)))
	mux.Handle("POST /messages/new/{userID}", requireAuth(http.HandlerFunc(h.startConversation)))
	mux.Handle("POST /messages/chat/{id}/members", requireAuth(http.HandlerFunc(h.addMember)))
	mux.Handle("POST /messages-typing/{id}", requireAuth(http.HandlerFunc(h.handleTyping)))
	mux.Handle("POST /messages-pin/{userID}", requireAuth(http.HandlerFunc(h.togglePinContact)))
}

func (h *Handler) isParticipant(ctx context.Context, convID, userID string) bool {
	var exists bool
	err := h.db.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM conversation_participants
			WHERE conversation_id = $1 AND user_id = $2
		)`, convID, userID,
	).Scan(&exists)

	return err == nil && exists
}

// findDirectConversation returns the id of the existing 1:1 conversation between
// exactly userA and userB — a conversation with exactly two participants — or an
// error (e.g. sql.ErrNoRows) if none exists. The participant-count constraint
// (cp3 ... = 2) is critical: without it, a group conversation that merely
// includes both users would match, routing a direct message into that group.
func (h *Handler) findDirectConversation(ctx context.Context, userA, userB string) (string, error) {
	var id string
	err := h.db.QueryRowContext(ctx,
		`SELECT cp1.conversation_id
		 FROM conversation_participants cp1
		 JOIN conversation_participants cp2
		 	ON cp1.conversation_id = cp2.conversation_id
		 WHERE cp1.user_id = $1 AND cp2.user_id = $2
		   AND (SELECT COUNT(*) FROM conversation_participants cp3
		        WHERE cp3.conversation_id = cp1.conversation_id) = 2
		 LIMIT 1`,
		userA, userB,
	).Scan(&id)
	return id, err
}

// directKey returns the canonical, order-independent key for a 1:1 conversation
// between two users (the two ids sorted ascending, joined by '-'). It must match
// migration 048's backfill so conversations.direct_key — guarded by a partial
// unique index — prevents duplicate direct conversations for the same pair.
func directKey(userA, userB string) string {
	if userA < userB {
		return userA + "-" + userB
	}
	return userB + "-" + userA
}

func (h *Handler) startConversation(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	otherUserID := r.PathValue("userID")

	if userID == otherUserID {
		http.Error(w, "Cannot message yourself", http.StatusBadRequest)
		return
	}

	// Reuse an existing DIRECT (1:1) conversation if one exists.
	existingID, err := h.findDirectConversation(r.Context(), userID, otherUserID)

	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": existingID})
		return
	}

	// Create new conversation
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		slog.Error("failed to begin conversation tx", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var convID string
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO conversations (direct_key) VALUES ($1) RETURNING id`,
		directKey(userID, otherUserID),
	).Scan(&convID)
	if err != nil {
		// A concurrent request may have created this direct conversation first,
		// tripping the unique index on direct_key. Recover by returning it.
		if existing, lookupErr := h.findDirectConversation(r.Context(), userID, otherUserID); lookupErr == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"id": existing})
			return
		}
		slog.Error("failed to create conversation", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO conversation_participants (conversation_id, user_id)
		 VALUES ($1, $2), ($1, $3)`,
		convID, userID, otherUserID,
	)
	if err != nil {
		slog.Error("failed to add participants", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit conversation", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": convID})
}

func (h *Handler) streamMessages(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	resp_cont := http.NewResponseController(w)
	resp_cont.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.broker.Subscribe(userID)
	defer h.broker.Unsubscribe(userID)

	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprint(w, sseDataLine(ev))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

type searchedUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (h *Handler) searchUsers(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	scope := r.URL.Query().Get("scope")
	if q == "" || len(q) < 2 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]searchedUser{})
		return
	}

	query := `SELECT u.id, CONCAT(u.first_name, ' ', u.last_name)
		 FROM users u
		 WHERE u.id != $1
		   AND (LOWER(u.first_name) LIKE LOWER($2) OR LOWER(u.last_name) LIKE LOWER($2) OR LOWER(CONCAT(u.first_name, ' ', u.last_name)) LIKE LOWER($2))`

	if scope != "all" {
		query += ` AND EXISTS(
			SELECT 1 FROM connections c
			WHERE c.status = 'accepted'
			  AND ((c.requester_id = $1 AND c.addressee_id = u.id) OR (c.addressee_id = $1 AND c.requester_id = u.id))
		)`
	}

	query += ` ORDER BY u.first_name, u.last_name LIMIT 10`

	rows, err := h.db.QueryContext(r.Context(), query, userID, "%"+q+"%")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]searchedUser{})
		return
	}
	defer rows.Close()

	var users []searchedUser
	for rows.Next() {
		var u searchedUser
		if err := rows.Scan(&u.ID, &u.Name); err == nil {
			users = append(users, u)
		}
	}
	if users == nil {
		users = []searchedUser{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (h *Handler) startGroupConversation(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	r.ParseForm()
	userIDs := r.Form["user_ids"]

	if len(userIDs) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"no users selected"}`))
		return
	}

	// For 1:1, reuse an existing DIRECT conversation if one exists. Uses the same
	// exactly-two-participants lookup as startConversation so a single message is
	// never routed into a group that happens to include the recipient.
	if len(userIDs) == 1 {
		existingID, err := h.findDirectConversation(r.Context(), userID, userIDs[0])

		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"id": existingID})
			return
		}
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// A 1:1 selection gets a canonical direct_key so the unique index dedupes it;
	// true groups (2+ others) leave it NULL and stay unconstrained.
	var directKeyVal interface{}
	if len(userIDs) == 1 && userIDs[0] != userID {
		directKeyVal = directKey(userID, userIDs[0])
	}

	// Group naming: a true group (more than two participants — i.e. more than one
	// selected other user) may carry an optional display name. 1:1 conversations
	// leave name NULL and fall back to the participant-name display.
	var nameVal interface{}
	if len(userIDs) > 1 {
		if name := strings.TrimSpace(r.FormValue("name")); name != "" {
			nameVal = name
		}
	}

	var convID string
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO conversations (direct_key, name) VALUES ($1, $2) RETURNING id`,
		directKeyVal, nameVal,
	).Scan(&convID)
	if err != nil {
		// Race on a concurrent direct-conversation create: recover for the 1:1 case.
		if directKeyVal != nil {
			if existing, lookupErr := h.findDirectConversation(r.Context(), userID, userIDs[0]); lookupErr == nil {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"id": existing})
				return
			}
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Add current user
	tx.ExecContext(r.Context(),
		`INSERT INTO conversation_participants (conversation_id, user_id) VALUES ($1, $2)`,
		convID, userID,
	)

	// Add selected users
	for _, uid := range userIDs {
		if uid != userID {
			tx.ExecContext(r.Context(),
				`INSERT INTO conversation_participants (conversation_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				convID, uid,
			)
		}
	}

	if err := tx.Commit(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": convID})
}

// addMember adds a user (form field `user_id`) as a participant to an existing
// conversation. Only an existing participant may add members (isParticipant
// gate). The insert uses ON CONFLICT DO NOTHING so re-adding an existing member
// is a no-op. This never alters direct_key, so the 1:1 dedupe guarantees are
// untouched (the unique index applies only to direct_key, which stays NULL for
// groups).
func (h *Handler) addMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	convID := r.PathValue("id")

	if !h.isParticipant(r.Context(), convID, userID) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"forbidden"}`))
		return
	}

	r.ParseForm()
	newUserID := strings.TrimSpace(r.FormValue("user_id"))
	if newUserID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"user_id required"}`))
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO conversation_participants (conversation_id, user_id)
		 VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		convID, newUserID,
	)
	if err != nil {
		slog.Error("failed to add conversation member", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

type msgAttachment struct {
	ID           string `json:"id"`
	OriginalName string `json:"original_name"`
	ContentType  string `json:"content_type"`
	Category     string `json:"category"`
}

type chatMsg struct {
	ID          string          `json:"id"`
	SenderID    string          `json:"sender_id"`
	Name        string          `json:"name"`
	Content     string          `json:"content"`
	IsOwn       bool            `json:"is_own"`
	CreatedAt   string          `json:"created_at"`
	ReadAt      string          `json:"read_at,omitempty"`
	Attachments []msgAttachment `json:"attachments,omitempty"`
}

// isRead decides whether an own message should be marked "Read". A message is
// Read when the other participant's last-read timestamp is at or after the
// message's send time (inclusive, so a same-second read still counts). The
// epoch sentinel (Year() <= 1970) means the other user never opened the
// conversation, so nothing is Read.
func isRead(isOwn bool, sentAt, othersLastRead time.Time) bool {
	if !isOwn {
		return false
	}
	// Year() > 1970 instead of IsZero() because COALESCE('1970-01-01') is not
	// Go's zero time.
	if othersLastRead.Year() <= 1970 {
		return false
	}
	return !sentAt.After(othersLastRead)
}

func (h *Handler) chatMessages(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	convID := r.PathValue("id")

	if !h.isParticipant(r.Context(), convID, userID) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"forbidden"}`))
		return
	}

	// Mark as read
	h.db.ExecContext(r.Context(),
		`INSERT INTO message_reads (conversation_id, user_id, last_read_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (conversation_id, user_id)
		 DO UPDATE SET last_read_at = NOW()`,
		convID, userID,
	)

	// Surface the group name (if any) for the open-chat header. The body stays a
	// plain message array — which the drawer's JS relies on — so the name rides
	// along as a response header that chat-drawer.js reads and renders. Direct
	// (unnamed) conversations leave this empty and keep the participant-name
	// header the drawer already shows.
	var convName sql.NullString
	h.db.QueryRowContext(r.Context(),
		`SELECT name FROM conversations WHERE id = $1`, convID,
	).Scan(&convName)
	if convName.Valid && strings.TrimSpace(convName.String) != "" {
		w.Header().Set("X-Conversation-Name", convName.String)
	}

	// Get other participants' last read time for read receipts
	var othersLastRead time.Time
	h.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(MAX(last_read_at), '1970-01-01')
		 FROM message_reads
		 WHERE conversation_id = $1 AND user_id != $2`,
		convID, userID,
	).Scan(&othersLastRead)

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT m.id, m.sender_id,
			CONCAT(u.first_name, ' ', u.last_name),
			m.content, m.created_at
		 FROM messages m
		 JOIN users u ON u.id = m.sender_id
		 WHERE m.conversation_id = $1
		 ORDER BY m.created_at ASC
		 LIMIT 50`,
		convID,
	)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]chatMsg{})
		return
	}
	defer rows.Close()

	var msgs []chatMsg
	for rows.Next() {
		var m chatMsg
		var t time.Time
		if err := rows.Scan(&m.ID, &m.SenderID, &m.Name, &m.Content, &t); err != nil {
			continue
		}
		m.IsOwn = m.SenderID == userID
		m.CreatedAt = t.Format("Jan 2, 3:04 PM")
		if isRead(m.IsOwn, t, othersLastRead) {
			m.ReadAt = "Read"
		}
		msgs = append(msgs, m)
	}
	// Load attachments for this conversation
	attRows, attErr := h.db.QueryContext(r.Context(),
		`SELECT id, message_id, original_name, content_type, file_category
		 FROM message_attachments
		 WHERE conversation_id = $1
		 ORDER BY created_at ASC`,
		convID,
	)
	if attErr == nil {
		defer attRows.Close()
		attMap := make(map[string][]msgAttachment)
		for attRows.Next() {
			var a msgAttachment
			var msgID string
			if err := attRows.Scan(&a.ID, &msgID, &a.OriginalName, &a.ContentType, &a.Category); err == nil {
				attMap[msgID] = append(attMap[msgID], a)
			}
		}
		for i := range msgs {
			if atts, ok := attMap[msgs[i].ID]; ok {
				msgs[i].Attachments = atts
			}
		}
	}

	if msgs == nil {
		msgs = []chatMsg{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs)
}

func (h *Handler) chatSend(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	convID := r.PathValue("id")

	if !h.isParticipant(r.Context(), convID, userID) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	r.ParseMultipartForm(100 << 20)
	content := strings.TrimSpace(r.FormValue("content"))
	file, header, fileErr := r.FormFile("attachment")

	// Require at least content or a file
	if content == "" && fileErr != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var validated *attachment.Validated
	if fileErr == nil {
		defer file.Close()
		var vErr error
		validated, vErr = attachment.ValidateFile(file, header, attachment.ContextMessage)
		if vErr != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": vErr.Error()})
			return
		}
	}

	// If no text but has attachment, use filename as content
	if content == "" && validated != nil {
		content = validated.OriginalName
	}

	var msgID string
	err := h.db.QueryRowContext(r.Context(),
		`INSERT INTO messages (conversation_id, sender_id, content)
		 VALUES ($1, $2, $3) RETURNING id`,
		convID, userID, content,
	).Scan(&msgID)
	if err != nil {
		slog.Error("failed to send message", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if validated != nil {
		_, err = h.db.ExecContext(r.Context(),
			`INSERT INTO message_attachments
			 (message_id, conversation_id, sender_id, file_data, original_name, content_type, file_category, file_size)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			msgID, convID, userID,
			validated.Data, validated.OriginalName, validated.ContentType,
			string(validated.Category), validated.FileSize,
		)
		if err != nil {
			slog.Error("failed to store message attachment", "error", err)
		}
	}

	h.db.ExecContext(r.Context(),
		`UPDATE conversations SET updated_at = NOW() WHERE id = $1`, convID,
	)

	// Notify other participant
	var recipientID string
	h.db.QueryRowContext(r.Context(),
		`SELECT user_id FROM conversation_participants WHERE conversation_id = $1 AND user_id != $2`,
		convID, userID,
	).Scan(&recipientID)
	if recipientID != "" {
		var senderName string
		h.db.QueryRowContext(r.Context(),
			`SELECT CONCAT(first_name, ' ', last_name) FROM users WHERE id = $1`, userID,
		).Scan(&senderName)
		h.notif.CreateNotification(context.Background(), recipientID, userID, "message", senderName+" sent you a message")
		h.broker.Notify(recipientID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func (h *Handler) serveAttachment(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	attID := r.PathValue("id")

	var data []byte
	var contentType, originalName, category, convID string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT file_data, content_type, original_name, file_category, conversation_id
		 FROM message_attachments WHERE id = $1`,
		attID,
	).Scan(&data, &contentType, &originalName, &category, &convID)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if !h.isParticipant(r.Context(), convID, userID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	attachment.ServeHeaders(w, contentType, originalName, attachment.Category(category))
	w.Write(data)
}

func (h *Handler) togglePinContact(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	contactID := r.PathValue("userID")

	var exists bool
	h.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM pinned_contacts WHERE user_id = $1 AND contact_id = $2)`,
		userID, contactID,
	).Scan(&exists)

	nowPinned := !exists
	if exists {
		h.db.ExecContext(r.Context(),
			`DELETE FROM pinned_contacts WHERE user_id = $1 AND contact_id = $2`,
			userID, contactID,
		)
	} else {
		h.db.ExecContext(r.Context(),
			`INSERT INTO pinned_contacts (user_id, contact_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			userID, contactID,
		)
	}

	// The chat drawer pins via fetch and expects JSON; fall back to a redirect
	// for non-JS form posts.
	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"pinned": nowPinned})
		return
	}

	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/messages"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

func (h *Handler) handleTyping(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	convID := r.PathValue("id")

	if !h.isParticipant(r.Context(), convID, userID) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Notify the other participant
	var recipientID string
	h.db.QueryRowContext(r.Context(),
		`SELECT user_id FROM conversation_participants WHERE conversation_id = $1 AND user_id != $2`,
		convID, userID,
	).Scan(&recipientID)

	if recipientID != "" {
		h.broker.NotifyTyping(recipientID)
	}

	w.WriteHeader(http.StatusNoContent)
}

type pinnedContact struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type recentConvo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	LastMessage string `json:"last_message"`
	Unread      int    `json:"unread"`
	IsGroup     bool   `json:"is_group"`
	// OtherUserID is the single other participant for a 1:1 conversation
	// (empty for group conversations). Used by the drawer's pin/unpin button.
	OtherUserID string `json:"other_user_id"`
	// Pinned reflects whether OtherUserID is currently a pinned contact.
	Pinned bool `json:"pinned"`
}

type chatDrawerData struct {
	Pinned []pinnedContact `json:"pinned"`
	Recent []recentConvo   `json:"recent"`
}

func (h *Handler) recentConversations(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	// Load pinned contacts
	var pinned []pinnedContact
	pinnedRows, err := h.db.QueryContext(r.Context(),
		`SELECT u.id, CONCAT(u.first_name, ' ', u.last_name)
		 FROM pinned_contacts pc
		 JOIN users u ON u.id = pc.contact_id
		 WHERE pc.user_id = $1
		 ORDER BY pc.pinned_at DESC`,
		userID,
	)
	if err == nil {
		defer pinnedRows.Close()
		for pinnedRows.Next() {
			var p pinnedContact
			if err := pinnedRows.Scan(&p.ID, &p.Name); err == nil {
				pinned = append(pinned, p)
			}
		}
	}

	pinnedSet := make(map[string]bool, len(pinned))
	for _, p := range pinned {
		pinnedSet[p.ID] = true
	}

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT c.id,
			COALESCE(NULLIF(c.name, ''),
			 (SELECT STRING_AGG(CONCAT(u.first_name, ' ', u.last_name), ', ')
			 FROM conversation_participants cp
			 JOIN users u ON u.id = cp.user_id
			 WHERE cp.conversation_id = c.id AND cp.user_id != $1)),
			COALESCE((SELECT content FROM messages WHERE conversation_id = c.id ORDER BY created_at DESC LIMIT 1), ''),
			(SELECT COUNT(*) FROM messages m WHERE m.conversation_id = c.id AND m.sender_id != $1
				AND m.created_at > COALESCE(
					(SELECT last_read_at FROM message_reads WHERE conversation_id = c.id AND user_id = $1),
					'epoch'
				)),
			(SELECT COUNT(*) FROM conversation_participants WHERE conversation_id = c.id) > 2,
			(SELECT cp.user_id
			 FROM conversation_participants cp
			 WHERE cp.conversation_id = c.id AND cp.user_id != $1
			 LIMIT 1) AS other_user_id
		 FROM conversations c
		 JOIN conversation_participants mp ON mp.conversation_id = c.id AND mp.user_id = $1
		 ORDER BY c.updated_at DESC
		 LIMIT 10`,
		userID,
	)
	if err != nil {
		slog.Error("failed to load recent conversations", "error", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]recentConvo{})
		return
	}
	defer rows.Close()

	var convos []recentConvo
	for rows.Next() {
		var c recentConvo
		var otherUserID sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &c.LastMessage, &c.Unread, &c.IsGroup, &otherUserID); err != nil {
			continue
		}
		if len(c.LastMessage) > 50 {
			c.LastMessage = c.LastMessage[:50] + "..."
		}
		// Only expose a pinnable contact for 1:1 conversations.
		if !c.IsGroup && otherUserID.Valid {
			c.OtherUserID = otherUserID.String
			c.Pinned = pinnedSet[c.OtherUserID]
		}
		convos = append(convos, c)
	}

	if convos == nil {
		convos = []recentConvo{}
	}
	if pinned == nil {
		pinned = []pinnedContact{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chatDrawerData{Pinned: pinned, Recent: convos})
}
