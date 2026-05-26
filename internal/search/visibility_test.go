package search

import (
	"database/sql"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	return db, mock
}

// TestSearchExcludesPrivateForNonAdmin verifies that a non-admin viewer's
// search excludes users whose profile_visibility is 'private'. We assert the
// query carries the profile_visibility filter and that a private user does not
// appear in the returned results.
func TestSearchExcludesPrivateForNonAdmin(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	h := &Handler{db: db}

	searchTerm := "%ada%"

	// The query must filter out private profiles for a non-admin viewer.
	mock.ExpectQuery("profile_visibility <> 'private'").
		WithArgs("viewer-1", searchTerm).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "first_name", "last_name", "headline", "avatar_url", "location",
		}).AddRow("u-public", "Ada", "Public", "Engineer", "", "DC"))

	r := httptest.NewRequest("GET", "/search?q=ada", nil)
	results, err := h.searchUsers(r, "ada", "viewer-1", "employee")
	if err != nil {
		t.Fatalf("searchUsers error: %v", err)
	}
	if len(results) != 1 || results[0].ID != "u-public" {
		t.Fatalf("expected only the public user, got %+v", results)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// TestSearchIncludesPrivateForAdmin verifies that an admin viewer's search does
// NOT apply the private-profile filter (admins can see everyone).
func TestSearchIncludesPrivateForAdmin(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	h := &Handler{db: db}

	searchTerm := "%ada%"

	// For admins the query has no profile_visibility filter; match the common
	// prefix and ensure args are just (viewer, term).
	mock.ExpectQuery("SELECT id, first_name, last_name, headline, avatar_url, location").
		WithArgs("admin-1", searchTerm).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "first_name", "last_name", "headline", "avatar_url", "location",
		}).AddRow("u-private", "Ada", "Private", "Analyst", "", "DC"))

	r := httptest.NewRequest("GET", "/search?q=ada", nil)
	results, err := h.searchUsers(r, "ada", "admin-1", "admin")
	if err != nil {
		t.Fatalf("searchUsers error: %v", err)
	}
	if len(results) != 1 || results[0].ID != "u-private" {
		t.Fatalf("expected the private user visible to admin, got %+v", results)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}
