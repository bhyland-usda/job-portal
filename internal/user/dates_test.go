package user

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// TestUpdateProfileFieldsSavesDates verifies that set birthday/hire_date values
// are written to the UPDATE as non-NULL.
func TestUpdateProfileFieldsSavesDates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("UPDATE users SET").
		WithArgs("Ada", "Lovelace", "", "", "", "1990-01-05", "2019-03-01", "telework", "2026-06-03", "everyone", "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	f := profileFields{
		FirstName:         "Ada",
		LastName:          "Lovelace",
		Birthday:          dateOrNull("1990-01-05"),
		HireDate:          dateOrNull("2019-03-01"),
		WorkStatus:        "telework",
		WorkStatusUntil:   dateOrNull("2026-06-03"),
		ProfileVisibility: "everyone",
	}
	if err := updateProfileFields(context.Background(), db, "user-1", f); err != nil {
		t.Fatalf("updateProfileFields error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestUpdateProfileFieldsEmptyDatesAreNull verifies blank date inputs persist as
// SQL NULL (not empty strings or zero dates).
func TestUpdateProfileFieldsEmptyDatesAreNull(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("UPDATE users SET").
		WithArgs("Ada", "Lovelace", "", "", "", nil, nil, "in_office", nil, "everyone", "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	f := profileFields{
		FirstName:         "Ada",
		LastName:          "Lovelace",
		Birthday:          dateOrNull(""),
		HireDate:          dateOrNull("   "),
		WorkStatus:        "in_office",
		WorkStatusUntil:   dateOrNull(""),
		ProfileVisibility: "everyone",
	}
	if err := updateProfileFields(context.Background(), db, "user-1", f); err != nil {
		t.Fatalf("updateProfileFields error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestGetProfileDataLoadsDates verifies birthday/hire_date round-trip from the
// users row into the ProfileData view model (so the template can render them).
func TestGetProfileDataLoadsDates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	h := &Handler{db: db}

	birthday := time.Date(1990, time.January, 5, 0, 0, 0, 0, time.UTC)
	hire := time.Date(2019, time.March, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT id, email, first_name, last_name, headline, about, avatar_url, location, created_at, birthday, hire_date").
		WithArgs("user-2").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "first_name", "last_name", "headline", "about",
			"avatar_url", "location", "created_at", "birthday", "hire_date",
			"work_status", "work_status_until", "profile_visibility",
		}).AddRow("user-2", "a@b.gov", "Ada", "Lovelace", "", "", "", "", time.Now(), birthday, hire, "in_office", nil, "everyone"))

	// getProfileData also loads experiences, educations, skills.
	mock.ExpectQuery("FROM experiences").WithArgs("user-2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "company", "location", "start_date", "end_date", "description"}))
	mock.ExpectQuery("FROM educations").WithArgs("user-2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "school", "degree", "field_of_study", "start_year", "end_year"}))
	mock.ExpectQuery("FROM skills").WithArgs("user-2", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "cnt", "endorsed"}))

	r := httptest.NewRequest("GET", "/profile/user-2", nil)
	data, err := h.getProfileData(r, "user-2", "user-1")
	if err != nil {
		t.Fatalf("getProfileData error: %v", err)
	}
	if data.User.Birthday == nil || !data.User.Birthday.Equal(birthday) {
		t.Errorf("expected birthday %v, got %v", birthday, data.User.Birthday)
	}
	if data.User.HireDate == nil || !data.User.HireDate.Equal(hire) {
		t.Errorf("expected hire_date %v, got %v", hire, data.User.HireDate)
	}
	if got := data.User.BirthdayLabel(); got != "January 5" {
		t.Errorf("expected BirthdayLabel 'January 5', got %q", got)
	}
	if got := data.User.HireYear(); got != 2019 {
		t.Errorf("expected HireYear 2019, got %d", got)
	}
}

// TestProfileViewRendersBirthdayAndAnniversary verifies the profile view template
// shows the birthday (month + day) and the "At USDA since" anniversary line.
func TestProfileViewRendersBirthdayAndAnniversary(t *testing.T) {
	birthday := time.Date(1990, time.January, 5, 0, 0, 0, 0, time.UTC)
	hire := time.Date(2019, time.March, 1, 0, 0, 0, 0, time.UTC)

	view := ProfileView{
		ProfileData: &ProfileData{
			User: User{ID: "u2", FirstName: "Ada", LastName: "Lovelace", Birthday: &birthday, HireDate: &hire},
		},
	}
	out := renderContent(t, view)

	if !strings.Contains(out, "January 5") {
		t.Errorf("expected birthday 'January 5' in output")
	}
	if !strings.Contains(out, "2019") {
		t.Errorf("expected hire year 2019 in output")
	}
}

// TestProfileViewHidesUnsetDates verifies birthday/anniversary lines are absent
// when no dates are set.
func TestProfileViewHidesUnsetDates(t *testing.T) {
	view := ProfileView{
		ProfileData: &ProfileData{
			User: User{ID: "u2", FirstName: "Ada", LastName: "Lovelace"},
		},
	}
	out := renderContent(t, view)
	if strings.Contains(out, "Birthday") {
		t.Errorf("did not expect a Birthday line when birthday unset")
	}
	if strings.Contains(out, "At USDA since") {
		t.Errorf("did not expect anniversary line when hire_date unset")
	}
}
