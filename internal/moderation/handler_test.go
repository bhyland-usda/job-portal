package moderation

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

// testPages parses the real moderation template (with layouts) so handler tests
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
		"moderation_queue.html": mk("moderation/queue.html"),
	}
}

// TestCreateReportInsertsAndRedirects verifies a valid report inserts a row with
// reporter_id, content_type, content_id, reason and redirects to /feed.
func TestCreateReportInsertsAndRedirects(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)

	mock.ExpectExec(`INSERT INTO content_reports`).
		WithArgs("reporter-1", "post", "post-123", "Contains an SSN").
		WillReturnResult(sqlmock.NewResult(0, 1))

	form := url.Values{}
	form.Set("content_type", "post")
	form.Set("content_id", "post-123")
	form.Set("reason", "Contains an SSN")

	req := httptest.NewRequest("POST", "/report", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "reporter-1"))
	rec := httptest.NewRecorder()

	h.createReport(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/feed" {
		t.Errorf("expected redirect to /feed, got %q", loc)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestCreateReportRequiresContentFields verifies missing content_type/content_id
// must NOT hit the database and returns a 400.
func TestCreateReportRequiresContentFields(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)

	form := url.Values{}
	form.Set("content_type", "")
	form.Set("content_id", "")
	req := httptest.NewRequest("POST", "/report", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "reporter-1"))
	rec := httptest.NewRecorder()

	h.createReport(rec, req)

	if rec.Code != 400 {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Errorf("should not redirect on validation error, got Location %q", loc)
	}
}

// TestShowQueueListsPendingReports verifies the queue query renders pending
// reports with reporter name, content type, and reason.
func TestShowQueueListsPendingReports(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	now := time.Now()
	mock.ExpectQuery(`FROM content_reports`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "reporter_id", "reporter_name", "content_type", "content_id", "reason", "status", "created_at",
		}).
			AddRow("r-1", "u-1", "Ada Lovelace", "post", "post-123", "Spam", "pending", now).
			AddRow("r-2", "u-2", "Alan Turing", "comment", "cmt-9", "Harassment", "pending", now))

	h := NewHandler(db, testPages())

	req := httptest.NewRequest("GET", "/admin/moderation", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.showQueue(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Ada Lovelace") {
		t.Errorf("expected reporter name in body, got: %s", body)
	}
	if !strings.Contains(body, "Spam") {
		t.Errorf("expected reason in body, got: %s", body)
	}
	if !strings.Contains(body, "Harassment") {
		t.Errorf("expected second report reason in body, got: %s", body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestResolveReportMarksReviewed verifies action=reviewed sets status='reviewed'
// with reviewed_by = current admin and redirects to the queue.
func TestResolveReportMarksReviewed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)

	mock.ExpectExec(`UPDATE content_reports SET status = \$1, reviewed_by = \$2 WHERE id = \$3`).
		WithArgs("reviewed", "admin-1", "r-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	form := url.Values{}
	form.Set("action", "reviewed")
	req := httptest.NewRequest("POST", "/admin/moderation/r-1/resolve", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "r-1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.resolveReport(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/moderation" {
		t.Errorf("expected redirect to /admin/moderation, got %q", loc)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestResolveReportDismiss verifies action=dismissed sets status='dismissed'.
func TestResolveReportDismiss(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)

	mock.ExpectExec(`UPDATE content_reports SET status = \$1, reviewed_by = \$2 WHERE id = \$3`).
		WithArgs("dismissed", "admin-1", "r-2").
		WillReturnResult(sqlmock.NewResult(0, 1))

	form := url.Values{}
	form.Set("action", "dismissed")
	req := httptest.NewRequest("POST", "/admin/moderation/r-2/resolve", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "r-2")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.resolveReport(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
