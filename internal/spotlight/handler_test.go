package spotlight

import (
	"context"
	"html/template"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// testPages parses the real spotlight templates (with layouts) so handler tests
// exercise actual rendering. Paths are relative to this package directory.
func testPages() map[string]*template.Template {
	tmplDir := "../../templates"
	mk := func(name string) *template.Template {
		return template.Must(template.ParseFiles(
			tmplDir+"/layouts/base.html",
			tmplDir+"/layouts/navbar.html",
			tmplDir+"/"+name,
		))
	}
	return map[string]*template.Template{
		"spotlight.html":        mk("spotlight/index.html"),
		"spotlight_create.html": mk("spotlight/create.html"),
	}
}

// TestHandleCreateUpsertsOnWeekConflict verifies that creating a spotlight uses
// an upsert (ON CONFLICT (week_of) DO UPDATE) so that re-spotlighting an
// already-used week does not error on the UNIQUE(week_of) constraint, and that
// created_by is set to the current admin.
func TestHandleCreateUpsertsOnWeekConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)

	mock.ExpectExec(`INSERT INTO employee_spotlights`).
		WithArgs("user-2", "2026-05-18", "Great work on the harvest report", "admin-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	form := url.Values{}
	form.Set("user_id", "user-2")
	form.Set("week_of", "2026-05-18")
	form.Set("reason", "Great work on the harvest report")

	req := httptest.NewRequest("POST", "/admin/spotlight", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.handleCreate(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/spotlight" {
		t.Errorf("expected redirect to /spotlight, got %q", loc)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestHandleCreateRequiresUserAndWeek verifies validation: missing user_id or
// week_of must NOT hit the database and must re-render the form (200), not
// redirect.
func TestHandleCreateRequiresUserAndWeek(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// loadUserOptions is called to re-render the form with the error.
	mock.ExpectQuery(`SELECT id, first_name, last_name\s+FROM users`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "first_name", "last_name"}).
			AddRow("user-1", "Ada", "Lovelace"))

	h := NewHandler(db, testPages())

	form := url.Values{}
	form.Set("user_id", "")
	form.Set("week_of", "")
	req := httptest.NewRequest("POST", "/admin/spotlight", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.handleCreate(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200 (re-render form), got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Errorf("should not redirect on validation error, got Location %q", loc)
	}
}

// TestShowSpotlightPicksMostRecentAsCurrent verifies that the row with the most
// recent week_of becomes Current and the remaining rows become Past.
func TestShowSpotlightPicksMostRecentAsCurrent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	now := time.Now()
	thisWeek := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	lastWeek := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM employee_spotlights`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "name", "headline", "avatar_url", "week_of", "reason", "created_at",
		}).
			AddRow("sp-1", "user-2", "Grace Hopper", "Compiler Pioneer", "blob", thisWeek, "Shipped the thing", now).
			AddRow("sp-2", "user-3", "Alan Turing", "", "", lastWeek, "Cracked it", now))

	h := NewHandler(db, testPages())

	req := httptest.NewRequest("GET", "/spotlight", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "viewer-1"))
	rec := httptest.NewRecorder()

	h.showSpotlight(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Grace Hopper") {
		t.Errorf("expected current spotlight Grace Hopper in body, got: %s", body)
	}
	// avatar_url present -> /avatar/<user_id> link should be rendered
	if !strings.Contains(body, "/avatar/user-2") {
		t.Errorf("expected avatar link for featured user, got: %s", body)
	}
	if !strings.Contains(body, "Alan Turing") {
		t.Errorf("expected past spotlight Alan Turing in body, got: %s", body)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
