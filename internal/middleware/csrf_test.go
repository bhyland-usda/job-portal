package middleware

import (
	"bytes"
	"crypto/tls"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequireCSRFTokensIssuesCookieOnGet(t *testing.T) {
	h := RequireCSRFTokens(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/feed", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("expected csrf cookie to be set")
	}
}

func TestRequireCSRFTokensBlocksUnsafeWithoutToken(t *testing.T) {
	h := RequireCSRFTokens(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/feed", strings.NewReader("x=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRequireCSRFTokensAllowsUnsafeWithMatchingHeader(t *testing.T) {
	h := RequireCSRFTokens(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/feed", nil)
	req.AddCookie(&http.Cookie{Name: CSRFTokenCookie, Value: "abc123"})
	req.Header.Set(CSRFTokenHeader, "abc123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestRequireCSRFTokensBlocksLoginWithoutToken(t *testing.T) {
	h := RequireCSRFTokens(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/login", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected login POST without token to be blocked, got %d", rec.Code)
	}
}

func TestRequireCSRFTokensAllowsUnsafeWithMatchingFormToken(t *testing.T) {
	h := RequireCSRFTokens(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/login", strings.NewReader("csrf_token=abc123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: CSRFTokenCookie, Value: "abc123"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestRequireCSRFTokensAllowsMultipartWithMatchingFormToken(t *testing.T) {
	h := RequireCSRFTokens(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("csrf_token", "abc123"); err != nil {
		t.Fatalf("failed to write csrf field: %v", err)
	}
	if err := w.WriteField("avatar", "dummy"); err != nil {
		t.Fatalf("failed to write dummy field: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/profile/avatar", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: CSRFTokenCookie, Value: "abc123"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for multipart form token, got %d", rec.Code)
	}
}

func TestCSRFCookieSecureByScheme(t *testing.T) {
	plainReq := httptest.NewRequest(http.MethodGet, "http://example.test/feed", nil)
	if csrfCookieSecure(plainReq) {
		t.Fatal("expected non-TLS request to use non-secure CSRF cookie")
	}

	httpsReq := httptest.NewRequest(http.MethodGet, "https://example.test/feed", nil)
	httpsReq.TLS = &tls.ConnectionState{}
	if !csrfCookieSecure(httpsReq) {
		t.Fatal("expected TLS request to use secure CSRF cookie")
	}

	proxiedReq := httptest.NewRequest(http.MethodGet, "http://example.test/feed", nil)
	proxiedReq.Header.Set("X-Forwarded-Proto", "https")
	if !csrfCookieSecure(proxiedReq) {
		t.Fatal("expected X-Forwarded-Proto=https request to use secure CSRF cookie")
	}
}
