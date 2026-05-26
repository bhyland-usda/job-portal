package spotlight

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// Spotlight is a single employee spotlight, joined to the featured user's
// profile fields for display.
type Spotlight struct {
	ID        string
	UserID    string
	UserName  string
	Headline  string
	AvatarURL string
	WeekOf    time.Time
	Reason    string
	CreatedAt time.Time
}

// UserOption is a selectable user for the admin create form.
type UserOption struct {
	ID   string
	Name string
}

type IndexPage struct {
	middleware.BaseData
	Current *Spotlight
	Past    []Spotlight
}

type CreatePage struct {
	middleware.BaseData
	Users []UserOption
	Error string
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth, requireAdmin func(http.Handler) http.Handler) {
	mux.Handle("GET /spotlight", requireAuth(http.HandlerFunc(h.showSpotlight)))
	mux.Handle("GET /admin/spotlight", requireAuth(requireAdmin(http.HandlerFunc(h.showCreate))))
	mux.Handle("POST /admin/spotlight", requireAuth(requireAdmin(http.HandlerFunc(h.handleCreate))))
}

func (h *Handler) showSpotlight(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT s.id, s.user_id, u.first_name || ' ' || u.last_name, u.headline, u.avatar_url, s.week_of, s.reason, s.created_at
		 FROM employee_spotlights s
		 JOIN users u ON u.id = s.user_id
		 ORDER BY s.week_of DESC
		 LIMIT 6`,
	)
	if err != nil {
		slog.Error("failed to load spotlights", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var all []Spotlight
	for rows.Next() {
		var s Spotlight
		var headline, avatar, reason sql.NullString
		if err := rows.Scan(&s.ID, &s.UserID, &s.UserName, &headline, &avatar, &s.WeekOf, &reason, &s.CreatedAt); err != nil {
			slog.Error("failed to scan spotlight", "error", err)
			continue
		}
		s.Headline = headline.String
		s.Reason = reason.String
		if avatar.Valid && avatar.String != "" {
			s.AvatarURL = "/avatar/" + s.UserID
		}
		all = append(all, s)
	}

	data := IndexPage{
		BaseData: middleware.NewBaseData(r),
	}
	if len(all) > 0 {
		data.Current = &all[0]
		data.Past = all[1:]
	}

	h.pages["spotlight.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) showCreate(w http.ResponseWriter, r *http.Request) {
	users, err := h.loadUserOptions(r)
	if err != nil {
		slog.Error("failed to load users for spotlight form", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := CreatePage{
		BaseData: middleware.NewBaseData(r),
		Users:    users,
	}

	h.pages["spotlight_create.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) loadUserOptions(r *http.Request) ([]UserOption, error) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, first_name, last_name
		 FROM users
		 ORDER BY first_name, last_name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserOption
	for rows.Next() {
		var id, first, last string
		if err := rows.Scan(&id, &first, &last); err != nil {
			return nil, err
		}
		users = append(users, UserOption{ID: id, Name: strings.TrimSpace(first + " " + last)})
	}
	return users, rows.Err()
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	adminID := middleware.GetUserID(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	userID := strings.TrimSpace(r.FormValue("user_id"))
	weekOf := strings.TrimSpace(r.FormValue("week_of"))
	reason := strings.TrimSpace(r.FormValue("reason"))

	if userID == "" || weekOf == "" {
		h.renderCreateError(w, r, "Please select an employee and a week.")
		return
	}

	// ON CONFLICT (week_of) DO UPDATE keeps a single spotlight per week and
	// avoids a hard error on the UNIQUE(week_of) constraint.
	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO employee_spotlights (user_id, week_of, reason, created_by)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (week_of) DO UPDATE
		   SET user_id = EXCLUDED.user_id,
		       reason = EXCLUDED.reason,
		       created_by = EXCLUDED.created_by`,
		userID, weekOf, reason, adminID,
	)
	if err != nil {
		slog.Error("failed to create spotlight", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/spotlight", http.StatusSeeOther)
}

func (h *Handler) renderCreateError(w http.ResponseWriter, r *http.Request, msg string) {
	users, err := h.loadUserOptions(r)
	if err != nil {
		slog.Error("failed to load users for spotlight form", "error", err)
		users = nil
	}
	data := CreatePage{
		BaseData: middleware.NewBaseData(r),
		Users:    users,
		Error:    msg,
	}
	h.pages["spotlight_create.html"].ExecuteTemplate(w, "base", data)
}
