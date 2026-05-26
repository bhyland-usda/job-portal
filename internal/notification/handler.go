package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type Handler struct {
	db     *sql.DB
	pages  map[string]*template.Template
	broker *Broker
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages, broker: NewBroker()}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("POST /notifications/{id}/read", requireAuth(http.HandlerFunc(h.markRead)))
	mux.Handle("POST /notifications/read-all", requireAuth(http.HandlerFunc(h.markAllRead)))
	mux.Handle("GET /notifications/count", requireAuth(http.HandlerFunc(h.getUnreadCount)))
	mux.Handle("GET /notifications/stream", requireAuth(http.HandlerFunc(h.streamNotifications)))
	mux.Handle("GET /notifications/{id}/click", requireAuth(http.HandlerFunc(h.handleClick)))
	mux.Handle("GET /notifications/recent", requireAuth(http.HandlerFunc(h.getRecent)))
	mux.Handle("POST /notifications/{id}/delete", requireAuth(http.HandlerFunc(h.deleteNotification)))
	mux.Handle("GET /notifications/preferences", requireAuth(http.HandlerFunc(h.preferences)))
	mux.Handle("POST /notifications/preferences", requireAuth(http.HandlerFunc(h.savePreferences)))
}

// notifPrefTypes is the canonical set of notification type strings used in
// CreateNotification calls across the app. Each gets its own preference toggle.
var notifPrefTypes = []struct {
	Type  string
	Label string
}{
	{"connection_request", "Connection requests"},
	{"connection_accepted", "Connection request accepted"},
	{"message", "Direct messages"},
	{"mention", "Mentions in posts"},
	{"feed", "Likes & comments on your posts"},
	{"kudos", "Kudos received"},
	{"mentorship", "Mentorship updates"},
	{"feedback", "Feedback requests & responses"},
}

type prefRow struct {
	Type    string
	Label   string
	Enabled bool
}

type preferencesPage struct {
	middleware.BaseData
	Prefs []prefRow
	Saved bool
}

func (h *Handler) preferences(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	// Load any explicit preferences; absence means the default (enabled).
	stored := make(map[string]bool)
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT type, enabled FROM notification_preferences WHERE user_id = $1`,
		userID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			var enabled bool
			if err := rows.Scan(&t, &enabled); err != nil {
				continue
			}
			stored[t] = enabled
		}
	}

	prefs := make([]prefRow, 0, len(notifPrefTypes))
	for _, nt := range notifPrefTypes {
		enabled := true // default ON when no row
		if v, ok := stored[nt.Type]; ok {
			enabled = v
		}
		prefs = append(prefs, prefRow{Type: nt.Type, Label: nt.Label, Enabled: enabled})
	}

	data := preferencesPage{
		BaseData: middleware.NewBaseData(r),
		Prefs:    prefs,
		Saved:    r.URL.Query().Get("saved") == "1",
	}

	if err := h.pages["notification_preferences.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render notification preferences", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) savePreferences(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	for _, nt := range notifPrefTypes {
		// Unchecked checkboxes are absent from the form -> disabled.
		enabled := r.FormValue(nt.Type) != ""
		_, err := h.db.ExecContext(r.Context(),
			`INSERT INTO notification_preferences (user_id, type, enabled)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (user_id, type) DO UPDATE SET enabled = EXCLUDED.enabled`,
			userID, nt.Type, enabled,
		)
		if err != nil {
			slog.Error("failed to save notification preference", "error", err, "type", nt.Type)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/notifications/preferences?saved=1", http.StatusSeeOther)
}

func (h *Handler) streamNotifications(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	respCont := http.NewResponseController(w)
	respCont.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.broker.Subscribe(userID)
	defer h.broker.Unsubscribe(userID, ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			var count int
			err := h.db.QueryRowContext(r.Context(),
				`SELECT COUNT(*) FROM notifications
					 WHERE user_id = $1 AND read = false`,
				userID,
			).Scan(&count)
			if err != nil {
				return
			}

			fmt.Fprintf(w, "data: {\"count\":%d}\n\n", count)
			flusher.Flush()
		}
	}
}

func (h *Handler) handleClick(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	notifID := r.PathValue("id")

	var senderID, notifType string
	err := h.db.QueryRowContext(r.Context(),
		`UPDATE notifications SET read = true
		 WHERE id = $1 AND user_id = $2
		 RETURNING sender_id, type`,
		notifID, userID,
	).Scan(&senderID, &notifType)

	if err != nil {
		http.Redirect(w, r, "/feed", http.StatusSeeOther)
		return
	}

	switch notifType {
	case "connection_request":
		http.Redirect(w, r, "/connections", http.StatusSeeOther)
	case "connection_accept", "connection_accepted":
		// The accepter is stored as sender_id; route to their profile.
		http.Redirect(w, r, "/profile/"+senderID, http.StatusSeeOther)
	case "feed":
		// Find the post the sender interacted with
		var postID string
		h.db.QueryRowContext(r.Context(),
			`SELECT p.id FROM posts p
				 WHERE p.user_id = $1
				   AND (EXISTS(SELECT 1 FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = $2)
				     OR EXISTS(SELECT 1 FROM comments c WHERE c.post_id = p.id AND c.user_id = $2))
				 ORDER BY p.created_at DESC LIMIT 1`,
			userID, senderID,
		).Scan(&postID)
		if postID != "" {
			http.Redirect(w, r, "/feed?highlight="+postID, http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/feed", http.StatusSeeOther)
		}
	case "kudos":
		http.Redirect(w, r, "/kudos", http.StatusSeeOther)
	case "message":
		var convID string
		h.db.QueryRowContext(r.Context(),
			`SELECT cp1.conversation_id
				 FROM conversation_participants cp1
				 JOIN conversation_participants cp2 ON cp1.conversation_id = cp2.conversation_id
				 WHERE cp1.user_id = $1 AND cp2.user_id = $2
				 LIMIT 1`,
			userID, senderID,
		).Scan(&convID)
		if convID != "" {
			http.Redirect(w, r, "/feed?open_chat="+convID, http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/feed", http.StatusSeeOther)
		}
	default:
		http.Redirect(w, r, "/feed", http.StatusSeeOther)
	}
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	notifID := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(),
		`UPDATE notifications SET read = true
		 WHERE id = $1 AND user_id = $2`,
		notifID, userID,
	)
	if err != nil {
		slog.Error("failed to mark notification read", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
		return
	}
	http.Redirect(w, r, "/feed", http.StatusSeeOther)
}

func (h *Handler) markAllRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	_, err := h.db.ExecContext(r.Context(),
		`UPDATE notifications SET read = true
		 WHERE user_id = $1 AND read = false`,
		userID,
	)
	if err != nil {
		slog.Error("failed to mark all notifications read", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
		return
	}
	http.Redirect(w, r, "/feed", http.StatusSeeOther)
}

func (h *Handler) getUnreadCount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var count int
	err := h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM notifications
		 WHERE user_id = $1 AND read = false`,
		userID,
	).Scan(&count)
	if err != nil {
		slog.Error("failed to get unread count", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"count": count})
}

func (h *Handler) deleteNotification(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	notifID := r.PathValue("id")

	h.db.ExecContext(r.Context(),
		`DELETE FROM notifications WHERE id = $1 AND user_id = $2`,
		notifID, userID,
	)

	// Return JSON for dropdown, redirect for full page
	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
		return
	}
	http.Redirect(w, r, "/feed", http.StatusSeeOther)
}

type recentNotif struct {
	ID              string `json:"id"`
	SenderInitials  string `json:"sender_initials"`
	SenderAvatarURL string `json:"sender_avatar"`
	Message         string `json:"message"`
	Read            bool   `json:"read"`
	ClickURL        string `json:"click_url"`
	TimeAgo         string `json:"time_ago"`
}

func (h *Handler) getRecent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT n.id, u.first_name, u.last_name, u.avatar_url,
			n.message, n.read, n.created_at
		 FROM notifications n
		 JOIN users u ON u.id = n.sender_id
		 WHERE n.user_id = $1
		 ORDER BY n.created_at DESC
		 LIMIT 15`, userID,
	)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]recentNotif{})
		return
	}
	defer rows.Close()

	var notifs []recentNotif
	now := time.Now()
	for rows.Next() {
		var n recentNotif
		var firstName, lastName string
		var avatarURL sql.NullString
		var createdAt time.Time
		if err := rows.Scan(&n.ID, &firstName, &lastName, &avatarURL,
			&n.Message, &n.Read, &createdAt); err != nil {
			continue
		}
		if len(firstName) > 0 && len(lastName) > 0 {
			n.SenderInitials = string(firstName[0]) + string(lastName[0])
		}
		if avatarURL.Valid && avatarURL.String != "" {
			n.SenderAvatarURL = "/avatar/" + avatarURL.String
		}
		n.ClickURL = "/notifications/" + n.ID + "/click"

		diff := now.Sub(createdAt)
		switch {
		case diff < time.Minute:
			n.TimeAgo = "just now"
		case diff < time.Hour:
			n.TimeAgo = fmt.Sprintf("%dm ago", int(diff.Minutes()))
		case diff < 24*time.Hour:
			n.TimeAgo = fmt.Sprintf("%dh ago", int(diff.Hours()))
		default:
			n.TimeAgo = createdAt.Format("Jan 2")
		}

		notifs = append(notifs, n)
	}
	if notifs == nil {
		notifs = []recentNotif{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notifs)
}

func (h *Handler) CreateNotification(ctx context.Context, userID, senderID, notifType, message string) error {
	// Respect the recipient's notification preferences. A row with
	// enabled=false suppresses this type; a missing row defaults to enabled.
	var enabled bool
	err := h.db.QueryRowContext(ctx,
		`SELECT enabled FROM notification_preferences
		 WHERE user_id = $1 AND type = $2`,
		userID, notifType,
	).Scan(&enabled)
	switch {
	case err == sql.ErrNoRows:
		// No preference set: default to enabled.
	case err != nil:
		return err
	case !enabled:
		// Recipient disabled this notification type: skip silently.
		return nil
	}

	_, err = h.db.ExecContext(ctx,
		`INSERT INTO notifications (user_id, sender_id, type, message)
		 VALUES ($1, $2, $3, $4)`,
		userID, senderID, notifType, message,
	)

	if err != nil {
		return err
	}

	h.broker.Notify(userID)
	return nil
}
