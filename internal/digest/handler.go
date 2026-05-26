// Package digest renders an on-screen "Weekly Digest" summarizing the current
// user's last 7 days of activity. There is no email/SMTP — this is a page only.
package digest

import (
	"context"
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// Connection is a connection that became accepted in the last 7 days.
type Connection struct {
	ID   string
	Name string
}

// PostEngagement is one of the user's posts plus the likes/comments it received.
type PostEngagement struct {
	ID        string
	Content   string
	Likes     int
	Comments  int
	CreatedAt time.Time
}

// Kudo is a kudos the user received in the last 7 days.
type Kudo struct {
	ID         string
	SenderName string
	Message    string
	CreatedAt  time.Time
}

// MatchingPosting is a posting created in the last 7 days matching a user skill.
type MatchingPosting struct {
	ID         string
	Title      string
	Department string
	Skill      string
	CreatedAt  time.Time
}

// DigestPage is the data rendered by templates/digest/index.html.
type DigestPage struct {
	middleware.BaseData

	Connections      []Connection
	PostEngagement   []PostEngagement
	Kudos            []Kudo
	NotificationsNew int
	Postings         []MatchingPosting

	// Aggregate counters for section headings.
	TotalLikes    int
	TotalComments int
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /digest", requireAuth(http.HandlerFunc(h.showDigest)))
}

func (h *Handler) showDigest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	data := DigestPage{
		BaseData: middleware.NewBaseData(r),
	}

	// 1. New accepted connections in the last 7 days.
	connections, err := h.recentConnections(ctx, userID)
	if err != nil {
		slog.Error("digest: failed to load connections", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	data.Connections = connections

	// 2. Engagement (likes + comments) on the user's posts in the last 7 days.
	engagement, likes, comments, err := h.recentEngagement(ctx, userID)
	if err != nil {
		slog.Error("digest: failed to load post engagement", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	data.PostEngagement = engagement
	data.TotalLikes = likes
	data.TotalComments = comments

	// 3. Kudos received in the last 7 days.
	kudos, err := h.recentKudos(ctx, userID)
	if err != nil {
		slog.Error("digest: failed to load kudos", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	data.Kudos = kudos

	// 4. Unread notifications count (created in the last 7 days).
	notifCount, err := h.recentUnreadNotifications(ctx, userID)
	if err != nil {
		slog.Error("digest: failed to count notifications", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	data.NotificationsNew = notifCount

	// 5. New postings matching the user's skills in the last 7 days.
	postings, err := h.matchingPostings(ctx, userID)
	if err != nil {
		slog.Error("digest: failed to load matching postings", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	data.Postings = postings

	h.pages["digest.html"].ExecuteTemplate(w, "base", data)
}

// recentConnections returns connections that became accepted in the last 7 days,
// resolving the other party's name. Filtered by userID + updated_at window.
func (h *Handler) recentConnections(ctx context.Context, userID string) ([]Connection, error) {
	rows, err := h.db.QueryContext(ctx,
		`SELECT u.id, u.first_name || ' ' || u.last_name AS name
		 FROM connections c
		 JOIN users u ON u.id = CASE
		 	WHEN c.requester_id = $1 THEN c.addressee_id
		 	ELSE c.requester_id
		 END
		 WHERE (c.requester_id = $1 OR c.addressee_id = $1)
		 	AND c.status = 'accepted'
		 	AND c.updated_at >= NOW() - INTERVAL '7 days'
		 ORDER BY c.updated_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Connection
	for rows.Next() {
		var c Connection
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// recentEngagement returns the user's posts that received likes or comments in
// the last 7 days, along with the total like/comment counts in that window.
func (h *Handler) recentEngagement(ctx context.Context, userID string) ([]PostEngagement, int, int, error) {
	rows, err := h.db.QueryContext(ctx,
		`SELECT p.id, p.content,
		        (SELECT COUNT(*) FROM post_likes pl
		           WHERE pl.post_id = p.id
		             AND pl.created_at >= NOW() - INTERVAL '7 days') AS likes,
		        (SELECT COUNT(*) FROM comments cm
		           WHERE cm.post_id = p.id
		             AND cm.created_at >= NOW() - INTERVAL '7 days') AS comments,
		        p.created_at
		 FROM posts p
		 WHERE p.user_id = $1
		   AND (
		     EXISTS (SELECT 1 FROM post_likes pl
		               WHERE pl.post_id = p.id
		                 AND pl.created_at >= NOW() - INTERVAL '7 days')
		     OR EXISTS (SELECT 1 FROM comments cm
		                  WHERE cm.post_id = p.id
		                    AND cm.created_at >= NOW() - INTERVAL '7 days')
		   )
		 ORDER BY p.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()

	var out []PostEngagement
	var totalLikes, totalComments int
	for rows.Next() {
		var e PostEngagement
		if err := rows.Scan(&e.ID, &e.Content, &e.Likes, &e.Comments, &e.CreatedAt); err != nil {
			return nil, 0, 0, err
		}
		totalLikes += e.Likes
		totalComments += e.Comments
		out = append(out, e)
	}
	return out, totalLikes, totalComments, rows.Err()
}

// recentKudos returns kudos the user received in the last 7 days.
func (h *Handler) recentKudos(ctx context.Context, userID string) ([]Kudo, error) {
	rows, err := h.db.QueryContext(ctx,
		`SELECT k.id, s.first_name || ' ' || s.last_name AS sender_name, k.message, k.created_at
		 FROM kudos k
		 JOIN users s ON s.id = k.sender_id
		 WHERE k.receiver_id = $1
		   AND k.created_at >= NOW() - INTERVAL '7 days'
		 ORDER BY k.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Kudo
	for rows.Next() {
		var k Kudo
		if err := rows.Scan(&k.ID, &k.SenderName, &k.Message, &k.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// recentUnreadNotifications counts unread notifications created in the last 7 days.
func (h *Handler) recentUnreadNotifications(ctx context.Context, userID string) (int, error) {
	var count int
	err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM notifications
		 WHERE user_id = $1
		   AND read = false
		   AND created_at >= NOW() - INTERVAL '7 days'`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// matchingPostings returns active postings created in the last 7 days whose
// required skills overlap with the user's skills.
func (h *Handler) matchingPostings(ctx context.Context, userID string) ([]MatchingPosting, error) {
	rows, err := h.db.QueryContext(ctx,
		`SELECT DISTINCT ON (p.id)
		        p.id, p.title, COALESCE(p.department, ''), ps.skill_name, p.created_at
		 FROM postings p
		 JOIN posting_skills ps ON ps.posting_id = p.id
		 JOIN skills us ON LOWER(us.name) = LOWER(ps.skill_name) AND us.user_id = $1
		 WHERE p.status = 'active'
		   AND p.created_at >= NOW() - INTERVAL '7 days'
		 ORDER BY p.id, p.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MatchingPosting
	for rows.Next() {
		var m MatchingPosting
		if err := rows.Scan(&m.ID, &m.Title, &m.Department, &m.Skill, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
