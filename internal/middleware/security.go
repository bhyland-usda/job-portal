package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

// RequireSameOriginUnsafeMethods blocks cross-site unsafe requests by requiring
// Origin/Referer to match the request host for POST/PUT/PATCH/DELETE.
func RequireSameOriginUnsafeMethods(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		expectedOrigin := requestOrigin(r)
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		referer := strings.TrimSpace(r.Referer())

		if origin != "" {
			if !matchesRequestOrigin(origin, expectedOrigin) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if referer != "" {
			if !matchesRequestOrigin(referer, expectedOrigin) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Unsafe requests without either header are denied.
		http.Error(w, "Forbidden", http.StatusForbidden)
	})
}

func matchesRequestOrigin(raw, expected string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if u.Scheme == "" || u.Host == "" {
		return false
	}
	return canonicalOrigin(u.Scheme, u.Host) == expected
}

// SafeRedirectTarget ensures redirects remain in-app.
func SafeRedirectTarget(raw, fallback string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return fallback
	}
	if !strings.HasPrefix(v, "/") || strings.HasPrefix(v, "//") {
		return fallback
	}
	if strings.Contains(v, "\\") {
		return fallback
	}
	return v
}

func canonicalHost(h string) string {
	h = strings.TrimSpace(strings.ToLower(h))
	if h == "" {
		return ""
	}
	if strings.HasPrefix(h, "[") {
		end := strings.Index(h, "]")
		if end != -1 {
			return strings.TrimPrefix(strings.TrimSuffix(h[:end+1], "]"), "[")
		}
	}
	if i := strings.Index(h, ":"); i != -1 {
		return h[:i]
	}
	return h
}

func requestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))); forwarded != "" {
		scheme = forwarded
	}
	return canonicalOrigin(scheme, r.Host)
}

func canonicalOrigin(scheme, host string) string {
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	host = strings.ToLower(strings.TrimSpace(host))
	if scheme == "" || host == "" {
		return ""
	}
	return scheme + "://" + host
}
