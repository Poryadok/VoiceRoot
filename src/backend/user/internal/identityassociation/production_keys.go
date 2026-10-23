package identityassociation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const productionKeyPurpose = "voice.user.identity-association.v1"

type productionKeyManifest struct {
	Purpose       string                      `json:"purpose"`
	Revision      uint64                      `json:"revision"`
	ActiveVersion uint32                      `json:"active_version"`
	Versions      []productionKeyVersionEntry `json:"versions"`
}

type productionKeyVersionEntry struct {
	Version uint32 `json:"version"`
	SHA256  string `json:"sha256"`
	File    string `json:"file"`
}

// LoadProductionKeyset reads only caller-provided, mounted User-purpose keys.
// It never creates keys or qualifies a database's source/currentness.
func LoadProductionKeyset(manifestPath, secretsDir string) (*Keyset, error) {
	if manifestPath == "" || secretsDir == "" {
		return nil, ErrKeysetIncomplete
	}
	data, err := readBoundedKeyFile(manifestPath, 64*1024)
	if err != nil {
		return nil, ErrKeysetIncomplete
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest productionKeyManifest
	if decoder.Decode(&manifest) != nil {
		return nil, ErrKeysetIncomplete
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF || manifest.Purpose != productionKeyPurpose || len(manifest.Versions) == 0 || len(manifest.Versions) > 256 {
		return nil, ErrKeysetIncomplete
	}
	snapshot := KeysetSnapshot{Revision: manifest.Revision, ActiveVersion: manifest.ActiveVersion, Versions: make(map[uint32][32]byte, len(manifest.Versions))}
	secrets := make(map[uint32][]byte, len(manifest.Versions))
	filenames := make(map[string]struct{}, len(manifest.Versions))
	for _, version := range manifest.Versions {
		if version.Version == 0 || !localKeyFilename(version.File) {
			return nil, ErrKeysetIncomplete
		}
		if _, ok := snapshot.Versions[version.Version]; ok {
			return nil, ErrKeysetIncomplete
		}
		if _, ok := filenames[version.File]; ok {
			return nil, ErrKeysetIncomplete
		}
		filenames[version.File] = struct{}{}
		fingerprintBytes, err := hex.DecodeString(version.SHA256)
		if err != nil || len(fingerprintBytes) != sha256.Size || version.SHA256 != hex.EncodeToString(fingerprintBytes) {
			return nil, ErrKeysetIncomplete
		}
		var fingerprint [32]byte
		copy(fingerprint[:], fingerprintBytes)
		secret, err := readBoundedKeyFile(filepath.Join(secretsDir, version.File), 4096)
		if err != nil {
			return nil, ErrKeysetIncomplete
		}
		snapshot.Versions[version.Version], secrets[version.Version] = fingerprint, secret
	}
	return NewKeyset(snapshot, secrets)
}

func localKeyFilename(name string) bool {
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, ".") || filepath.Base(name) != name {
		return false
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

func readBoundedKeyFile(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil || int64(len(data)) > maxBytes {
		return nil, ErrKeysetIncomplete
	}
	return data, nil
}
