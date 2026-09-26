package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFederationRuntimeRequiresOwnedDatabase(t *testing.T) {
	if err := validateFederationDatabaseName("federation_db"); err != nil {
		t.Fatalf("expected federation_db to be accepted: %v", err)
	}
	if err := validateFederationDatabaseName("voice_db"); err == nil || !strings.Contains(err.Error(), "federation_db") {
		t.Fatalf("expected another database to be rejected, got %v", err)
	}
}

func TestFederationComposeIsOptInAndRequiresAllRuntimeSettings(t *testing.T) {
	composePath := filepath.Join("..", "..", "..", "docker-compose.yml")
	raw, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Services map[string]struct {
			Profiles    []string          `yaml:"profiles"`
			Environment map[string]string `yaml:"environment"`
			Volumes     []string          `yaml:"volumes"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	federation, ok := document.Services["federation"]
	if !ok {
		t.Fatal("federation service is missing")
	}
	if len(federation.Profiles) != 1 || federation.Profiles[0] != "federation" {
		t.Fatalf("federation must be excluded from the standard app profile, got profiles %v", federation.Profiles)
	}
	expected := map[string]string{
		"FEDERATION_DATABASE_URL":         "${FEDERATION_DATABASE_URL:-}",
		"FEDERATION_TLS_CERT":             "/run/voice/federation/tls.crt",
		"FEDERATION_TLS_KEY":              "/run/voice/federation/tls.key",
		"FEDERATION_CLIENT_CA":            "/run/voice/federation/client-ca.crt",
		"FEDERATION_SIGNING_SEED_FILE":    "/run/voice/federation/signing-seed",
		"FEDERATION_KEY_ID":               "${FEDERATION_KEY_ID:-}",
		"FEDERATION_ISSUER":               "${FEDERATION_ISSUER:-}",
		"FEDERATION_ENVIRONMENT":          "${FEDERATION_ENVIRONMENT:-}",
		"FEDERATION_OPERATOR_CERT_SHA256": "${FEDERATION_OPERATOR_CERT_SHA256:-}",
	}
	for name, value := range expected {
		if federation.Environment[name] != value {
			t.Errorf("Compose %s = %q, want explicit fail-closed value %q", name, federation.Environment[name], value)
		}
	}
	if len(federation.Volumes) != 1 || !strings.HasSuffix(federation.Volumes[0], ":/run/voice/federation:ro") {
		t.Fatalf("Federation local secret mount must be read-only, got volumes %v", federation.Volumes)
	}
}
