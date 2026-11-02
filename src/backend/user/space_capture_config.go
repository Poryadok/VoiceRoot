package main

import (
	"errors"
	"os"
	"strings"

	"voice/backend/pkg/socialprincipal"
)

// validateSpaceCaptureConfig recognizes only this listener's transport fields.
// Shared verifier settings cannot enable it; present-empty fields cannot hide
// partial configuration behind the ordinary health-only startup path.
func validateSpaceCaptureConfig() (bool, error) {
	const prefix = "USER_SPACE_CAPTURE_PRINCIPAL_"
	enabled := false
	for _, field := range []string{"GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "REPLAY_REDIS_ADDR", "REPLAY_REDIS_PASSWORD", "JWKS_CA_FILE", "JWKS_CLIENT_CERT_FILE", "JWKS_CLIENT_KEY_FILE"} {
		if _, present := os.LookupEnv(prefix + field); present {
			enabled = true
		}
	}
	if !enabled {
		return false, nil
	}
	listen := strings.TrimSpace(os.Getenv(prefix + "GRPC_LISTEN"))
	if listen == "" {
		return true, errors.New("explicit capture GRPC_LISTEN required")
	}
	_, _, err := socialprincipal.LoadFromEnvWithAudience("user", "space-account-capture", prefix, listen)
	return true, err
}

// Capture-only dependency settings without the protected listener must not be
// mistaken for the legacy, entirely unconfigured User startup path.
func hasOrphanCaptureRuntimeConfig() bool {
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		name = strings.ToUpper(name)
		if strings.HasPrefix(name, "USER_CUSTODY_") || strings.HasPrefix(name, "USER_IDENTITY_AUTH_") || name == "USER_IDENTITY_KEY_MANIFEST" || name == "USER_IDENTITY_KEYS_DIR" || name == "USER_PRINCIPAL_JWKS_LISTEN" || name == "USER_PRINCIPAL_JWKS_TLS_CERT_FILE" || name == "USER_PRINCIPAL_JWKS_TLS_KEY_FILE" {
			return true
		}
	}
	return false
}
