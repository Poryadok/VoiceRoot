package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFilePrincipalKeysProjectedSecret(t *testing.T) {
	root := t.TempDir()
	projection := filepath.Join(root, "..2026_09_23_00_00_00")
	if err := os.Mkdir(projection, 0700); err != nil {
		t.Fatal(err)
	}
	for _, kid := range []string{"current", "next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		der, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(projection, kid+".pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(filepath.Base(projection), kid+".pem"), filepath.Join(root, kid+".pem")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	keys, err := loadFilePrincipalKeys(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("got %d keys, want 2", len(keys))
	}
}

func TestLoadFilePrincipalKeysRejectsEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "current.pem")
	if err := os.WriteFile(outside, []byte("not a key"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "current.pem")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := loadFilePrincipalKeys(root)
	if err == nil || !strings.Contains(err.Error(), "escapes directory") {
		t.Fatalf("got %v, want directory escape error", err)
	}
}
