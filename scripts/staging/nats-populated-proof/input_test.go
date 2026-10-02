package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInputRejectsUnwitnessedDeployedClaims(t *testing.T) {
	dir := t.TempDir()
	acl := filepath.Join(dir, "acl.yaml")
	if err := os.WriteFile(acl, []byte("version: 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateInput(dir, acl, "", false); err == nil {
		t.Fatal("unwitnessed input was accepted as deployed identity/config provenance")
	}
}

func TestInputRejectsFixtureAsDeployedEvidence(t *testing.T) {
	dir := t.TempDir()
	acl := filepath.Join(dir, "acl.yaml")
	if err := os.WriteFile(acl, []byte("version: 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"schema":"isolated-deployed-config-clone-v1","kind":"fixture"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateInput(dir, acl, "trusted-key", false); err == nil {
		t.Fatal("fixture input was accepted as deployed proof input")
	}
}
