package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// helper: build a request carrying an authenticated user in context.
func authedRequest(method, path, userID string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(r.Context(), UserIDKey, userID)
	return r.WithContext(ctx)
}

// okHandler records that the wrapped handler was reached.
func okHandler(reached *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireAUP_RedirectsUnacceptedUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// aup_accepted_at IS NULL -> not yet accepted
	mock.ExpectQuery("SELECT aup_accepted_at FROM users").
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"aup_accepted_at"}).AddRow(nil))

	reached := false
	h := RequireAUP(db)(okHandler(&reached))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, authedRequest("GET", "/feed", "user-1"))

	if reached {
		t.Error("expected wrapped handler NOT to be reached for un-accepted user")
	}
	if w.Code != http.StatusSeeOther {
		t.Errorf("expected status %d, got %d", http.StatusSeeOther, w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/aup" {
		t.Errorf("expected redirect to /aup, got %q", loc)
	}
}

func TestRequireAUP_AllowsAcceptedUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// aup_accepted_at is set -> accepted
	mock.ExpectQuery("SELECT aup_accepted_at FROM users").
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"aup_accepted_at"}).AddRow(time.Now()))

	reached := false
	h := RequireAUP(db)(okHandler(&reached))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, authedRequest("GET", "/feed", "user-1"))

	if !reached {
		t.Error("expected wrapped handler to be reached for accepted user")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRequireAUP_AllowsExemptPaths(t *testing.T) {
	// These paths must pass through WITHOUT a DB lookup and WITHOUT a redirect,
	// otherwise the AUP gate would loop or block logout/static/the AUP pages.
	exempt := []string{"/aup", "/aup/accept", "/logout", "/static/css/main.css"}

	for _, p := range exempt {
		t.Run(p, func(t *testing.T) {
			db, _, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock db: %v", err)
			}
			defer db.Close()
			// No DB expectations set: a query here would fail the test.

			reached := false
			h := RequireAUP(db)(okHandler(&reached))

			w := httptest.NewRecorder()
			h.ServeHTTP(w, authedRequest("GET", p, "user-1"))

			if !reached {
				t.Errorf("expected exempt path %q to pass through", p)
			}
			if w.Code == http.StatusSeeOther {
				t.Errorf("exempt path %q should not be redirected", p)
			}
		})
	}
}

func TestRequireAUP_PassesThroughWhenUnauthenticated(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()
	// No user in context, no DB expectations.

	reached := false
	h := RequireAUP(db)(okHandler(&reached))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/feed", nil))

	if !reached {
		t.Error("expected unauthenticated request to pass through (auth handled elsewhere)")
	}
}
