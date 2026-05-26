package user

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

// loadPrintTemplate parses the standalone print template the same way main.go
// does for the resume page (own top-level template, no site base).
func loadPrintTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := template.ParseFiles("../../templates/profile/print.html")
	if err != nil {
		t.Fatalf("failed to parse print template: %v", err)
	}
	return tmpl
}

// TestPrintProfileRendersKeyFieldsStandalone verifies FEATURE 2: the print page
// renders the profile's key fields and is standalone (no navbar / site chrome).
func TestPrintProfileRendersKeyFieldsStandalone(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	const profileID = "p-1"
	const viewerID = "v-1"

	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC)

	// User row (matches getProfileData column order).
	mock.ExpectQuery(`SELECT id, email, first_name, last_name`).
		WithArgs(profileID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "first_name", "last_name", "headline", "about",
			"avatar_url", "location", "created_at", "birthday", "hire_date",
			"work_status", "work_status_until", "profile_visibility",
		}).AddRow(profileID, "ada@usda.gov", "Ada", "Lovelace", "Engineer",
			"My bio paragraph", "", "Washington DC", time.Now(), nil, nil,
			"in_office", nil, "everyone"))

	// Experiences.
	mock.ExpectQuery(`FROM experiences`).
		WithArgs(profileID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "company", "location", "start_date", "end_date", "description",
		}).AddRow("e1", "Lead Dev", "ACME", "DC", start, end, "Built systems"))

	// Educations.
	mock.ExpectQuery(`FROM educations`).
		WithArgs(profileID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "school", "degree", "field_of_study", "start_year", "end_year",
		}).AddRow("ed1", "MIT", "BSc", "CS", 2010, 2014))

	// Skills with endorsements (getSkillsWithEndorsements, two args).
	mock.ExpectQuery(`FROM skills`).
		WithArgs(profileID, viewerID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "count", "endorsed",
		}).AddRow("s1", "Golang", 3, false))

	tmpl := loadPrintTemplate(t)
	pages := map[string]*template.Template{"profile_print.html": tmpl}
	h := NewHandler(db, pages, nil)

	req := httptest.NewRequest("GET", "/profile/"+profileID+"/print", nil)
	req.SetPathValue("id", profileID)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, viewerID))
	rec := httptest.NewRecorder()

	h.showPrintProfile(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	for _, want := range []string{
		"Ada", "Lovelace", "Engineer", "Washington DC", "My bio paragraph",
		"Lead Dev", "ACME", "MIT", "Golang",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected print output to contain %q", want)
		}
	}

	// Standalone: must be a full HTML document with no navbar/site chrome.
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("expected print page to be a standalone HTML document")
	}
	if strings.Contains(body, "navbar") || strings.Contains(body, "/feed") {
		t.Error("print page should not include site navigation chrome")
	}
	if !strings.Contains(body, "@media print") {
		t.Error("expected print-optimized @media print styles")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
