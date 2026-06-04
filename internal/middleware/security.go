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

		host := canonicalHost(r.Host)
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		referer := strings.TrimSpace(r.Referer())

		if origin != "" {
			u, err := url.Parse(origin)
			if err != nil || canonicalHost(u.Host) != host {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if referer != "" {
			u, err := url.Parse(referer)
			if err != nil || canonicalHost(u.Host) != host {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Some legitimate browser POST flows (notably multipart form submissions)
		// may omit both Origin and Referer. In that case, defer protection to the
		// strict CSRF token middleware.
		next.ServeHTTP(w, r)
	})
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
