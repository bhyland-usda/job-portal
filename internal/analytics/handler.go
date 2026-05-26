package analytics

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type SkillGap struct {
	SkillName   string
	DemandCount int
	SupplyCount int
	GapPercent  int
}

type LearningRec struct {
	SkillName    string
	PostingCount int
}

type SkillsGapPage struct {
	middleware.BaseData
	Gaps []SkillGap
}

type LearningPage struct {
	middleware.BaseData
	Recommendations []LearningRec
}

// NamedCount is a generic label/count pair used for dashboard tables.
type NamedCount struct {
	Name  string
	Count int
}

type EngagementTotals struct {
	Posts       int
	Comments    int
	Connections int
	Kudos       int
}

type WorkforcePage struct {
	middleware.BaseData
	TotalEmployees int
	Departments    []NamedCount
	Roles          []NamedCount
	TopSkills      []NamedCount
	Engagement     EngagementTotals
}

type LeaderboardEntry struct {
	Rank       int
	UserID     string
	Name       string
	Department string
	Score      int
}

type DepartmentOption struct {
	ID   string
	Name string
}

type LeaderboardPage struct {
	middleware.BaseData
	Entries            []LeaderboardEntry
	DepartmentOptions  []DepartmentOption
	SelectedDepartment string
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth, requireManager func(http.Handler) http.Handler) {
	mux.Handle("GET /analytics/skills-gap", requireAuth(requireManager(http.HandlerFunc(h.showSkillsGap))))
	mux.Handle("GET /analytics/learning", requireAuth(http.HandlerFunc(h.showLearning)))
	mux.Handle("GET /analytics/workforce", requireAuth(requireManager(http.HandlerFunc(h.showWorkforce))))
	mux.Handle("GET /analytics/leaderboard", requireAuth(http.HandlerFunc(h.showLeaderboard)))
}

func (h *Handler) showSkillsGap(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT ps.skill_name,
			COUNT(DISTINCT ps.posting_id) as demand,
			(SELECT COUNT(DISTINCT s.user_id) FROM skills s WHERE LOWER(s.name) = LOWER(ps.skill_name)) as supply
		 FROM posting_skills ps
		 JOIN postings p ON p.id = ps.posting_id AND p.status = 'active'
		 GROUP BY ps.skill_name
		 ORDER BY demand DESC
		 LIMIT 30`,
	)
	if err != nil {
		slog.Error("failed to load skills gap", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var gaps []SkillGap
	for rows.Next() {
		var g SkillGap
		if err := rows.Scan(&g.SkillName, &g.DemandCount, &g.SupplyCount); err != nil {
			continue
		}
		if g.DemandCount > 0 && g.SupplyCount < g.DemandCount {
			g.GapPercent = ((g.DemandCount - g.SupplyCount) * 100) / g.DemandCount
		}
		gaps = append(gaps, g)
	}

	data := SkillsGapPage{BaseData: middleware.NewBaseData(r), Gaps: gaps}
	h.pages["skills_gap.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) showLearning(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT ps.skill_name, COUNT(DISTINCT ps.posting_id) as cnt
		 FROM posting_skills ps
		 JOIN postings p ON p.id = ps.posting_id AND p.status = 'active'
		 WHERE NOT EXISTS(
			SELECT 1 FROM skills s WHERE s.user_id = $1 AND LOWER(s.name) = LOWER(ps.skill_name)
		 )
		 GROUP BY ps.skill_name
		 ORDER BY cnt DESC
		 LIMIT 10`,
		userID,
	)
	if err != nil {
		slog.Error("failed to load learning recs", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var recs []LearningRec
	for rows.Next() {
		var r LearningRec
		if err := rows.Scan(&r.SkillName, &r.PostingCount); err != nil {
			continue
		}
		recs = append(recs, r)
	}

	data := LearningPage{BaseData: middleware.NewBaseData(r), Recommendations: recs}
	h.pages["learning.html"].ExecuteTemplate(w, "base", data)
}

// scanNamedCounts reads (name, count) pairs into a slice of NamedCount.
func scanNamedCounts(rows *sql.Rows) []NamedCount {
	defer rows.Close()
	var out []NamedCount
	for rows.Next() {
		var nc NamedCount
		if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
			continue
		}
		out = append(out, nc)
	}
	return out
}

// queryCount runs a single COUNT(*) query and returns the scalar (0 on error).
func (h *Handler) queryCount(r *http.Request, query string) int {
	var n int
	if err := h.db.QueryRowContext(r.Context(), query).Scan(&n); err != nil {
		slog.Error("failed to load count", "error", err, "query", query)
		return 0
	}
	return n
}

// showWorkforce renders the leadership workforce analytics dashboard:
// total active employees, headcount by department, role breakdown, top skills
// org-wide and engagement totals. Manager/admin only.
func (h *Handler) showWorkforce(w http.ResponseWriter, r *http.Request) {
	data := WorkforcePage{BaseData: middleware.NewBaseData(r)}

	data.TotalEmployees = h.queryCount(r, `SELECT COUNT(*) FROM users`)

	if rows, err := h.db.QueryContext(r.Context(),
		`SELECT d.name, COUNT(u.id)
		 FROM users u
		 JOIN departments d ON d.id = u.department_id
		 GROUP BY d.name
		 ORDER BY COUNT(u.id) DESC`,
	); err != nil {
		slog.Error("failed to load department headcount", "error", err)
	} else {
		data.Departments = scanNamedCounts(rows)
	}

	if rows, err := h.db.QueryContext(r.Context(),
		`SELECT role, COUNT(*)
		 FROM users
		 GROUP BY role
		 ORDER BY COUNT(*) DESC`,
	); err != nil {
		slog.Error("failed to load role breakdown", "error", err)
	} else {
		data.Roles = scanNamedCounts(rows)
	}

	if rows, err := h.db.QueryContext(r.Context(),
		`SELECT name, COUNT(DISTINCT user_id) AS cnt
		 FROM skills
		 GROUP BY name
		 ORDER BY cnt DESC
		 LIMIT 10`,
	); err != nil {
		slog.Error("failed to load top skills", "error", err)
	} else {
		data.TopSkills = scanNamedCounts(rows)
	}

	data.Engagement = EngagementTotals{
		Posts:       h.queryCount(r, `SELECT COUNT(*) FROM posts`),
		Comments:    h.queryCount(r, `SELECT COUNT(*) FROM comments`),
		Connections: h.queryCount(r, `SELECT COUNT(*) FROM connections WHERE status = 'accepted'`),
		Kudos:       h.queryCount(r, `SELECT COUNT(*) FROM kudos`),
	}

	h.pages["workforce_dashboard.html"].ExecuteTemplate(w, "base", data)
}

// showLeaderboard ranks users by an engagement score equal to
// posts + comments + kudos_received + accepted_connections, showing the top 25.
// An optional ?department=<id> filter restricts the ranking to one department.
func (h *Handler) showLeaderboard(w http.ResponseWriter, r *http.Request) {
	data := LeaderboardPage{
		BaseData:           middleware.NewBaseData(r),
		SelectedDepartment: r.URL.Query().Get("department"),
	}

	if rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, name FROM departments ORDER BY name`,
	); err != nil {
		slog.Error("failed to load departments", "error", err)
	} else {
		func() {
			defer rows.Close()
			for rows.Next() {
				var opt DepartmentOption
				if err := rows.Scan(&opt.ID, &opt.Name); err != nil {
					continue
				}
				data.DepartmentOptions = append(data.DepartmentOptions, opt)
			}
		}()
	}

	// Engagement score = posts + comments + kudos received + accepted connections.
	// Each component is pre-aggregated per user so a user with zero of one kind
	// still ranks correctly (LEFT JOIN keeps users with partial activity).
	query := `
		SELECT u.id,
		       u.first_name || ' ' || u.last_name AS name,
		       COALESCE(d.name, '') AS department,
		       (COALESCE(p.cnt, 0) + COALESCE(c.cnt, 0) + COALESCE(k.cnt, 0) + COALESCE(cn.cnt, 0)) AS score
		FROM users u
		LEFT JOIN departments d ON d.id = u.department_id
		LEFT JOIN (SELECT user_id, COUNT(*) cnt FROM posts GROUP BY user_id) p ON p.user_id = u.id
		LEFT JOIN (SELECT user_id, COUNT(*) cnt FROM comments GROUP BY user_id) c ON c.user_id = u.id
		LEFT JOIN (SELECT receiver_id, COUNT(*) cnt FROM kudos GROUP BY receiver_id) k ON k.receiver_id = u.id
		LEFT JOIN (
			SELECT uid, COUNT(*) cnt FROM (
				SELECT requester_id AS uid FROM connections WHERE status = 'accepted'
				UNION ALL
				SELECT addressee_id AS uid FROM connections WHERE status = 'accepted'
			) conns GROUP BY uid
		) cn ON cn.uid = u.id`

	var rows *sql.Rows
	var err error
	if data.SelectedDepartment != "" {
		query += `
		WHERE u.department_id = $1
		ORDER BY score DESC, name ASC
		LIMIT 25`
		rows, err = h.db.QueryContext(r.Context(), query, data.SelectedDepartment)
	} else {
		query += `
		ORDER BY score DESC, name ASC
		LIMIT 25`
		rows, err = h.db.QueryContext(r.Context(), query)
	}
	if err != nil {
		slog.Error("failed to load leaderboard", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	rank := 0
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(&e.UserID, &e.Name, &e.Department, &e.Score); err != nil {
			slog.Error("failed to scan leaderboard entry", "error", err)
			continue
		}
		rank++
		e.Rank = rank
		data.Entries = append(data.Entries, e)
	}

	h.pages["leaderboard.html"].ExecuteTemplate(w, "base", data)
}
