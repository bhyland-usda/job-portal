package news

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type NewsArticle struct {
	ID         string
	AuthorID   string
	AuthorName string
	Title      string
	Content    string
	Published  bool
	CreatedAt  time.Time
}

type NewsPage struct {
	middleware.BaseData
	Articles []NewsArticle
}

type ArticlePage struct {
	middleware.BaseData
	Article  NewsArticle
	IsAuthor bool
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
	mux.Handle("GET /news", requireAuth(http.HandlerFunc(h.showNews)))
	mux.Handle("GET /news/create", requireAuth(requireAdmin(http.HandlerFunc(h.showCreate))))
	mux.Handle("POST /news/create", requireAuth(requireAdmin(http.HandlerFunc(h.handleCreate))))
	mux.Handle("GET /news/{id}", requireAuth(http.HandlerFunc(h.showArticle)))
	mux.Handle("POST /news/{id}/publish", requireAuth(requireAdmin(http.HandlerFunc(h.publishArticle))))
}

func (h *Handler) showNews(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT a.id, a.author_id, u.first_name || ' ' || u.last_name, a.title, a.content, a.created_at
		 FROM news_articles a
		 JOIN users u ON u.id = a.author_id
		 WHERE a.published = true
		 ORDER BY a.created_at DESC
		 LIMIT 20`,
	)
	if err != nil {
		slog.Error("failed to load news articles", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var articles []NewsArticle
	for rows.Next() {
		var a NewsArticle
		if err := rows.Scan(&a.ID, &a.AuthorID, &a.AuthorName, &a.Title, &a.Content, &a.CreatedAt); err != nil {
			slog.Error("failed to scan news article", "error", err)
			continue
		}
		articles = append(articles, a)
	}

	data := NewsPage{
		BaseData: middleware.NewBaseData(r),
		Articles: articles,
	}

	h.pages["news.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) showArticle(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	articleID := r.PathValue("id")

	var a NewsArticle
	err := h.db.QueryRowContext(r.Context(),
		`SELECT a.id, a.author_id, u.first_name || ' ' || u.last_name, a.title, a.content, a.published, a.created_at
		 FROM news_articles a
		 JOIN users u ON u.id = a.author_id
		 WHERE a.id = $1`,
		articleID,
	).Scan(&a.ID, &a.AuthorID, &a.AuthorName, &a.Title, &a.Content, &a.Published, &a.CreatedAt)
	if err != nil {
		slog.Error("failed to load article", "error", err)
		http.NotFound(w, r)
		return
	}

	if !a.Published && a.AuthorID != userID {
		http.NotFound(w, r)
		return
	}

	data := ArticlePage{
		BaseData: middleware.NewBaseData(r),
		Article:  a,
		IsAuthor: a.AuthorID == userID,
	}

	h.pages["news_view.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) showCreate(w http.ResponseWriter, r *http.Request) {
	data := CreatePage{
		BaseData: middleware.NewBaseData(r),
	}

	h.pages["news_create.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	content := strings.TrimSpace(r.FormValue("content"))

	if title == "" || content == "" {
		data := CreatePage{
			BaseData: middleware.NewBaseData(r),
			Error:    "Title and content are required.",
		}
		h.pages["news_create.html"].ExecuteTemplate(w, "base", data)
		return
	}

	var id string
	err := h.db.QueryRowContext(r.Context(),
		`INSERT INTO news_articles (author_id, title, content)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		userID, title, content,
	).Scan(&id)
	if err != nil {
		slog.Error("failed to create article", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/news/"+id, http.StatusSeeOther)
}

func (h *Handler) publishArticle(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	articleID := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(),
		`UPDATE news_articles SET published = NOT published WHERE id = $1 AND author_id = $2`,
		articleID, userID,
	)
	if err != nil {
		slog.Error("failed to toggle publish", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/news/"+articleID, http.StatusSeeOther)
}
