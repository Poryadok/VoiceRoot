package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDevIssuanceKeepsIndependentNamespaceAndProtectedSeedBackup(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux custody required")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	cert, key, ca := testTLS(t, parent, "voice-nats")
	dest := filepath.Join(parent, "dev-issued")
	if err := issue(canonicalACL(), dest, "voice-dev", cert, key, ca); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dest, "secrets.json"))
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Items []secret `json:"items"`
	}
	if err := json.Unmarshal(contents, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 4 {
		t.Fatalf("got %d secrets", len(list.Items))
	}
	for _, item := range list.Items {
		if item.Metadata.Namespace != "voice-dev" {
			t.Fatal("cross-namespace secret")
		}
	}
	info, err := os.Stat(filepath.Join(dest, "signing-seeds/operator.seed"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("seed backup custody invalid")
	}
}
