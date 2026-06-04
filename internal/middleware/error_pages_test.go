package middleware

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestErrorTemplate(title string) *template.Template {
	return template.Must(template.New("error").Parse("<html><body><h1>" + title + "</h1><p>{{.Path}}</p></body></html>"))
}

func TestWithErrorPagesRendersCustomNotFoundForHTMLRequests(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	h := WithErrorPages(next, map[int]*template.Template{
		http.StatusNotFound: newTestErrorTemplate("Not found page"),
	})

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
	if !strings.Contains(strings.ToLower(rec.Header().Get("Content-Type")), "text/html") {
		t.Fatalf("expected text/html content type, got %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "Not found page") {
		t.Fatalf("expected custom not found page body, got %q", rec.Body.String())
	}
}

func TestWithErrorPagesPreservesJSONResponses(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	})

	h := WithErrorPages(next, map[int]*template.Template{
		http.StatusForbidden: newTestErrorTemplate("Forbidden page"),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/thing", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
	if !strings.Contains(strings.ToLower(rec.Header().Get("Content-Type")), "application/json") {
		t.Fatalf("expected application/json content type, got %q", rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != `{"error":"forbidden"}` {
		t.Fatalf("expected json body to pass through, got %q", rec.Body.String())
	}
}

func TestWithErrorPagesRendersCustomForbiddenForHTMLPost(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Forbidden", http.StatusForbidden)
	})

	h := WithErrorPages(next, map[int]*template.Template{
		http.StatusForbidden: newTestErrorTemplate("Forbidden page"),
	})

	req := httptest.NewRequest(http.MethodPost, "/profile/avatar", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
	if !strings.Contains(strings.ToLower(rec.Header().Get("Content-Type")), "text/html") {
		t.Fatalf("expected text/html content type, got %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "Forbidden page") {
		t.Fatalf("expected custom forbidden page body, got %q", rec.Body.String())
	}
}
