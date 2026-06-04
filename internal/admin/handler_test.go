package admin

import (
	"context"
	"database/sql"
	"html/template"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// TestWriteAuditInsertsRow verifies the audit helper writes an audit_log row
// with the actor, action, target, and details supplied by the caller.
func TestWriteAuditInsertsRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO audit_log`)).
		WithArgs("admin-1", "role_change", "user-9", "employee -> manager").
		WillReturnResult(sqlmock.NewResult(0, 1))

	h.writeAudit(context.Background(), "admin-1", "role_change", "user-9", "employee -> manager")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestWriteAuditNonFatalOnError verifies a DB failure during the audit insert
// does not panic (it is best-effort; the caller's action must not break).
func TestWriteAuditNonFatalOnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO audit_log`)).
		WillReturnError(context.DeadlineExceeded)

	// Must not panic.
	h.writeAudit(context.Background(), "admin-1", "user_delete", "user-9", "deleted X")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestShowDashboardRendersMetrics verifies the admin dashboard handler runs its
// metric queries and renders the headline numbers and recent-activity rows.
func TestShowDashboardRendersMetrics(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	tmpl := template.Must(template.ParseFiles(
		"../../templates/layouts/base.html",
		"../../templates/layouts/navbar.html",
		"../../templates/admin/dashboard.html",
	))
	pages := map[string]*template.Template{"admin_dashboard.html": tmpl}
	h := NewHandler(db, pages)

	now := time.Now()

	// Users by role.
	mock.ExpectQuery(`GROUP BY role`).
		WillReturnRows(sqlmock.NewRows([]string{"role", "count"}).
			AddRow("employee", 42).
			AddRow("manager", 7).
			AddRow("admin", 2))
	// Total departments.
	mock.ExpectQuery(`FROM departments`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(8))
	// Posts this week.
	mock.ExpectQuery(`FROM posts`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(13))
	// New connections this week.
	mock.ExpectQuery(`FROM connections`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
	// Active postings.
	mock.ExpectQuery(`FROM postings`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(4))
	// Pending applications.
	mock.ExpectQuery(`FROM posting_applications`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(9))
	// Recent audit entries.
	mock.ExpectQuery(`FROM audit_log`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "action", "details", "created_at", "actor", "target",
		}).AddRow("a1", "role_change", "employee -> manager", now, "Sys Admin", "Jane Doe"))
	// Recent signups.
	mock.ExpectQuery(`FROM users`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "first_name", "last_name", "role", "created_at",
		}).AddRow("u1", "newbie@usda.gov", "New", "Bie", "employee", now))

	req := httptest.NewRequest("GET", "/admin/dashboard", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.showDashboard(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"42",          // employee count
		"13",          // posts this week
		"role_change", // recent audit
		"New Bie",     // recent signup
		"newbie@usda.gov",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected dashboard to contain %q, got:\n%s", want, body)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestShowReportRendersStandalonePrintPage verifies the export report handler
// queries headcounts/skills/engagement and renders a standalone print page
// (no navbar/base chrome — its own "report_base" template).
func TestShowReportRendersStandalonePrintPage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	tmpl := template.Must(template.ParseFiles("../../templates/admin/report.html"))
	pages := map[string]*template.Template{"admin_report.html": tmpl}
	h := NewHandler(db, pages)

	// Headcounts by department.
	mock.ExpectQuery(`GROUP BY d.name`).
		WillReturnRows(sqlmock.NewRows([]string{"name", "count"}).
			AddRow("NRCS", 12).
			AddRow("Forest Service", 8))
	// Headcounts by role.
	mock.ExpectQuery(`GROUP BY role`).
		WillReturnRows(sqlmock.NewRows([]string{"role", "count"}).
			AddRow("employee", 18).
			AddRow("manager", 2))
	// Skills coverage.
	mock.ExpectQuery(`FROM skills`).
		WillReturnRows(sqlmock.NewRows([]string{"name", "count"}).
			AddRow("Go", 9).
			AddRow("SQL", 6))
	// Engagement summary (single aggregate row).
	mock.ExpectQuery(`engagement`).
		WillReturnRows(sqlmock.NewRows([]string{
			"total_users", "with_skills", "total_posts", "total_connections",
		}).AddRow(20, 11, 55, 30))

	req := httptest.NewRequest("GET", "/admin/report", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.showReport(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	// Print-friendly standalone page: must carry its own doctype + print CSS,
	// and must NOT pull in the app navbar.
	for _, want := range []string{
		"<!DOCTYPE html>",
		"@media print",
		"NRCS",
		"Go", // skill
		"Workforce Report",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected report page to contain %q, got:\n%s", want, body)
		}
	}
	if strings.Contains(body, `class="navbar"`) {
		t.Errorf("standalone print report should not render the app navbar, got:\n%s", body)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestShowAuditRendersEntries verifies the audit-list handler queries the log
// and renders each entry (actor name, action, details) in the page.
func TestShowAuditRendersEntries(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	tmpl := template.Must(template.ParseFiles(
		"../../templates/layouts/base.html",
		"../../templates/layouts/navbar.html",
		"../../templates/admin/audit.html",
	))
	pages := map[string]*template.Template{"admin_audit.html": tmpl}
	h := NewHandler(db, pages)

	now := time.Now()
	mock.ExpectQuery(`FROM audit_log`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "action", "details", "created_at", "actor", "target",
		}).AddRow("a1", "role_change", "employee -> manager", now, "Sys Admin", "Jane Doe"))

	req := httptest.NewRequest("GET", "/admin/audit", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.showAudit(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"role_change", "employee -&gt; manager", "Sys Admin", "Jane Doe"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected audit page to contain %q, got:\n%s", want, body)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestChangeRolePersistsAndAudits verifies role changes persist exactly once
// and write an audit row in the same transaction.
func TestChangeRolePersistsAndAudits(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT role FROM users WHERE id = $1`)).
		WithArgs("user-9").
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow("employee"))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE users SET role = $1 WHERE id = $2`)).
		WithArgs("manager", "user-9").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO audit_log (actor_id, action, target_id, details) VALUES ($1, $2, $3, $4)`)).
		WithArgs("admin-1", "role_change", "user-9", "employee -> manager").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	req := httptest.NewRequest(http.MethodPost, "/admin/users/user-9/role", strings.NewReader("role=manager"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "user-9")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.changeRole(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected %d, got %d", http.StatusSeeOther, rec.Code)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "flash_kind=success") || !strings.Contains(location, "Role+updated+successfully") {
		t.Fatalf("unexpected redirect location: %s", location)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestChangeDepartmentPersistsAndAudits verifies department changes persist
// and produce a department_change audit row.
func TestChangeDepartmentPersistsAndAudits(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT u.department_id::text, d.name
		 FROM users u
		 LEFT JOIN departments d on d.id = u.department_id
		 WHERE u.id = $1`)).
		WithArgs("user-9").
		WillReturnRows(sqlmock.NewRows([]string{"department_id", "name"}).AddRow("dept-old", "Operations"))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT name FROM departments WHERE id = $1`)).
		WithArgs("dept-new").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Engineering"))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE users SET department_id = $1 WHERE id = $2`)).
		WithArgs("dept-new", "user-9").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO audit_log (actor_id, action, target_id, details) VALUES ($1, $2, $3, $4)`)).
		WithArgs("admin-1", "department_change", "user-9", "Operations -> Engineering").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	req := httptest.NewRequest(http.MethodPost, "/admin/users/user-9/department", strings.NewReader("department_id=dept-new"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "user-9")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.changeDepartment(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected %d, got %d", http.StatusSeeOther, rec.Code)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "flash_kind=success") || !strings.Contains(location, "Department+updated+successfully") {
		t.Fatalf("unexpected redirect location: %s", location)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestDeleteUserPersistsAndAudits verifies deleting a user removes exactly one
// row and writes a matching audit entry before commit.
func TestDeleteUserPersistsAndAudits(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`DELETE FROM users WHERE id = $1 RETURNING
			COALESCE(CONCAT(first_name, ' ', last_name), 'unknown'),
			COALESCE(email, '')`)).
		WithArgs("user-9").
		WillReturnRows(sqlmock.NewRows([]string{"name", "email"}).AddRow("Jane Doe", "jane@usda.gov"))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO audit_log (actor_id, action, target_id, details) VALUES ($1, $2, $3, $4)`)).
		WithArgs("admin-1", "user_delete", "user-9", "deleted Jane Doe (jane@usda.gov)").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	req := httptest.NewRequest(http.MethodPost, "/admin/users/user-9/delete", nil)
	req.SetPathValue("id", "user-9")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.deleteUser(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected %d, got %d", http.StatusSeeOther, rec.Code)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "flash_kind=success") || !strings.Contains(location, "User+deleted+successfully") {
		t.Fatalf("unexpected redirect location: %s", location)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestDeleteUserNoRowsDoesNotAudit verifies that when no row is deleted,
// the handler redirects with an error and does not write an audit row.
func TestDeleteUserNoRowsDoesNotAudit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`DELETE FROM users WHERE id = $1 RETURNING
			COALESCE(CONCAT(first_name, ' ', last_name), 'unknown'),
			COALESCE(email, '')`)).
		WithArgs("missing-user").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	req := httptest.NewRequest(http.MethodPost, "/admin/users/missing-user/delete", nil)
	req.SetPathValue("id", "missing-user")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.deleteUser(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected %d, got %d", http.StatusSeeOther, rec.Code)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "flash_kind=error") || !strings.Contains(location, "User+not+found") {
		t.Fatalf("unexpected redirect location: %s", location)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestShowDashboardAuditFallbacks verifies the dashboard audit query uses
// resilient SQL fallbacks so missing actor names and blank details do not
// render as visually empty rows.
func TestShowDashboardAuditFallbacks(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	tmpl := template.Must(template.ParseFiles(
		"../../templates/layouts/base.html",
		"../../templates/layouts/navbar.html",
		"../../templates/admin/dashboard.html",
	))
	pages := map[string]*template.Template{"admin_dashboard.html": tmpl}
	h := NewHandler(db, pages)

	now := time.Now()

	mock.ExpectQuery(`GROUP BY role`).
		WillReturnRows(sqlmock.NewRows([]string{"role", "count"}).AddRow("admin", 1))
	mock.ExpectQuery(`FROM departments`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`FROM posts`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`FROM connections`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`FROM postings`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`FROM posting_applications`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT a.id, a.action,
			COALESCE(NULLIF(TRIM(a.details), ''), '(no details)'), a.created_at,
			COALESCE(NULLIF(TRIM(CONCAT(actor.first_name, ' ', actor.last_name)), ''), 'System'),
			COALESCE(NULLIF(TRIM(CONCAT(target.first_name, ' ', target.last_name)), ''), 'Unknown user')
		 FROM audit_log a
		 LEFT JOIN users actor ON actor.id = a.actor_id
		 LEFT JOIN users target ON target.id = a.target_id
		 ORDER BY a.created_at DESC
		 LIMIT 10`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "action", "details", "created_at", "actor", "target",
		}).AddRow("a1", "user_delete", "(no details)", now, "System", "Unknown user"))

	mock.ExpectQuery(`FROM users`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "first_name", "last_name", "role", "created_at",
		}).AddRow("u1", "admin@usda.gov", "Sys", "Admin", "admin", now))

	req := httptest.NewRequest("GET", "/admin/dashboard", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.showDashboard(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"Recent Audit Activity", "System", "(no details)"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected dashboard to contain %q, got:\n%s", want, body)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
