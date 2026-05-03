package config

import (
	"encoding/json"
	"os"
	"strings"
)

// TrimEnv returns strings.TrimSpace(os.Getenv(key)).
func TrimEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// SplitCSV splits a comma-separated list, trims spaces, drops empty tokens.
func SplitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

// LoadJSONEnv unmarshals os.Getenv(name) into dst when non-empty.
// On invalid JSON, logInvalid is called if non-nil; dst is left unchanged.
func LoadJSONEnv(name string, dst any, logInvalid func(name string, err error)) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return
	}
	if err := json.Unmarshal([]byte(raw), dst); err != nil && logInvalid != nil {
		logInvalid(name, err)
	}
}
