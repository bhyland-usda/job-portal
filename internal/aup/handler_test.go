package aup

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

func TestHandleAccept_SetsTimestamp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("UPDATE users SET aup_accepted_at = NOW\\(\\) WHERE id =").
		WithArgs("user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	h := NewHandler(db, nil)

	r := httptest.NewRequest("POST", "/aup/accept", nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.UserIDKey, "user-1"))
	w := httptest.NewRecorder()

	h.handleAccept(w, r)

	if w.Code != 303 {
		t.Errorf("expected redirect 303, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/feed" {
		t.Errorf("expected redirect to /feed, got %q", loc)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet db expectations: %v", err)
	}
}

func TestHandleAccept_Unauthenticated(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()
	// No user in context, no UPDATE should run.

	h := NewHandler(db, nil)

	r := httptest.NewRequest("POST", "/aup/accept", nil)
	w := httptest.NewRecorder()

	h.handleAccept(w, r)

	if w.Code != 303 {
		t.Errorf("expected redirect 303 to login, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); !strings.Contains(loc, "login") {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}
