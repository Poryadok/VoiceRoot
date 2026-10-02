package tombstone

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDevelopmentTombstoneHMACGoldenAndRotationBoundary(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	created := time.Now().UTC().Add(-time.Hour)
	raw, err := json.Marshal(map[string]any{"version": "development-v1", "key_base64": base64.StdEncoding.EncodeToString(key), "created_at": created})
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "fixture.json")
	require.NoError(t, os.WriteFile(path, raw, 0600))
	provider, err := LoadDevelopmentKey(path)
	require.NoError(t, err)
	account := uuid.MustParse("12345678-1234-4234-8234-123456789abc")
	digest, version, err := provider.HashAccount(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "development-v1", version)
	require.Equal(t, "edf13f3b40c4d11bd4d1b5b1e081538927bae9937298c07d0d6549d84e4c027b", hex.EncodeToString(digest), "domain, NUL separator and raw UUID bytes are contractual")
	provider.now = func() time.Time { return created.Add(90*24*time.Hour - time.Nanosecond) }
	_, _, err = provider.HashAccount(context.Background(), account)
	require.NoError(t, err)
	provider.now = func() time.Time { return created.Add(90 * 24 * time.Hour) }
	_, _, err = provider.HashAccount(context.Background(), account)
	require.Error(t, err)
	provider.now = func() time.Time { return created.Add(-time.Nanosecond) }
	_, _, err = provider.HashAccount(context.Background(), account)
	require.Error(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = provider.HashAccount(ctx, account)
	require.ErrorIs(t, err, context.Canceled)
}
