package r2avatar

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ObjectKey returns a unique object path under avatars/{profile_id}/...
func ObjectKey(profileID uuid.UUID, ext string) string {
	ext = strings.TrimSpace(ext)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return fmt.Sprintf("avatars/%s/%s%s", profileID.String(), uuid.New().String(), ext)
}

// JoinPublicURL builds the stable URL stored in profiles.avatar_url.
func JoinPublicURL(publicBaseURL, objectKey string) string {
	b := strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	k := strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if b == "" {
		return k
	}
	if k == "" {
		return b
	}
	return b + "/" + k
}
