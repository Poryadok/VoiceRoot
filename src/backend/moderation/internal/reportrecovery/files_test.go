package reportrecovery

import (
	"os"
	"path/filepath"
	"testing"
)

func privateTempDir(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	info, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("operator fixture parent mode %04o -> 0700", info.Mode().Perm())
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestPublishCompleteDirectoryAndIdempotentReadback(t *testing.T) {
	parent := privateTempDir(t)
	m, chunks, cut := fixture(t, fixtureView(), line(t, fixtureRecord()))
	directory, err := PublishDirectory(parent, m, chunks, cut)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := ReadDirectory(directory, cut)
	if err != nil || len(snapshot.Records()) != 1 {
		t.Fatalf("readback: %v", err)
	}
	again, err := PublishDirectory(parent, m, chunks, cut)
	if err != nil || again != directory {
		t.Fatalf("idempotent publication: %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 1 {
		t.Fatalf("orphan staging directory: %v", err)
	}
	info, err := os.Stat(directory)
	if err != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatalf("directory is not private: %v", err)
	}
	for _, name := range []string{"manifest.json", cut.Artifacts[0].Kind + ".ndjson"} {
		info, err := os.Stat(filepath.Join(directory, name))
		if err != nil || info.Mode().Perm()&0077 != 0 {
			t.Fatalf("artifact is not private: %v", err)
		}
	}
}

func TestDirectoryRejectsTamperMissingExtraAndSymlink(t *testing.T) {
	for _, kind := range []string{"tamper", "missing", "extra", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			m, chunks, cut := fixture(t, fixtureView(), line(t, fixtureRecord()))
			parent := privateTempDir(t)
			directory, err := PublishDirectory(parent, m, chunks, cut)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, cut.Artifacts[0].Kind+".ndjson")
			switch kind {
			case "tamper":
				err = os.WriteFile(path, []byte("corrupt\n"), 0600)
			case "missing":
				err = os.Remove(path)
			case "extra":
				err = os.WriteFile(filepath.Join(directory, "extra"), nil, 0600)
			case "symlink":
				if err = os.Remove(path); err == nil {
					err = os.Symlink(filepath.Join(directory, "manifest.json"), path)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ReadDirectory(directory, cut); err == nil {
				t.Fatal("accepted altered directory")
			}
			if _, err := PublishDirectory(parent, m, chunks, cut); err == nil {
				t.Fatal("silently replaced altered existing snapshot")
			}
		})
	}
}

func TestInvalidInputCannotPublishAndExpectedPathsAreBounded(t *testing.T) {
	m, chunks, cut := fixture(t, fixtureView(), line(t, fixtureRecord()))
	parent := privateTempDir(t)
	m[0] = '!'
	if _, err := PublishDirectory(parent, m, chunks, cut); err == nil {
		t.Fatal("invalid bytes published")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid input created partial output: %v", err)
	}
	cut.Artifacts[0].Kind = "../private"
	if _, err := ReadDirectory(parent, cut); err == nil {
		t.Fatal("unsafe expected path accepted")
	}
}

func TestPublishRefusesPublicParent(t *testing.T) {
	parent := privateTempDir(t)
	if err := os.Chmod(parent, 0755); err != nil {
		t.Fatal(err)
	}
	m, chunks, cut := fixture(t, fixtureView(), line(t, fixtureRecord()))
	if _, err := PublishDirectory(parent, m, chunks, cut); err == nil {
		t.Fatal("public parent accepted")
	}
}

func TestInterruptedStagingAndConcurrentSameSnapshotPublication(t *testing.T) {
	parent := privateTempDir(t)
	m, chunks, cut := fixture(t, fixtureView(), line(t, fixtureRecord()))
	staging, err := os.MkdirTemp(parent, ".report-journal-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, cut.Artifacts[0].Kind+".ndjson"), chunks[cut.Artifacts[0].Kind], 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadDirectory(staging, cut); err == nil {
		t.Fatal("interrupted staging accepted without manifest")
	}
	type result struct {
		path string
		err  error
	}
	done := make(chan result, 2)
	for range 2 {
		go func() { path, err := PublishDirectory(parent, m, chunks, cut); done <- result{path, err} }()
	}
	first, second := <-done, <-done
	if first.err != nil || second.err != nil || first.path != second.path {
		t.Fatalf("concurrent publication: %v / %v", first.err, second.err)
	}
	if _, err := ReadDirectory(first.path, cut); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(staging); err != nil {
		t.Fatal("publication removed an earlier operation's private staging")
	}
	changed := fixtureRecord()
	changed.EvidenceVersion++
	otherManifest, otherChunks, otherCut, err := Encode(cut.View, []Record{changed})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PublishDirectory(parent, otherManifest, otherChunks, otherCut); err == nil {
		t.Fatal("same snapshot UUID silently replaced with different bytes")
	}
}
