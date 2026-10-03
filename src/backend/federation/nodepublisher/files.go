package nodepublisher

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"voice/backend/federation/mediaauthority"
)

type FileSink struct{ directory string }

func NewFileSink(directory string) (*FileSink, error) {
	if !filepath.IsAbs(directory) {
		return nil, ErrConfig
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrConfig
	}
	return &FileSink{directory: directory}, nil
}

// Publish writes only a complete verified bundle and syncs its rename. The
// Linux node volume must give this writer ownership and the SFU read access;
// dot-prefixed temp files never match the SFU's *.json reader.
func (s *FileSink) Publish(ctx context.Context, space string, bundle mediaauthority.Bundle) error {
	id, err := uuid.Parse(space)
	if s == nil || err != nil || id == uuid.Nil || id.String() != space || bundle.Scope.SpaceID != space {
		return ErrConfig
	}
	raw, err := json.Marshal(bundle)
	if err != nil || len(raw) > MaxBundleBytes || ctx.Err() != nil {
		return ErrUnavailable
	}
	temporary, err := os.CreateTemp(s.directory, "."+space+"-*.tmp")
	if err != nil {
		return ErrUnavailable
	}
	defer func() { _ = temporary.Close(); _ = os.Remove(temporary.Name()) }()
	if temporary.Chmod(0640) != nil {
		return ErrUnavailable
	}
	if _, err = temporary.Write(raw); err != nil {
		return ErrUnavailable
	}
	if temporary.Sync() != nil || temporary.Close() != nil || ctx.Err() != nil {
		return ErrUnavailable
	}
	if os.Rename(temporary.Name(), filepath.Join(s.directory, space+".json")) != nil {
		return ErrUnavailable
	}
	directory, err := os.Open(s.directory)
	if err != nil {
		return ErrUnavailable
	}
	defer func() { _ = directory.Close() }()
	if directory.Sync() != nil {
		return ErrUnavailable
	}
	return nil
}
