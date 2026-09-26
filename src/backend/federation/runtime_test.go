package main

import (
	"strings"
	"testing"
)

func TestFederationRuntimeRequiresOwnedDatabase(t *testing.T) {
	if err := validateFederationDatabaseName("federation_db"); err != nil {
		t.Fatalf("expected federation_db to be accepted: %v", err)
	}
	if err := validateFederationDatabaseName("voice_db"); err == nil || !strings.Contains(err.Error(), "federation_db") {
		t.Fatalf("expected another database to be rejected, got %v", err)
	}
}
