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

func TestProductionKeyReaderRetainsEveryVersion(t *testing.T) {
	dir := t.TempDir()
	old, active := []byte(strings.Repeat("o", 32)), []byte(strings.Repeat("a", 32))
	writeKeyFile(t, dir, "old.key", old)
	writeKeyFile(t, dir, "active.key", active)
	keys, err := LoadProductionKeyset(writeKeyManifest(t, dir, keyManifestFixture(old, active)), dir)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := keys.Snapshot()
	if snapshot.Revision != 7 || snapshot.ActiveVersion != 2 || len(snapshot.Versions) != 2 || snapshot.Versions[1] != sha256.Sum256(old) || snapshot.Versions[2] != sha256.Sum256(active) {
		t.Fatalf("incomplete User-only keyset: %+v", snapshot)
	}
}

func TestProductionKeyReaderFailsClosed(t *testing.T) {
	old, active := []byte(strings.Repeat("o", 32)), []byte(strings.Repeat("a", 32))
	base := keyManifestFixture(old, active)
	cases := []struct {
		name, manifest string
		secrets        map[string][]byte
	}{
		{"missing retained version", base, map[string][]byte{"active.key": active}},
		{"replaced retained version", base, map[string][]byte{"old.key": active, "active.key": active}},
		{"foreign purpose", strings.Replace(base, "voice.user.identity-association.v1", "voice.auth.identity-association.v1", 1), map[string][]byte{"old.key": old, "active.key": active}},
		{"duplicate version", strings.Replace(base, `"version":2`, `"version":1`, 1), map[string][]byte{"old.key": old, "active.key": active}},
		{"missing active version", strings.Replace(base, `"active_version":2`, `"active_version":3`, 1), map[string][]byte{"old.key": old, "active.key": active}},
		{"trailing document", base + ` {}`, map[string][]byte{"old.key": old, "active.key": active}},
		{"path escape", strings.Replace(base, `"file":"old.key"`, `"file":"../old.key"`, 1), map[string][]byte{"old.key": old, "active.key": active}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, secret := range tc.secrets {
				writeKeyFile(t, dir, name, secret)
			}
			_, err := LoadProductionKeyset(writeKeyManifest(t, dir, tc.manifest), dir)
			if err == nil {
				t.Fatal("partial or foreign User keyset accepted")
			}
			if strings.Contains(err.Error(), string(old)) || strings.Contains(err.Error(), string(active)) {
				t.Fatal("key material leaked in error")
			}
		})
	}
}

func keyManifestFixture(old, active []byte) string {
	a, b := sha256.Sum256(old), sha256.Sum256(active)
	return fmt.Sprintf(`{"purpose":"voice.user.identity-association.v1","revision":7,"active_version":2,"versions":[{"version":1,"sha256":"%s","file":"old.key"},{"version":2,"sha256":"%s","file":"active.key"}]}`, hex.EncodeToString(a[:]), hex.EncodeToString(b[:]))
}

func writeKeyManifest(t *testing.T, dir, value string) string {
	t.Helper()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeKeyFile(t *testing.T, dir, name string, value []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), value, 0600); err != nil {
		t.Fatal(err)
	}
}
