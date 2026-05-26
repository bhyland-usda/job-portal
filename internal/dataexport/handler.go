package dataexport

import (
	"context"
	"database/sql"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// IndexPage is the data passed to the data-export landing page.
type IndexPage struct {
	middleware.BaseData
}

// Profile is the current user's own users-row, excluding password_hash.
type Profile struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Headline  string    `json:"headline"`
	About     string    `json:"about"`
	AvatarURL string    `json:"avatar_url"`
	Location  string    `json:"location"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Post is one of the user's own feed posts.
type Post struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Comment is one of the user's own comments.
type Comment struct {
	ID        string    `json:"id"`
	PostID    string    `json:"post_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Connection is a connection the user requested or received.
type Connection struct {
	ID          string    `json:"id"`
	RequesterID string    `json:"requester_id"`
	AddresseeID string    `json:"addressee_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// KudosSent is a kudos the user gave to someone.
type KudosSent struct {
	ID         string    `json:"id"`
	ReceiverID string    `json:"receiver_id"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"created_at"`
}

// KudosReceived is a kudos the user received from someone.
type KudosReceived struct {
	ID        string    `json:"id"`
	SenderID  string    `json:"sender_id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// Accomplishment is one of the user's recorded accomplishments.
type Accomplishment struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	PeriodType  string    `json:"period_type"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	CreatedAt   time.Time `json:"created_at"`
}

// ExportDocument is the full JSON payload served on download.
type ExportDocument struct {
	ExportedAt      time.Time        `json:"exported_at"`
	Profile         *Profile         `json:"profile"`
	Posts           []Post           `json:"posts"`
	Comments        []Comment        `json:"comments"`
	Connections     []Connection     `json:"connections"`
	KudosSent       []KudosSent      `json:"kudos_sent"`
	KudosReceived   []KudosReceived  `json:"kudos_received"`
	Accomplishments []Accomplishment `json:"accomplishments"`
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /data-export", requireAuth(http.HandlerFunc(h.showIndex)))
	mux.Handle("GET /data-export/download", requireAuth(http.HandlerFunc(h.downloadData)))
}

func (h *Handler) showIndex(w http.ResponseWriter, r *http.Request) {
	data := IndexPage{BaseData: middleware.NewBaseData(r)}
	h.pages["data_export.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) downloadData(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	ctx := r.Context()

	doc := ExportDocument{
		ExportedAt:      time.Now().UTC(),
		Profile:         h.loadProfile(ctx, userID),
		Posts:           h.loadPosts(ctx, userID),
		Comments:        h.loadComments(ctx, userID),
		Connections:     h.loadConnections(ctx, userID),
		KudosSent:       h.loadKudosSent(ctx, userID),
		KudosReceived:   h.loadKudosReceived(ctx, userID),
		Accomplishments: h.loadAccomplishments(ctx, userID),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="my-data.json"`)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		slog.Error("failed to encode data export", "error", err)
	}
}

// loadProfile returns the user's own users row (no password_hash). Best-effort:
// returns nil on error.
func (h *Handler) loadProfile(ctx context.Context, userID string) *Profile {
	var p Profile
	err := h.db.QueryRowContext(ctx,
		`SELECT id, email, first_name, last_name, headline, about, avatar_url, location, created_at, updated_at
		 FROM users WHERE id = $1`,
		userID,
	).Scan(&p.ID, &p.Email, &p.FirstName, &p.LastName, &p.Headline, &p.About, &p.AvatarURL, &p.Location, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		slog.Error("data export: failed to load profile", "error", err)
		return nil
	}
	return &p
}

func (h *Handler) loadPosts(ctx context.Context, userID string) []Post {
	rows, err := h.db.QueryContext(ctx,
		`SELECT id, content, created_at FROM posts WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		slog.Error("data export: failed to load posts", "error", err)
		return nil
	}
	defer rows.Close()

	var out []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Content, &p.CreatedAt); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out
}

func (h *Handler) loadComments(ctx context.Context, userID string) []Comment {
	rows, err := h.db.QueryContext(ctx,
		`SELECT id, post_id, content, created_at FROM comments WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		slog.Error("data export: failed to load comments", "error", err)
		return nil
	}
	defer rows.Close()

	var out []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.PostID, &c.Content, &c.CreatedAt); err != nil {
			continue
		}
		out = append(out, c)
	}
	return out
}

func (h *Handler) loadConnections(ctx context.Context, userID string) []Connection {
	rows, err := h.db.QueryContext(ctx,
		`SELECT id, requester_id, addressee_id, status, created_at
		 FROM connections WHERE requester_id = $1 OR addressee_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		slog.Error("data export: failed to load connections", "error", err)
		return nil
	}
	defer rows.Close()

	var out []Connection
	for rows.Next() {
		var c Connection
		if err := rows.Scan(&c.ID, &c.RequesterID, &c.AddresseeID, &c.Status, &c.CreatedAt); err != nil {
			continue
		}
		out = append(out, c)
	}
	return out
}

func (h *Handler) loadKudosSent(ctx context.Context, userID string) []KudosSent {
	rows, err := h.db.QueryContext(ctx,
		`SELECT id, receiver_id, message, created_at FROM kudos WHERE sender_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		slog.Error("data export: failed to load kudos sent", "error", err)
		return nil
	}
	defer rows.Close()

	var out []KudosSent
	for rows.Next() {
		var k KudosSent
		if err := rows.Scan(&k.ID, &k.ReceiverID, &k.Message, &k.CreatedAt); err != nil {
			continue
		}
		out = append(out, k)
	}
	return out
}

func (h *Handler) loadKudosReceived(ctx context.Context, userID string) []KudosReceived {
	rows, err := h.db.QueryContext(ctx,
		`SELECT id, sender_id, message, created_at FROM kudos WHERE receiver_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		slog.Error("data export: failed to load kudos received", "error", err)
		return nil
	}
	defer rows.Close()

	var out []KudosReceived
	for rows.Next() {
		var k KudosReceived
		if err := rows.Scan(&k.ID, &k.SenderID, &k.Message, &k.CreatedAt); err != nil {
			continue
		}
		out = append(out, k)
	}
	return out
}

func (h *Handler) loadAccomplishments(ctx context.Context, userID string) []Accomplishment {
	rows, err := h.db.QueryContext(ctx,
		`SELECT id, title, description, period_type, period_start, period_end, created_at
		 FROM accomplishments WHERE user_id = $1 ORDER BY period_start DESC`,
		userID,
	)
	if err != nil {
		slog.Error("data export: failed to load accomplishments", "error", err)
		return nil
	}
	defer rows.Close()

	var out []Accomplishment
	for rows.Next() {
		var a Accomplishment
		if err := rows.Scan(&a.ID, &a.Title, &a.Description, &a.PeriodType, &a.PeriodStart, &a.PeriodEnd, &a.CreatedAt); err != nil {
			continue
		}
		out = append(out, a)
	}
	return out
}
