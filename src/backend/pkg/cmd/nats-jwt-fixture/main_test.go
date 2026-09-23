package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCreatesDistinctServiceCredentialsWithoutBroadJetStreamAPI(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "fixture")
	if err := generate(dest); err != nil {
		t.Fatal(err)
	}
	for _, name := range serviceNames {
		contents, err := os.ReadFile(filepath.Join(dest, "creds", name+".creds"))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(contents), "BEGIN NATS USER JWT") || !strings.Contains(string(contents), "BEGIN USER NKEY SEED") {
			t.Fatalf("%s is not a NATS creds file", name)
		}
	}
	contract, err := os.ReadFile(filepath.Join(dest, "acl-intent.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contract), "$JS.API.>") {
		t.Fatal("fixture must not grant a broad JetStream API wildcard")
	}
	if err := generate(dest); err == nil {
		t.Fatal("existing destination must be refused")
	}
}
