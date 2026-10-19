// Package recoverybundle inspects finite recovery artifacts. Inspection proves
// file integrity and bindings only; it never proves currentness or readiness.
package recoverybundle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/google/uuid"
)

var ErrInvalid = errors.New("invalid recovery artifact bundle")

type Store struct {
	Name        string `json:"name"`
	Incarnation string `json:"incarnation"`
}
type Artifact struct {
	Kind          string `json:"kind"`
	Store         string `json:"store"`
	Path          string `json:"path"`
	Length        int64  `json:"length"`
	SHA256        string `json:"sha256"`
	SchemaVersion int    `json:"schema_version"`
	Generation    string `json:"generation"`
	RecordCount   int64  `json:"record_count"`
	Cut           string `json:"cut"`
}
type Manifest struct {
	ProtocolVersion      int        `json:"protocol_version"`
	OperationID          string     `json:"operation_id"`
	Environment          string     `json:"environment"`
	DeploymentGeneration string     `json:"deployment_generation"`
	Method               string     `json:"method"`
	Stores               []Store    `json:"stores"`
	Artifacts            []Artifact `json:"artifacts"`
}

// Expected comes from the protected operation adapter, never from the bundle.
type Expected struct {
	OperationID, Environment, DeploymentGeneration, Method string
	Stores                                                 map[string]string
	RequiredKinds                                          map[string]string // artifact kind -> logical store
}

// Inspected retains the actual verified bytes. It cannot advance a BootGate.
type Inspected struct {
	files    map[string][]byte
	manifest Manifest
}

func (i *Inspected) Manifest() Manifest {
	if i == nil {
		return Manifest{}
	}
	m := i.manifest
	m.Stores = append([]Store(nil), m.Stores...)
	m.Artifacts = append([]Artifact(nil), m.Artifacts...)
	return m
}

func (i *Inspected) Artifact(kind string) []byte {
	if i == nil {
		return nil
	}
	return append([]byte(nil), i.files[kind]...)
}

func Inspect(directory string, expected Expected) (*Inspected, error) {
	if !validExpected(expected) {
		return nil, ErrInvalid
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, ErrInvalid
	}
	defer root.Close()
	raw, err := readFile(root, "manifest.json", 2<<20)
	if err != nil {
		return nil, ErrInvalid
	}
	var manifest Manifest
	if strictJSON(raw, &manifest) != nil || !boundManifest(manifest, expected) {
		return nil, ErrInvalid
	}
	result := &Inspected{files: make(map[string][]byte), manifest: manifest}
	listed := map[string]bool{"manifest.json": true}
	var total int64
	for _, artifact := range manifest.Artifacts {
		store, required := expected.RequiredKinds[artifact.Kind]
		if !required || !validPath(artifact.Path) || listed[artifact.Path] || result.files[artifact.Kind] != nil || store != artifact.Store || artifact.Length <= 0 || artifact.Length > 16<<20 || artifact.SchemaVersion != 1 || !validUUID(artifact.Generation) || artifact.RecordCount < 0 || artifact.RecordCount > 1000000000 || !validLabel(artifact.Cut, 512) || !validDigest(artifact.SHA256) {
			return nil, ErrInvalid
		}
		total += artifact.Length
		if total > 64<<20 {
			return nil, ErrInvalid
		}
		listed[artifact.Path] = true
		data, err := readFile(root, artifact.Path, artifact.Length)
		if err != nil {
			return nil, ErrInvalid
		}
		digest := sha256.Sum256(data)
		if int64(len(data)) != artifact.Length || hex.EncodeToString(digest[:]) != artifact.SHA256 {
			return nil, ErrInvalid
		}
		result.files[artifact.Kind] = data
	}
	if len(result.files) != len(expected.RequiredKinds) {
		return nil, ErrInvalid
	}
	// Every regular file is inventoried. Symlinks and special files are rejected,
	// including directory symlinks, before they can hide an unlisted subtree.
	entries := 0
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		entries++
		if entries > 512 {
			return ErrInvalid
		}
		if walkErr != nil || entry == nil || entry.Type()&os.ModeSymlink != 0 {
			return ErrInvalid
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() || !listed[name] {
			return ErrInvalid
		}
		return nil
	})
	if err != nil {
		return nil, ErrInvalid
	}
	return result, nil
}

func validUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil && parsed.String() == value
}
func validLabel(value string, maximum int) bool {
	if len(value) == 0 || len(value) > maximum || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if r < 33 || r > 126 {
			return false
		}
	}
	return true
}
func validDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == value
}
func validPath(value string) bool {
	return fs.ValidPath(value) && path.Clean(value) == value && value != "." && value != "manifest.json" && !strings.ContainsAny(value, "\\:") && len(value) <= 256
}
func validExpected(e Expected) bool {
	if !validUUID(e.OperationID) || !validUUID(e.DeploymentGeneration) || !validLabel(e.Environment, 128) || (e.Method != "CURRENT_SOURCE_CAPTURE" && e.Method != "VERIFIED_PITR_HISTORY") || len(e.Stores) == 0 || len(e.Stores) > 16 || len(e.RequiredKinds) == 0 || len(e.RequiredKinds) > 128 {
		return false
	}
	for name, id := range e.Stores {
		if !validLabel(name, 128) || !validUUID(id) {
			return false
		}
	}
	for kind, store := range e.RequiredKinds {
		if !validLabel(kind, 128) || e.Stores[store] == "" {
			return false
		}
	}
	return true
}
func boundManifest(m Manifest, e Expected) bool {
	if m.ProtocolVersion != 1 || m.OperationID != e.OperationID || m.Environment != e.Environment || m.DeploymentGeneration != e.DeploymentGeneration || m.Method != e.Method || len(m.Stores) != len(e.Stores) || len(m.Artifacts) != len(e.RequiredKinds) {
		return false
	}
	seen := map[string]bool{}
	for _, store := range m.Stores {
		if seen[store.Name] || e.Stores[store.Name] != store.Incarnation {
			return false
		}
		seen[store.Name] = true
	}
	return true
}
func readFile(root *os.Root, name string, maximum int64) ([]byte, error) {
	// The protected mount must be read-only. OpenRoot additionally confines any
	// concurrent path change; the immutable returned bytes are used thereafter.
	parts := strings.Split(name, "/")
	for index := range parts {
		info, err := root.Lstat(strings.Join(parts[:index+1], "/"))
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrInvalid
		}
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maximum {
		return nil, ErrInvalid
	}
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(data)) > maximum {
		return nil, ErrInvalid
	}
	return data, nil
}

func strictJSON(raw []byte, destination any) error {
	check := json.NewDecoder(bytes.NewReader(raw))
	check.UseNumber()
	if err := checkJSONValue(check, 0); err != nil {
		return err
	}
	if _, err := check.Token(); err != io.EOF {
		return ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(destination) != nil {
		return ErrInvalid
	}
	return nil
}
func checkJSONValue(decoder *json.Decoder, depth int) error {
	if depth > 32 {
		return ErrInvalid
	}
	token, err := decoder.Token()
	if err != nil {
		return ErrInvalid
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			key, err := decoder.Token()
			name, ok := key.(string)
			if err != nil || !ok || seen[name] {
				return ErrInvalid
			}
			seen[name] = true
			if checkJSONValue(decoder, depth+1) != nil {
				return ErrInvalid
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return ErrInvalid
		}
	case '[':
		for decoder.More() {
			if checkJSONValue(decoder, depth+1) != nil {
				return ErrInvalid
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}
