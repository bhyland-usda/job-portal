package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

const (
	CSRFTokenCookie = "csrf_token"
	CSRFTokenField  = "csrf_token"
	CSRFTokenHeader = "X-CSRF-Token"
)

type csrfCtxKey struct{}

func GetCSRFToken(ctx context.Context) string {
	t, _ := ctx.Value(csrfCtxKey{}).(string)
	return t
}

func RequireCSRFTokens(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := readCSRFCookie(r)
		if token == "" {
			token = generateCSRFToken()
			http.SetCookie(w, &http.Cookie{
				Name:     CSRFTokenCookie,
				Value:    token,
				Path:     "/",
				HttpOnly: false,
				Secure:   csrfCookieSecure(r),
				SameSite: http.SameSiteLaxMode,
			})
		}

		ctx := context.WithValue(r.Context(), csrfCtxKey{}, token)
		r = r.WithContext(ctx)

		if !isUnsafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		provided := strings.TrimSpace(r.Header.Get(CSRFTokenHeader))
		if provided == "" {
			ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
			if strings.HasPrefix(ct, "multipart/form-data") {
				_ = r.ParseMultipartForm(32 << 20)
			} else {
				_ = r.ParseForm()
			}
			provided = strings.TrimSpace(r.FormValue(CSRFTokenField))
		}

		if token == "" || provided == "" || subtle.ConstantTimeCompare([]byte(token), []byte(provided)) != 1 {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func readCSRFCookie(r *http.Request) string {
	c, err := r.Cookie(CSRFTokenCookie)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(c.Value)
}

func generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func csrfCookieSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	proto := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")))
	return proto == "https"
}
