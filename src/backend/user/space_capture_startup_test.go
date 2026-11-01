package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func clearSpaceCaptureTestConfig(t *testing.T) {
	t.Helper()
	for _, field := range []string{"GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "REPLAY_REDIS_ADDR", "REPLAY_REDIS_PASSWORD", "JWKS_CA_FILE", "JWKS_CLIENT_CERT_FILE", "JWKS_CLIENT_KEY_FILE"} {
		name := "USER_SPACE_CAPTURE_PRINCIPAL_" + field
		value, present := os.LookupEnv(name)
		require.NoError(t, os.Unsetenv(name))
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(name, value)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
	t.Setenv("VOICE_RELEASE_PROFILE", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("LISTEN_ADDR", "127.0.0.1:0")
}

func TestStartupRejectsPartialSpaceCaptureConfig(t *testing.T) {
	for _, field := range []string{"GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "REPLAY_REDIS_ADDR", "REPLAY_REDIS_PASSWORD", "JWKS_CA_FILE", "JWKS_CLIENT_CERT_FILE", "JWKS_CLIENT_KEY_FILE"} {
		for _, value := range []string{"", "configured"} {
			t.Run(field+"/"+value, func(t *testing.T) {
				clearSpaceCaptureTestConfig(t)
				t.Setenv("USER_SPACE_CAPTURE_PRINCIPAL_"+field, value)
				ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
				defer cancel()
				require.ErrorContains(t, runUser(ctx), "user Space capture principal config")
			})
		}
	}
}

func TestStartupSpaceCaptureDisabledWithOnlySharedVerifierConfig(t *testing.T) {
	clearSpaceCaptureTestConfig(t)
	t.Setenv("S2S_JWKS_URLS_JSON", `{"space":"https://space.example/jwks"}`)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	require.NoError(t, runUser(ctx), "absent capture settings preserve health-only startup")
}

func TestStartupRejectsCaptureDependenciesWithoutListener(t *testing.T) {
	for _, name := range []string{"USER_CUSTODY_ADDR", "USER_IDENTITY_AUTH_GRPC_ADDR", "USER_IDENTITY_KEY_MANIFEST", "USER_IDENTITY_KEYS_DIR", "USER_PRINCIPAL_JWKS_LISTEN", "USER_PRINCIPAL_JWKS_TLS_CERT_FILE", "USER_PRINCIPAL_JWKS_TLS_KEY_FILE"} {
		t.Run(name, func(t *testing.T) {
			clearSpaceCaptureTestConfig(t)
			t.Setenv(name, "configured")
			require.ErrorContains(t, runUser(t.Context()), "capture listener required")
		})
	}
}

func completeSpaceCaptureTestConfig(t *testing.T) {
	t.Helper()
	root := repoRootFromComposeTest(t)
	ca := filepath.Join(root, "src/backend/auth/src/test/resources/principal-tls/wrong-ca-cert.pem")
	for field, value := range map[string]string{
		"GRPC_LISTEN": "127.0.0.1:0", "TLS_CERT_FILE": "configured-cert", "TLS_KEY_FILE": "configured-key",
		"CLIENT_CA_FILE": ca, "REPLAY_REDIS_ADDR": "redis.invalid:6379",
		"JWKS_CLIENT_CERT_FILE": "configured-jwks-cert", "JWKS_CLIENT_KEY_FILE": "configured-jwks-key",
	} {
		t.Setenv("USER_SPACE_CAPTURE_PRINCIPAL_"+field, value)
	}
	t.Setenv("S2S_JWKS_URLS_JSON", `{"space":"https://space.example/jwks"}`)
}

func TestStartupSpaceCaptureCompleteTransportStillRequiresQualifiedSource(t *testing.T) {
	clearSpaceCaptureTestConfig(t)
	completeSpaceCaptureTestConfig(t)
	// Invalid ordinary listener addresses would fail if startup got as far as
	// binding. Configured capture must deny before any listener or DB dependency.
	t.Setenv("LISTEN_ADDR", "invalid-http-listen")
	t.Setenv("USER_GRPC_ADDR", "invalid-ordinary-listen")
	require.EqualError(t, runUser(t.Context()), "qualified User capture source unavailable")
	for _, field := range []string{"GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "REPLAY_REDIS_ADDR", "JWKS_CLIENT_CERT_FILE", "JWKS_CLIENT_KEY_FILE"} {
		t.Run("missing/"+field, func(t *testing.T) {
			t.Setenv("USER_SPACE_CAPTURE_PRINCIPAL_"+field, "")
			require.ErrorContains(t, runUser(t.Context()), "user Space capture principal config")
		})
	}
	t.Run("wrong-issuer", func(t *testing.T) {
		t.Setenv("S2S_JWKS_URLS_JSON", `{"social":"https://social.example/jwks"}`)
		require.ErrorContains(t, runUser(t.Context()), "user Space capture principal config")
	})
	t.Run("malformed-listen", func(t *testing.T) {
		t.Setenv("USER_SPACE_CAPTURE_PRINCIPAL_GRPC_LISTEN", "not-a-listen-address")
		require.ErrorContains(t, runUser(t.Context()), "user Space capture principal config")
	})
}

func TestStartupSpaceCaptureRealProcessCannotAdmitWithoutSource(t *testing.T) {
	if os.Getenv("VOICE_USER_CAPTURE_CONFIG_CHILD") == "1" {
		clearSpaceCaptureTestConfig(t)
		completeSpaceCaptureTestConfig(t)
		t.Setenv("LISTEN_ADDR", "invalid-http-listen")
		require.EqualError(t, runUser(t.Context()), "qualified User capture source unavailable")
		return
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestStartupSpaceCaptureRealProcessCannotAdmitWithoutSource$")
	cmd.Env = append(os.Environ(), "VOICE_USER_CAPTURE_CONFIG_CHILD=1")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	require.False(t, strings.Contains(string(output), `"msg":"listening"`), "%s", output)
}
