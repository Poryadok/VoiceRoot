package mediaauthority

import (
	"crypto/ed25519"
	"crypto/rand"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/federation/protocol"
)

func TestBootRegistryRequiresFreshLeaseForItsOwnProcess(t *testing.T) {
	public, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	verifier := Verifier{Issuer: "master", Environment: "sandbox", NodeID: uuid.NewString(), Keys: map[string]ed25519.PublicKey{"current": public}}
	first, err := NewBootRegistry(verifier, 250*time.Millisecond)
	require.NoError(t, err)
	restarted, err := NewBootRegistry(verifier, 250*time.Millisecond)
	require.NoError(t, err)
	require.NotEqual(t, first.BootNonce(), restarted.BootNonce())
	grant := Grant{Version: 1, Issuer: verifier.Issuer, Audience: "voice-node-media", Environment: verifier.Environment, NodeID: verifier.NodeID, SpaceID: uuid.NewString(), Generation: 1, AuthorityEpoch: 1, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), ResourceID: uuid.NewString(), SessionEpoch: 1, RoomName: "fresh-room", RoutingGeneration: 1, Nonce: uuid.NewString(), IssuedAt: now.UnixMilli(), ExpiresAt: now.Add(30 * time.Second).UnixMilli()}
	bundle := authorityBundle(t, key, grant, 1, now.Add(1900*time.Millisecond), now, true)
	require.ErrorIs(t, first.Apply(bundle, now), ErrDenied, "legacy file cannot establish boot authority")
	lease, err := protocol.VerifyEnvelope(public, bundle.Lease, now)
	require.NoError(t, err)
	lease.ReceiverBootNonces = []string{first.BootNonce()}
	bundle.Lease, err = protocol.SignEnvelope(key, "current", lease)
	require.NoError(t, err)
	require.NoError(t, first.Apply(bundle, now))
	require.ErrorIs(t, restarted.Apply(bundle, now), ErrDenied, "still-valid restored bytes cannot admit a new process")
	// Replaying at the old wall-clock value does not give the new process the
	// old UUID. Only a fresh master lease for the new boot can activate policy.
	require.ErrorIs(t, restarted.Apply(bundle, now), ErrDenied)
	old := bundle
	lease.ReceiverBootNonces = []string{first.BootNonce(), restarted.BootNonce()}
	slices.Sort(lease.ReceiverBootNonces)
	bundle.Lease, err = protocol.SignEnvelope(key, "current", lease)
	require.NoError(t, err)
	require.NoError(t, restarted.Apply(bundle, now))
	token, err := Sign(key, "current", grant, now)
	require.NoError(t, err)
	admission, err := restarted.Admit(token, grant.RoomName, grant.ProfileID, now)
	require.NoError(t, err)
	require.ErrorIs(t, restarted.Apply(old, now), ErrDenied)
	require.NoError(t, restarted.Check(admission, now), "bad boot replay cannot replace current verified authority")
	third, err := NewBootRegistry(verifier, 250*time.Millisecond)
	require.NoError(t, err)
	require.ErrorIs(t, third.Apply(bundle, now), ErrDenied, "another restart needs another master round trip")
}
