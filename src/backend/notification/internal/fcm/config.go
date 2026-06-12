package fcm

import (
	"encoding/json"
	"os"
	"strings"
)

// Config holds Firebase service account credentials for FCM HTTP v1.
type Config struct {
	CredentialsJSON []byte
	ProjectID       string
}

// ConfigFromEnv reads FCM_CREDENTIALS_JSON (service account JSON).
func ConfigFromEnv() (Config, bool) {
	raw := strings.TrimSpace(os.Getenv("FCM_CREDENTIALS_JSON"))
	if raw == "" {
		return Config{}, false
	}
	var meta struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return Config{}, false
	}
	if meta.ProjectID == "" {
		return Config{}, false
	}
	return Config{
		CredentialsJSON: []byte(raw),
		ProjectID:       meta.ProjectID,
	}, true
}
