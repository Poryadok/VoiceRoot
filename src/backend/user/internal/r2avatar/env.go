package r2avatar

import (
	"os"
	"strings"
)

// EnvConfigFromOSEnv loads USER_R2_* variables used by User Service for Phase 1 avatar uploads.
// Returns zero S3R2Config if any required value is missing (caller treats as disabled).
func EnvConfigFromOSEnv() S3R2Config {
	return S3R2Config{
		Endpoint:        strings.TrimSpace(os.Getenv("USER_R2_ENDPOINT")),
		Region:          strings.TrimSpace(os.Getenv("USER_R2_REGION")),
		AccessKeyID:     strings.TrimSpace(os.Getenv("USER_R2_ACCESS_KEY_ID")),
		SecretAccessKey: strings.TrimSpace(os.Getenv("USER_R2_SECRET_ACCESS_KEY")),
		Bucket:          strings.TrimSpace(os.Getenv("USER_R2_BUCKET")),
		PublicBaseURL:   strings.TrimSpace(os.Getenv("USER_R2_PUBLIC_BASE_URL")),
	}
}
