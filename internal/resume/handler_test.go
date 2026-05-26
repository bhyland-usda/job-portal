package resume

import (
	"context"
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// TestGenerateResumeWithEndDate verifies BUG-1: generating a resume for a user
// whose experience has a non-null end date renders successfully. Previously the
// end_date was scanned into sql.NullString while the template called
// .EndDate.Time.Format, which is a runtime template error.
func TestGenerateResumeWithEndDate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	const userID = "user-123"

	tmpl := template.Must(template.ParseFiles("../../templates/resume/view.html"))
	pages := map[string]*template.Template{"resume_view.html": tmpl}
	h := NewHandler(db, pages)

	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC)

	// User profile
	mock.ExpectQuery(`SELECT first_name, last_name`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"first_name", "last_name", "headline", "location", "about", "avatar_url",
		}).AddRow("Ada", "Lovelace", "Engineer", "DC", "Bio", nil))

	// Experiences (one row WITH a non-null end_date)
	mock.ExpectQuery(`FROM experiences`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"title", "company", "location", "description", "start_date", "end_date",
		}).AddRow("Dev", "ACME", "DC", "Built things", start, end))

	// Educations (none)
	mock.ExpectQuery(`FROM educations`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"school", "degree", "field_of_study", "start_year", "end_year",
		}))

	// Skills (none)
	mock.ExpectQuery(`from skills`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"name"}))

	req := httptest.NewRequest("GET", "/resumes/generate", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()

	h.generateResume(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	// The end date should render formatted, not crash and not say "Present".
	if !strings.Contains(body, "Jun 2022") {
		t.Errorf("expected formatted end date 'Jun 2022' in output, got:\n%s", body)
	}
	if strings.Contains(body, "Present") {
		t.Errorf("experience with an end date should not show 'Present', got:\n%s", body)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestGenerateResumeOpenEndedExperience verifies an experience with a NULL end
// date renders "Present" (the open-ended case).
func TestGenerateResumeOpenEndedExperience(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	const userID = "user-456"

	tmpl := template.Must(template.ParseFiles("../../templates/resume/view.html"))
	pages := map[string]*template.Template{"resume_view.html": tmpl}
	h := NewHandler(db, pages)

	start := time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT first_name, last_name`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"first_name", "last_name", "headline", "location", "about", "avatar_url",
		}).AddRow("Grace", "Hopper", "", "", "", nil))

	mock.ExpectQuery(`FROM experiences`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"title", "company", "location", "description", "start_date", "end_date",
		}).AddRow("Lead", "Navy", "", "", start, nil))

	mock.ExpectQuery(`FROM educations`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"school", "degree", "field_of_study", "start_year", "end_year",
		}))

	mock.ExpectQuery(`from skills`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"name"}))

	req := httptest.NewRequest("GET", "/resumes/generate", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()

	h.generateResume(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Present") {
		t.Errorf("open-ended experience should show 'Present', got:\n%s", rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
