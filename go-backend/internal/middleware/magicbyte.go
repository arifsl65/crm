// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// MagicBytes defines known file type signatures.
// These are the first few bytes of a file that identify its type.
var MagicBytes = map[string][]byte{
	// Images
	"image/jpeg":    {0xFF, 0xD8, 0xFF},
	"image/png":     {0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
	"image/gif":     {0x47, 0x49, 0x46, 0x38},
	"image/webp":    {0x52, 0x49, 0x46, 0x46}, // RIFF header (need to also check for WEBP)
	"image/bmp":     {0x42, 0x4D},
	"image/tiff":    {0x49, 0x49, 0x2A, 0x00}, // Little-endian TIFF
	"image/tiff-be": {0x4D, 0x4D, 0x00, 0x2A}, // Big-endian TIFF

	// Documents
	"application/pdf": {0x25, 0x50, 0x44, 0x46}, // %PDF

	// Office documents (ZIP-based: docx, xlsx, pptx)
	"application/zip": {0x50, 0x4B, 0x03, 0x04}, // PK..

	// Archives
	"application/gzip": {0x1F, 0x8B},
	"application/x-rar-compressed": {0x52, 0x61, 0x72, 0x21}, // Rar!

	// Plain text (no magic bytes, validated separately)
	"text/plain": nil,
	"text/csv":   nil,
}

// FileTypeCategory defines allowed file type categories.
type FileTypeCategory string

const (
	FileTypeCategoryImage    FileTypeCategory = "image"
	FileTypeCategoryDocument FileTypeCategory = "document"
	FileTypeCategoryArchive  FileTypeCategory = "archive"
	FileTypeCategoryText     FileTypeCategory = "text"
)

// AllowedFileTypes maps categories to specific MIME types.
var AllowedFileTypes = map[FileTypeCategory][]string{
	FileTypeCategoryImage: {
		"image/jpeg",
		"image/png",
		"image/gif",
		"image/webp",
		"image/bmp",
	},
	FileTypeCategoryDocument: {
		"application/pdf",
		"application/zip", // Office documents are ZIP files
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",   // docx
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",         // xlsx
		"application/vnd.openxmlformats-officedocument.presentationml.presentation", // pptx
		"application/msword",      // doc
		"application/vnd.ms-excel", // xls
	},
	FileTypeCategoryArchive: {
		"application/zip",
		"application/gzip",
		"application/x-rar-compressed",
	},
	FileTypeCategoryText: {
		"text/plain",
		"text/csv",
	},
}

// MagicByteConfig holds configuration for magic byte validation.
type MagicByteConfig struct {
	// AllowedCategories specifies which file type categories are allowed
	AllowedCategories []FileTypeCategory
	// AllowedTypes specifies exact MIME types allowed (overrides categories if set)
	AllowedTypes []string
	// MaxFileSize is the maximum file size in bytes (0 = no limit)
	MaxFileSize int64
	// FormFieldName is the name of the form field containing the file
	FormFieldName string
}

// ValidateMagicBytes returns a middleware that validates file uploads by magic bytes.
// This provides defense-in-depth against malicious file uploads by verifying
// the actual file content matches the claimed MIME type.
//
// Usage:
//
//	router.POST("/upload", middleware.ValidateMagicBytes(middleware.MagicByteConfig{
//	    AllowedCategories: []middleware.FileTypeCategory{middleware.FileTypeCategoryImage},
//	    MaxFileSize:       10 * 1024 * 1024, // 10MB
//	    FormFieldName:     "file",
//	}), handlers.UploadFile)
func ValidateMagicBytes(cfg MagicByteConfig) gin.HandlerFunc {
	// Default field name
	if cfg.FormFieldName == "" {
		cfg.FormFieldName = "file"
	}

	// Build allowed types set
	allowedTypes := make(map[string]bool)
	if len(cfg.AllowedTypes) > 0 {
		for _, t := range cfg.AllowedTypes {
			allowedTypes[t] = true
		}
	} else {
		for _, cat := range cfg.AllowedCategories {
			for _, t := range AllowedFileTypes[cat] {
				allowedTypes[t] = true
			}
		}
	}

	return func(c *gin.Context) {
		// Skip if no file upload
		file, header, err := c.Request.FormFile(cfg.FormFieldName)
		if err != nil {
			// No file in request, continue (might be optional upload)
			c.Next()
			return
		}
		defer file.Close()

		// Check file size
		if cfg.MaxFileSize > 0 && header.Size > cfg.MaxFileSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":   "file_too_large",
				"message": "File exceeds maximum allowed size",
				"max_size": cfg.MaxFileSize,
			})
			c.Abort()
			return
		}

		// Read first bytes to check magic bytes
		header_bytes := make([]byte, 16)
		n, err := file.Read(header_bytes)
		if err != nil && err != io.EOF {
			log.Error().Err(err).Msg("Failed to read file header for magic byte validation")
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "file_read_error",
				"message": "Failed to read uploaded file",
			})
			c.Abort()
			return
		}
		header_bytes = header_bytes[:n]

		// Reset file pointer for downstream handlers
		if seeker, ok := file.(io.Seeker); ok {
			_, _ = seeker.Seek(0, io.SeekStart)
		}

		// Detect actual file type
		detectedType := detectFileType(header_bytes)
		claimedType := header.Header.Get("Content-Type")

		log.Debug().
			Str("claimed", claimedType).
			Str("detected", detectedType).
			Str("filename", header.Filename).
			Msg("Magic byte validation")

		// If we couldn't detect the type and it's not text, reject
		if detectedType == "" && !isTextType(claimedType) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "unknown_file_type",
				"message": "Could not determine file type",
			})
			c.Abort()
			return
		}

		// For text files, use the claimed type
		if isTextType(claimedType) {
			detectedType = claimedType
		}

		// Check if detected type is allowed
		if !allowedTypes[detectedType] {
			// Also check if the detected category matches any allowed category
			if !isTypeInAllowedCategories(detectedType, cfg.AllowedCategories) {
				c.JSON(http.StatusUnsupportedMediaType, gin.H{
					"error":   "invalid_file_type",
					"message": "File type not allowed",
					"detected_type": detectedType,
				})
				c.Abort()
				return
			}
		}

		// Verify claimed type matches detected type (prevent MIME type spoofing)
		if !typesMatch(claimedType, detectedType) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "mime_type_mismatch",
				"message": "File content does not match claimed MIME type",
				"claimed": claimedType,
				"actual":  detectedType,
			})
			c.Abort()
			return
		}

		// Store validated type in context for handlers
		c.Set("validated_file_type", detectedType)
		c.Set("validated_file_size", header.Size)

		c.Next()
	}
}

// detectFileType detects file type based on magic bytes.
func detectFileType(data []byte) string {
	for mimeType, magic := range MagicBytes {
		if magic == nil {
			continue // Skip text types (no magic bytes)
		}
		if len(data) >= len(magic) && bytes.Equal(data[:len(magic)], magic) {
			// Special handling for RIFF container (could be WEBP or other)
			if mimeType == "image/webp" {
				if len(data) >= 12 && string(data[8:12]) == "WEBP" {
					return "image/webp"
				}
				continue // Not WEBP, try other types
			}
			return mimeType
		}
	}
	return ""
}

// isTextType checks if a MIME type is a text type.
func isTextType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "text/") ||
		mimeType == "application/json" ||
		mimeType == "application/xml"
}

// isTypeInAllowedCategories checks if a type belongs to any allowed category.
func isTypeInAllowedCategories(mimeType string, categories []FileTypeCategory) bool {
	for _, cat := range categories {
		for _, t := range AllowedFileTypes[cat] {
			if t == mimeType {
				return true
			}
		}
	}
	return false
}

// typesMatch checks if claimed and detected types are compatible.
func typesMatch(claimed, detected string) bool {
	// Direct match
	if claimed == detected {
		return true
	}

	// Handle generic matches (e.g., "image/*" matches "image/jpeg")
	if strings.HasSuffix(claimed, "/*") {
		prefix := strings.TrimSuffix(claimed, "/*")
		if strings.HasPrefix(detected, prefix+"/") {
			return true
		}
	}

	// Office documents are ZIP files, so zip magic bytes are valid
	if detected == "application/zip" && isOfficeDocument(claimed) {
		return true
	}

	// TIFF has two variants
	if (claimed == "image/tiff" || claimed == "image/tiff-be") &&
		(detected == "image/tiff" || detected == "image/tiff-be") {
		return true
	}

	return false
}

// isOfficeDocument checks if MIME type is an Office document.
func isOfficeDocument(mimeType string) bool {
	officeTypes := []string{
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
	}
	for _, t := range officeTypes {
		if mimeType == t {
			return true
		}
	}
	return false
}

// ValidateDocumentUpload is a convenience middleware for document uploads.
// Allows images, PDFs, and Office documents up to 25MB.
func ValidateDocumentUpload() gin.HandlerFunc {
	return ValidateMagicBytes(MagicByteConfig{
		AllowedCategories: []FileTypeCategory{
			FileTypeCategoryImage,
			FileTypeCategoryDocument,
		},
		MaxFileSize:   25 * 1024 * 1024, // 25MB
		FormFieldName: "file",
	})
}

// ValidateImageUpload is a convenience middleware for image-only uploads.
// Allows images up to 10MB.
func ValidateImageUpload() gin.HandlerFunc {
	return ValidateMagicBytes(MagicByteConfig{
		AllowedCategories: []FileTypeCategory{FileTypeCategoryImage},
		MaxFileSize:       10 * 1024 * 1024, // 10MB
		FormFieldName:     "image",
	})
}
