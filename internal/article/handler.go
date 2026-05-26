package article

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type Article struct {
	ID         string
	AuthorID   string
	AuthorName string
	Title      string
	Content    string
	Published  bool
	CreatedAt  time.Time
}

type ListPage struct {
	middleware.BaseData
	Articles []Article
}

type ViewPage struct {
	middleware.BaseData
	Article    Article
	IsAuthor   bool
	Bookmarked bool
}

type FormPage struct {
	middleware.BaseData
	Article *Article
	Error   string
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /articles", requireAuth(http.HandlerFunc(h.showArticles)))
	mux.Handle("GET /articles/write", requireAuth(http.HandlerFunc(h.showWrite)))
	mux.Handle("POST /articles/write", requireAuth(http.HandlerFunc(h.handleWrite)))
	mux.Handle("GET /articles/{id}", requireAuth(http.HandlerFunc(h.showArticle)))
	mux.Handle("GET /articles/{id}/edit", requireAuth(http.HandlerFunc(h.showEdit)))
	mux.Handle("POST /articles/{id}/edit", requireAuth(http.HandlerFunc(h.handleEdit)))
}

func (h *Handler) showArticles(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT a.id, a.author_id, CONCAT(u.first_name, ' ', u.last_name),
			a.title, a.content, a.created_at
		 FROM articles a
		 JOIN users u ON u.id = a.author_id
		 WHERE a.published = true
		 ORDER BY a.created_at DESC
		 LIMIT 30`,
	)
	if err != nil {
		slog.Error("failed to load articles", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.AuthorID, &a.AuthorName,
			&a.Title, &a.Content, &a.CreatedAt); err != nil {
			continue
		}
		articles = append(articles, a)
	}

	data := ListPage{BaseData: middleware.NewBaseData(r), Articles: articles}
	h.pages["articles.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) showArticle(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	articleID := r.PathValue("id")

	var a Article
	err := h.db.QueryRowContext(r.Context(),
		`SELECT a.id, a.author_id, CONCAT(u.first_name, ' ', u.last_name),
			a.title, a.content, a.published, a.created_at
		 FROM articles a
		 JOIN users u ON u.id = a.author_id
		 WHERE a.id = $1`, articleID,
	).Scan(&a.ID, &a.AuthorID, &a.AuthorName, &a.Title, &a.Content, &a.Published, &a.CreatedAt)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if !a.Published && a.AuthorID != userID {
		http.NotFound(w, r)
		return
	}

	var bookmarked bool
	h.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM bookmarks WHERE user_id = $1 AND target_type = 'article' AND target_id = $2)`,
		userID, articleID,
	).Scan(&bookmarked)

	data := ViewPage{BaseData: middleware.NewBaseData(r), Article: a, IsAuthor: a.AuthorID == userID, Bookmarked: bookmarked}
	h.pages["article_view.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) showWrite(w http.ResponseWriter, r *http.Request) {
	data := FormPage{BaseData: middleware.NewBaseData(r)}
	h.pages["article_form.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) handleWrite(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	r.ParseForm()
	title := strings.TrimSpace(r.FormValue("title"))
	content := strings.TrimSpace(r.FormValue("content"))

	if title == "" || content == "" {
		data := FormPage{BaseData: middleware.NewBaseData(r), Error: "Title and content are required"}
		h.pages["article_form.html"].ExecuteTemplate(w, "base", data)
		return
	}

	var id string
	err := h.db.QueryRowContext(r.Context(),
		`INSERT INTO articles (author_id, title, content) VALUES ($1, $2, $3) RETURNING id`,
		userID, title, content,
	).Scan(&id)
	if err != nil {
		slog.Error("failed to create article", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/articles/"+id, http.StatusSeeOther)
}

func (h *Handler) showEdit(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	articleID := r.PathValue("id")

	var a Article
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, title, content FROM articles WHERE id = $1 AND author_id = $2`,
		articleID, userID,
	).Scan(&a.ID, &a.Title, &a.Content)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	data := FormPage{BaseData: middleware.NewBaseData(r), Article: &a}
	h.pages["article_form.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) handleEdit(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	articleID := r.PathValue("id")
	r.ParseForm()
	title := strings.TrimSpace(r.FormValue("title"))
	content := strings.TrimSpace(r.FormValue("content"))

	if title == "" || content == "" {
		a := &Article{ID: articleID, Title: title, Content: content}
		data := FormPage{BaseData: middleware.NewBaseData(r), Article: a, Error: "Title and content are required"}
		h.pages["article_form.html"].ExecuteTemplate(w, "base", data)
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		`UPDATE articles SET title = $1, content = $2, updated_at = NOW() WHERE id = $3 AND author_id = $4`,
		title, content, articleID, userID,
	)
	if err != nil {
		slog.Error("failed to update article", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/articles/"+articleID, http.StatusSeeOther)
}
