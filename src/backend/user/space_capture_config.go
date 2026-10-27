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
