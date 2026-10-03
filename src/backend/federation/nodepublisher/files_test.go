package nodepublisher

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"voice/backend/federation/mediaauthority"
	"voice/backend/federation/protocol"
)

func TestLinuxFileSinkAtomicallyReplacesCompleteBundleAndPreservesItOnCancellation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux node filesystem durability proof runs in the controller artifact")
	}
	directory := t.TempDir()
	sink, err := NewFileSink(directory)
	require.NoError(t, err)
	space := uuid.NewString()
	bundle := mediaauthority.Bundle{Scope: protocol.Scope{SpaceID: space}, Manifest: protocol.Envelope{Payload: "first"}}
	require.NoError(t, sink.Publish(context.Background(), space, bundle))
	bundle.Manifest.Payload = "second"
	require.NoError(t, sink.Publish(context.Background(), space, bundle))
	path := filepath.Join(directory, space+".json")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var read mediaauthority.Bundle
	require.NoError(t, json.Unmarshal(raw, &read))
	require.Equal(t, bundle, read)
	before := append([]byte(nil), raw...)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bundle.Manifest.Payload = "cancelled"
	require.ErrorIs(t, sink.Publish(ctx, space, bundle), ErrUnavailable)
	raw, err = os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, before, raw)
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	require.Len(t, entries, 1, "temporary partial files do not survive")
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0640), info.Mode().Perm())
}
