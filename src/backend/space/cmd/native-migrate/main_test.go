package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	"io"
	"strings"
	"testing"
	"voice/backend/space/internal/nativeinstaller"
	"voice/backend/space/internal/store"
)

func TestInstallerDoesNotDefaultToTrustExisting(t *testing.T) {
	for _, mode := range []string{"", "up", "force", "trust-existing", "resume"} {
		require.Error(t, nativeinstaller.Run(context.Background(), nativeinstaller.Config{Mode: mode, Output: io.Discard}))
	}
}

type evidenceReadCloser struct {
	io.Reader
	closeErr error
	closes   int
}

func (r *evidenceReadCloser) Close() error {
	r.closes++
	return r.closeErr
}

func TestReadReceiptAndClosePreservesParseAndCloseFailures(t *testing.T) {
	f := &store.NativeInstallerSourceFacts{Version: 44, DatabaseSystemID: "cluster", DatabaseName: "space_db", DatabaseOID: 42, InstallerRole: "installer", Epoch: "94db7e08-e958-44fa-94b2-988410ab0044", SealSHA256: strings.Repeat("a", 64)}
	p := &store.NativeMembershipCatalog45Receipt{DatabaseSystemID: f.DatabaseSystemID, DatabaseName: f.DatabaseName, DatabaseOID: f.DatabaseOID, InstallerRole: f.InstallerRole, Epoch: f.Epoch, OldSealSHA256: f.SealSHA256, NewSealSHA256: strings.Repeat("b", 64), Changed: true}
	q := store.NativeMembershipCatalog50Receipt(*p)
	q.OldSealSHA256, q.NewSealSHA256 = p.NewSealSHA256, strings.Repeat("c", 64)
	end := *f
	end.Version, end.SealSHA256 = 50, q.NewSealSHA256
	r := &nativeinstaller.Receipt{Created: true, Creation: f, Trusted44: f, Transition45: p, Transition50: &q, Final: &end, SourceSHA256: strings.Repeat("d", 64), ExecutableSHA256: strings.Repeat("e", 64), MigratorSHA256: strings.Repeat("f", 64), InventorySHA256: strings.Repeat("a", 64)}
	valid, err := json.Marshal(map[string]any{"event": "complete", "receipt": r})
	require.NoError(t, err)
	closeErr := errors.New("postgres://secret-cleanup-fixture@invalid/retained")
	for _, tc := range []struct {
		name       string
		raw        []byte
		closeErr   error
		parseError bool
	}{
		{"valid", valid, nil, false},
		{"close failure", valid, closeErr, false},
		{"parse failure", []byte("{"), nil, true},
		{"parse and close failure", []byte("{"), closeErr, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &evidenceReadCloser{Reader: bytes.NewReader(tc.raw), closeErr: tc.closeErr}
			got, err := readReceiptAndClose(reader)
			require.Equal(t, 1, reader.closes)
			if !tc.parseError && tc.closeErr == nil {
				require.NoError(t, err)
				require.NotNil(t, got)
				return
			}
			require.Error(t, err)
			require.Nil(t, got)
			if tc.parseError {
				require.ErrorContains(t, err, "invalid retained installer evidence")
			}
			if tc.closeErr != nil {
				require.ErrorIs(t, err, tc.closeErr)
				require.ErrorContains(t, err, "retained evidence close failed")
			}
			require.NotContains(t, err.Error(), "secret-cleanup-fixture")
		})
	}
}
