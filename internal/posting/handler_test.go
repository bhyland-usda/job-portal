package posting

import (
	"bytes"
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

// testPages parses the real posting templates (with layouts) so handler tests
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
		"posting_view.html":    mk("posting/view.html"),
		"posting_outcome.html": mk("posting/outcome.html"),
		"posting_history.html": mk("posting/history.html"),
	}
}

// renderView parses the real posting view template and renders only its
// "content" block against the given PostingPage data. This lets us assert the
// UI-visibility rules for the Apply button and View Applications link without a
// running server or database.
func renderView(t *testing.T, data PostingPage) string {
	t.Helper()
	tmpl, err := template.ParseFiles("../../templates/posting/view.html")
	if err != nil {
		t.Fatalf("parse view.html: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "content", data); err != nil {
		t.Fatalf("execute content: %v", err)
	}
	return buf.String()
}

// TestViewerSeesApplyButton verifies BUG-2: a non-author viewing an active
// posting must be offered a way to apply (a link/form targeting the apply route).
func TestViewerSeesApplyButton(t *testing.T) {
	data := PostingPage{
		BaseData: middleware.BaseData{UserID: "viewer-1"},
		Posting: Posting{
			ID:       "posting-1",
			AuthorID: "author-9",
			Title:    "Detail Opportunity",
			Status:   "active",
		},
	}

	html := renderView(t, data)

	applyPath := "/postings/posting-1/apply"
	if !strings.Contains(html, applyPath) {
		t.Errorf("expected apply control targeting %q for non-author viewer, got:\n%s", applyPath, html)
	}
}

// TestAuthorDoesNotSeeApplyButton verifies the author of a posting is not shown
// an Apply control (they cannot apply to their own posting).
func TestAuthorDoesNotSeeApplyButton(t *testing.T) {
	data := PostingPage{
		BaseData: middleware.BaseData{UserID: "author-9"},
		Posting: Posting{
			ID:       "posting-1",
			AuthorID: "author-9",
			Status:   "active",
		},
	}

	html := renderView(t, data)

	if strings.Contains(html, "/postings/posting-1/apply") {
		t.Errorf("author should not see an apply control, got:\n%s", html)
	}
}

// TestApplyHiddenWhenClosed verifies a non-author does not get an Apply control
// when the posting is closed (it is no longer accepting applications).
func TestApplyHiddenWhenClosed(t *testing.T) {
	data := PostingPage{
		BaseData: middleware.BaseData{UserID: "viewer-1"},
		Posting: Posting{
			ID:       "posting-1",
			AuthorID: "author-9",
			Status:   "closed",
		},
	}

	html := renderView(t, data)

	if strings.Contains(html, "/postings/posting-1/apply") {
		t.Errorf("closed posting should not show an apply control, got:\n%s", html)
	}
}

// TestAuthorSeesViewApplications verifies BUG-2: the posting author/manager gets
// a link to review applications.
func TestAuthorSeesViewApplications(t *testing.T) {
	data := PostingPage{
		BaseData: middleware.BaseData{UserID: "author-9"},
		Posting: Posting{
			ID:       "posting-1",
			AuthorID: "author-9",
			Status:   "active",
		},
	}

	html := renderView(t, data)

	appsPath := "/postings/posting-1/applications"
	if !strings.Contains(html, appsPath) {
		t.Errorf("expected author to see View Applications link %q, got:\n%s", appsPath, html)
	}
}

// TestViewerDoesNotSeeViewApplications verifies a non-author cannot see the
// View Applications link.
func TestViewerDoesNotSeeViewApplications(t *testing.T) {
	data := PostingPage{
		BaseData: middleware.BaseData{UserID: "viewer-1"},
		Posting: Posting{
			ID:       "posting-1",
			AuthorID: "author-9",
			Status:   "active",
		},
	}

	html := renderView(t, data)

	if strings.Contains(html, "/postings/posting-1/applications") {
		t.Errorf("non-author should not see View Applications link, got:\n%s", html)
	}
}

// TestViewShowsRecordedOutcome verifies that when a posting has a recorded
// outcome, the read-only outcome block is rendered on the posting view.
func TestViewShowsRecordedOutcome(t *testing.T) {
	data := PostingPage{
		BaseData: middleware.BaseData{UserID: "viewer-1"},
		Posting: Posting{
			ID:            "posting-1",
			AuthorID:      "author-9",
			Title:         "Detail Opportunity",
			Status:        "closed",
			HasOutcome:    true,
			OutcomeStatus: "completed",
			Outcome:       "Delivered the migration ahead of schedule.",
			CompletedAt:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	html := renderView(t, data)

	if !strings.Contains(html, "Delivered the migration ahead of schedule.") {
		t.Errorf("expected recorded outcome text in view, got:\n%s", html)
	}
	if !strings.Contains(html, "completed") {
		t.Errorf("expected outcome status in view, got:\n%s", html)
	}
}

// TestAuthorSeesRecordOutcomeLink verifies the author sees a control linking to
// the record-outcome page.
func TestAuthorSeesRecordOutcomeLink(t *testing.T) {
	data := PostingPage{
		BaseData: middleware.BaseData{UserID: "author-9"},
		Posting: Posting{
			ID:       "posting-1",
			AuthorID: "author-9",
			Status:   "active",
		},
	}

	html := renderView(t, data)

	if !strings.Contains(html, "/postings/posting-1/outcome") {
		t.Errorf("expected author to see record-outcome link, got:\n%s", html)
	}
}

// TestViewerDoesNotSeeRecordOutcomeLink verifies a non-author cannot see the
// record-outcome control.
func TestViewerDoesNotSeeRecordOutcomeLink(t *testing.T) {
	data := PostingPage{
		BaseData: middleware.BaseData{UserID: "viewer-1"},
		Posting: Posting{
			ID:       "posting-1",
			AuthorID: "author-9",
			Status:   "active",
		},
	}

	html := renderView(t, data)

	if strings.Contains(html, "/postings/posting-1/outcome") {
		t.Errorf("non-author should not see record-outcome link, got:\n%s", html)
	}
}

// TestHandleOutcomeSavesAndCloses verifies POST outcome by the author saves the
// outcome, outcome_status and completed_at, sets the posting status to closed
// when completing, and redirects back to the posting.
func TestHandleOutcomeSavesAndCloses(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// Authorization lookup: current user is the author.
	mock.ExpectQuery(`SELECT author_id FROM postings WHERE id = \$1`).
		WithArgs("posting-1").
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}).AddRow("author-9"))

	// Save outcome + close (status = closed for a completed outcome).
	mock.ExpectExec(`UPDATE postings SET outcome = \$1, outcome_status = \$2, completed_at = NOW\(\), status = 'closed', updated_at = NOW\(\) WHERE id = \$3`).
		WithArgs("Wrapped up successfully", "completed", "posting-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	h := NewHandler(db, testPages())

	form := url.Values{}
	form.Set("outcome", "Wrapped up successfully")
	form.Set("outcome_status", "completed")

	req := httptest.NewRequest("POST", "/postings/posting-1/outcome", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "posting-1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "author-9"))
	rec := httptest.NewRecorder()

	h.handleOutcome(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/postings/posting-1" {
		t.Errorf("expected redirect to /postings/posting-1, got %q", loc)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestHandleOutcomeForbiddenForNonAuthor verifies a non-author, non-admin user
// cannot record an outcome.
func TestHandleOutcomeForbiddenForNonAuthor(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT author_id FROM postings WHERE id = \$1`).
		WithArgs("posting-1").
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}).AddRow("author-9"))
	// Role lookup returns a non-admin role.
	mock.ExpectQuery(`SELECT role FROM users WHERE id = \$1`).
		WithArgs("intruder-1").
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow("employee"))

	h := NewHandler(db, testPages())

	form := url.Values{}
	form.Set("outcome", "Sneaky")
	form.Set("outcome_status", "completed")

	req := httptest.NewRequest("POST", "/postings/posting-1/outcome", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "posting-1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "intruder-1"))
	rec := httptest.NewRecorder()

	h.handleOutcome(rec, req)

	if rec.Code != 403 {
		t.Fatalf("expected 403 Forbidden, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestShowHistoryListsAcceptedPostings verifies the current user's detail
// history lists postings they were accepted to, including any recorded outcome.
func TestShowHistoryListsAcceptedPostings(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	created := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	accepted := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	completed := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM posting_applications`).
		WithArgs("user-7").
		WillReturnRows(sqlmock.NewRows([]string{
			"posting_id", "title", "department", "type", "status",
			"created_at", "accepted_at", "outcome", "outcome_status", "completed_at",
		}).
			AddRow("posting-1", "Data Migration Detail", "OCIO", "detail", "closed",
				created, accepted, "Migrated everything", "completed", completed))

	h := NewHandler(db, testPages())

	req := httptest.NewRequest("GET", "/postings/history", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-7"))
	rec := httptest.NewRecorder()

	h.showHistory(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Data Migration Detail") {
		t.Errorf("expected accepted posting title in history, got:\n%s", body)
	}
	if !strings.Contains(body, "Migrated everything") {
		t.Errorf("expected recorded outcome in history, got:\n%s", body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
