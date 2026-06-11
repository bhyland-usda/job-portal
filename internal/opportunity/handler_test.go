package opportunity

import (
	"bytes"
	"context"
	"database/sql"
	"html/template"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
	"github.com/bhyland-usda/job-portal/internal/notification"
	"github.com/bhyland-usda/job-portal/internal/semantic"
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
		"posting_view.html":    mk("opportunity/view.html"),
		"posting_matches.html": mk("opportunity/matches.html"),
		"posting_outcome.html": mk("opportunity/outcome.html"),
		"posting_history.html": mk("opportunity/history.html"),
		"posting_edit.html":    mk("opportunity/edit.html"),
	}
}

func expectLoadPostingQuery(mock sqlmock.Sqlmock, postingID, authorID string) {
	mock.ExpectQuery(`SELECT p.id, p.author_id,`).
		WithArgs(postingID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "author_id", "author_name", "title", "description", "type",
			"location", "department", "status", "created_at",
			"outcome", "outcome_status", "completed_at",
			"start_date", "duration_days", "reporting_manager_name", "reporting_manager_email",
			"location_type", "application_close_date", "number_of_people", "learning_outcomes",
		}).AddRow(
			postingID,
			authorID,
			"Author User",
			"Existing Opportunity",
			"Existing description",
			"project",
			"Remote",
			"OCIO",
			"active",
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			nil,
			nil,
			nil,
			time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			90,
			"Manager Name",
			"manager@example.gov",
			"remote",
			time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC),
			2,
			"Learn outcomes",
		))
}

// renderView parses the real posting view template and renders only its
// "content" block against the given PostingPage data. This lets us assert the
// UI-visibility rules for the Apply button and View Applications link without a
// running server or database.
func renderView(t *testing.T, data PostingPage) string {
	t.Helper()
	tmpl, err := template.ParseFiles("../../templates/opportunity/view.html")
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

	applyPath := "/opportunities/posting-1/apply"
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

	if strings.Contains(html, "/opportunities/posting-1/apply") {
		t.Errorf("author should not see an apply control, got:\n%s", html)
	}
}

func TestPostingMatchCountLinksToMatchesPage(t *testing.T) {
	data := PostingPage{
		BaseData: middleware.BaseData{UserID: "viewer-1"},
		Posting: Posting{
			ID:         "posting-1",
			AuthorID:   "author-9",
			Title:      "Detail Opportunity",
			MatchCount: 3,
		},
	}

	html := renderView(t, data)

	if !strings.Contains(html, "/opportunities/posting-1/matches") {
		t.Fatalf("expected matches link in posting view, got:\n%s", html)
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

	if strings.Contains(html, "/opportunities/posting-1/apply") {
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

	appsPath := "/opportunities/posting-1/applications"
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

	if strings.Contains(html, "/opportunities/posting-1/applications") {
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

	if !strings.Contains(html, "/opportunities/posting-1/outcome") {
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

	if strings.Contains(html, "/opportunities/posting-1/outcome") {
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

	req := httptest.NewRequest("POST", "/opportunities/posting-1/outcome", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "posting-1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "author-9"))
	rec := httptest.NewRecorder()

	h.handleOutcome(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/opportunities/posting-1" {
		t.Errorf("expected redirect to /opportunities/posting-1, got %q", loc)
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

	req := httptest.NewRequest("POST", "/opportunities/posting-1/outcome", strings.NewReader(form.Encode()))
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

func TestShowMatchesRendersMatchedEmployees(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT p.id, p.author_id, p.author_name, p.title,`).
		WithArgs("posting-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "author_id", "author_name", "title", "department", "created_at",
		}).AddRow(
			"posting-1", "manager-1", "Manager User", "Data Detail", "OCIO",
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		))

	mock.ExpectQuery(`SELECT u.id,`).
		WithArgs("posting-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "first_name", "last_name", "headline", "avatar_url", "department",
			"location", "matched_skill_count", "matched_skills",
		}).
			AddRow("user-1", "Ada", "Lovelace", "Data Engineer", "https://cdn.example/avatar.png", "OCIO", "Washington, DC", 2, "Go||SQL").
			AddRow("user-2", "Grace", "Hopper", "Program Manager", "", "FPAC", "Kansas City, MO", 1, "Leadership"))

	h := NewHandler(db, testPages())

	req := httptest.NewRequest("GET", "/opportunities/posting-1/matches", nil)
	req.SetPathValue("id", "posting-1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserInfoKey, middleware.UserInfo{
		ID:        "manager-1",
		FirstName: "Manager",
		LastName:  "User",
		Role:      "manager",
	}))
	rec := httptest.NewRecorder()

	h.showMatches(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Employees matching Data Detail",
		"Ada Lovelace",
		"Grace Hopper",
		"Go",
		"SQL",
		"Leadership",
		"/profile/user-1",
		"/profile/user-2",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected response to contain %q, got:\n%s", want, body)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
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

	req := httptest.NewRequest("GET", "/opportunities/history", nil)
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

func TestHandleEditRejectsInvalidOpportunityType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	expectLoadPostingQuery(mock, "posting-1", "author-9")

	h := NewHandler(db, testPages())

	form := url.Values{}
	form.Set("title", "Updated Opportunity")
	form.Set("description", "Updated description")
	form.Set("type", "internship")
	form.Set("location", "Remote")
	form.Set("department", "OCIO")
	form.Set("start_date", "2026-06-01")
	form.Set("duration_days", "60")
	form.Set("reporting_manager_name", "Manager Name")
	form.Set("reporting_manager_email", "manager@example.gov")
	form.Set("location_type", "remote")
	form.Set("application_close_date", "2026-05-15")
	form.Set("number_of_people", "2")
	form.Set("learning_outcomes", "Learned outcomes")

	req := httptest.NewRequest("POST", "/opportunities/posting-1/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "posting-1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "author-9"))
	rec := httptest.NewRecorder()

	h.handleEdit(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200 response for validation error, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Opportunity type must be project or detail") {
		t.Errorf("expected invalid type message, got:\n%s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestHandleEditRejectsInvalidLocationType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	expectLoadPostingQuery(mock, "posting-1", "author-9")

	h := NewHandler(db, testPages())

	form := url.Values{}
	form.Set("title", "Updated Opportunity")
	form.Set("description", "Updated description")
	form.Set("type", "project")
	form.Set("location", "Remote")
	form.Set("department", "OCIO")
	form.Set("start_date", "2026-06-01")
	form.Set("duration_days", "60")
	form.Set("reporting_manager_name", "Manager Name")
	form.Set("reporting_manager_email", "manager@example.gov")
	form.Set("location_type", "office")
	form.Set("application_close_date", "2026-05-15")
	form.Set("number_of_people", "2")
	form.Set("learning_outcomes", "Learned outcomes")

	req := httptest.NewRequest("POST", "/opportunities/posting-1/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "posting-1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "author-9"))
	rec := httptest.NewRecorder()

	h.handleEdit(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200 response for validation error, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Location type must be remote, hybrid, or onsite") {
		t.Errorf("expected invalid location type message, got:\n%s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestHandleEditRejectsCloseDateAfterStartDate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	expectLoadPostingQuery(mock, "posting-1", "author-9")

	h := NewHandler(db, testPages())

	form := url.Values{}
	form.Set("title", "Updated Opportunity")
	form.Set("description", "Updated description")
	form.Set("type", "project")
	form.Set("location", "Remote")
	form.Set("department", "OCIO")
	form.Set("start_date", "2026-06-01")
	form.Set("duration_days", "60")
	form.Set("reporting_manager_name", "Manager Name")
	form.Set("reporting_manager_email", "manager@example.gov")
	form.Set("location_type", "remote")
	form.Set("application_close_date", "2026-06-15")
	form.Set("number_of_people", "2")
	form.Set("learning_outcomes", "Learned outcomes")

	req := httptest.NewRequest("POST", "/opportunities/posting-1/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "posting-1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "author-9"))
	rec := httptest.NewRecorder()

	h.handleEdit(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200 response for validation error, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Application close date cannot be after start date") {
		t.Errorf("expected close date ordering message, got:\n%s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestGetMatchedPostingsSemanticPath(t *testing.T) {
	t.Setenv("SEMANTIC_MATCHING_ENABLED", "true")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	corpus := "Data Engineer\nBuilding ETL\npython sql"
	vec := semantic.ToPGVectorLiteral(semantic.GenerateEmbedding(corpus))

	mock.ExpectQuery(`SELECT COALESCE\(u.headline, ''\),`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"headline", "about", "skills"}).AddRow("Data Engineer", "Building ETL", "python sql"))

	mock.ExpectQuery(`JOIN semantic_embeddings se`).
		WithArgs(semantic.EntityTypeOpportunity, vec).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "author_id", "author_name", "title", "description", "type", "location", "department", "status", "created_at",
		}).AddRow(
			"posting-1", "author-1", "Ada Lovelace", "Data Modernization", "Build data pipeline", "project", "Remote", "OCIO", "active", time.Now(),
		))

	postings, err := GetMatchedPostings(db, context.Background(), "user-1", true, semantic.DefaultLocationTypePreference())
	if err != nil {
		t.Fatalf("GetMatchedPostings returned error: %v", err)
	}
	if len(postings) != 1 || postings[0].ID != "posting-1" {
		t.Fatalf("unexpected semantic postings result: %+v", postings)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}

	_ = os.Unsetenv("SEMANTIC_MATCHING_ENABLED")
}

func TestLocationTypeWhereClauseUsesInferenceFallback(t *testing.T) {
	pref := semantic.LocationTypePreference{Remote: true, Hybrid: true}
	clause, args, next := locationTypeWhereClause(pref, 3)

	if clause == "" {
		t.Fatalf("expected non-empty SQL clause")
	}
	if !strings.Contains(clause, "NULLIF(p.location_type, '')") {
		t.Fatalf("expected clause to normalize empty location_type, got: %s", clause)
	}
	if !strings.Contains(clause, "LIKE '%remote%'") || !strings.Contains(clause, "LIKE '%hybrid%'") {
		t.Fatalf("expected clause to include location text inference, got: %s", clause)
	}
	if len(args) != 2 || args[0] != "remote" || args[1] != "hybrid" {
		t.Fatalf("unexpected args: %#v", args)
	}
	if next != 5 {
		t.Fatalf("expected next arg index 5, got %d", next)
	}
}

func TestHandleApplyRejectsPastApplicationCloseDate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT p.id, p.author_id,`).
		WithArgs("posting-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "author_id", "author_name", "title", "description", "type",
			"location", "department", "status", "created_at",
			"outcome", "outcome_status", "completed_at",
			"start_date", "duration_days", "reporting_manager_name", "reporting_manager_email",
			"location_type", "application_close_date", "number_of_people", "learning_outcomes",
		}).AddRow(
			"posting-1", "manager-1", "Manager User", "Data Detail", "desc", "detail",
			"Remote", "OCIO", "active", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			nil, nil, nil,
			time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), 30, "Manager", "manager@example.gov",
			"remote", time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), 1, "outcomes",
		))

	h := NewHandler(db, testPages())

	form := url.Values{}
	form.Set("selection_why", "Good fit")
	form.Set("project_tackle_approach", "I will start with discovery")

	req := httptest.NewRequest("POST", "/opportunities/posting-1/apply", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "posting-1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()

	h.handleApply(rec, req)

	if rec.Code != 400 {
		t.Fatalf("expected 400 for past close date, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "no longer accepting applications") {
		t.Fatalf("expected close-date error message, got: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestHandleApplyCreatesManagerNotification(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT p.id, p.author_id,`).
		WithArgs("posting-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "author_id", "author_name", "title", "description", "type",
			"location", "department", "status", "created_at",
			"outcome", "outcome_status", "completed_at",
			"start_date", "duration_days", "reporting_manager_name", "reporting_manager_email",
			"location_type", "application_close_date", "number_of_people", "learning_outcomes",
		}).AddRow(
			"posting-1", "manager-1", "Manager User", "Data Detail", "desc", "detail",
			"Remote", "OCIO", "active", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			nil, nil, nil,
			time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), 30, "Manager", "manager@example.gov",
			"remote", time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), 1, "outcomes",
		))

	mock.ExpectExec(`INSERT INTO posting_applications`).
		WithArgs("posting-1", "user-1", "Good fit", "Good fit", "I will start with discovery").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(`SELECT first_name, last_name FROM users WHERE id = \$1`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"first_name", "last_name"}).AddRow("Ada", "Lovelace"))

	mock.ExpectQuery(`SELECT enabled FROM notification_preferences`).
		WithArgs("manager-1", "opportunity_application").
		WillReturnError(sql.ErrNoRows)

	mock.ExpectExec(`INSERT INTO notifications`).
		WithArgs("manager-1", "user-1", "opportunity_application", "Ada Lovelace applied to your opportunity: Data Detail").
		WillReturnResult(sqlmock.NewResult(0, 1))

	h := NewHandler(db, testPages())
	h.SetNotificationHandler(notification.NewHandler(db, nil))

	form := url.Values{}
	form.Set("selection_why", "Good fit")
	form.Set("project_tackle_approach", "I will start with discovery")

	req := httptest.NewRequest("POST", "/opportunities/posting-1/apply", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "posting-1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()

	h.handleApply(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/opportunities/posting-1" {
		t.Fatalf("expected redirect to posting view, got %q", loc)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
