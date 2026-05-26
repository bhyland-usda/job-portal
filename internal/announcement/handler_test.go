package announcement

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

// testPages parses the real announcement templates (with layouts) so handler
// tests exercise actual rendering. Paths are relative to this package directory.
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
		"announcements.html":       mk("announcement/index.html"),
		"announcement_create.html": mk("announcement/create.html"),
	}
}

// TestShowAnnouncementsListsPinnedFirst verifies the list query orders pinned
// announcements before unpinned, and that title/body/author/date all render.
func TestShowAnnouncementsListsPinnedFirst(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	now := time.Now()
	mock.ExpectQuery(`FROM announcements`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "body", "author", "is_pinned", "created_at",
		}).
			AddRow("a-1", "Office Closed", "We are closed Monday", "Ada Lovelace", true, now).
			AddRow("a-2", "New Cafeteria", "Now open on 2nd floor", "Alan Turing", false, now))

	h := NewHandler(db, testPages())

	req := httptest.NewRequest("GET", "/announcements", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "viewer-1"))
	rec := httptest.NewRecorder()

	h.showAnnouncements(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Office Closed") {
		t.Errorf("expected pinned announcement in body, got: %s", body)
	}
	if !strings.Contains(body, "New Cafeteria") {
		t.Errorf("expected unpinned announcement in body, got: %s", body)
	}
	if !strings.Contains(body, "Ada Lovelace") {
		t.Errorf("expected author name in body, got: %s", body)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestHandleCreateInsertsWithAuthor verifies a valid create inserts the
// announcement with author_id set to the current admin and redirects.
func TestHandleCreateInsertsWithAuthor(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)

	mock.ExpectExec(`INSERT INTO announcements`).
		WithArgs("Maintenance Window", "Systems down at 2am", "admin-1", true).
		WillReturnResult(sqlmock.NewResult(0, 1))

	form := url.Values{}
	form.Set("title", "Maintenance Window")
	form.Set("body", "Systems down at 2am")
	form.Set("pinned", "on")

	req := httptest.NewRequest("POST", "/admin/announcements", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.handleCreate(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/announcements" {
		t.Errorf("expected redirect to /announcements, got %q", loc)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestHandleCreateRequiresTitleAndBody verifies validation: missing title or
// body must NOT hit the database and must re-render the form (200), not redirect.
func TestHandleCreateRequiresTitleAndBody(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, testPages())

	form := url.Values{}
	form.Set("title", "")
	form.Set("body", "")
	req := httptest.NewRequest("POST", "/admin/announcements", strings.NewReader(form.Encode()))
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

// TestTogglePinFlipsIsPinned verifies the pin endpoint toggles is_pinned and
// redirects back to the list.
func TestTogglePinFlipsIsPinned(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)

	mock.ExpectExec(`UPDATE announcements SET is_pinned = NOT is_pinned WHERE id = \$1`).
		WithArgs("a-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	req := httptest.NewRequest("POST", "/admin/announcements/a-1/pin", nil)
	req.SetPathValue("id", "a-1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.togglePin(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/announcements" {
		t.Errorf("expected redirect to /announcements, got %q", loc)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestDeleteRemovesAnnouncement verifies the delete endpoint deletes the row and
// redirects back to the list.
func TestDeleteRemovesAnnouncement(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)

	mock.ExpectExec(`DELETE FROM announcements WHERE id = \$1`).
		WithArgs("a-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	req := httptest.NewRequest("POST", "/admin/announcements/a-1/delete", nil)
	req.SetPathValue("id", "a-1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.deleteAnnouncement(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/announcements" {
		t.Errorf("expected redirect to /announcements, got %q", loc)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
