package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSafeRedirectTarget(t *testing.T) {
	fallback := "/feed"
	tests := []struct {
		in   string
		want string
	}{
		{"/profile/me", "/profile/me"},
		{"https://evil.example/phish", fallback},
		{"//evil.example", fallback},
		{"", fallback},
	}

	for _, tt := range tests {
		if got := SafeRedirectTarget(tt.in, fallback); got != tt.want {
			t.Errorf("SafeRedirectTarget(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRequireSameOriginUnsafeMethods(t *testing.T) {
	passed := false
	h := RequireSameOriginUnsafeMethods(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		passed = true
		w.WriteHeader(http.StatusNoContent)
	}))

	// Same-host Origin passes.
	{
		passed = false
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/feed", nil)
		req.Host = "localhost:8080"
		req.Header.Set("Origin", "http://localhost:8080")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent || !passed {
			t.Fatalf("expected same-origin POST to pass, code=%d passed=%v", rec.Code, passed)
		}
	}

	// Cross-site Origin is blocked.
	{
		passed = false
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/feed", nil)
		req.Host = "localhost:8080"
		req.Header.Set("Origin", "http://evil.example")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || passed {
			t.Fatalf("expected cross-origin POST to be blocked, code=%d passed=%v", rec.Code, passed)
		}
	}

	// Same host but different port is blocked.
	{
		passed = false
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/feed", nil)
		req.Host = "localhost:8080"
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || passed {
			t.Fatalf("expected cross-port POST to be blocked, code=%d passed=%v", rec.Code, passed)
		}
	}

	// Same host and port but different scheme is blocked.
	{
		passed = false
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/feed", nil)
		req.Host = "localhost:8080"
		req.Header.Set("Origin", "https://localhost:8080")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || passed {
			t.Fatalf("expected cross-scheme POST to be blocked, code=%d passed=%v", rec.Code, passed)
		}
	}

	// Missing Origin/Referer is blocked.
	{
		passed = false
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/feed", nil)
		req.Host = "localhost:8080"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || passed {
			t.Fatalf("expected no-origin POST to be blocked, code=%d passed=%v", rec.Code, passed)
		}
	}
}
