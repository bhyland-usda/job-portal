package department

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type Department struct {
	ID            string
	Name          string
	Description   string
	ParentID      string
	ParentName    string
	EmployeeCount int
}

type Employee struct {
	ID        string
	FirstName string
	LastName  string
	Headline  string
	AvatarURL string
}

type DeptPage struct {
	middleware.BaseData
	Departments []Department
}

type DeptDetailPage struct {
	middleware.BaseData
	Department Department
	Employees  []Employee
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /departments", requireAuth(http.HandlerFunc(h.showDepartments)))
	mux.Handle("GET /departments/{id}", requireAuth(http.HandlerFunc(h.showDepartment)))
}

func (h *Handler) showDepartments(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT d.id, d.name, COALESCE(d.description, ''),
			d.parent_id, COALESCE(p.name, ''),
			COUNT(u.id)
		 FROM departments d
		 LEFT JOIN departments p ON p.id = d.parent_id
		 LEFT JOIN users u ON u.department_id = d.id
		 GROUP BY d.id, d.name, d.description, d.parent_id, p.name
		 ORDER BY d.name`)
	if err != nil {
		slog.Error("failed to load departments", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var departments []Department
	for rows.Next() {
		var d Department
		var parentID sql.NullString
		if err := rows.Scan(&d.ID, &d.Name, &d.Description,
			&parentID, &d.ParentName, &d.EmployeeCount); err != nil {
			slog.Error("failed to scan department", "error", err)
			continue
		}
		if parentID.Valid {
			d.ParentID = parentID.String
		}
		departments = append(departments, d)
	}
	if err := rows.Err(); err != nil {
		slog.Error("failed to iterate departments", "error", err)
	}

	data := DeptPage{
		BaseData:    middleware.NewBaseData(r),
		Departments: departments,
	}

	if err := h.pages["departments.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render departments", "error", err)
	}
}

func (h *Handler) showDepartment(w http.ResponseWriter, r *http.Request) {
	deptID := r.PathValue("id")

	var d Department
	var parentID sql.NullString
	err := h.db.QueryRowContext(r.Context(),
		`SELECT d.id, d.name, COALESCE(d.description, ''),
			d.parent_id, COALESCE(p.name, '')
		 FROM departments d
		 LEFT JOIN departments p ON p.id = d.parent_id
		 WHERE d.id = $1`, deptID,
	).Scan(&d.ID, &d.Name, &d.Description, &parentID, &d.ParentName)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if parentID.Valid {
		d.ParentID = parentID.String
	}

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, first_name, last_name, COALESCE(headline, ''), COALESCE(avatar_url, '')
		 FROM users
		 WHERE department_id = $1
		 ORDER BY last_name, first_name`, deptID,
	)
	if err != nil {
		slog.Error("failed to load department employees", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		var e Employee
		if err := rows.Scan(&e.ID, &e.FirstName, &e.LastName, &e.Headline, &e.AvatarURL); err != nil {
			slog.Error("failed to scan employee", "error", err)
			continue
		}
		e.AvatarURL = middleware.NormalizeAvatarURL(e.ID, e.AvatarURL)
		employees = append(employees, e)
	}
	if err := rows.Err(); err != nil {
		slog.Error("failed to iterate employees", "error", err)
	}

	data := DeptDetailPage{
		BaseData:   middleware.NewBaseData(r),
		Department: d,
		Employees:  employees,
	}

	if err := h.pages["department_view.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render department detail", "error", err)
	}
}
