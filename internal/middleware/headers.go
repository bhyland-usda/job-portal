package middleware

import "net/http"

// WithSecurityHeaders applies baseline response hardening headers for all
// responses while preserving any route-specific CSP overrides.
func WithSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if w.Header().Get("Content-Security-Policy") == "" {
			w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'; base-uri 'self'")
		}
		next.ServeHTTP(w, r)
	})
}
