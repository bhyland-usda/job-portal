package user

import (
	"bytes"
	"context"
	"html/template"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// TestHandleEditProfileSavesWorkStatus verifies FEATURE 1: posting the edit form
// with a work_status and work_status_until persists both values (the until date
// flows to the DB as a non-NULL value).
func TestHandleEditProfileSavesWorkStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	const userID = "user-1"

	mock.ExpectExec(`UPDATE users SET`).
		WithArgs(
			"Ada", "Lovelace", "Engineer", "Bio", "DC",
			sqlmock.AnyArg(), sqlmock.AnyArg(), // birthday, hire_date
			"telework", "2026-06-03",
			"everyone", // profile_visibility default
			userID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	h := NewHandler(db, map[string]*template.Template{}, nil)

	form := url.Values{}
	form.Set("first_name", "Ada")
	form.Set("last_name", "Lovelace")
	form.Set("headline", "Engineer")
	form.Set("about", "Bio")
	form.Set("location", "DC")
	form.Set("work_status", "telework")
	form.Set("work_status_until", "2026-06-03")

	req := httptest.NewRequest("POST", "/profile/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()

	h.handleEditProfile(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected redirect 303, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestHandleEditProfileInvalidWorkStatusFallsBack verifies invalid statuses are
// rejected and replaced with the default ("in_office"), and that an empty until
// date is persisted as SQL NULL.
func TestHandleEditProfileInvalidWorkStatusFallsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	const userID = "user-2"

	mock.ExpectExec(`UPDATE users SET`).
		WithArgs(
			"Grace", "Hopper", "", "", "",
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			"in_office", nil, // invalid -> default; empty until -> NULL
			"everyone", // profile_visibility default
			userID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	h := NewHandler(db, map[string]*template.Template{}, nil)

	form := url.Values{}
	form.Set("first_name", "Grace")
	form.Set("last_name", "Hopper")
	form.Set("work_status", "bogus")
	form.Set("work_status_until", "")

	req := httptest.NewRequest("POST", "/profile/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()

	h.handleEditProfile(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected redirect 303, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestProfileViewRendersWorkStatusBadge verifies the work-status badge appears on
// the profile view, including the "until" date suffix.
func TestProfileViewRendersWorkStatusBadge(t *testing.T) {
	until := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	view := ProfileView{
		ProfileData: &ProfileData{
			User: User{
				ID: "u2", FirstName: "Ada", LastName: "Lovelace",
				WorkStatus: "telework", WorkStatusUntil: &until,
			},
			IsOwnProfile: false,
		},
	}

	out := renderContent(t, view)

	if !strings.Contains(out, "Telework") {
		t.Errorf("expected work status badge label 'Telework' in output:\n%s", out)
	}
	if !strings.Contains(out, "Jun 3") {
		t.Errorf("expected work status 'until' date 'Jun 3' in output:\n%s", out)
	}
}

// TestProfileEditRendersWorkStatusSelect verifies the edit form renders a
// work_status <select> with the user's current value pre-selected and an "until"
// date input.
func TestProfileEditRendersWorkStatusSelect(t *testing.T) {
	tmpl, err := template.ParseFS(
		os.DirFS("../../templates"),
		"layouts/*.html",
		"profile/edit.html",
	)
	if err != nil {
		t.Fatalf("failed to parse edit template set: %v", err)
	}

	until := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	data := ProfileData{
		BaseData: middleware.BaseData{UserID: "u1", Nav: middleware.UserInfo{ID: "u1"}},
		User: User{
			ID: "u1", FirstName: "Ada", LastName: "Lovelace",
			WorkStatus: "telework", WorkStatusUntil: &until,
		},
		IsOwnProfile: true,
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", data); err != nil {
		t.Fatalf("failed to render edit template: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, `name="work_status"`) {
		t.Error("expected a work_status select in the edit form")
	}
	if !strings.Contains(out, `name="work_status_until"`) {
		t.Error("expected a work_status_until date input in the edit form")
	}
	if !strings.Contains(out, `2026-06-03`) {
		t.Error("expected the current until date pre-filled in the edit form")
	}
}
