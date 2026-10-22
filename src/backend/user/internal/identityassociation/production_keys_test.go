package identityassociation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProductionKeysetCompleteRetainedVersions(t *testing.T) {
	dir := t.TempDir()
	first := []byte(strings.Repeat("a", 32))
	second := []byte(strings.Repeat("b", 32))
	writeProductionSecret(t, dir, "old.key", first)
	writeProductionSecret(t, dir, "active.key", second)
	manifest := writeProductionManifest(t, dir, productionManifestForTest(first, second))

	keys, err := LoadProductionKeyset(manifest, dir)
	if err != nil {
		t.Fatalf("load complete User keyset: %v", err)
	}
	snapshot := keys.Snapshot()
	if snapshot.Revision != 7 || snapshot.ActiveVersion != 2 || len(snapshot.Versions) != 2 {
		t.Fatalf("incomplete retained versions: %+v", snapshot)
	}
	if snapshot.Versions[1] != sha256.Sum256(first) || snapshot.Versions[2] != sha256.Sum256(second) {
		t.Fatal("loaded keyset fingerprints differ from independent manifest")
	}
}

func TestLoadProductionKeysetFailsClosed(t *testing.T) {
	first := []byte(strings.Repeat("a", 32))
	second := []byte(strings.Repeat("b", 32))
	base := productionManifestForTest(first, second)
	cases := []struct {
		name     string
		manifest string
		secrets  map[string][]byte
	}{
		{"missing retained key", base, map[string][]byte{"active.key": second}},
		{"replaced retained key", base, map[string][]byte{"old.key": second, "active.key": second}},
		{"wrong purpose", strings.Replace(base, "voice.user.identity-association.v1", "voice.auth.identity-association.v1", 1), map[string][]byte{"old.key": first, "active.key": second}},
		{"duplicate version", strings.Replace(base, `"version":2`, `"version":1`, 1), map[string][]byte{"old.key": first, "active.key": second}},
		{"missing active version", strings.Replace(base, `"active_version":2`, `"active_version":3`, 1), map[string][]byte{"old.key": first, "active.key": second}},
		{"trailing document", base + ` {}`, map[string][]byte{"old.key": first, "active.key": second}},
		{"path escape", strings.Replace(base, `"file":"old.key"`, `"file":"../old.key"`, 1), map[string][]byte{"old.key": first, "active.key": second}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, value := range tc.secrets {
				writeProductionSecret(t, dir, name, value)
			}
			_, err := LoadProductionKeyset(writeProductionManifest(t, dir, tc.manifest), dir)
			if err == nil {
				t.Fatal("partial or foreign User keyset was accepted")
			}
			if strings.Contains(err.Error(), string(first)) || strings.Contains(err.Error(), string(second)) {
				t.Fatal("key material appeared in error")
			}
		})
	}
}

func productionManifestForTest(first, second []byte) string {
	a := sha256.Sum256(first)
	b := sha256.Sum256(second)
	return fmt.Sprintf(`{"purpose":"voice.user.identity-association.v1","revision":7,"active_version":2,"versions":[{"version":1,"sha256":"%s","file":"old.key"},{"version":2,"sha256":"%s","file":"active.key"}]}`, hex.EncodeToString(a[:]), hex.EncodeToString(b[:]))
}

func writeProductionManifest(t *testing.T, dir, value string) string {
	t.Helper()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeProductionSecret(t *testing.T, dir, name string, value []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), value, 0600); err != nil {
		t.Fatal(err)
	}
}
