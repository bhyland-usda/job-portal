package group

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type Group struct {
	ID          string
	Name        string
	Description string
	CreatedBy   string
	MemberCount int
	CreatedAt   time.Time
	IsMember    bool
}

type GroupPost struct {
	ID         string
	UserID     string
	AuthorName string
	Content    string
	CreatedAt  time.Time
}

type GroupPage struct {
	middleware.BaseData
	Group        Group
	Posts        []GroupPost
	IsMember     bool
	IsGroupAdmin bool
}

type ListPage struct {
	middleware.BaseData
	Groups []Group
}

type CreateGroupPage struct {
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

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /groups", requireAuth(http.HandlerFunc(h.listGroups)))
	mux.Handle("GET /groups/create", requireAuth(http.HandlerFunc(h.showCreate)))
	mux.Handle("POST /groups/create", requireAuth(http.HandlerFunc(h.handleCreate)))
	mux.Handle("GET /groups/{id}", requireAuth(http.HandlerFunc(h.showGroup)))
	mux.Handle("POST /groups/{id}/join", requireAuth(http.HandlerFunc(h.joinGroup)))
	mux.Handle("POST /groups/{id}/leave", requireAuth(http.HandlerFunc(h.leaveGroup)))
	mux.Handle("POST /groups/{id}/post", requireAuth(http.HandlerFunc(h.addPost)))
}

func (h *Handler) listGroups(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT g.id, g.name, g.description, g.created_by, g.created_at,
			(SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = g.id) AS member_count,
			EXISTS(SELECT 1 FROM group_members gm WHERE gm.group_id = g.id AND gm.user_id = $1) AS is_member
		 FROM interest_groups g
		 ORDER BY g.created_at DESC`, userID,
	)
	if err != nil {
		slog.Error("failed to load groups", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.CreatedBy,
			&g.CreatedAt, &g.MemberCount, &g.IsMember); err != nil {
			slog.Error("failed to scan group", "error", err)
			continue
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		slog.Error("failed to iterate groups", "error", err)
	}

	data := ListPage{BaseData: middleware.NewBaseData(r), Groups: groups}
	if err := h.pages["groups.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render groups", "error", err)
	}
}

func (h *Handler) showCreate(w http.ResponseWriter, r *http.Request) {
	data := CreateGroupPage{BaseData: middleware.NewBaseData(r)}
	if err := h.pages["group_create.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render group create", "error", err)
	}
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))

	if name == "" {
		data := CreateGroupPage{BaseData: middleware.NewBaseData(r), Error: "Group name is required."}
		h.pages["group_create.html"].ExecuteTemplate(w, "base", data)
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		slog.Error("failed to begin transaction", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var groupID string
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO interest_groups (name, description, created_by)
		 VALUES ($1, $2, $3) RETURNING id`,
		name, description, userID,
	).Scan(&groupID)
	if err != nil {
		slog.Error("failed to create group", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, 'admin')`,
		groupID, userID,
	)
	if err != nil {
		slog.Error("failed to add creator as admin", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit group creation", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/groups/"+groupID, http.StatusSeeOther)
}

func (h *Handler) showGroup(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	groupID := r.PathValue("id")

	var g Group
	err := h.db.QueryRowContext(r.Context(),
		`SELECT g.id, g.name, g.description, g.created_by, g.created_at,
			(SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = g.id) AS member_count
		 FROM interest_groups g
		 WHERE g.id = $1`, groupID,
	).Scan(&g.ID, &g.Name, &g.Description, &g.CreatedBy, &g.CreatedAt, &g.MemberCount)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Check membership and admin status
	var isMember bool
	var isGroupAdmin bool

	var memberRole string
	err = h.db.QueryRowContext(r.Context(),
		`SELECT role FROM group_members WHERE group_id = $1 AND user_id = $2`,
		groupID, userID,
	).Scan(&memberRole)
	if err == nil {
		isMember = true
		isGroupAdmin = memberRole == "admin"
	}

	// Load posts
	postRows, err := h.db.QueryContext(r.Context(),
		`SELECT gp.id, gp.user_id, CONCAT(u.first_name, ' ', u.last_name),
			gp.content, gp.created_at
		 FROM group_posts gp
		 JOIN users u ON u.id = gp.user_id
		 WHERE gp.group_id = $1
		 ORDER BY gp.created_at DESC
		 LIMIT 50`, groupID,
	)
	if err != nil {
		slog.Error("failed to load group posts", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer postRows.Close()

	var posts []GroupPost
	for postRows.Next() {
		var p GroupPost
		if err := postRows.Scan(&p.ID, &p.UserID, &p.AuthorName,
			&p.Content, &p.CreatedAt); err != nil {
			slog.Error("failed to scan group post", "error", err)
			continue
		}
		posts = append(posts, p)
	}
	if err := postRows.Err(); err != nil {
		slog.Error("failed to iterate group posts", "error", err)
	}

	data := GroupPage{
		BaseData:     middleware.NewBaseData(r),
		Group:        g,
		Posts:        posts,
		IsMember:     isMember,
		IsGroupAdmin: isGroupAdmin,
	}

	if err := h.pages["group_view.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render group", "error", err)
	}
}

func (h *Handler) joinGroup(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	groupID := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO group_members (group_id, user_id, role)
		 VALUES ($1, $2, 'member')
		 ON CONFLICT DO NOTHING`,
		groupID, userID,
	)
	if err != nil {
		slog.Error("failed to join group", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/groups/"+groupID, http.StatusSeeOther)
}

func (h *Handler) leaveGroup(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	groupID := r.PathValue("id")

	// Prevent leaving if user is the only admin
	var adminCount int
	err := h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM group_members
		 WHERE group_id = $1 AND role = 'admin'`,
		groupID,
	).Scan(&adminCount)
	if err != nil {
		slog.Error("failed to check admin count", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var userRole string
	_ = h.db.QueryRowContext(r.Context(),
		`SELECT role FROM group_members WHERE group_id = $1 AND user_id = $2`,
		groupID, userID,
	).Scan(&userRole)

	if userRole == "admin" && adminCount <= 1 {
		http.Error(w, "Cannot leave: you are the only admin", http.StatusBadRequest)
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`,
		groupID, userID,
	)
	if err != nil {
		slog.Error("failed to leave group", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/groups/"+groupID, http.StatusSeeOther)
}

func (h *Handler) addPost(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	groupID := r.PathValue("id")

	// Verify membership
	var exists bool
	err := h.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2)`,
		groupID, userID,
	).Scan(&exists)
	if err != nil || !exists {
		http.Error(w, "You must be a member to post", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	content := strings.TrimSpace(r.FormValue("content"))
	if content == "" {
		http.Redirect(w, r, "/groups/"+groupID, http.StatusSeeOther)
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO group_posts (group_id, user_id, content) VALUES ($1, $2, $3)`,
		groupID, userID, content,
	)
	if err != nil {
		slog.Error("failed to create group post", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/groups/"+groupID, http.StatusSeeOther)
}
