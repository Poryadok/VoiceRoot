package r2avatar

import (
	"fmt"
	"strings"
)

// MaxAvatarBytes is the Phase 1 upper bound from PLAN.md § R2 / аватар (ориентир 2–5 MB).
const MaxAvatarBytes = 5 * 1024 * 1024

var allowedExtByMIME = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// ValidateUploadParams enforces PLAN Phase 1 avatar presign limits (whitelist MIME, max size).
func ValidateUploadParams(contentType string, contentLength int64) error {
	ct := strings.TrimSpace(strings.ToLower(contentType))
	if _, ok := allowedExtByMIME[ct]; !ok {
		return fmt.Errorf("content_type: unsupported or not an allowed image type")
	}
	if contentLength <= 0 {
		return fmt.Errorf("content_length: must be positive")
	}
	if contentLength > MaxAvatarBytes {
		return fmt.Errorf("content_length: exceeds max of %d bytes", MaxAvatarBytes)
	}
	return nil
}

// FileExtForContentType returns a stable extension for the object key, or "" if unknown.
func FileExtForContentType(contentType string) string {
	ct := strings.TrimSpace(strings.ToLower(contentType))
	return allowedExtByMIME[ct]
}
