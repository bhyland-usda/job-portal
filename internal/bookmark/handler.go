package bookmark

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type Bookmark struct {
	ID         string
	UserID     string
	TargetType string
	TargetID   string
	CreatedAt  time.Time
	Title      string
}

type BookmarkPage struct {
	middleware.BaseData
	Bookmarks []Bookmark
	Tab       string
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /bookmarks", requireAuth(http.HandlerFunc(h.showBookmarks)))
	mux.Handle("POST /bookmarks/add", requireAuth(http.HandlerFunc(h.handleAdd)))
	mux.Handle("POST /bookmarks/{id}/delete", requireAuth(http.HandlerFunc(h.handleDelete)))
}

func (h *Handler) showBookmarks(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "all"
	}

	query := `SELECT id, target_type, target_id, created_at FROM bookmarks WHERE user_id = $1`
	args := []interface{}{userID}

	if tab != "all" {
		query += ` AND target_type = $2`
		args = append(args, tab)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		slog.Error("failed to load bookmarks", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var bookmarks []Bookmark
	for rows.Next() {
		var b Bookmark
		if err := rows.Scan(&b.ID, &b.TargetType, &b.TargetID, &b.CreatedAt); err != nil {
			slog.Error("failed to scan bookmark", "error", err)
			continue
		}
		b.UserID = userID
		bookmarks = append(bookmarks, b)
	}
	if err := rows.Err(); err != nil {
		slog.Error("failed to iterate bookmarks", "error", err)
	}

	// Load titles for each bookmark
	for i := range bookmarks {
		bookmarks[i].Title = h.loadTitle(r, bookmarks[i].TargetType, bookmarks[i].TargetID)
	}

	data := BookmarkPage{
		BaseData:  middleware.NewBaseData(r),
		Bookmarks: bookmarks,
		Tab:       tab,
	}

	if err := h.pages["bookmarks.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render bookmarks", "error", err)
	}
}

func (h *Handler) loadTitle(r *http.Request, targetType, targetID string) string {
	var title string
	var err error

	switch targetType {
	case "post":
		var content string
		err = h.db.QueryRowContext(r.Context(),
			`SELECT content FROM posts WHERE id = $1`, targetID,
		).Scan(&content)
		if err == nil {
			content = strings.TrimSpace(content)
			if len(content) > 50 {
				title = content[:50] + "..."
			} else {
				title = content
			}
		}
	case "posting":
		err = h.db.QueryRowContext(r.Context(),
			`SELECT title FROM postings WHERE id = $1`, targetID,
		).Scan(&title)
	case "article":
		err = h.db.QueryRowContext(r.Context(),
			`SELECT title FROM articles WHERE id = $1`, targetID,
		).Scan(&title)
	case "profile":
		err = h.db.QueryRowContext(r.Context(),
			`SELECT CONCAT(first_name, ' ', last_name) FROM users WHERE id = $1`, targetID,
		).Scan(&title)
	}

	if err != nil {
		title = "(deleted)"
	}
	return title
}

func (h *Handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	targetType := strings.TrimSpace(r.FormValue("target_type"))
	targetID := strings.TrimSpace(r.FormValue("target_id"))

	if targetType == "" || targetID == "" {
		http.Error(w, "Missing target", http.StatusBadRequest)
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO bookmarks (user_id, target_type, target_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING`,
		userID, targetType, targetID,
	)
	if err != nil {
		slog.Error("failed to add bookmark", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/bookmarks"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	bookmarkID := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(),
		`DELETE FROM bookmarks WHERE id = $1 AND user_id = $2`,
		bookmarkID, userID,
	)
	if err != nil {
		slog.Error("failed to delete bookmark", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/bookmarks", http.StatusSeeOther)
}
