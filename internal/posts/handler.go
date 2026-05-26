package posts

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type SocialPost struct {
	ID           string
	Content      string
	LikeCount    int
	CommentCount int
	ViewCount    int
	CreatedAt    time.Time
}

type Posting struct {
	ID         string
	Title      string
	Type       string
	Status     string
	MatchCount int
	CreatedAt  time.Time
}

type MyPostsPage struct {
	middleware.BaseData
	Tab      string
	Posts    []SocialPost
	Postings []Posting
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /my-posts", requireAuth(http.HandlerFunc(h.showMyPosts)))
}

func (h *Handler) showMyPosts(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tab := r.URL.Query().Get("tab")
	if tab != "postings" {
		tab = "social"
	}

	var posts []SocialPost
	var postings []Posting

	if tab == "social" {
		rows, err := h.db.QueryContext(r.Context(),
			`SELECT p.id, p.content,
				(SELECT COUNT(*) FROM post_likes WHERE post_id = p.id),
				(SELECT COUNT(*) FROM comments WHERE post_id = p.id),
				(SELECT COUNT(*) FROM post_views WHERE post_id = p.id),
				p.created_at
			 FROM posts p
			 WHERE p.user_id = $1
			 ORDER BY p.created_at DESC`,
			userID,
		)
		if err != nil {
			slog.Error("failed to load my posts", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var sp SocialPost
			if err := rows.Scan(&sp.ID, &sp.Content, &sp.LikeCount,
				&sp.CommentCount, &sp.ViewCount, &sp.CreatedAt); err != nil {
				continue
			}
			posts = append(posts, sp)
		}
	} else {
		rows, err := h.db.QueryContext(r.Context(),
			`SELECT p.id, p.title, p.type, p.status,
				(SELECT COUNT(DISTINCT s.user_id)
				 FROM skills s
				 JOIN posting_skills ps on LOWER(ps.skill_name) = LOWER(s.name)
				 WHERE ps.posting_id = p.id),
				 p.created_at
			 FROM postings p
			 WHERE p.author_id = $1
			 ORDER BY p.created_at DESC`,
			userID,
		)
		if err != nil {
			slog.Error("failed to load my postings", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var p Posting
			if err := rows.Scan(&p.ID, &p.Title, &p.Type, &p.Status,
				&p.MatchCount, &p.CreatedAt); err != nil {
				continue
			}
			postings = append(postings, p)
		}
	}

	data := MyPostsPage{
		BaseData: middleware.NewBaseData(r),
		Tab:      tab,
		Posts:    posts,
		Postings: postings,
	}

	h.pages["my_posts.html"].ExecuteTemplate(w, "base", data)
}
