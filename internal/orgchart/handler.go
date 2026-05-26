package orgchart

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// maxDepth caps how deep the rendered tree may go. It protects against
// pathological data (deep chains or cycles that survive the seen-guard) and
// keeps rendering bounded for ~100 users.
const maxDepth = 50

// Node is a single user in the reporting hierarchy. Children are populated by
// buildTree from the flat list of users.
type Node struct {
	ID         string
	Name       string
	Headline   string
	Department string
	ManagerID  string
	Children   []*Node
}

// SelectUser is a lightweight user option for the assignment <select>s.
type SelectUser struct {
	ID   string
	Name string
}

type ChartPage struct {
	middleware.BaseData
	Roots []*Node
}

type AssignPage struct {
	middleware.BaseData
	Users []SelectUser
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
	mux.Handle("GET /orgchart", requireAuth(http.HandlerFunc(h.showChart)))
	mux.Handle("GET /admin/orgchart", requireAuth(requireAdmin(http.HandlerFunc(h.showAssign))))
	mux.Handle("POST /admin/orgchart", requireAuth(requireAdmin(http.HandlerFunc(h.handleAssign))))
}

// buildTree converts a flat slice of Node into a forest. A user becomes a child
// of the node referenced by its ManagerID. Users with no manager (or a manager
// that does not exist) are roots. Cycles are handled by the bounded depth used
// at render time; here we additionally promote any node that is not reachable
// from a real root to a root so cyclic members are never silently dropped.
func buildTree(nodes []Node) []*Node {
	byID := make(map[string]*Node, len(nodes))
	for i := range nodes {
		n := nodes[i]
		n.Children = nil
		byID[n.ID] = &n
	}

	var roots []*Node
	for _, n := range byID {
		mgr, ok := byID[n.ManagerID]
		if n.ManagerID == "" || !ok || mgr == n {
			roots = append(roots, n)
			continue
		}
		mgr.Children = append(mgr.Children, n)
	}

	// Detect nodes trapped in a pure cycle (e.g. A->B->A) that are therefore
	// not reachable from any root, and promote them so they remain visible.
	reachable := make(map[string]bool, len(byID))
	var mark func(n *Node, depth int)
	mark = func(n *Node, depth int) {
		if n == nil || reachable[n.ID] || depth > maxDepth {
			return
		}
		reachable[n.ID] = true
		for _, c := range n.Children {
			mark(c, depth+1)
		}
	}
	for _, r := range roots {
		mark(r, 0)
	}
	for _, n := range byID {
		if !reachable[n.ID] {
			roots = append(roots, n)
			mark(n, 0)
		}
	}

	return roots
}

func (h *Handler) showChart(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT u.id,
		        u.first_name || ' ' || u.last_name AS name,
		        COALESCE(u.headline, '') AS headline,
		        COALESCE(d.name, '') AS department,
		        COALESCE(u.manager_id::text, '') AS manager_id
		 FROM users u
		 LEFT JOIN departments d ON d.id = u.department_id
		 ORDER BY u.first_name, u.last_name`,
	)
	if err != nil {
		slog.Error("failed to load org chart users", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Name, &n.Headline, &n.Department, &n.ManagerID); err != nil {
			slog.Error("failed to scan org chart user", "error", err)
			continue
		}
		nodes = append(nodes, n)
	}

	data := ChartPage{
		BaseData: middleware.NewBaseData(r),
		Roots:    buildTree(nodes),
	}

	if h.pages == nil {
		return
	}
	h.pages["orgchart.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) showAssign(w http.ResponseWriter, r *http.Request) {
	data := AssignPage{
		BaseData: middleware.NewBaseData(r),
		Users:    h.loadUsers(r),
	}

	if h.pages == nil {
		return
	}
	h.pages["orgchart_assign.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) handleAssign(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	employeeID := r.FormValue("employee_id")
	managerID := r.FormValue("manager_id")

	if employeeID == "" {
		http.Error(w, "An employee must be selected", http.StatusBadRequest)
		return
	}
	if employeeID == managerID {
		http.Error(w, "A user cannot be their own manager", http.StatusBadRequest)
		return
	}

	// An empty manager clears the relationship; a non-empty one sets it.
	var mgr interface{}
	if managerID != "" {
		mgr = managerID
	}

	_, err := h.db.ExecContext(r.Context(),
		`UPDATE users SET manager_id = $1 WHERE id = $2`,
		mgr, employeeID,
	)
	if err != nil {
		slog.Error("failed to set manager", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/orgchart", http.StatusSeeOther)
}

func (h *Handler) loadUsers(r *http.Request) []SelectUser {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, first_name || ' ' || last_name AS name
		 FROM users
		 ORDER BY first_name, last_name`,
	)
	if err != nil {
		slog.Error("failed to load users for assignment", "error", err)
		return nil
	}
	defer rows.Close()

	var users []SelectUser
	for rows.Next() {
		var u SelectUser
		if err := rows.Scan(&u.ID, &u.Name); err != nil {
			continue
		}
		users = append(users, u)
	}
	return users
}
