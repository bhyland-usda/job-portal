package feed

import (
	"net"
	"testing"
)

// TestParseOGMetadata verifies extraction of title, description, and image from
// an HTML head containing Open Graph and standard tags. This needs no network.
func TestParseOGMetadata(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
  <title>Fallback Title</title>
  <meta property="og:title" content="Open Graph Title" />
  <meta name="description" content="Fallback description" />
  <meta property="og:description" content="An OG description here" />
  <meta property="og:image" content="https://example.com/img.png" />
</head>
<body>ignored</body>
</html>`

	meta := parseOGMetadata(html)

	if meta.Title != "Open Graph Title" {
		t.Errorf("title: expected %q, got %q", "Open Graph Title", meta.Title)
	}
	if meta.Description != "An OG description here" {
		t.Errorf("description: expected %q, got %q", "An OG description here", meta.Description)
	}
	if meta.ImageURL != "https://example.com/img.png" {
		t.Errorf("image: expected %q, got %q", "https://example.com/img.png", meta.ImageURL)
	}
}

// TestParseOGMetadataTitleFallback verifies that with no og:title we fall back to
// the <title> element, and missing fields stay empty.
func TestParseOGMetadataTitleFallback(t *testing.T) {
	html := `<head><title>Just A Title</title></head>`

	meta := parseOGMetadata(html)

	if meta.Title != "Just A Title" {
		t.Errorf("title: expected %q, got %q", "Just A Title", meta.Title)
	}
	if meta.Description != "" {
		t.Errorf("description: expected empty, got %q", meta.Description)
	}
	if meta.ImageURL != "" {
		t.Errorf("image: expected empty, got %q", meta.ImageURL)
	}
}

// TestParseOGMetadataDescriptionFallback verifies that with no og:description we
// fall back to the standard <meta name="description"> tag.
func TestParseOGMetadataDescriptionFallback(t *testing.T) {
	html := `<head>
  <meta name="description" content="standard desc">
  <meta content="reverse order title" property="og:title">
</head>`

	meta := parseOGMetadata(html)

	if meta.Title != "reverse order title" {
		t.Errorf("title: expected %q, got %q", "reverse order title", meta.Title)
	}
	if meta.Description != "standard desc" {
		t.Errorf("description: expected %q, got %q", "standard desc", meta.Description)
	}
}

// TestExtractFirstURL verifies the first http(s) URL is pulled from post content.
func TestExtractFirstURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"check this https://example.com/page out", "https://example.com/page"},
		{"two http://a.test and https://b.test", "http://a.test"},
		{"no url here", ""},
		{"trailing punctuation https://example.com.", "https://example.com"},
		{"ftp://nope.test only", ""},
	}
	for _, tt := range tests {
		if got := extractFirstURL(tt.input); got != tt.want {
			t.Errorf("extractFirstURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidatePreviewURL(t *testing.T) {
	origLookup := lookupIP
	defer func() { lookupIP = origLookup }()

	lookupIP = func(host string) ([]net.IP, error) {
		switch host {
		case "example.com":
			return []net.IP{net.ParseIP("93.184.216.34")}, nil
		case "internal.local":
			return []net.IP{net.ParseIP("127.0.0.1")}, nil
		default:
			return []net.IP{net.ParseIP("93.184.216.34")}, nil
		}
	}

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "public http", url: "http://example.com/page", wantErr: false},
		{name: "localhost blocked", url: "http://localhost:8080/login", wantErr: true},
		{name: "loopback ip blocked", url: "http://127.0.0.1/admin", wantErr: true},
		{name: "non-http blocked", url: "file:///etc/passwd", wantErr: true},
	}

	for _, tt := range tests {
		err := validatePreviewURL(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: validatePreviewURL(%q) err=%v, wantErr=%v", tt.name, tt.url, err, tt.wantErr)
		}
	}
}
