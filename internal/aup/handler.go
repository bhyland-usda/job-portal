package aup

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// IndexPage is the data passed to the AUP template.
type IndexPage struct {
	middleware.BaseData
	Accepted bool
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /aup", requireAuth(http.HandlerFunc(h.showPolicy)))
	mux.Handle("POST /aup/accept", requireAuth(http.HandlerFunc(h.handleAccept)))
}

func (h *Handler) showPolicy(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var acceptedAt sql.NullTime
	if userID != "" {
		err := h.db.QueryRowContext(r.Context(),
			`SELECT aup_accepted_at FROM users WHERE id = $1`,
			userID,
		).Scan(&acceptedAt)
		if err != nil && err != sql.ErrNoRows {
			slog.Error("failed to load aup status", "error", err)
		}
	}

	data := IndexPage{
		BaseData: middleware.NewBaseData(r),
		Accepted: acceptedAt.Valid,
	}

	h.pages["aup.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) handleAccept(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		`UPDATE users SET aup_accepted_at = NOW() WHERE id = $1`,
		userID,
	)
	if err != nil {
		slog.Error("failed to record aup acceptance", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/feed", http.StatusSeeOther)
}
