package admin

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type User struct {
	ID             string
	Email          string
	FirstName      string
	LastName       string
	Role           string
	DepartmentID   string
	DepartmentName string
}

type Department struct {
	ID   string
	Name string
}

type AdminPage struct {
	middleware.BaseData
	Users       []User
	Departments []Department
}

type AuditEntry struct {
	ID         string
	ActorName  string
	Action     string
	TargetName string
	Details    string
	CreatedAt  time.Time
}

type AuditPage struct {
	middleware.BaseData
	Entries []AuditEntry
}

// RoleCount is a (role, count) pair used by the dashboard and report.
type RoleCount struct {
	Role  string
	Count int
}

// NameCount is a (name, count) pair used for department and skill breakdowns.
type NameCount struct {
	Name  string
	Count int
}

// Signup is a recently-registered user shown on the dashboard.
type Signup struct {
	ID        string
	Email     string
	FirstName string
	LastName  string
	Role      string
	CreatedAt time.Time
}

// DashboardPage backs templates/admin/dashboard.html.
type DashboardPage struct {
	middleware.BaseData
	TotalUsers             int
	UsersByRole            []RoleCount
	TotalDepartments       int
	PostsThisWeek          int
	NewConnectionsThisWeek int
	ActivePostings         int
	PendingApplications    int
	RecentSignups          []Signup
	RecentAudit            []AuditEntry
}

// ReportPage backs templates/admin/report.html. It is a standalone print page
// and therefore does not embed middleware.BaseData (no app chrome).
type ReportPage struct {
	GeneratedAt      time.Time
	HeadcountsByDept []NameCount
	HeadcountsByRole []RoleCount
	SkillsCoverage   []NameCount
	TotalUsers       int
	UsersWithSkills  int
	TotalPosts       int
	TotalConnections int
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth, requireAdmin func(http.Handler) http.Handler) {
	mux.Handle("GET /admin/users", requireAuth(requireAdmin(http.HandlerFunc(h.showUsers))))
	mux.Handle("POST /admin/users/{id}/role", requireAuth(requireAdmin(http.HandlerFunc(h.changeRole))))
	mux.Handle("POST /admin/users/{id}/department", requireAuth(requireAdmin(http.HandlerFunc(h.changeDepartment))))
	mux.Handle("POST /admin/users/{id}/delete", requireAuth(requireAdmin(http.HandlerFunc(h.deleteUser))))
	mux.Handle("GET /admin/audit", requireAuth(requireAdmin(http.HandlerFunc(h.showAudit))))
	mux.Handle("GET /admin/dashboard", requireAuth(requireAdmin(http.HandlerFunc(h.showDashboard))))
	mux.Handle("GET /admin/report", requireAuth(requireAdmin(http.HandlerFunc(h.showReport))))
	mux.Handle("GET /me/role", requireAuth(http.HandlerFunc(h.getRole)))
}

// showDashboard renders the admin dashboard with headline workforce metrics
// and recent-activity panels (signups + audit log).
func (h *Handler) showDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	data := DashboardPage{
		BaseData: middleware.NewBaseData(r),
	}

	// Users by role (and derived total).
	roleRows, err := h.db.QueryContext(ctx,
		`SELECT role, COUNT(*) FROM users GROUP BY role ORDER BY COUNT(*) DESC`)
	if err != nil {
		slog.Error("dashboard: failed to load users by role", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	for roleRows.Next() {
		var rc RoleCount
		if err := roleRows.Scan(&rc.Role, &rc.Count); err != nil {
			slog.Error("dashboard: failed to scan role count", "error", err)
			continue
		}
		data.UsersByRole = append(data.UsersByRole, rc)
		data.TotalUsers += rc.Count
	}
	roleRows.Close()

	// Total departments.
	if err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM departments`).Scan(&data.TotalDepartments); err != nil {
		slog.Error("dashboard: failed to count departments", "error", err)
	}

	// Posts created in the last 7 days.
	if err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM posts WHERE created_at >= NOW() - INTERVAL '7 days'`).
		Scan(&data.PostsThisWeek); err != nil {
		slog.Error("dashboard: failed to count posts this week", "error", err)
	}

	// Connections accepted in the last 7 days.
	if err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM connections
		 WHERE status = 'accepted' AND created_at >= NOW() - INTERVAL '7 days'`).
		Scan(&data.NewConnectionsThisWeek); err != nil {
		slog.Error("dashboard: failed to count new connections", "error", err)
	}

	// Active postings.
	if err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM postings WHERE status = 'active'`).
		Scan(&data.ActivePostings); err != nil {
		slog.Error("dashboard: failed to count active postings", "error", err)
	}

	// Pending applications.
	if err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM posting_applications WHERE status = 'pending'`).
		Scan(&data.PendingApplications); err != nil {
		slog.Error("dashboard: failed to count pending applications", "error", err)
	}

	// Recent audit entries.
	auditRows, err := h.db.QueryContext(ctx,
		`SELECT a.id, a.action, COALESCE(a.details, ''), a.created_at,
			COALESCE(CONCAT(actor.first_name, ' ', actor.last_name), 'System'),
			COALESCE(CONCAT(target.first_name, ' ', target.last_name), '')
		 FROM audit_log a
		 LEFT JOIN users actor ON actor.id = a.actor_id
		 LEFT JOIN users target ON target.id = a.target_id
		 ORDER BY a.created_at DESC
		 LIMIT 10`)
	if err != nil {
		slog.Error("dashboard: failed to load audit log", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	for auditRows.Next() {
		var e AuditEntry
		if err := auditRows.Scan(&e.ID, &e.Action, &e.Details, &e.CreatedAt,
			&e.ActorName, &e.TargetName); err != nil {
			slog.Error("dashboard: failed to scan audit entry", "error", err)
			continue
		}
		data.RecentAudit = append(data.RecentAudit, e)
	}
	auditRows.Close()

	// Recent signups.
	signupRows, err := h.db.QueryContext(ctx,
		`SELECT id, email, first_name, last_name, role, created_at
		 FROM users
		 ORDER BY created_at DESC
		 LIMIT 10`)
	if err != nil {
		slog.Error("dashboard: failed to load recent signups", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	for signupRows.Next() {
		var s Signup
		if err := signupRows.Scan(&s.ID, &s.Email, &s.FirstName, &s.LastName,
			&s.Role, &s.CreatedAt); err != nil {
			slog.Error("dashboard: failed to scan signup", "error", err)
			continue
		}
		data.RecentSignups = append(data.RecentSignups, s)
	}
	signupRows.Close()

	if err := h.pages["admin_dashboard.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("dashboard: failed to render", "error", err)
	}
}

// showReport renders a standalone, print-friendly workforce report. It does not
// use the shared site chrome; it renders its own top-level template so it can be
// printed or exported to PDF directly from the browser.
func (h *Handler) showReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	data := ReportPage{
		GeneratedAt: time.Now(),
	}

	// Headcounts by department.
	deptRows, err := h.db.QueryContext(ctx,
		`SELECT d.name, COUNT(u.id)
		 FROM departments d
		 LEFT JOIN users u ON u.department_id = d.id
		 GROUP BY d.name
		 ORDER BY COUNT(u.id) DESC, d.name`)
	if err != nil {
		slog.Error("report: failed to load dept headcounts", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	for deptRows.Next() {
		var nc NameCount
		if err := deptRows.Scan(&nc.Name, &nc.Count); err != nil {
			slog.Error("report: failed to scan dept headcount", "error", err)
			continue
		}
		data.HeadcountsByDept = append(data.HeadcountsByDept, nc)
	}
	deptRows.Close()

	// Headcounts by role.
	roleRows, err := h.db.QueryContext(ctx,
		`SELECT role, COUNT(*) FROM users GROUP BY role ORDER BY COUNT(*) DESC`)
	if err != nil {
		slog.Error("report: failed to load role headcounts", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	for roleRows.Next() {
		var rc RoleCount
		if err := roleRows.Scan(&rc.Role, &rc.Count); err != nil {
			slog.Error("report: failed to scan role headcount", "error", err)
			continue
		}
		data.HeadcountsByRole = append(data.HeadcountsByRole, rc)
	}
	roleRows.Close()

	// Skills coverage: how many users hold each skill, most common first.
	skillRows, err := h.db.QueryContext(ctx,
		`SELECT name, COUNT(DISTINCT user_id) AS cnt
		 FROM skills
		 GROUP BY name
		 ORDER BY cnt DESC, name
		 LIMIT 25`)
	if err != nil {
		slog.Error("report: failed to load skills coverage", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	for skillRows.Next() {
		var nc NameCount
		if err := skillRows.Scan(&nc.Name, &nc.Count); err != nil {
			slog.Error("report: failed to scan skill coverage", "error", err)
			continue
		}
		data.SkillsCoverage = append(data.SkillsCoverage, nc)
	}
	skillRows.Close()

	// Engagement summary (single aggregate row).
	if err := h.db.QueryRowContext(ctx,
		`SELECT
			(SELECT COUNT(*) FROM users) AS total_users,
			(SELECT COUNT(DISTINCT user_id) FROM skills) AS with_skills,
			(SELECT COUNT(*) FROM posts) AS total_posts,
			(SELECT COUNT(*) FROM connections WHERE status = 'accepted') AS total_connections
		 /* engagement summary */`).
		Scan(&data.TotalUsers, &data.UsersWithSkills, &data.TotalPosts, &data.TotalConnections); err != nil {
		slog.Error("report: failed to load engagement summary", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.pages["admin_report.html"].ExecuteTemplate(w, "report_base", data); err != nil {
		slog.Error("report: failed to render", "error", err)
	}
}

// writeAudit records an administrative action in the audit_log table. It is
// best-effort: failures are logged but never break the originating action.
func (h *Handler) writeAudit(ctx context.Context, actorID, action, targetID, details string) {
	var actor interface{} = actorID
	if actorID == "" {
		actor = nil
	}
	var target interface{} = targetID
	if targetID == "" {
		target = nil
	}
	_, err := h.db.ExecContext(ctx,
		`INSERT INTO audit_log (actor_id, action, target_id, details)
		 VALUES ($1, $2, $3, $4)`,
		actor, action, target, details,
	)
	if err != nil {
		slog.Error("failed to write audit log", "action", action, "error", err)
	}
}

func (h *Handler) showUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT u.id, u.email, u.first_name, u.last_name, u.role,
			COALESCE(u.department_id::text, ''),
			COALESCE(d.name, '')
		 FROM users u
		 LEFT JOIN departments d ON d.id = u.department_id
		 ORDER BY u.last_name, u.first_name`,
	)
	if err != nil {
		slog.Error("failed to load users", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName,
			&user.Role, &user.DepartmentID, &user.DepartmentName); err != nil {
			slog.Error("failed to scan user", "error", err)
			continue
		}
		users = append(users, user)
	}

	// Load departments for dropdown
	var departments []Department
	deptRows, err := h.db.QueryContext(r.Context(),
		`SELECT id, name FROM departments ORDER BY name`,
	)
	if err == nil {
		defer deptRows.Close()
		for deptRows.Next() {
			var d Department
			if err := deptRows.Scan(&d.ID, &d.Name); err == nil {
				departments = append(departments, d)
			}
		}
	}

	data := AdminPage{
		BaseData:    middleware.NewBaseData(r),
		Users:       users,
		Departments: departments,
	}

	h.pages["admin_users.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) showAudit(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT a.id, a.action, COALESCE(a.details, ''), a.created_at,
			COALESCE(CONCAT(actor.first_name, ' ', actor.last_name), 'System'),
			COALESCE(CONCAT(target.first_name, ' ', target.last_name), '')
		 FROM audit_log a
		 LEFT JOIN users actor ON actor.id = a.actor_id
		 LEFT JOIN users target ON target.id = a.target_id
		 ORDER BY a.created_at DESC
		 LIMIT 200`,
	)
	if err != nil {
		slog.Error("failed to load audit log", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Action, &e.Details, &e.CreatedAt,
			&e.ActorName, &e.TargetName); err != nil {
			slog.Error("failed to scan audit entry", "error", err)
			continue
		}
		entries = append(entries, e)
	}

	data := AuditPage{
		BaseData: middleware.NewBaseData(r),
		Entries:  entries,
	}

	h.pages["admin_audit.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) getRole(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var firstName, lastName, role string
	var avatarURL sql.NullString
	err := h.db.QueryRowContext(r.Context(),
		`SELECT first_name, last_name, role, avatar_url FROM users WHERE id = $1`,
		userID,
	).Scan(&firstName, &lastName, &role, &avatarURL)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Support both /me/role (text) and /me/info (json)
	if r.URL.Path == "/me/role" {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(role))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	avatar := ""
	if avatarURL.Valid && avatarURL.String != "" {
		avatar = "/avatar/" + userID
	}
	fmt.Fprintf(w, `{"id":"%s","name":"%s %s","initials":"%s%s","role":"%s","avatar":"%s"}`,
		userID, firstName, lastName,
		string(firstName[0]), string(lastName[0]),
		role, avatar)
}

func (h *Handler) changeRole(w http.ResponseWriter, r *http.Request) {
	adminID := middleware.GetUserID(r.Context())
	targetID := r.PathValue("id")

	if adminID == targetID {
		http.Error(w, "Cannot change your own role", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	newRole := r.FormValue("role")
	validRoles := map[string]bool{
		"employee": true,
		"manager":  true,
		"admin":    true,
	}

	if !validRoles[newRole] {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	// Capture the previous role so the audit detail reads "old -> new".
	var oldRole string
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT role FROM users WHERE id = $1`, targetID,
	).Scan(&oldRole); err != nil {
		oldRole = "unknown"
	}

	_, err := h.db.ExecContext(r.Context(),
		`UPDATE users SET role = $1 WHERE id = $2`,
		newRole, targetID,
	)
	if err != nil {
		slog.Error("failed to change role", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	slog.Info("role changed", "admin", adminID, "target", targetID, "role", newRole)
	h.writeAudit(r.Context(), adminID, "role_change", targetID,
		fmt.Sprintf("%s -> %s", oldRole, newRole))

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (h *Handler) changeDepartment(w http.ResponseWriter, r *http.Request) {
	adminID := middleware.GetUserID(r.Context())
	targetID := r.PathValue("id")

	r.ParseForm()
	deptID := r.FormValue("department_id")

	auditDetails := "removed from department"
	if deptID == "" {
		// Remove from department
		_, err := h.db.ExecContext(r.Context(),
			`UPDATE users SET department_id = NULL WHERE id = $1`,
			targetID,
		)
		if err != nil {
			slog.Error("failed to remove department", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	} else {
		_, err := h.db.ExecContext(r.Context(),
			`UPDATE users SET department_id = $1 WHERE id = $2`,
			deptID, targetID,
		)
		if err != nil {
			slog.Error("failed to change department", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		var deptName string
		if err := h.db.QueryRowContext(r.Context(),
			`SELECT name FROM departments WHERE id = $1`, deptID,
		).Scan(&deptName); err != nil {
			deptName = deptID
		}
		auditDetails = "assigned to " + deptName
	}

	slog.Info("department changed", "admin", adminID, "target", targetID, "department", deptID)
	h.writeAudit(r.Context(), adminID, "department_change", targetID, auditDetails)
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	adminID := middleware.GetUserID(r.Context())
	targetID := r.PathValue("id")

	if adminID == targetID {
		http.Error(w, "Cannot delete your own account", http.StatusBadRequest)
		return
	}

	// Capture identity before the row is gone so the audit detail is meaningful.
	var deletedName, deletedEmail string
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT CONCAT(first_name, ' ', last_name), email FROM users WHERE id = $1`,
		targetID,
	).Scan(&deletedName, &deletedEmail); err != nil {
		deletedName = "unknown"
	}

	_, err := h.db.ExecContext(r.Context(),
		`DELETE FROM users WHERE id = $1`,
		targetID,
	)
	if err != nil {
		slog.Error("failed to delete user", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	slog.Info("user deleted", "admin", adminID, "target", targetID)
	h.writeAudit(r.Context(), adminID, "user_delete", targetID,
		fmt.Sprintf("deleted %s (%s)", deletedName, deletedEmail))
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}
