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

// ConfigFromEnv reads FCM credentials from environment variables.
// Preferred: FCM_CREDENTIALS_JSON (full service account JSON).
// Deploy manifests may instead set FCM_PROJECT_ID + FCM_SERVICE_ACCOUNT_JSON.
func ConfigFromEnv() (Config, bool) {
	if raw := strings.TrimSpace(os.Getenv("FCM_CREDENTIALS_JSON")); raw != "" {
		return configFromJSON([]byte(raw))
	}
	projectID := strings.TrimSpace(os.Getenv("FCM_PROJECT_ID"))
	serviceJSON := strings.TrimSpace(os.Getenv("FCM_SERVICE_ACCOUNT_JSON"))
	if projectID == "" || serviceJSON == "" {
		return Config{}, false
	}
	cfg, ok := configFromJSON([]byte(serviceJSON))
	if !ok {
		return Config{}, false
	}
	if cfg.ProjectID == "" {
		cfg.ProjectID = projectID
	}
	return cfg, true
}

func configFromJSON(raw []byte) (Config, bool) {
	if len(raw) == 0 {
		return Config{}, false
	}
	var meta struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return Config{}, false
	}
	if meta.ProjectID == "" {
		return Config{}, false
	}
	return Config{
		CredentialsJSON: raw,
		ProjectID:       meta.ProjectID,
	}, true
}
