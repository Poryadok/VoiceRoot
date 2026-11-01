package main

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpaceCaptureRuntimeRequiresCompleteCanonicalConfiguration(t *testing.T) {
	values := map[string]string{
		"USER_CUSTODY_ADDR": "custody.example:443", "USER_CUSTODY_TLS_CA_FILE": "ca", "USER_CUSTODY_TLS_SERVER_NAME": "custody.example", "USER_CUSTODY_TLS_CERT_FILE": "cert", "USER_CUSTODY_TLS_KEY_FILE": "key", "USER_CUSTODY_SIGNING_KEYS_DIR": "signers", "USER_CUSTODY_ACTIVE_KID": "current", "USER_CUSTODY_HEARTBEAT_TIMEOUT": "2s",
		"USER_PRINCIPAL_JWKS_LISTEN": "127.0.0.1:9443", "USER_PRINCIPAL_JWKS_TLS_CERT_FILE": "cert", "USER_PRINCIPAL_JWKS_TLS_KEY_FILE": "key",
		"USER_IDENTITY_AUTH_GRPC_ADDR": "auth.example:443", "USER_IDENTITY_AUTH_TLS_SERVER_NAME": "auth.example", "USER_IDENTITY_AUTH_TLS_CA_FILE": "ca", "USER_IDENTITY_AUTH_TLS_CERT_FILE": "cert", "USER_IDENTITY_AUTH_TLS_KEY_FILE": "key",
		"USER_CUSTODY_EPOCH": "1", "USER_CUSTODY_SOURCE_ID": "user-source", "USER_CUSTODY_HISTORY_GENERATION": "11111111-1111-4111-8111-111111111111", "USER_CUSTODY_USER_SOURCE_GENERATION": "22222222-2222-4222-8222-222222222222", "USER_CUSTODY_KEYSET_DIGEST": strings.Repeat("ab", 32), "USER_IDENTITY_KEY_MANIFEST": "manifest", "USER_IDENTITY_KEYS_DIR": "keys",
	}
	lookup := func(name string) (string, bool) { value, ok := values[name]; return value, ok }
	config, err := loadSpaceCaptureRuntimeConfig(lookup)
	require.NoError(t, err)
	require.Equal(t, int64(1), config.binding.Epoch)
	for _, key := range []string{"USER_CUSTODY_ADDR", "USER_PRINCIPAL_JWKS_LISTEN", "USER_IDENTITY_AUTH_GRPC_ADDR", "USER_CUSTODY_SOURCE_ID", "USER_CUSTODY_KEYSET_DIGEST", "USER_IDENTITY_KEYS_DIR"} {
		t.Run("missing/"+key, func(t *testing.T) {
			original := values[key]
			delete(values, key)
			defer func() { values[key] = original }()
			_, err := loadSpaceCaptureRuntimeConfig(lookup)
			require.Error(t, err)
		})
	}
	for key, value := range map[string]string{"USER_CUSTODY_EPOCH": "01", "USER_CUSTODY_HISTORY_GENERATION": values["USER_CUSTODY_USER_SOURCE_GENERATION"], "USER_CUSTODY_KEYSET_DIGEST": strings.Repeat("00", 32), "USER_IDENTITY_KEY_MANIFEST": " manifest"} {
		t.Run("invalid/"+key, func(t *testing.T) {
			original := values[key]
			values[key] = value
			defer func() { values[key] = original }()
			_, err := loadSpaceCaptureRuntimeConfig(lookup)
			require.Error(t, err)
		})
	}
}

func TestSpaceCaptureRuntimeRejectsIncompleteDependencies(t *testing.T) {
	_, err := startSpaceCaptureRuntime(context.Background(), spaceCaptureRuntimeConfig{})
	require.Error(t, err)
	var runtime *spaceCaptureRuntime
	_, _, err = runtime.bind(context.Background(), nil)
	require.Error(t, err)
	require.Error(t, runtime.waitAdmission(context.Background()))
}
