package feed

import (
	"context"
	"database/sql"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// LinkPreview is a cached Open Graph preview of an external URL.
type LinkPreview struct {
	URL         string
	Title       string
	Description string
	ImageURL    string
}

// urlRegex matches the first http(s) URL in a string. Trailing sentence
// punctuation is trimmed separately in extractFirstURL.
var urlRegex = regexp.MustCompile(`https?://[^\s<>"]+`)

// extractFirstURL returns the first http(s) URL found in s, with trailing
// sentence punctuation removed. Returns "" if none is present.
func extractFirstURL(s string) string {
	u := urlRegex.FindString(s)
	if u == "" {
		return ""
	}
	u = strings.TrimRight(u, ".,;:!?)]}'\"")
	return u
}

var (
	titleTagRegex = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	metaTagRegex  = regexp.MustCompile(`(?is)<meta\b[^>]*>`)
	attrRegex     = regexp.MustCompile(`(?is)(property|name|content)\s*=\s*("([^"]*)"|'([^']*)'|([^\s>]+))`)
)

// metaAttr extracts the property/name and content attributes from a single
// <meta> tag string.
func metaAttr(tag string) (key, content string) {
	for _, m := range attrRegex.FindAllStringSubmatch(tag, -1) {
		val := m[3]
		if val == "" {
			val = m[4]
		}
		if val == "" {
			val = m[5]
		}
		switch strings.ToLower(m[1]) {
		case "property", "name":
			if key == "" {
				key = strings.ToLower(strings.TrimSpace(val))
			}
		case "content":
			content = html.UnescapeString(strings.TrimSpace(val))
		}
	}
	return key, content
}

// parseOGMetadata extracts Open Graph (and fallback) metadata from raw HTML.
// Preference order: og:title then <title>; og:description then
// <meta name="description">; og:image. Parsing is best-effort and never errors.
func parseOGMetadata(htmlStr string) LinkPreview {
	var meta LinkPreview
	var ogTitle, htmlTitle, ogDesc, metaDesc string

	if m := titleTagRegex.FindStringSubmatch(htmlStr); m != nil {
		htmlTitle = html.UnescapeString(strings.TrimSpace(m[1]))
	}

	for _, tag := range metaTagRegex.FindAllString(htmlStr, -1) {
		key, content := metaAttr(tag)
		if content == "" {
			continue
		}
		switch key {
		case "og:title":
			ogTitle = content
		case "og:description":
			ogDesc = content
		case "og:image":
			if meta.ImageURL == "" {
				meta.ImageURL = content
			}
		case "description":
			if metaDesc == "" {
				metaDesc = content
			}
		}
	}

	if ogTitle != "" {
		meta.Title = ogTitle
	} else {
		meta.Title = htmlTitle
	}
	if ogDesc != "" {
		meta.Description = ogDesc
	} else {
		meta.Description = metaDesc
	}
	return meta
}

// previewClient is used to fetch remote pages with a short timeout so a slow or
// hostile server can never block post creation.
var previewClient = &http.Client{Timeout: 5 * time.Second}

// fetchAndCachePreview fetches Open Graph metadata for url and upserts it into
// link_previews. Failures are non-fatal: callers ignore the returned error.
func fetchAndCachePreview(ctx context.Context, db *sql.DB, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "job-portal-linkpreview/1.0")

	resp, err := previewClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}

	// Only parse the head; cap the read so a huge body can't exhaust memory.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil {
		return err
	}

	meta := parseOGMetadata(string(body))
	if meta.Title == "" && meta.Description == "" && meta.ImageURL == "" {
		return nil
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO link_previews (url, title, description, image_url, fetched_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (url) DO UPDATE
		   SET title = EXCLUDED.title,
		       description = EXCLUDED.description,
		       image_url = EXCLUDED.image_url,
		       fetched_at = NOW()`,
		url, meta.Title, meta.Description, meta.ImageURL,
	)
	return err
}

// loadPreview returns the cached preview for url, or nil if none is cached.
func loadPreview(ctx context.Context, db *sql.DB, url string) *LinkPreview {
	var p LinkPreview
	p.URL = url
	err := db.QueryRowContext(ctx,
		`SELECT COALESCE(title, ''), COALESCE(description, ''), COALESCE(image_url, '')
		 FROM link_previews WHERE url = $1`, url,
	).Scan(&p.Title, &p.Description, &p.ImageURL)
	if err != nil {
		return nil
	}
	if p.Title == "" && p.Description == "" && p.ImageURL == "" {
		return nil
	}
	return &p
}
