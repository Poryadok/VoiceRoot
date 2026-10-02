package mediaauthority

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/federation/protocol"
)

func TestRegistryIsolatesSpacesAndNeverActivatesPartialOrRolledBackPolicy(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	verifier := Verifier{Issuer: "master", Environment: "sandbox", NodeID: uuid.NewString(), Keys: map[string]ed25519.PublicKey{"current": public}}
	registry, err := NewRegistry(verifier, 250*time.Millisecond)
	require.NoError(t, err)
	grant := Grant{Version: 1, Issuer: verifier.Issuer, Audience: "voice-node-media", Environment: verifier.Environment,
		NodeID: verifier.NodeID, SpaceID: uuid.NewString(), Generation: 1, AuthorityEpoch: 1, AccountID: uuid.NewString(),
		ProfileID: uuid.NewString(), ResourceID: uuid.NewString(), SessionEpoch: 7, RoomName: "explicit-room-a", IssuedAt: now.UnixMilli(), ExpiresAt: now.Add(30 * time.Second).UnixMilli()}
	second := grant
	second.SpaceID, second.ResourceID, second.RoomName = uuid.NewString(), uuid.NewString(), "explicit-room-b"
	firstBundle := authorityBundle(t, private, grant, 1, now.Add(time.Second), now, true)
	secondBundle := authorityBundle(t, private, second, 1, now.Add(2*time.Second), now, true)
	require.NoError(t, registry.Apply(firstBundle, now))
	require.NoError(t, registry.Apply(secondBundle, now))
	token, err := Sign(private, "current", grant, now)
	require.NoError(t, err)
	otherToken, err := Sign(private, "current", second, now)
	require.NoError(t, err)
	admission, err := registry.Admit(token, grant.RoomName, grant.ProfileID, now)
	require.NoError(t, err)
	otherAdmission, err := registry.Admit(otherToken, second.RoomName, second.ProfileID, now)
	require.NoError(t, err)
	require.ErrorIs(t, registry.Check(admission, now.Add(time.Second)), ErrDenied)
	require.NoError(t, registry.Check(otherAdmission, now.Add(time.Second)), "one expired Space cannot block another")
	require.NoError(t, registry.Check(otherAdmission, now.Add(900*time.Millisecond)), "bounded out-of-order clock samples conservatively retain healthy media")
	_, err = registry.Admit(token, grant.RoomName, grant.ProfileID, now.Add(time.Second))
	require.ErrorIs(t, err, ErrDenied, "a valid JWT/grant cannot bypass an expired authority lease")

	continued := now.Add(time.Second)
	partial := authorityBundle(t, private, second, 2, now.Add(2*time.Second), continued, false)
	partial.Pages = nil
	require.ErrorIs(t, registry.Apply(partial, continued), ErrDenied)
	require.NoError(t, registry.Check(otherAdmission, now.Add(time.Second)), "partial policy does not overwrite the complete active revision")
	revoked := authorityBundle(t, private, second, 2, now.Add(2*time.Second), continued, false)
	require.NoError(t, registry.Apply(revoked, continued))
	require.ErrorIs(t, registry.Check(otherAdmission, now), ErrDenied)
	_, err = registry.Admit(otherToken, second.RoomName, second.ProfileID, now)
	require.ErrorIs(t, err, ErrDenied, "stale reconnect is denied synchronously after revocation")
	require.ErrorIs(t, registry.Apply(secondBundle, now), ErrDenied, "an older signed backup cannot restore permissions")
	require.ErrorIs(t, registry.Check(Admission{}, now), ErrDenied)

	newGeneration := second
	newGeneration.Generation, newGeneration.AuthorityEpoch = 2, 2
	require.NoError(t, registry.Apply(authorityBundle(t, private, newGeneration, 1, now.Add(2*time.Second), continued, true), continued))
	_, err = registry.Admit(otherToken, second.RoomName, second.ProfileID, now)
	require.ErrorIs(t, err, ErrDenied, "old generation credentials cannot enter the restored generation")
	require.ErrorIs(t, registry.Apply(revoked, now), ErrDenied)
	// A signed higher revision that was saved before expiry cannot make an
	// elapsed deadline fresh by rolling the receiver's wall clock backward.
	require.ErrorIs(t, registry.Check(otherAdmission, now.Add(3*time.Second)), ErrDenied)
	newerBackup := authorityBundle(t, private, newGeneration, 2, now.Add(2*time.Second), continued, true)
	require.ErrorIs(t, registry.Apply(newerBackup, continued), ErrDenied)
}

func TestRegistryDoesNotReplayExpiredGrantAcrossBoundedClockRollback(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	grant := Grant{Version: 1, Issuer: "master", Audience: "voice-node-media", Environment: "sandbox", NodeID: uuid.NewString(), SpaceID: uuid.NewString(),
		Generation: 1, AuthorityEpoch: 1, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), ResourceID: uuid.NewString(), SessionEpoch: 1,
		RoomName: "exact-room", IssuedAt: now.UnixMilli(), ExpiresAt: now.Add(time.Second).UnixMilli()}
	registry, err := NewRegistry(Verifier{Issuer: grant.Issuer, Environment: grant.Environment, NodeID: grant.NodeID, Keys: map[string]ed25519.PublicKey{"current": public}}, 250*time.Millisecond)
	require.NoError(t, err)
	require.NoError(t, registry.Apply(authorityBundle(t, private, grant, 1, now.Add(2*time.Second), now, true), now))
	token, err := Sign(private, "current", grant, now)
	require.NoError(t, err)
	admission, err := registry.Admit(token, grant.RoomName, grant.ProfileID, now)
	require.NoError(t, err)
	_, err = registry.Admit(token, grant.RoomName, grant.ProfileID, now.Add(800*time.Millisecond))
	require.ErrorIs(t, err, ErrDenied)
	_, err = registry.Admit(token, grant.RoomName, grant.ProfileID, now.Add(650*time.Millisecond))
	require.ErrorIs(t, err, ErrDenied, "earlier wall time cannot make the expired admission credential usable")
	require.NoError(t, registry.Check(admission, now.Add(650*time.Millisecond)), "a healthy active session still follows fresh authority independently")
}

func authorityBundle(t *testing.T, private ed25519.PrivateKey, grant Grant, revision int64, until, now time.Time, allow bool) Bundle {
	t.Helper()
	permissions := []protocol.Permission{}
	if allow {
		permissions = append(permissions, protocol.Permission{AccountID: grant.AccountID, ProfileID: grant.ProfileID, ResourceID: grant.ResourceID, SessionEpoch: grant.SessionEpoch, Actions: []string{"media"}})
	}
	snapshot := protocol.Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: revision, ValidUntil: until.UnixMilli(), Permissions: permissions}
	manifest, err := protocol.SignSnapshotManifest(private, "current", grant.Scope(), protocol.ManifestFor(snapshot), now)
	require.NoError(t, err)
	pages, err := protocol.SignSnapshotPages(private, "current", grant.Scope(), snapshot, now)
	require.NoError(t, err)
	lease, err := protocol.SignEnvelope(private, "current", protocol.Claims{Version: 1, Kind: "lease", Issuer: grant.Issuer,
		Audience: "voice-node", Environment: grant.Environment, NodeID: grant.NodeID, SpaceID: grant.SpaceID, Generation: grant.Generation,
		Epoch: grant.AuthorityEpoch, Revision: revision, IssuedAt: now.UnixMilli(), ExpiresAt: until.UnixMilli(), Hash: protocol.SnapshotDigest(snapshot)})
	require.NoError(t, err)
	return Bundle{Scope: grant.Scope(), Manifest: manifest, Pages: pages, Lease: lease}
}
