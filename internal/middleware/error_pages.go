package middleware

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
)

type ErrorPageData struct {
	Code    int
	Title   string
	Message string
	Path    string
}

// WithErrorPages renders custom HTML pages for browser-facing errors.
// It applies to navigation requests that accept text/html.
func WithErrorPages(next http.Handler, pages map[int]*template.Template) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !wantsHTMLPage(r) {
			next.ServeHTTP(w, r)
			return
		}

		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)

		status := rec.Code
		if status == 0 {
			status = http.StatusOK
		}

		tpl, ok := pages[status]
		if !ok || !canRenderCustomError(rec.Header()) {
			copyRecordedResponse(w, rec)
			return
		}

		for k, vals := range rec.Header() {
			if strings.EqualFold(k, "Content-Type") || strings.EqualFold(k, "Content-Length") {
				continue
			}
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)

		if err := tpl.Execute(w, errorPageData(status, r.URL.Path)); err != nil {
			http.Error(w, http.StatusText(status), status)
		}
	})
}

func wantsHTMLPage(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		return false
	}
	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	if accept == "" {
		return false
	}
	return strings.Contains(accept, "text/html")
}

func canRenderCustomError(h http.Header) bool {
	contentType := strings.ToLower(strings.TrimSpace(h.Get("Content-Type")))
	if contentType == "" {
		return true
	}
	return strings.Contains(contentType, "text/plain") || strings.Contains(contentType, "text/html")
}

func copyRecordedResponse(w http.ResponseWriter, rec *httptest.ResponseRecorder) {
	for k, vals := range rec.Header() {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	if rec.Code > 0 {
		w.WriteHeader(rec.Code)
	}
	_, _ = w.Write(rec.Body.Bytes())
}

func errorPageData(code int, path string) ErrorPageData {
	data := ErrorPageData{Code: code, Path: path}
	switch code {
	case http.StatusForbidden:
		data.Title = "Access denied"
		data.Message = "You do not have permission to access this resource, or your request failed security checks."
	case http.StatusNotFound:
		data.Title = "Page not found"
		data.Message = "The page you requested does not exist or may have been moved."
	case http.StatusInternalServerError:
		data.Title = "Server error"
		data.Message = "Something went wrong while processing your request. Please try again."
	default:
		data.Title = "Request error"
		data.Message = "The request could not be completed."
	}
	return data
}
