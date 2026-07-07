package r2avatar

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// MaxAvatarBytes is the app stack upper bound from PLAN.md § R2 / аватар (ориентир 2–5 MB).
const MaxAvatarBytes = 5 * 1024 * 1024

var allowedExtByMIME = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// ValidateUploadParams enforces PLAN app stack avatar presign limits (whitelist MIME, max size).
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

var allowedAvatarFileExt = map[string]struct{}{
	".png": {}, ".jpg": {}, ".webp": {},
}

func validateAvatarLeafFile(name string) error {
	i := strings.LastIndex(name, ".")
	if i <= 0 || i == len(name)-1 {
		return fmt.Errorf("avatar object: invalid file name")
	}
	ext := strings.ToLower(name[i:])
	if _, ok := allowedAvatarFileExt[ext]; !ok {
		return fmt.Errorf("avatar object: unsupported extension")
	}
	id, err := uuid.Parse(name[:i])
	if err != nil || id == uuid.Nil {
		return fmt.Errorf("avatar object: invalid file id")
	}
	return nil
}

// ValidateAvatarObjectKeyForProfile checks keys produced by ObjectKey for the given profile
// (PLAN.md § R2: stable object key path under avatars/{profile_id}/).
func ValidateAvatarObjectKeyForProfile(profileID uuid.UUID, key string) error {
	k := strings.TrimLeft(strings.TrimSpace(key), "/")
	parts := strings.Split(k, "/")
	if len(parts) != 3 || parts[0] != "avatars" {
		return fmt.Errorf("avatar object key: invalid path")
	}
	pid, err := uuid.Parse(parts[1])
	if err != nil || pid != profileID {
		return fmt.Errorf("avatar object key: profile mismatch")
	}
	return validateAvatarLeafFile(parts[2])
}

// ValidateAvatarPublicURLForProfile ensures full URL matches JoinPublicURL(publicBase, objectKey)
// for an object key owned by profileID (prefix + profile folder + leaf file).
func ValidateAvatarPublicURLForProfile(publicBase string, profileID uuid.UUID, fullURL string) error {
	base := strings.TrimSpace(publicBase)
	u := strings.TrimSpace(fullURL)
	if base == "" || u == "" {
		return fmt.Errorf("avatar public url: empty")
	}
	if strings.ContainsAny(u, "?#") {
		return fmt.Errorf("avatar public url: must not contain query or fragment")
	}
	prefix := JoinPublicURL(base, "avatars/")
	if !strings.HasPrefix(u, prefix) {
		return fmt.Errorf("avatar public url: wrong host or path prefix")
	}
	rest := strings.TrimPrefix(u, prefix)
	if !strings.HasPrefix(rest, profileID.String()+"/") {
		return fmt.Errorf("avatar public url: profile folder mismatch")
	}
	leaf := strings.TrimPrefix(rest, profileID.String()+"/")
	return validateAvatarLeafFile(leaf)
}
