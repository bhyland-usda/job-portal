// Package attachment provides shared file validation for all attachment contexts.
package attachment

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// Context defines which attachment rules apply
type Context string

const (
	ContextPost    Context = "post"    // Social posts: images and videos only
	ContextPosting Context = "posting" // Detail/project postings: documents and images
	ContextMessage Context = "message" // Chat messages: documents, images, code, archives, videos
)

// Category classifies the file for rendering decisions
type Category string

const (
	CategoryImage    Category = "image"
	CategoryVideo    Category = "video"
	CategoryDocument Category = "document"
	CategoryCode     Category = "code"
	CategoryArchive  Category = "archive"
)

// Validated holds a validated file ready for storage
type Validated struct {
	Data         []byte
	OriginalName string
	ContentType  string
	Category     Category
	FileSize     int
}

// Size limits per category
var maxSizes = map[Category]int{
	CategoryImage:    10 << 20,  // 10 MB
	CategoryVideo:    100 << 20, // 100 MB
	CategoryDocument: 25 << 20,  // 25 MB
	CategoryCode:     5 << 20,   // 5 MB
	CategoryArchive:  50 << 20,  // 50 MB
}

// Allowed MIME types per context
var allowedTypes = map[Context]map[string]Category{
	ContextPost: {
		"image/jpeg":      CategoryImage,
		"image/png":       CategoryImage,
		"image/gif":       CategoryImage,
		"image/webp":      CategoryImage,
		"video/mp4":       CategoryVideo,
		"video/webm":      CategoryVideo,
		"video/quicktime": CategoryVideo,
	},
	ContextPosting: {
		"image/jpeg":         CategoryImage,
		"image/png":          CategoryImage,
		"image/gif":          CategoryImage,
		"image/webp":         CategoryImage,
		"application/pdf":    CategoryDocument,
		"application/msword": CategoryDocument,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   CategoryDocument,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         CategoryDocument,
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": CategoryDocument,
		"application/vnd.ms-excel":      CategoryDocument,
		"application/vnd.ms-powerpoint": CategoryDocument,
		"text/csv":                      CategoryDocument,
		"text/plain":                    CategoryDocument,
	},
	ContextMessage: {
		// Images
		"image/jpeg": CategoryImage,
		"image/png":  CategoryImage,
		"image/gif":  CategoryImage,
		"image/webp": CategoryImage,
		// Videos
		"video/mp4":       CategoryVideo,
		"video/webm":      CategoryVideo,
		"video/quicktime": CategoryVideo,
		// Documents
		"application/pdf":    CategoryDocument,
		"application/msword": CategoryDocument,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   CategoryDocument,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         CategoryDocument,
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": CategoryDocument,
		"application/vnd.ms-excel":      CategoryDocument,
		"application/vnd.ms-powerpoint": CategoryDocument,
		"text/csv":                      CategoryDocument,
		"text/plain":                    CategoryDocument,
		// Archives (forced download only)
		"application/zip":   CategoryArchive,
		"application/gzip":  CategoryArchive,
		"application/x-tar": CategoryArchive,
		// Code files (forced download only, detect by extension)
		"text/x-go":     CategoryCode,
		"text/x-python": CategoryCode,
		"text/x-java":   CategoryCode,
		"text/x-c":      CategoryCode,
	},
}

// Code file extensions (detected by name since MIME detection is unreliable for code)
var codeExtensions = map[string]bool{
	".go": true, ".py": true, ".js": true, ".ts": true, ".java": true,
	".c": true, ".cpp": true, ".h": true, ".rs": true, ".rb": true,
	".sh": true, ".sql": true, ".yaml": true, ".yml": true, ".json": true,
	".xml": true, ".toml": true, ".ini": true, ".cfg": true, ".conf": true,
}

// ValidateFile validates an uploaded file against the rules for the given context.
func ValidateFile(file multipart.File, header *multipart.FileHeader, ctx Context) (*Validated, error) {
	// Read magic bytes for content type detection
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read file")
	}

	detectedType := http.DetectContentType(buf[:n])

	// Check if code file by extension (MIME detection often returns text/plain for code)
	ext := ""
	if dotIdx := strings.LastIndex(header.Filename, "."); dotIdx >= 0 {
		ext = strings.ToLower(header.Filename[dotIdx:])
	}

	allowed := allowedTypes[ctx]
	if allowed == nil {
		return nil, fmt.Errorf("unknown attachment context")
	}

	// Determine category
	category, ok := allowed[detectedType]
	if !ok {
		// Check code extensions for message context
		if ctx == ContextMessage && codeExtensions[ext] {
			category = CategoryCode
		} else {
			return nil, fmt.Errorf("file type %s is not allowed for %s attachments", detectedType, ctx)
		}
	}

	// Check size limit
	maxSize := maxSizes[category]
	if header.Size > int64(maxSize) {
		return nil, fmt.Errorf("file too large: %d bytes exceeds %d MB limit for %s files",
			header.Size, maxSize>>20, category)
	}

	// Seek back to start and read full file
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to process file")
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file")
	}

	return &Validated{
		Data:         data,
		OriginalName: header.Filename,
		ContentType:  detectedType,
		Category:     category,
		FileSize:     len(data),
	}, nil
}

// IsAnimatedImage returns true if the content type is an animated image format (GIF).
// Videos are NOT included — use this to distinguish GIFs from static images in templates.
func IsAnimatedImage(contentType string) bool {
	return contentType == "image/gif"
}

// ServeHeaders sets appropriate security headers based on file category.
func ServeHeaders(w http.ResponseWriter, contentType, originalName string, category Category) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")

	switch category {
	case CategoryCode, CategoryArchive:
		// Force download — never render in browser
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, originalName))
	case CategoryVideo, CategoryImage:
		// Allow inline rendering
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, originalName))
	default:
		// Documents — allow inline for PDF, force download for others
		if contentType == "application/pdf" {
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, originalName))
		} else {
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, originalName))
		}
	}
}
