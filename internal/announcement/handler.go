package announcement

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// Announcement is a single org-wide announcement, joined to the author's name
// for display.
type Announcement struct {
	ID         string
	Title      string
	Body       string
	AuthorName string
	IsPinned   bool
	CreatedAt  time.Time
}

type IndexPage struct {
	middleware.BaseData
	Announcements []Announcement
}

type CreatePage struct {
	middleware.BaseData
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
	mux.Handle("GET /announcements", requireAuth(http.HandlerFunc(h.showAnnouncements)))
	mux.Handle("GET /admin/announcements/new", requireAuth(requireAdmin(http.HandlerFunc(h.showCreate))))
	mux.Handle("POST /admin/announcements", requireAuth(requireAdmin(http.HandlerFunc(h.handleCreate))))
	mux.Handle("POST /admin/announcements/{id}/pin", requireAuth(requireAdmin(http.HandlerFunc(h.togglePin))))
	mux.Handle("POST /admin/announcements/{id}/delete", requireAuth(requireAdmin(http.HandlerFunc(h.deleteAnnouncement))))
}

func (h *Handler) showAnnouncements(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT a.id, a.title, a.body,
		        COALESCE(u.first_name || ' ' || u.last_name, 'USDA'),
		        a.is_pinned, a.created_at
		 FROM announcements a
		 LEFT JOIN users u ON u.id = a.author_id
		 ORDER BY a.is_pinned DESC, a.created_at DESC
		 LIMIT 100`,
	)
	if err != nil {
		slog.Error("failed to load announcements", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var announcements []Announcement
	for rows.Next() {
		var a Announcement
		if err := rows.Scan(&a.ID, &a.Title, &a.Body, &a.AuthorName, &a.IsPinned, &a.CreatedAt); err != nil {
			slog.Error("failed to scan announcement", "error", err)
			continue
		}
		announcements = append(announcements, a)
	}

	data := IndexPage{
		BaseData:      middleware.NewBaseData(r),
		Announcements: announcements,
	}

	h.pages["announcements.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) showCreate(w http.ResponseWriter, r *http.Request) {
	data := CreatePage{
		BaseData: middleware.NewBaseData(r),
	}

	h.pages["announcement_create.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	body := strings.TrimSpace(r.FormValue("body"))
	isPinned := r.FormValue("pinned") != ""

	if title == "" || body == "" {
		data := CreatePage{
			BaseData: middleware.NewBaseData(r),
			Error:    "Title and body are required.",
		}
		h.pages["announcement_create.html"].ExecuteTemplate(w, "base", data)
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO announcements (title, body, author_id, is_pinned)
		 VALUES ($1, $2, $3, $4)`,
		title, body, userID, isPinned,
	)
	if err != nil {
		slog.Error("failed to create announcement", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/announcements", http.StatusSeeOther)
}

func (h *Handler) togglePin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(),
		`UPDATE announcements SET is_pinned = NOT is_pinned WHERE id = $1`,
		id,
	)
	if err != nil {
		slog.Error("failed to toggle announcement pin", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/announcements", http.StatusSeeOther)
}

func (h *Handler) deleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(),
		`DELETE FROM announcements WHERE id = $1`,
		id,
	)
	if err != nil {
		slog.Error("failed to delete announcement", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/announcements", http.StatusSeeOther)
}
