package workspace

import (
	"context"
	"database/sql"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
	"github.com/bhyland-usda/job-portal/internal/semantic"
	"github.com/bhyland-usda/job-portal/internal/skillgraph"
)

type Workspace struct {
	ID               string
	Name             string
	Description      string
	CreatedBy        string
	OwnerNames       string
	MeetingFrequency string
	PrimaryAudience  string
	HowToJoin        string
	MemberCount      int
	CreatedAt        time.Time
	IsMember         bool
}

type WorkspaceMeeting struct {
	ID            string
	Title         string
	Description   string
	MeetingAt     time.Time
	Location      string
	JoinURL       string
	CreatedBy     string
	CreatedByName string
	CreatedAt     time.Time
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
	Workspace          Workspace
	Members            []Member
	Notes              []Note
	Meetings           []WorkspaceMeeting
	IsMember           bool
	IsWorkspaceManager bool
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
	mux.Handle("GET /workspaces/{id}/join", requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/workspaces/"+r.PathValue("id"), http.StatusSeeOther)
	})))
	mux.Handle("POST /workspaces/{id}/notes", requireAuth(http.HandlerFunc(h.addNote)))
	mux.Handle("POST /workspaces/{id}/members", requireAuth(http.HandlerFunc(h.addMember)))
	mux.Handle("GET /workspaces/{id}/member-search", requireAuth(http.HandlerFunc(h.searchMembersByName)))
	mux.Handle("POST /workspaces/{id}/settings", requireAuth(http.HandlerFunc(h.updateWorkspaceSettings)))
	mux.Handle("POST /workspaces/{id}/meetings", requireAuth(http.HandlerFunc(h.addMeeting)))
}

type memberSearchResult struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Department string `json:"department"`
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

func (h *Handler) membershipRole(r *http.Request, workspaceID, userID string) (string, error) {
	var role string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`,
		workspaceID, userID,
	).Scan(&role)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return role, err
}

func isWorkspaceManagerRole(role string) bool {
	return role == "owner" || role == "moderator"
}

func workspaceSelectClause() string {
	return `SELECT ws.id, ws.name, ws.description, ws.created_by, ws.created_at,
		COALESCE(ws.meeting_frequency, ''),
		COALESCE(ws.primary_audience, ''),
		COALESCE(ws.how_to_join, ''),
		COALESCE((
			SELECT string_agg(TRIM(CONCAT(u.first_name, ' ', u.last_name)), ', ' ORDER BY u.first_name, u.last_name)
			FROM workspace_members wm2
			JOIN users u ON u.id = wm2.user_id
			WHERE wm2.workspace_id = ws.id AND wm2.role = 'owner'
		), ''),
		(SELECT COUNT(*) FROM workspace_members wm WHERE wm.workspace_id = ws.id) AS member_count`
}

func scanWorkspace(scanner interface{ Scan(dest ...any) error }, ws *Workspace) error {
	return scanner.Scan(
		&ws.ID,
		&ws.Name,
		&ws.Description,
		&ws.CreatedBy,
		&ws.CreatedAt,
		&ws.MeetingFrequency,
		&ws.PrimaryAudience,
		&ws.HowToJoin,
		&ws.OwnerNames,
		&ws.MemberCount,
	)
}

func (h *Handler) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	activeTab := strings.TrimSpace(r.URL.Query().Get("tab"))
	if activeTab != "recommended" && activeTab != "search" && activeTab != "create" {
		activeTab = "mine"
	}

	rows, err := h.db.QueryContext(r.Context(),
		workspaceSelectClause()+`
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
		if err := scanWorkspace(rows, &ws); err != nil {
			slog.Error("failed to scan workspace", "error", err)
			continue
		}
		workspaces = append(workspaces, ws)
	}
	if err := rows.Err(); err != nil {
		slog.Error("failed to iterate workspaces", "error", err)
	}

	matchingEnabled := semantic.EnabledForRequest(r)

	recommendedWorkspaces, err := GetMatchedWorkspaces(h.db, r.Context(), userID, matchingEnabled)
	if err != nil {
		slog.Error("failed to load workspace recommendations", "error", err)
	}

	searchResults, err := SearchWorkspaces(h.db, r.Context(), userID, searchQuery, matchingEnabled)
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
	meetingFrequency := strings.TrimSpace(r.FormValue("meeting_frequency"))
	primaryAudience := strings.TrimSpace(r.FormValue("primary_audience"))
	howToJoin := strings.TrimSpace(r.FormValue("how_to_join"))

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
		`INSERT INTO workspaces (name, description, created_by, meeting_frequency, primary_audience, how_to_join)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		name, description, userID, meetingFrequency, primaryAudience, howToJoin,
	).Scan(&workspaceID)
	if err != nil {
		slog.Error("failed to create workspace", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`,
		workspaceID, userID,
	)
	if err != nil {
		slog.Error("failed to add creator as owner", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit workspace creation", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.syncWorkspaceEmbedding(r.Context(), workspaceID)

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
		workspaceSelectClause()+`
		 FROM workspaces ws
		 JOIN workspace_members m ON m.workspace_id = ws.id AND m.user_id = $1
		 ORDER BY ws.created_at DESC`, userID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ws Workspace
			if err := scanWorkspace(rows, &ws); err != nil {
				continue
			}
			data.Workspaces = append(data.Workspaces, ws)
		}
	}

	matchingEnabled := semantic.EnabledForRequest(r)

	recommendedWorkspaces, err := GetMatchedWorkspaces(h.db, r.Context(), userID, matchingEnabled)
	if err != nil {
		slog.Error("failed to load workspace recommendations", "error", err)
	} else {
		data.RecommendedWorkspaces = recommendedWorkspaces
	}

	searchResults, err := SearchWorkspaces(h.db, r.Context(), userID, searchQuery, matchingEnabled)
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

	membershipRole, err := h.membershipRole(r, workspaceID, userID)
	if err != nil {
		slog.Error("failed to load workspace membership role", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var ws Workspace
	err = scanWorkspace(h.db.QueryRowContext(r.Context(),
		workspaceSelectClause()+`
		 FROM workspaces ws
		 WHERE ws.id = $1`, workspaceID,
	), &ws)
	if err != nil {
		http.NotFound(w, r)
		return
	}

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

	meetingRows, err := h.db.QueryContext(r.Context(),
		`SELECT m.id, m.title, COALESCE(m.description, ''), m.meeting_at,
			COALESCE(m.location, ''), COALESCE(m.join_url, ''),
			m.created_by, CONCAT(u.first_name, ' ', u.last_name), m.created_at
		 FROM workspace_meetings m
		 JOIN users u ON u.id = m.created_by
		 WHERE m.workspace_id = $1
		 ORDER BY m.meeting_at ASC, m.created_at DESC
		 LIMIT 25`, workspaceID,
	)
	if err != nil {
		slog.Error("failed to load workspace meetings", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer meetingRows.Close()

	var meetings []WorkspaceMeeting
	for meetingRows.Next() {
		var m WorkspaceMeeting
		if err := meetingRows.Scan(&m.ID, &m.Title, &m.Description, &m.MeetingAt,
			&m.Location, &m.JoinURL, &m.CreatedBy, &m.CreatedByName, &m.CreatedAt); err != nil {
			slog.Error("failed to scan workspace meeting", "error", err)
			continue
		}
		meetings = append(meetings, m)
	}
	if err := meetingRows.Err(); err != nil {
		slog.Error("failed to iterate workspace meetings", "error", err)
	}

	data := ViewPage{
		BaseData:           middleware.NewBaseData(r),
		Workspace:          ws,
		Members:            members,
		Notes:              notes,
		Meetings:           meetings,
		IsMember:           member,
		IsWorkspaceManager: isWorkspaceManagerRole(membershipRole),
	}

	if err := h.pages["workspace_view.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render workspace", "error", err)
	}
}

// GetMatchedWorkspaces returns workspaces whose title or description match at
// least one skill on the current user's profile.
func GetMatchedWorkspaces(db *sql.DB, ctx context.Context, userID string, matchingEnabled bool) ([]Workspace, error) {
	if matchingEnabled && skillgraph.Enabled() {
		graphMatches, err := getMatchedWorkspacesBySkillGraph(ctx, db, userID)
		if err != nil {
			slog.Error("skill-graph workspace matching failed; using semantic/vector fallback", "error", err)
		} else if len(graphMatches) > 0 {
			return graphMatches, nil
		}
	}

	if matchingEnabled && semantic.Enabled() {
		text, err := userWorkspaceCorpus(ctx, db, userID)
		if err != nil {
			slog.Error("failed to build user workspace corpus", "user_id", userID, "error", err)
		} else if text != "" {
			rows, err := db.QueryContext(ctx,
				workspaceSelectClause()+`
				 FROM workspaces ws
				 JOIN semantic_embeddings se
				   ON se.entity_type = $2 AND se.entity_id = ws.id
				 WHERE NOT EXISTS (
				 	SELECT 1 FROM workspace_members m
				 	WHERE m.workspace_id = ws.id AND m.user_id = $1
				 )
				 ORDER BY se.embedding <=> $3::vector, ws.created_at DESC
				 LIMIT 50`,
				userID,
				semantic.EntityTypeWorkspace,
				semantic.ToPGVectorLiteral(semantic.GenerateEmbedding(text)),
			)
			if err == nil {
				defer rows.Close()

				var workspaces []Workspace
				for rows.Next() {
					var ws Workspace
					if err := scanWorkspace(rows, &ws); err != nil {
						continue
					}
					workspaces = append(workspaces, ws)
				}
				if rows.Err() == nil && len(workspaces) > 0 {
					return workspaces, nil
				}
			} else {
				slog.Error("semantic workspace matching failed; using skill keyword fallback", "error", err)
			}
		}
	}

	rows, err := db.QueryContext(ctx,
		workspaceSelectClause()+`
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
		if err := scanWorkspace(rows, &ws); err != nil {
			continue
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, rows.Err()
}

func getMatchedWorkspacesBySkillGraph(ctx context.Context, db *sql.DB, userID string) ([]Workspace, error) {
	rows, err := db.QueryContext(ctx,
		`WITH user_skills AS (
		    SELECT DISTINCT COALESCE(sa.canonical_skill, lower(s.name)) AS skill
		    FROM skills s
		    LEFT JOIN skill_aliases sa ON sa.alias_skill = lower(s.name)
		    WHERE s.user_id = $1
		),
		expanded_skills AS (
		    SELECT skill, 1.0::float8 AS weight FROM user_skills
		    UNION ALL
		    SELECT sg.to_skill AS skill,
		           GREATEST(0, LEAST(1, sg.weight))::float8 AS weight
		    FROM skill_adjacency sg
		    JOIN user_skills us ON us.skill = sg.from_skill
		),
		consolidated_skills AS (
		    SELECT skill, MAX(weight) AS weight
		    FROM expanded_skills
		    GROUP BY skill
		),
		workspace_scores AS (
		    SELECT ws.id,
		           SUM(CASE
		               WHEN LOWER(ws.name) LIKE '%' || cs.skill || '%'
		                 OR LOWER(COALESCE(ws.description, '')) LIKE '%' || cs.skill || '%'
		               THEN cs.weight ELSE 0
		           END) AS skill_score
		    FROM workspaces ws
		    CROSS JOIN consolidated_skills cs
		    GROUP BY ws.id
		)
		SELECT ws.id, ws.name, ws.description, ws.created_by, ws.created_at,
		       COALESCE(ws.meeting_frequency, ''),
		       COALESCE(ws.primary_audience, ''),
		       COALESCE(ws.how_to_join, ''),
		       COALESCE((
		           SELECT string_agg(TRIM(CONCAT(u.first_name, ' ', u.last_name)), ', ' ORDER BY u.first_name, u.last_name)
		           FROM workspace_members wm2
		           JOIN users u ON u.id = wm2.user_id
		           WHERE wm2.workspace_id = ws.id AND wm2.role = 'owner'
		       ), ''),
		       (SELECT COUNT(*) FROM workspace_members wm WHERE wm.workspace_id = ws.id) AS member_count,
		       COALESCE(sc.skill_score, 0) AS skill_score
		FROM workspaces ws
		JOIN workspace_scores sc ON sc.id = ws.id
		WHERE NOT EXISTS (
		    SELECT 1 FROM workspace_members m
		    WHERE m.workspace_id = ws.id AND m.user_id = $1
		)
		  AND sc.skill_score > 0
		ORDER BY sc.skill_score DESC, ws.created_at DESC
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
		var score float64
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Description, &ws.CreatedBy, &ws.CreatedAt,
			&ws.MeetingFrequency, &ws.PrimaryAudience, &ws.HowToJoin, &ws.OwnerNames,
			&ws.MemberCount, &score); err != nil {
			continue
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, rows.Err()
}

// SearchWorkspaces returns non-member workspaces matching a free-text query.
func SearchWorkspaces(db *sql.DB, ctx context.Context, userID, query string, matchingEnabled bool) ([]Workspace, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	if matchingEnabled && semantic.Enabled() {
		rows, err := db.QueryContext(ctx,
			workspaceSelectClause()+`,
				EXISTS(
					SELECT 1 FROM workspace_members wm
					WHERE wm.workspace_id = ws.id AND wm.user_id = $1
				) AS is_member
			 FROM workspaces ws
			 JOIN semantic_embeddings se
			   ON se.entity_type = $2 AND se.entity_id = ws.id
			 ORDER BY is_member DESC, se.embedding <=> $3::vector, ws.created_at DESC
			 LIMIT 50`,
			userID,
			semantic.EntityTypeWorkspace,
			semantic.ToPGVectorLiteral(semantic.GenerateEmbedding(query)),
		)
		if err == nil {
			defer rows.Close()

			var workspaces []Workspace
			for rows.Next() {
				var ws Workspace
				if err := rows.Scan(&ws.ID, &ws.Name, &ws.Description, &ws.CreatedBy, &ws.CreatedAt,
					&ws.MeetingFrequency, &ws.PrimaryAudience, &ws.HowToJoin, &ws.OwnerNames,
					&ws.MemberCount, &ws.IsMember); err != nil {
					continue
				}
				workspaces = append(workspaces, ws)
			}
			if rows.Err() == nil && len(workspaces) > 0 {
				return workspaces, nil
			}
		} else {
			slog.Error("semantic workspace search failed; using keyword fallback", "error", err)
		}
	}

	likePattern := "%" + query + "%"
	rows, err := db.QueryContext(ctx,
		workspaceSelectClause()+`,
			EXISTS(
				SELECT 1 FROM workspace_members wm
				WHERE wm.workspace_id = ws.id AND wm.user_id = $1
			) AS is_member
		 FROM workspaces ws
		 WHERE (
		 	LOWER(ws.name) LIKE LOWER($2)
		 	OR LOWER(COALESCE(ws.description, '')) LIKE LOWER($3)
		 	OR LOWER(COALESCE(ws.primary_audience, '')) LIKE LOWER($4)
		 )
		 ORDER BY is_member DESC, ws.created_at DESC
		 LIMIT 50`,
		userID, likePattern, likePattern, likePattern,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []Workspace
	for rows.Next() {
		var ws Workspace
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Description, &ws.CreatedBy, &ws.CreatedAt,
			&ws.MeetingFrequency, &ws.PrimaryAudience, &ws.HowToJoin, &ws.OwnerNames,
			&ws.MemberCount, &ws.IsMember); err != nil {
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

func (h *Handler) syncWorkspaceEmbedding(ctx context.Context, workspaceID string) {
	if !semantic.Enabled() {
		return
	}

	if err := semantic.UpsertWorkspaceEmbeddingByID(ctx, h.db, workspaceID); err != nil {
		slog.Error("failed to upsert workspace embedding", "workspace_id", workspaceID, "error", err)
	}
}

func userWorkspaceCorpus(ctx context.Context, db *sql.DB, userID string) (string, error) {
	var headline, about, skills sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT COALESCE(u.headline, ''),
		        COALESCE(u.about, ''),
		        COALESCE(string_agg(s.name, ' '), '')
		 FROM users u
		 LEFT JOIN skills s ON s.user_id = u.id
		 WHERE u.id = $1
		 GROUP BY u.id, u.headline, u.about`,
		userID,
	).Scan(&headline, &about, &skills)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(strings.Join([]string{headline.String, about.String, skills.String}, "\n")), nil
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

	requestedRole := strings.TrimSpace(strings.ToLower(r.FormValue("role")))
	if requestedRole == "" {
		requestedRole = "member"
	}
	if requestedRole != "member" && requestedRole != "moderator" {
		http.Error(w, "Invalid workspace role", http.StatusBadRequest)
		return
	}

	actorRole, err := h.membershipRole(r, workspaceID, userID)
	if err != nil {
		slog.Error("failed to load acting member role", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if requestedRole == "moderator" && !isWorkspaceManagerRole(actorRole) {
		http.Error(w, "Only owner or moderators can add moderators", http.StatusForbidden)
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

	if requestedRole == "moderator" {
		_, err = h.db.ExecContext(r.Context(),
			`UPDATE workspace_members
			 SET role = 'moderator'
			 WHERE workspace_id = $1 AND user_id = $2`,
			workspaceID, newUserID,
		)
		if err != nil {
			slog.Error("failed to promote workspace member to moderator", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/workspaces/"+workspaceID, http.StatusSeeOther)
}

func (h *Handler) searchMembersByName(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	workspaceID := r.PathValue("id")

	member, err := h.isMember(r, workspaceID, userID)
	if err != nil || !member {
		http.Error(w, "You must be a member to search users", http.StatusForbidden)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) < 2 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]memberSearchResult{})
		return
	}

	likePattern := "%" + query + "%"
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT u.id,
			TRIM(CONCAT(u.first_name, ' ', u.last_name)) AS full_name,
			u.email,
			COALESCE(d.name, '') AS department_name
		 FROM users u
		 LEFT JOIN departments d ON d.id = u.department_id
		 WHERE (
		 	LOWER(u.first_name) LIKE LOWER($2)
		 	OR LOWER(u.last_name) LIKE LOWER($3)
		 	OR LOWER(TRIM(CONCAT(u.first_name, ' ', u.last_name))) LIKE LOWER($4)
		 )
		 AND NOT EXISTS (
		 	SELECT 1 FROM workspace_members wm
		 	WHERE wm.workspace_id = $1 AND wm.user_id = u.id
		 )
		 ORDER BY u.first_name, u.last_name
		 LIMIT 10`,
		workspaceID, likePattern, likePattern, likePattern,
	)
	if err != nil {
		slog.Error("failed to search workspace member candidates", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := make([]memberSearchResult, 0, 10)
	for rows.Next() {
		var item memberSearchResult
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.Department); err != nil {
			continue
		}
		results = append(results, item)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		slog.Error("failed to encode workspace member search response", "error", err)
	}
}

func (h *Handler) updateWorkspaceSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	workspaceID := r.PathValue("id")

	role, err := h.membershipRole(r, workspaceID, userID)
	if err != nil {
		slog.Error("failed to check workspace role", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !isWorkspaceManagerRole(role) {
		http.Error(w, "Only owners and moderators can manage workspace settings", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	meetingFrequency := strings.TrimSpace(r.FormValue("meeting_frequency"))
	primaryAudience := strings.TrimSpace(r.FormValue("primary_audience"))
	howToJoin := strings.TrimSpace(r.FormValue("how_to_join"))

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE workspaces
		 SET meeting_frequency = $2,
		     primary_audience = $3,
		     how_to_join = $4
		 WHERE id = $1`,
		workspaceID, meetingFrequency, primaryAudience, howToJoin,
	)
	if err != nil {
		slog.Error("failed to update workspace settings", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.syncWorkspaceEmbedding(r.Context(), workspaceID)
	http.Redirect(w, r, "/workspaces/"+workspaceID, http.StatusSeeOther)
}

func (h *Handler) addMeeting(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	workspaceID := r.PathValue("id")

	role, err := h.membershipRole(r, workspaceID, userID)
	if err != nil {
		slog.Error("failed to check workspace role", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !isWorkspaceManagerRole(role) {
		http.Error(w, "Only owners and moderators can schedule meetings", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	meetingAtText := strings.TrimSpace(r.FormValue("meeting_at"))
	description := strings.TrimSpace(r.FormValue("description"))
	location := strings.TrimSpace(r.FormValue("location"))
	joinURL := strings.TrimSpace(r.FormValue("join_url"))

	if title == "" || meetingAtText == "" {
		http.Error(w, "Meeting title and date/time are required", http.StatusBadRequest)
		return
	}

	meetingAt, err := time.Parse("2006-01-02T15:04", meetingAtText)
	if err != nil {
		http.Error(w, "Invalid meeting date/time", http.StatusBadRequest)
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO workspace_meetings (workspace_id, title, description, meeting_at, location, join_url, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		workspaceID, title, description, meetingAt.UTC(), location, joinURL, userID,
	)
	if err != nil {
		slog.Error("failed to create workspace meeting", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/workspaces/"+workspaceID, http.StatusSeeOther)
}
