package admin

import (
	"context"
	"html/template"
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
