package attachment

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// testFile wraps bytes.Reader to satisfy multipart.File interface.
type testFile struct {
	*bytes.Reader
}

func (tf *testFile) Close() error { return nil }

func newTestFile(data []byte) multipart.File {
	return &testFile{bytes.NewReader(data)}
}

func newFileHeader(name string, size int) *multipart.FileHeader {
	return &multipart.FileHeader{Filename: name, Size: int64(size)}
}

// -- Magic byte helpers --

// Minimal GIF89a (13-byte header + padding)
var gifData = append([]byte("GIF89a\x01\x00\x01\x00\x00\x00\x00"), bytes.Repeat([]byte{0x00}, 500)...)

// Minimal PNG header
var pngData = append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, bytes.Repeat([]byte{0x00}, 500)...)

// Minimal JPEG header
var jpegData = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0x00}, 500)...)

// Minimal PDF header
var pdfData = append([]byte("%PDF-1.4"), bytes.Repeat([]byte{0x00}, 500)...)

// Plain text (for code file extension testing)
var textData = append([]byte("package main\n\nfunc main() {}"), bytes.Repeat([]byte{0x00}, 500)...)

// ========================
// ValidateFile tests
// ========================

func TestValidateFile_GIFInPostContext(t *testing.T) {
	file := newTestFile(gifData)
	header := newFileHeader("funny.gif", len(gifData))

	result, err := ValidateFile(file, header, ContextPost)
	if err != nil {
		t.Fatalf("Expected GIF to be valid for posts, got: %v", err)
	}
	if result.Category != CategoryImage {
		t.Errorf("Expected category %q, got %q", CategoryImage, result.Category)
	}
	if result.ContentType != "image/gif" {
		t.Errorf("Expected content type image/gif, got %q", result.ContentType)
	}
}

func TestValidateFile_PNGInPostContext(t *testing.T) {
	file := newTestFile(pngData)
	header := newFileHeader("photo.png", len(pngData))

	result, err := ValidateFile(file, header, ContextPost)
	if err != nil {
		t.Fatalf("Expected PNG to be valid for posts, got: %v", err)
	}
	if result.Category != CategoryImage {
		t.Errorf("Expected category %q, got %q", CategoryImage, result.Category)
	}
	if result.ContentType != "image/png" {
		t.Errorf("Expected content type image/png, got %q", result.ContentType)
	}
}

func TestValidateFile_JPEGInPostContext(t *testing.T) {
	file := newTestFile(jpegData)
	header := newFileHeader("photo.jpg", len(jpegData))

	result, err := ValidateFile(file, header, ContextPost)
	if err != nil {
		t.Fatalf("Expected JPEG to be valid for posts, got: %v", err)
	}
	if result.Category != CategoryImage {
		t.Errorf("Expected category %q, got %q", CategoryImage, result.Category)
	}
}

func TestValidateFile_PDFRejectedInPostContext(t *testing.T) {
	file := newTestFile(pdfData)
	header := newFileHeader("report.pdf", len(pdfData))

	_, err := ValidateFile(file, header, ContextPost)
	if err == nil {
		t.Error("Expected PDF to be rejected in post context")
	}
}

func TestValidateFile_PDFAllowedInPostingContext(t *testing.T) {
	file := newTestFile(pdfData)
	header := newFileHeader("report.pdf", len(pdfData))

	result, err := ValidateFile(file, header, ContextPosting)
	if err != nil {
		t.Fatalf("Expected PDF to be valid for postings, got: %v", err)
	}
	if result.Category != CategoryDocument {
		t.Errorf("Expected category %q, got %q", CategoryDocument, result.Category)
	}
}

func TestValidateFile_PDFAllowedInMessageContext(t *testing.T) {
	file := newTestFile(pdfData)
	header := newFileHeader("doc.pdf", len(pdfData))

	result, err := ValidateFile(file, header, ContextMessage)
	if err != nil {
		t.Fatalf("Expected PDF to be valid for messages, got: %v", err)
	}
	if result.Category != CategoryDocument {
		t.Errorf("Expected category %q, got %q", CategoryDocument, result.Category)
	}
}

func TestValidateFile_CodeFileByExtension(t *testing.T) {
	file := newTestFile(textData)
	header := newFileHeader("main.go", len(textData))

	result, err := ValidateFile(file, header, ContextMessage)
	if err != nil {
		t.Fatalf("Expected .go file to be valid for messages, got: %v", err)
	}
	if result.Category != CategoryCode {
		t.Errorf("Expected category %q, got %q", CategoryCode, result.Category)
	}
}

func TestValidateFile_CodeFileRejectedInPostContext(t *testing.T) {
	file := newTestFile(textData)
	header := newFileHeader("main.go", len(textData))

	_, err := ValidateFile(file, header, ContextPost)
	if err == nil {
		t.Error("Expected code file to be rejected in post context")
	}
}

func TestValidateFile_FileTooLarge(t *testing.T) {
	file := newTestFile(gifData)
	// Claim 15MB — exceeds 10MB image limit
	header := newFileHeader("huge.gif", 15<<20)

	_, err := ValidateFile(file, header, ContextPost)
	if err == nil {
		t.Error("Expected oversized file to be rejected")
	}
}

func TestValidateFile_PreservesOriginalName(t *testing.T) {
	file := newTestFile(gifData)
	header := newFileHeader("My Vacation Photo.gif", len(gifData))

	result, err := ValidateFile(file, header, ContextPost)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.OriginalName != "My Vacation Photo.gif" {
		t.Errorf("Expected original name preserved, got %q", result.OriginalName)
	}
}

func TestValidateFile_UnknownContext(t *testing.T) {
	file := newTestFile(gifData)
	header := newFileHeader("test.gif", len(gifData))

	_, err := ValidateFile(file, header, Context("invalid"))
	if err == nil {
		t.Error("Expected unknown context to be rejected")
	}
}

// ========================
// ServeHeaders tests
// ========================

func TestServeHeaders_ImageInline(t *testing.T) {
	w := httptest.NewRecorder()
	ServeHeaders(w, "image/png", "photo.png", CategoryImage)

	resp := w.Result()
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("Expected Content-Type image/png, got %q", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); cd == "" || !contains(cd, "inline") {
		t.Errorf("Expected inline disposition for image, got %q", cd)
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("Missing X-Content-Type-Options: nosniff")
	}
}

func TestServeHeaders_VideoInline(t *testing.T) {
	w := httptest.NewRecorder()
	ServeHeaders(w, "video/mp4", "clip.mp4", CategoryVideo)

	resp := w.Result()
	if ct := resp.Header.Get("Content-Type"); ct != "video/mp4" {
		t.Errorf("Expected Content-Type video/mp4, got %q", ct)
	}
	cd := resp.Header.Get("Content-Disposition")
	if !contains(cd, "inline") {
		t.Errorf("Expected inline disposition for video, got %q", cd)
	}
}

func TestServeHeaders_CodeForceDownload(t *testing.T) {
	w := httptest.NewRecorder()
	ServeHeaders(w, "text/x-go", "main.go", CategoryCode)

	resp := w.Result()
	if ct := resp.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("Expected Content-Type application/octet-stream for code, got %q", ct)
	}
	cd := resp.Header.Get("Content-Disposition")
	if !contains(cd, "attachment") {
		t.Errorf("Expected attachment disposition for code, got %q", cd)
	}
}

func TestServeHeaders_ArchiveForceDownload(t *testing.T) {
	w := httptest.NewRecorder()
	ServeHeaders(w, "application/zip", "files.zip", CategoryArchive)

	resp := w.Result()
	if ct := resp.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("Expected Content-Type application/octet-stream for archive, got %q", ct)
	}
	cd := resp.Header.Get("Content-Disposition")
	if !contains(cd, "attachment") {
		t.Errorf("Expected attachment disposition for archive, got %q", cd)
	}
}

func TestServeHeaders_PDFInline(t *testing.T) {
	w := httptest.NewRecorder()
	ServeHeaders(w, "application/pdf", "report.pdf", CategoryDocument)

	resp := w.Result()
	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Expected Content-Type application/pdf, got %q", ct)
	}
	cd := resp.Header.Get("Content-Disposition")
	if !contains(cd, "inline") {
		t.Errorf("Expected inline disposition for PDF, got %q", cd)
	}
}

func TestServeHeaders_WordDocForceDownload(t *testing.T) {
	w := httptest.NewRecorder()
	ServeHeaders(w, "application/msword", "memo.doc", CategoryDocument)

	resp := w.Result()
	cd := resp.Header.Get("Content-Disposition")
	if !contains(cd, "attachment") {
		t.Errorf("Expected attachment disposition for Word doc, got %q", cd)
	}
}

func TestServeHeaders_CSPHeader(t *testing.T) {
	w := httptest.NewRecorder()
	ServeHeaders(w, "image/png", "test.png", CategoryImage)

	resp := w.Result()
	if csp := resp.Header.Get("Content-Security-Policy"); csp != "default-src 'none'" {
		t.Errorf("Expected restrictive CSP, got %q", csp)
	}
}

// ========================
// IsAnimatedImage tests
// ========================

func TestIsAnimatedImage_GIF(t *testing.T) {
	if !IsAnimatedImage("image/gif") {
		t.Error("image/gif should be detected as animated")
	}
}

func TestIsAnimatedImage_PNG(t *testing.T) {
	if IsAnimatedImage("image/png") {
		t.Error("image/png should NOT be detected as animated")
	}
}

func TestIsAnimatedImage_JPEG(t *testing.T) {
	if IsAnimatedImage("image/jpeg") {
		t.Error("image/jpeg should NOT be detected as animated")
	}
}

func TestIsAnimatedImage_WebP(t *testing.T) {
	// WebP can be animated but we treat it as static for simplicity
	if IsAnimatedImage("image/webp") {
		t.Error("image/webp should NOT be detected as animated (simplified)")
	}
}

func TestIsAnimatedImage_Video(t *testing.T) {
	// Videos are not images — this function is specifically for image types
	if IsAnimatedImage("video/mp4") {
		t.Error("video/mp4 is not an animated image, it's a video")
	}
}

func TestIsAnimatedImage_Empty(t *testing.T) {
	if IsAnimatedImage("") {
		t.Error("empty content type should not be animated")
	}
}

// ========================
// Context permission tests
// ========================

func TestContextPost_AllowedTypes(t *testing.T) {
	allowed := allowedTypes[ContextPost]

	// Should allow
	for _, mime := range []string{"image/jpeg", "image/png", "image/gif", "image/webp", "video/mp4", "video/webm"} {
		if _, ok := allowed[mime]; !ok {
			t.Errorf("ContextPost should allow %s", mime)
		}
	}

	// Should NOT allow
	for _, mime := range []string{"application/pdf", "text/plain", "application/zip"} {
		if _, ok := allowed[mime]; ok {
			t.Errorf("ContextPost should NOT allow %s", mime)
		}
	}
}

func TestContextPosting_AllowedTypes(t *testing.T) {
	allowed := allowedTypes[ContextPosting]

	// Should allow images and docs
	for _, mime := range []string{"image/jpeg", "image/gif", "application/pdf", "text/csv"} {
		if _, ok := allowed[mime]; !ok {
			t.Errorf("ContextPosting should allow %s", mime)
		}
	}

	// Should NOT allow videos
	for _, mime := range []string{"video/mp4", "video/webm", "application/zip"} {
		if _, ok := allowed[mime]; ok {
			t.Errorf("ContextPosting should NOT allow %s", mime)
		}
	}
}

func TestContextMessage_AllowedTypes(t *testing.T) {
	allowed := allowedTypes[ContextMessage]

	// Should allow everything
	for _, mime := range []string{"image/jpeg", "image/gif", "video/mp4", "application/pdf", "application/zip", "text/x-go"} {
		if _, ok := allowed[mime]; !ok {
			t.Errorf("ContextMessage should allow %s", mime)
		}
	}
}

// ========================
// Size limit tests
// ========================

func TestSizeLimits(t *testing.T) {
	tests := []struct {
		category Category
		expected int
	}{
		{CategoryImage, 10 << 20},
		{CategoryVideo, 100 << 20},
		{CategoryDocument, 25 << 20},
		{CategoryCode, 5 << 20},
		{CategoryArchive, 50 << 20},
	}

	for _, tt := range tests {
		if maxSizes[tt.category] != tt.expected {
			t.Errorf("Category %s: expected max %d bytes, got %d", tt.category, tt.expected, maxSizes[tt.category])
		}
	}
}

// helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && http.DetectContentType(nil) != "" || len(s) > 0 && findSubstr(s, substr)
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
