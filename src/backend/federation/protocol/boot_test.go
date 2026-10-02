package protocol

import (
	"crypto/ed25519"
	"crypto/rand"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestLeaseReceiverBootNonceListIsBoundedCanonicalAndSigned(t *testing.T) {
	public, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	nonces := []string{uuid.NewString(), uuid.NewString()}
	slices.Sort(nonces)
	claims := Claims{Version: 1, Kind: "lease", Issuer: "master", Audience: "voice-node", Environment: "sandbox", NodeID: uuid.NewString(), SpaceID: uuid.NewString(), Generation: 1, Epoch: 1, Revision: 1, IssuedAt: now.UnixMilli(), ExpiresAt: now.Add(time.Second).UnixMilli(), Hash: SnapshotDigest(Snapshot{}), ReceiverBootNonces: nonces}
	for name, list := range map[string][]string{"legacy": nil, "two receivers": nonces, "empty nonce": {""}, "duplicate": {nonces[0], nonces[0]}, "unsorted": {nonces[1], nonces[0]}, "noncanonical": {"BOOT"}, "too many": make([]string, MaxReceiverBootNonces+1)} {
		t.Run(name, func(t *testing.T) {
			c := claims
			c.ReceiverBootNonces = list
			signed, err := SignEnvelope(key, "current", c)
			require.NoError(t, err)
			verified, err := VerifyEnvelope(public, signed, now)
			if name == "legacy" || name == "two receivers" {
				require.NoError(t, err)
				require.Equal(t, list, verified.ReceiverBootNonces)
			} else {
				require.Error(t, err)
			}
		})
	}
}
