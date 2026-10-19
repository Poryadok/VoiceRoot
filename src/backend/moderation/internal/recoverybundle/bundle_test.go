package recoverybundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func fixture(t *testing.T) (string, Manifest, Expected) {
	t.Helper()
	dir := t.TempDir()
	data := []byte("opaque report journal fixture\n")
	digest := sha256.Sum256(data)
	operation, generation, store := uuid.NewString(), uuid.NewString(), uuid.NewString()
	m := Manifest{ProtocolVersion: 1, OperationID: operation, Environment: "test", DeploymentGeneration: generation, Method: "CURRENT_SOURCE_CAPTURE", Stores: []Store{{Name: "moderation", Incarnation: store}}, Artifacts: []Artifact{{Kind: "report_journal", Store: "moderation", Path: "journal.jsonl", Length: int64(len(data)), SHA256: hex.EncodeToString(digest[:]), SchemaVersion: 1, Generation: uuid.NewString(), RecordCount: 1, Cut: "finite-cut"}}}
	if err := os.WriteFile(filepath.Join(dir, "journal.jsonl"), data, 0600); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, dir, m)
	return dir, m, Expected{OperationID: operation, Environment: "test", DeploymentGeneration: generation, Method: "CURRENT_SOURCE_CAPTURE", Stores: map[string]string{"moderation": store}, RequiredKinds: map[string]string{"report_journal": "moderation"}}
}
func writeManifest(t *testing.T, dir string, m Manifest) {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestInspectRejectsUnboundOperationAndMissingCoverage(t *testing.T) {
	for _, change := range []string{"operation", "environment", "generation", "store", "missing", "method"} {
		t.Run(change, func(t *testing.T) {
			dir, m, e := fixture(t)
			switch change {
			case "operation":
				e.OperationID = uuid.NewString()
			case "environment":
				e.Environment = "other"
			case "generation":
				e.DeploymentGeneration = uuid.NewString()
			case "store":
				e.Stores["moderation"] = uuid.NewString()
			case "missing":
				e.RequiredKinds["auth_terminal"] = "auth"
			case "method":
				m.Method = "OPERATOR_COMPLETE"
				writeManifest(t, dir, m)
			}
			if _, err := Inspect(dir, e); err == nil {
				t.Fatal("integrity-only bundle accepted as independently bound inventory")
			}
		})
	}
}
func TestInspectRejectsAmbiguousJSONAndInventory(t *testing.T) {
	for _, change := range []string{"duplicate", "unknown", "extra-file", "duplicate-kind", "escape"} {
		t.Run(change, func(t *testing.T) {
			dir, m, e := fixture(t)
			switch change {
			case "duplicate", "unknown":
				raw, _ := json.Marshal(m)
				field := `"protocol_version":1,`
				if change == "unknown" {
					field = `"complete":true,`
				}
				raw = []byte("{" + field + strings.TrimPrefix(string(raw), "{"))
				if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0600); err != nil {
					t.Fatal(err)
				}
			case "extra-file":
				if err := os.WriteFile(filepath.Join(dir, "unexpected"), []byte("data"), 0600); err != nil {
					t.Fatal(err)
				}
			case "duplicate-kind":
				m.Artifacts = append(m.Artifacts, m.Artifacts[0])
				writeManifest(t, dir, m)
			case "escape":
				m.Artifacts[0].Path = "./journal.jsonl"
				writeManifest(t, dir, m)
			}
			if _, err := Inspect(dir, e); err == nil {
				t.Fatal("ambiguous or unconfined inventory accepted")
			}
		})
	}
}
func TestInspectRetainsVerifiedBytes(t *testing.T) {
	dir, _, e := fixture(t)
	inspected, err := Inspect(dir, e)
	if err != nil {
		t.Fatal(err)
	}
	original := inspected.Artifact("report_journal")
	if err = os.WriteFile(filepath.Join(dir, "journal.jsonl"), []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	original[0] = 'X'
	if string(inspected.Artifact("report_journal")) != "opaque report journal fixture\n" {
		t.Fatal("inspected input changed after verification")
	}
	m := inspected.Manifest()
	m.Artifacts[0].Cut = "mutated"
	m.Stores[0].Name = "mutated"
	if inspected.Manifest().Artifacts[0].Cut != "finite-cut" || inspected.Manifest().Stores[0].Name != "moderation" {
		t.Fatal("inspection metadata was mutable")
	}
}

func TestInspectRejectsInvalidArtifactMetadata(t *testing.T) {
	for _, change := range []string{"version", "length", "hash", "count", "artifact-generation", "schema", "unknown-kind", "mixed-store", "parent-path", "absolute-path", "windows-path", "method-mismatch", "trailing-json", "nested-duplicate"} {
		t.Run(change, func(t *testing.T) {
			dir, m, e := fixture(t)
			switch change {
			case "version":
				m.ProtocolVersion = 2
			case "length":
				m.Artifacts[0].Length = 16<<20 + 1
			case "hash":
				m.Artifacts[0].SHA256 = strings.Repeat("0", 64)
			case "count":
				m.Artifacts[0].RecordCount = -1
			case "artifact-generation":
				m.Artifacts[0].Generation = "not-a-uuid"
			case "schema":
				m.Artifacts[0].SchemaVersion = 2
			case "unknown-kind":
				m.Artifacts[0].Kind = "unknown"
				m.Artifacts[0].Store = ""
			case "mixed-store":
				m.Artifacts[0].Store = "auth"
			case "parent-path":
				m.Artifacts[0].Path = "../journal.jsonl"
			case "absolute-path":
				m.Artifacts[0].Path = "/journal.jsonl"
			case "windows-path":
				m.Artifacts[0].Path = `C:\journal.jsonl`
			case "method-mismatch":
				m.Method = "VERIFIED_PITR_HISTORY"
			}
			writeManifest(t, dir, m)
			if change == "trailing-json" || change == "nested-duplicate" {
				raw, _ := json.Marshal(m)
				if change == "trailing-json" {
					raw = append(raw, []byte(" {}")...)
				} else {
					raw = []byte(strings.Replace(string(raw), `"record_count":1`, `"record_count":1,"record_count":1`, 1))
				}
				if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Inspect(dir, e); err == nil {
				t.Fatal("invalid artifact metadata accepted")
			}
		})
	}
}

func TestInspectRejectsSymlinkedArtifactsAndDirectories(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "directory"}[directory], func(t *testing.T) {
			dir, m, e := fixture(t)
			outside := t.TempDir()
			data, err := os.ReadFile(filepath.Join(dir, "journal.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(outside, "journal.jsonl"), data, 0600); err != nil {
				t.Fatal(err)
			}
			if err = os.Remove(filepath.Join(dir, "journal.jsonl")); err != nil {
				t.Fatal(err)
			}
			if directory {
				if err = os.Symlink(outside, filepath.Join(dir, "linked")); err != nil {
					t.Fatal(err)
				}
				m.Artifacts[0].Path = "linked/journal.jsonl"
				writeManifest(t, dir, m)
			} else {
				if err = os.Symlink(filepath.Join(outside, "journal.jsonl"), filepath.Join(dir, "journal.jsonl")); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = Inspect(dir, e); err == nil {
				t.Fatal("symlink outside protected inventory accepted")
			}
		})
	}
}
