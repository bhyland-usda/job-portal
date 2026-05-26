package kudos

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/badge"
	"github.com/bhyland-usda/job-portal/internal/middleware"
	"github.com/bhyland-usda/job-portal/internal/notification"
)

type Kudo struct {
	ID           string
	SenderID     string
	SenderName   string
	ReceiverID   string
	ReceiverName string
	Message      string
	CreatedAt    time.Time
}

type KudosPage struct {
	middleware.BaseData
	Kudos []Kudo
	Tab   string
}

type SendPage struct {
	middleware.BaseData
	ReceiverID   string
	ReceiverName string
	Error        string
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
	notif *notification.Handler
}

func NewHandler(db *sql.DB, pages map[string]*template.Template, notif *notification.Handler) *Handler {
	return &Handler{db: db, pages: pages, notif: notif}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /kudos", requireAuth(http.HandlerFunc(h.showKudos)))
	mux.Handle("GET /kudos/send/{userID}", requireAuth(http.HandlerFunc(h.showSend)))
	mux.Handle("POST /kudos/send/{userID}", requireAuth(http.HandlerFunc(h.handleSend)))
}

func (h *Handler) showKudos(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "received"
	}

	var rows *sql.Rows
	var err error

	if tab == "sent" {
		rows, err = h.db.QueryContext(r.Context(),
			`SELECT k.id, k.sender_id,
				CONCAT(s.first_name, ' ', s.last_name),
				k.receiver_id,
				CONCAT(rv.first_name, ' ', rv.last_name),
				k.message, k.created_at
			 FROM kudos k
			 JOIN users s ON s.id = k.sender_id
			 JOIN users rv ON rv.id = k.receiver_id
			 WHERE k.sender_id = $1
			 ORDER BY k.created_at DESC`, userID,
		)
	} else {
		rows, err = h.db.QueryContext(r.Context(),
			`SELECT k.id, k.sender_id,
				CONCAT(s.first_name, ' ', s.last_name),
				k.receiver_id,
				CONCAT(rv.first_name, ' ', rv.last_name),
				k.message, k.created_at
			 FROM kudos k
			 JOIN users s ON s.id = k.sender_id
			 JOIN users rv ON rv.id = k.receiver_id
			 WHERE k.receiver_id = $1
			 ORDER BY k.created_at DESC`, userID,
		)
	}
	if err != nil {
		slog.Error("failed to load kudos", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var kudos []Kudo
	for rows.Next() {
		var k Kudo
		if err := rows.Scan(&k.ID, &k.SenderID, &k.SenderName,
			&k.ReceiverID, &k.ReceiverName, &k.Message, &k.CreatedAt); err != nil {
			slog.Error("failed to scan kudo", "error", err)
			continue
		}
		kudos = append(kudos, k)
	}
	if err := rows.Err(); err != nil {
		slog.Error("failed to iterate kudos", "error", err)
	}

	data := KudosPage{
		BaseData: middleware.NewBaseData(r),
		Kudos:    kudos,
		Tab:      tab,
	}

	if err := h.pages["kudos.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render kudos", "error", err)
	}
}

func (h *Handler) showSend(w http.ResponseWriter, r *http.Request) {
	receiverID := r.PathValue("userID")

	var firstName, lastName string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT first_name, last_name FROM users WHERE id = $1`, receiverID,
	).Scan(&firstName, &lastName)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	data := SendPage{
		BaseData:     middleware.NewBaseData(r),
		ReceiverID:   receiverID,
		ReceiverName: firstName + " " + lastName,
	}

	if err := h.pages["kudos_send.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render kudos send", "error", err)
	}
}

func (h *Handler) handleSend(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	receiverID := r.PathValue("userID")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	message := strings.TrimSpace(r.FormValue("message"))

	if message == "" {
		var firstName, lastName string
		_ = h.db.QueryRowContext(r.Context(),
			`SELECT first_name, last_name FROM users WHERE id = $1`, receiverID,
		).Scan(&firstName, &lastName)

		data := SendPage{
			BaseData:     middleware.NewBaseData(r),
			ReceiverID:   receiverID,
			ReceiverName: firstName + " " + lastName,
			Error:        "Message cannot be empty.",
		}
		h.pages["kudos_send.html"].ExecuteTemplate(w, "base", data)
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO kudos (sender_id, receiver_id, message)
		 VALUES ($1, $2, $3)`,
		userID, receiverID, message,
	)
	if err != nil {
		slog.Error("failed to send kudos", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Award badges to the RECIPIENT (e.g. Team Player after 5 kudos received).
	// Non-fatal: never block sending kudos.
	badge.CheckAndAward(r.Context(), h.db, receiverID)

	// Create notification
	var senderFirst, senderLast string
	_ = h.db.QueryRowContext(r.Context(),
		`SELECT first_name, last_name FROM users WHERE id = $1`, userID,
	).Scan(&senderFirst, &senderLast)

	notifMsg := senderFirst + " " + senderLast + " gave you kudos: " + message
	if len(notifMsg) > 200 {
		notifMsg = notifMsg[:197] + "..."
	}
	h.notif.CreateNotification(r.Context(), receiverID, userID, "kudos", notifMsg)

	http.Redirect(w, r, "/profile/"+receiverID, http.StatusSeeOther)
}
