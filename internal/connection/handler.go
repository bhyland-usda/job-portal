package connection

import(
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler { db: db, pages: pages }
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /connections", requireAuth(http.HandlerFunc(h.showConnections)))
	mux.Handle("POST /connections/request/{id}", requireAuth(http.HandlerFunc(h.sendRequest)))
	mux.Handle("POST /connections/accept/{id}", requireAuth(http.HandlerFunc(h.acceptRequest)))
	mux.Handle("POST /connections/reject/{id}", requireAuth(http.HandlerFunc(h.rejectRequest)))
}

type ConnectionUser struct {
	ID        string
	FirstName string
	LastName  string
	Headline  string
	AvatarURL string
}

type ConnectionsPage struct {
	UserID   string
	Pending  []ConnectionUser
	Accepted []ConnectionUser
}

func(h *Handler) showConnections(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	pending, err := h.getPending(r, userID)
	if err != nil {
		slog.Error("failed to load pending connections", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	accepted, err := h.getAccepted(r, userID)
	if err != nil {
		slog.Error("failed to load connections", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := ConnectionsPage {
		UserID:   userID,
		Pending:  pending,
		Accepted: accepted,
	}

	if err := h.pages["connections.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render connections", "error", err)
	}
}

func (h *Handler) GetConnectionStatus(userID, otherID string) (string, error) {
	if userID == otherID {
		return "self", nil
	}

	var status string
	var requesterID string

	err := h.db.QueryRow(
		`SELECT status, requester_id FROM connections
		 WHERE (requester_id = $1 AND addressee_id = $2)
			OR (requester_id = $2 AND addressee_id = $1)`,
		userID, otherID).Scan(&status, &requesterID)

	if err == sql.ErrNoRows {
		return "none", nil
	}
	if err != nil {
		return "", err
	}

	if status == "accepted" {
		return "connected", nil
	}

	if requesterID == userID {
		return "pending_sent", nil
	}

	return "pending_received", nil
}

func (h *Handler) getPending(r *http.Request, userID string) ([]ConnectionUser, error) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT u.id, u.first_name, u.last_name, u.headline, u.avatar_url
		 FROM connections c
		 JOIN users u on u.id = c.requester_id
		 WHERE c.addressee_id = $1 AND c.status = 'pending'
		 ORDER BY c.created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []ConnectionUser
	for rows.Next() {
		var user ConnectionUser
		if err := rows.Scan(&user.ID, &user.FirstName, &user.LastName, &user.Headline, &user.AvatarURL); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

func (h *Handler) getAccepted(r *http.Request, userID string) ([]ConnectionUser, error) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT u.id, u.first_name, u.last_name, u.headline, u.avatar_url
		 FROM connections c
		 JOIN users u on u.id = CASE
		 	WHEN c.requester_id = $1 THEN c.addressee_id
		 	ELSE c.requester_id
		 END
		 WHERE (c.requester_id = $1 OR c.addressee_id = $1)
		 	AND c.status = 'accepted'
		 ORDER BY c.updated_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []ConnectionUser
	for rows.Next() {
		var user ConnectionUser
		if err := rows.Scan(&user.ID, &user.FirstName, &user.LastName, &user.Headline, &user.AvatarURL); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

func (h *Handler) sendRequest(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	targetID := r.PathValue("id")

	if userID == targetID {
		http.Error(w, "Cannot connect with yourself", http.StatusBadRequest)
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO connections (requester_id, addressee_id, status)
		 VALUES ($1, $2, 'pending')
		 ON CONFLICT (requester_id, addressee_id) DO NOTHING`,
		 userID, targetID,
	)
	if err != nil {
		slog.Error("failed to send connection request", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile/" + targetID, http.StatusSeeOther)
}

func (h *Handler) acceptRequest(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	requesterID := r.PathValue("id")

	result, err := h.db.ExecContext(r.Context(),
		`UPDATE connections SET status = 'accepted', updated_at = NOW()
		 WHERE requester_id = $1 AND addressee_id = $2 AND status = 'pending'`,
		 requesterID, userID,
	)
	if err != nil {
		slog.Error("failed to accept connection", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "No pending requests found", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/connections", http.StatusSeeOther)
}

func (h *Handler) rejectRequest(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	requesterID := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(),
		`UPDATE connections SET status = 'rejected', updated_at = NOW()
		 WHERE requester_id = $1 AND addressee_id = $2 AND status = 'pending'`,
		 requesterID, userID,
	)
	if err != nil {
		slog.Error("failed to reject connection", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/connections", http.StatusSeeOther)
}
