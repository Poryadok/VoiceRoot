package reportrecovery

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// PublishDirectory requires a private operator-owned parent with no unrelated
// concurrent writers. Files are synchronized and read back in a private staging
// directory; renaming the complete nonempty directory publishes its manifest.
// A pre-existing snapshot is verified, never silently replaced or repaired.
func PublishDirectory(parent string, manifest []byte, chunks map[string][]byte, expected Cut) (string, error) {
	if _, err := Parse(manifest, chunks, expected); err != nil {
		return "", err
	}
	parent, err := filepath.Abs(parent)
	if err != nil {
		return "", err
	}
	if err := privateDirectory(parent); err != nil {
		return "", err
	}
	final := filepath.Join(parent, "report-journal-"+expected.View.SnapshotID.String())
	if _, err := os.Lstat(final); err == nil {
		if _, err := ReadDirectory(final, expected); err != nil {
			return "", err
		}
		return final, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	staging, err := os.MkdirTemp(parent, ".report-journal-")
	if err != nil {
		return "", err
	}
	// MkdirTemp created this exact child of the verified absolute parent. No
	// other path or existing operator output is passed to recursive cleanup.
	defer func() { _ = os.RemoveAll(staging) }()
	for _, artifact := range expected.Artifacts {
		if err := writeFileSync(filepath.Join(staging, artifact.Kind+".ndjson"), chunks[artifact.Kind]); err != nil {
			return "", err
		}
	}
	if err := writeFileSync(filepath.Join(staging, "manifest.json"), manifest); err != nil {
		return "", err
	}
	if _, err := ReadDirectory(staging, expected); err != nil {
		return "", err
	}
	if err := syncDirectory(staging); err != nil {
		return "", err
	}
	if err := os.Rename(staging, final); err != nil {
		// Another publisher of this same immutable snapshot may have won. Its
		// complete identity must agree before treating the retry as successful.
		if _, readErr := ReadDirectory(final, expected); readErr != nil {
			return "", err
		}
	}
	if err := syncDirectory(parent); err != nil {
		return "", err
	}
	return final, nil
}

// ReadDirectory uses only the validated fixed artifact names from the expected
// inventory and rejects symlinks, extra files and incomplete publication.
func ReadDirectory(directory string, expected Cut) (*Snapshot, error) {
	if !validFileInventory(expected) {
		return nil, ErrInvalid
	}
	if err := privateDirectory(directory); err != nil {
		return nil, err
	}
	dir, err := os.Open(directory)
	if err != nil {
		return nil, err
	}
	entries, readErr := dir.ReadDir(len(expected.Artifacts) + 2)
	closeErr := dir.Close()
	if (readErr != nil && !errors.Is(readErr, io.EOF)) || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if len(entries) != len(expected.Artifacts)+1 {
		return nil, ErrInvalid
	}
	names := map[string]bool{"manifest.json": true}
	for _, artifact := range expected.Artifacts {
		names[artifact.Kind+".ndjson"] = true
	}
	for _, entry := range entries {
		if !names[entry.Name()] || !entry.Type().IsRegular() {
			return nil, ErrInvalid
		}
	}
	manifest, err := readBoundedFile(filepath.Join(directory, "manifest.json"), MaxManifestBytes)
	if err != nil {
		return nil, err
	}
	chunks := make(map[string][]byte, len(expected.Artifacts))
	for _, artifact := range expected.Artifacts {
		b, err := readBoundedFile(filepath.Join(directory, artifact.Kind+".ndjson"), artifact.ByteLength)
		if err != nil {
			return nil, err
		}
		chunks[artifact.Kind] = b
	}
	return Parse(manifest, chunks, expected)
}

func validFileInventory(c Cut) bool {
	if !validView(c.View) || !validHash(c.ManifestSHA256) || len(c.Artifacts) != c.View.ChunkCount {
		return false
	}
	total := 0
	for i, a := range c.Artifacts {
		if a.Kind != chunkKind(i+1) || a.Ordinal != i+1 || a.Schema != 1 || a.RecordCount != chunkRecords(c.View, i+1) || a.ByteLength <= 0 || a.ByteLength > MaxChunkBytes || !validHash(a.SHA256) {
			return false
		}
		total += a.ByteLength
	}
	return total <= MaxTotalBytes
}

func privateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return ErrInvalid
	}
	return nil
}

func readBoundedFile(path string, limit int) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > int64(limit) {
		return nil, ErrInvalid
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	b, readErr := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	closeErr := f.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if len(b) > limit {
		return nil, ErrInvalid
	}
	return b, nil
}

func writeFileSync(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, writeErr := f.Write(b)
	if writeErr == nil && n != len(b) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = f.Sync()
	}
	return errors.Join(writeErr, f.Close())
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}
