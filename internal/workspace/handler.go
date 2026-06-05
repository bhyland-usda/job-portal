package workspace

import (
	"context"
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type Workspace struct {
	ID          string
	Name        string
	Description string
	CreatedBy   string
	MemberCount int
	CreatedAt   time.Time
	IsMember    bool
}

type Member struct {
	UserID   string
	Name     string
	JoinedAt time.Time
}

type Note struct {
	ID         string
	AuthorID   string
	AuthorName string
	Body       string
	CreatedAt  time.Time
}

type ListPage struct {
	middleware.BaseData
	Workspaces            []Workspace
	RecommendedWorkspaces []Workspace
	SearchQuery           string
	SearchResults         []Workspace
	ActiveTab             string
	Error                 string
}

type ViewPage struct {
	middleware.BaseData
	Workspace Workspace
	Members   []Member
	Notes     []Note
	IsMember  bool
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /workspaces", requireAuth(http.HandlerFunc(h.listWorkspaces)))
	mux.Handle("POST /workspaces", requireAuth(http.HandlerFunc(h.handleCreate)))
	mux.Handle("GET /workspaces/{id}", requireAuth(http.HandlerFunc(h.showWorkspace)))
	mux.Handle("POST /workspaces/{id}/join", requireAuth(http.HandlerFunc(h.joinWorkspace)))
	mux.Handle("POST /workspaces/{id}/notes", requireAuth(http.HandlerFunc(h.addNote)))
	mux.Handle("POST /workspaces/{id}/members", requireAuth(http.HandlerFunc(h.addMember)))
}

// isMember reports whether the given user belongs to the workspace.
func (h *Handler) isMember(r *http.Request, workspaceID, userID string) (bool, error) {
	var exists bool
	err := h.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2)`,
		workspaceID, userID,
	).Scan(&exists)
	return exists, err
}

func (h *Handler) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	activeTab := strings.TrimSpace(r.URL.Query().Get("tab"))
	if activeTab != "recommended" && activeTab != "search" && activeTab != "create" {
		activeTab = "mine"
	}

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT ws.id, ws.name, ws.description, ws.created_by, ws.created_at,
			(SELECT COUNT(*) FROM workspace_members wm WHERE wm.workspace_id = ws.id) AS member_count
		 FROM workspaces ws
		 JOIN workspace_members m ON m.workspace_id = ws.id AND m.user_id = $1
		 ORDER BY ws.created_at DESC`, userID,
	)
	if err != nil {
		slog.Error("failed to load workspaces", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var workspaces []Workspace
	for rows.Next() {
		var ws Workspace
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Description, &ws.CreatedBy,
			&ws.CreatedAt, &ws.MemberCount); err != nil {
			slog.Error("failed to scan workspace", "error", err)
			continue
		}
		workspaces = append(workspaces, ws)
	}
	if err := rows.Err(); err != nil {
		slog.Error("failed to iterate workspaces", "error", err)
	}

	recommendedWorkspaces, err := GetMatchedWorkspaces(h.db, r.Context(), userID)
	if err != nil {
		slog.Error("failed to load workspace recommendations", "error", err)
	}

	searchResults, err := SearchWorkspaces(h.db, r.Context(), userID, searchQuery)
	if err != nil {
		slog.Error("failed to search workspaces", "error", err)
	}

	data := ListPage{BaseData: middleware.NewBaseData(r), Workspaces: workspaces, RecommendedWorkspaces: recommendedWorkspaces, SearchQuery: searchQuery, SearchResults: searchResults, ActiveTab: activeTab}
	if err := h.pages["workspaces.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render workspaces", "error", err)
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
		data := ListPage{BaseData: middleware.NewBaseData(r), Error: "Workspace name is required."}
		h.listWorkspacesWithData(w, r, &data)
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		slog.Error("failed to begin transaction", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var workspaceID string
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO workspaces (name, description, created_by)
		 VALUES ($1, $2, $3) RETURNING id`,
		name, description, userID,
	).Scan(&workspaceID)
	if err != nil {
		slog.Error("failed to create workspace", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO workspace_members (workspace_id, user_id) VALUES ($1, $2)`,
		workspaceID, userID,
	)
	if err != nil {
		slog.Error("failed to add creator as member", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit workspace creation", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/workspaces/"+workspaceID, http.StatusSeeOther)
}

// listWorkspacesWithData re-renders the list page reusing supplied data (e.g. with an error),
// populating the user's workspaces.
func (h *Handler) listWorkspacesWithData(w http.ResponseWriter, r *http.Request, data *ListPage) {
	userID := middleware.GetUserID(r.Context())
	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	data.SearchQuery = searchQuery
	activeTab := strings.TrimSpace(r.URL.Query().Get("tab"))
	if activeTab != "recommended" && activeTab != "search" && activeTab != "create" {
		activeTab = "mine"
	}
	data.ActiveTab = activeTab

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT ws.id, ws.name, ws.description, ws.created_by, ws.created_at,
			(SELECT COUNT(*) FROM workspace_members wm WHERE wm.workspace_id = ws.id) AS member_count
		 FROM workspaces ws
		 JOIN workspace_members m ON m.workspace_id = ws.id AND m.user_id = $1
		 ORDER BY ws.created_at DESC`, userID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ws Workspace
			if err := rows.Scan(&ws.ID, &ws.Name, &ws.Description, &ws.CreatedBy,
				&ws.CreatedAt, &ws.MemberCount); err != nil {
				continue
			}
			data.Workspaces = append(data.Workspaces, ws)
		}
	}

	recommendedWorkspaces, err := GetMatchedWorkspaces(h.db, r.Context(), userID)
	if err != nil {
		slog.Error("failed to load workspace recommendations", "error", err)
	} else {
		data.RecommendedWorkspaces = recommendedWorkspaces
	}

	searchResults, err := SearchWorkspaces(h.db, r.Context(), userID, searchQuery)
	if err != nil {
		slog.Error("failed to search workspaces", "error", err)
	} else {
		data.SearchResults = searchResults
	}

	if err := h.pages["workspaces.html"].ExecuteTemplate(w, "base", *data); err != nil {
		slog.Error("failed to render workspaces", "error", err)
	}
}

func (h *Handler) showWorkspace(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	workspaceID := r.PathValue("id")

	member, err := h.isMember(r, workspaceID, userID)
	if err != nil {
		slog.Error("failed to check workspace membership", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var ws Workspace
	err = h.db.QueryRowContext(r.Context(),
		`SELECT ws.id, ws.name, ws.description, ws.created_by, ws.created_at,
			(SELECT COUNT(*) FROM workspace_members wm WHERE wm.workspace_id = ws.id) AS member_count
		 FROM workspaces ws
		 WHERE ws.id = $1`, workspaceID,
	).Scan(&ws.ID, &ws.Name, &ws.Description, &ws.CreatedBy, &ws.CreatedAt, &ws.MemberCount)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Load members
	memberRows, err := h.db.QueryContext(r.Context(),
		`SELECT wm.user_id, CONCAT(u.first_name, ' ', u.last_name), wm.joined_at
		 FROM workspace_members wm
		 JOIN users u ON u.id = wm.user_id
		 WHERE wm.workspace_id = $1
		 ORDER BY wm.joined_at ASC`, workspaceID,
	)
	if err != nil {
		slog.Error("failed to load workspace members", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer memberRows.Close()

	var members []Member
	for memberRows.Next() {
		var m Member
		if err := memberRows.Scan(&m.UserID, &m.Name, &m.JoinedAt); err != nil {
			slog.Error("failed to scan workspace member", "error", err)
			continue
		}
		members = append(members, m)
	}
	if err := memberRows.Err(); err != nil {
		slog.Error("failed to iterate workspace members", "error", err)
	}

	// Load notes (newest first)
	noteRows, err := h.db.QueryContext(r.Context(),
		`SELECT n.id, n.author_id, CONCAT(u.first_name, ' ', u.last_name), n.body, n.created_at
		 FROM workspace_notes n
		 JOIN users u ON u.id = n.author_id
		 WHERE n.workspace_id = $1
		 ORDER BY n.created_at DESC
		 LIMIT 50`, workspaceID,
	)
	if err != nil {
		slog.Error("failed to load workspace notes", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer noteRows.Close()

	var notes []Note
	for noteRows.Next() {
		var n Note
		if err := noteRows.Scan(&n.ID, &n.AuthorID, &n.AuthorName, &n.Body, &n.CreatedAt); err != nil {
			slog.Error("failed to scan workspace note", "error", err)
			continue
		}
		notes = append(notes, n)
	}
	if err := noteRows.Err(); err != nil {
		slog.Error("failed to iterate workspace notes", "error", err)
	}

	data := ViewPage{
		BaseData:  middleware.NewBaseData(r),
		Workspace: ws,
		Members:   members,
		Notes:     notes,
		IsMember:  member,
	}

	if err := h.pages["workspace_view.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render workspace", "error", err)
	}
}

// GetMatchedWorkspaces returns workspaces whose title or description match at
// least one skill on the current user's profile.
func GetMatchedWorkspaces(db *sql.DB, ctx context.Context, userID string) ([]Workspace, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT ws.id, ws.name, ws.description, ws.created_by, ws.created_at,
			(SELECT COUNT(*) FROM workspace_members wm WHERE wm.workspace_id = ws.id) AS member_count
		 FROM workspaces ws
		 WHERE NOT EXISTS (
		 	SELECT 1 FROM workspace_members m
		 	WHERE m.workspace_id = ws.id AND m.user_id = $1
		 )
		 AND EXISTS (
		 	SELECT 1 FROM skills s
		 	WHERE s.user_id = $1
		 	  AND (
		 		LOWER(ws.name) LIKE '%' || LOWER(s.name) || '%'
		 		OR LOWER(COALESCE(ws.description, '')) LIKE '%' || LOWER(s.name) || '%'
		 	  )
		 )
		 ORDER BY ws.created_at DESC
		 LIMIT 50`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []Workspace
	for rows.Next() {
		var ws Workspace
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Description, &ws.CreatedBy, &ws.CreatedAt, &ws.MemberCount); err != nil {
			continue
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, rows.Err()
}

// SearchWorkspaces returns non-member workspaces matching a free-text query.
func SearchWorkspaces(db *sql.DB, ctx context.Context, userID, query string) ([]Workspace, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	likePattern := "%" + query + "%"
	rows, err := db.QueryContext(ctx,
		`SELECT ws.id, ws.name, ws.description, ws.created_by, ws.created_at,
			(SELECT COUNT(*) FROM workspace_members wm WHERE wm.workspace_id = ws.id) AS member_count,
			EXISTS(
				SELECT 1 FROM workspace_members wm
				WHERE wm.workspace_id = ws.id AND wm.user_id = $1
			) AS is_member
		 FROM workspaces ws
		 WHERE (
		 	LOWER(ws.name) LIKE LOWER($2)
		 	OR LOWER(COALESCE(ws.description, '')) LIKE LOWER($3)
		 )
		 ORDER BY is_member DESC, ws.created_at DESC
		 LIMIT 50`,
		userID, likePattern, likePattern,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []Workspace
	for rows.Next() {
		var ws Workspace
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Description, &ws.CreatedBy, &ws.CreatedAt, &ws.MemberCount, &ws.IsMember); err != nil {
			continue
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, rows.Err()
}

func (h *Handler) joinWorkspace(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	workspaceID := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO workspace_members (workspace_id, user_id)
		 VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`,
		workspaceID, userID,
	)
	if err != nil {
		slog.Error("failed to join workspace", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/workspaces/"+workspaceID, http.StatusSeeOther)
}

func (h *Handler) addNote(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	workspaceID := r.PathValue("id")

	member, err := h.isMember(r, workspaceID, userID)
	if err != nil || !member {
		http.Error(w, "You must be a member to add a note", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		http.Redirect(w, r, "/workspaces/"+workspaceID, http.StatusSeeOther)
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO workspace_notes (workspace_id, author_id, body) VALUES ($1, $2, $3)`,
		workspaceID, userID, body,
	)
	if err != nil {
		slog.Error("failed to create workspace note", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/workspaces/"+workspaceID, http.StatusSeeOther)
}

func (h *Handler) addMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	workspaceID := r.PathValue("id")

	member, err := h.isMember(r, workspaceID, userID)
	if err != nil || !member {
		http.Error(w, "You must be a member to add others", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	newUserID := strings.TrimSpace(r.FormValue("user_id"))
	if newUserID == "" {
		http.Redirect(w, r, "/workspaces/"+workspaceID, http.StatusSeeOther)
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO workspace_members (workspace_id, user_id)
		 VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`,
		workspaceID, newUserID,
	)
	if err != nil {
		slog.Error("failed to add workspace member", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/workspaces/"+workspaceID, http.StatusSeeOther)
}
