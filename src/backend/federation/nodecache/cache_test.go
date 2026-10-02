package nodecache

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/federation/protocol"
)

func TestSnapshotPagesActivateAtomicallyAndOnlyAppliedRevisionCanBeAcknowledged(t *testing.T) {
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Millisecond)
	scope := Scope{Issuer: "master", Environment: "sandbox", NodeID: uuid.NewString(), SpaceID: uuid.NewString(), Generation: 1, Epoch: 2}
	cache := New(scope, map[string]ed25519.PublicKey{"authority-1": pub})

	first := snapshot(1, now.Add(1500*time.Millisecond), 1)
	applySnapshot(t, cache, private, scope, first, now)
	activeBefore := cache.Current()
	require.EqualValues(t, 1, activeBefore.Revision)

	second := snapshot(2, now.Add(1500*time.Millisecond), protocol.SnapshotPagePermissionLimit+1)
	manifest := signedManifest(t, private, scope, second, now)
	require.NoError(t, cache.StageManifest(manifest, now))
	ack, err := cache.LeaseAck(uuid.NewString())
	require.NoError(t, err)
	require.EqualValues(t, 1, ack.Revision, "an incomplete stage cannot advance the lease ACK")
	require.Equal(t, activeBefore, cache.Current(), "staging must not expose a partial revision")

	pages := signedPages(t, private, scope, second, now)
	require.Len(t, pages, 2)
	require.NoError(t, cache.StagePage(pages[1], now), "pages may arrive out of order")
	require.Equal(t, activeBefore, cache.Current())
	require.NoError(t, cache.StagePage(pages[0], now))
	require.Equal(t, second, cache.Current(), "complete verified revision becomes active atomically")
	ack, err = cache.LeaseAck(uuid.NewString())
	require.NoError(t, err)
	require.EqualValues(t, second.Revision, ack.Revision)
	require.Equal(t, protocol.SnapshotDigest(second), ack.Hash)
}

func TestSnapshotStageRejectsPageTamperingScopeAndRevisionGaps(t *testing.T) {
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Millisecond)
	scope := Scope{Issuer: "master", Environment: "sandbox", NodeID: uuid.NewString(), SpaceID: uuid.NewString(), Generation: 1, Epoch: 1}
	cache := New(scope, map[string]ed25519.PublicKey{"authority-1": pub})
	initial := snapshot(1, now.Add(time.Second), 0)
	applySnapshot(t, cache, private, scope, initial, now)

	gap := snapshot(3, now.Add(time.Second), 0)
	resnapshot, err := protocol.SignRevisionStream(private, "authority-1", scope,
		protocol.RevisionStream{AfterRevision: 1, CurrentRevision: 3, ResnapshotRequired: true, Events: []protocol.RevisionEvent{}}, gap.ValidUntil, now)
	require.NoError(t, err)
	err = cache.ObserveRevisionStream(resnapshot, now)
	require.ErrorIs(t, err, ErrRevisionGap)
	require.NoError(t, cache.StageManifest(signedManifest(t, private, scope, gap, now), now), "full resnapshot may bridge missing revision history")
	require.NoError(t, cache.StagePage(signedPages(t, private, scope, gap, now)[0], now))
	require.EqualValues(t, 3, cache.Current().Revision)

	next := snapshot(4, now.Add(time.Second), protocol.SnapshotPagePermissionLimit+1)
	require.NoError(t, cache.StageManifest(signedManifest(t, private, scope, next, now), now))
	pages := signedPages(t, private, scope, next, now)
	mutated := pages[0]
	mutated.Payload += "x"
	require.Error(t, cache.StagePage(mutated, now), "tampered signature/payload is rejected")

	otherScope := scope
	otherScope.NodeID = uuid.NewString()
	foreign := signedPages(t, private, otherScope, next, now)[0]
	require.Error(t, cache.StagePage(foreign, now), "a valid signature for another node is out of scope")
	require.EqualValues(t, 3, cache.Current().Revision)
}

func TestRevisionEventsRequireContiguousHistoryOrExplicitResnapshot(t *testing.T) {
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Millisecond)
	scope := Scope{Issuer: "master", Environment: "sandbox", NodeID: uuid.NewString(), SpaceID: uuid.NewString(), Generation: 1, Epoch: 1}
	cache := New(scope, map[string]ed25519.PublicKey{"authority-1": pub})
	initial := snapshot(1, now.Add(time.Second), 0)
	applySnapshot(t, cache, private, scope, initial, now)
	eventHash := protocol.EmptySnapshotDigest(2, now.Add(time.Second))
	event := protocol.RevisionEvent{Revision: 2, Hash: eventHash, ValidUntil: now.Add(time.Second).UnixMilli()}
	stream := protocol.RevisionStream{AfterRevision: 1, CurrentRevision: 2, Events: []protocol.RevisionEvent{event}}
	envelope, err := protocol.SignRevisionStream(private, "authority-1", scope, stream, now.Add(time.Second).UnixMilli(), now)
	require.NoError(t, err)
	require.NoError(t, cache.ObserveRevisionStream(envelope, now))
	require.EqualValues(t, 1, cache.Current().Revision, "revision events do not mutate active policy without pages")
	conflictingEvent := event
	conflictingEvent.Hash = protocol.EmptySnapshotDigest(2, now.Add(900*time.Millisecond))
	conflictingStream := protocol.RevisionStream{AfterRevision: 1, CurrentRevision: 2, Events: []protocol.RevisionEvent{conflictingEvent}}
	conflictingEnvelope, err := protocol.SignRevisionStream(private, "authority-1", scope, conflictingStream, now.Add(900*time.Millisecond).UnixMilli(), now)
	require.NoError(t, err)
	require.ErrorIs(t, cache.ObserveRevisionStream(conflictingEnvelope, now), ErrRevisionConflict)

	gap := protocol.RevisionStream{AfterRevision: 0, CurrentRevision: 2, Events: []protocol.RevisionEvent{
		{Revision: 1, Hash: protocol.EmptySnapshotDigest(1, now.Add(time.Second)), ValidUntil: now.Add(time.Second).UnixMilli()},
		{Revision: 2, Hash: protocol.EmptySnapshotDigest(2, now.Add(time.Second)), ValidUntil: now.Add(time.Second).UnixMilli()},
	}}
	gapEnvelope, err := protocol.SignRevisionStream(private, "authority-1", scope, gap, now.Add(time.Second).UnixMilli(), now)
	require.NoError(t, err)
	require.ErrorIs(t, cache.ObserveRevisionStream(gapEnvelope, now), ErrRevisionGap)
}

func TestAuthorizeUsesCompleteSnapshotAndFailsClosedAtExpiryOrUncertainClock(t *testing.T) {
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Millisecond)
	scope := Scope{Issuer: "master", Environment: "sandbox", NodeID: uuid.NewString(), SpaceID: uuid.NewString(), Generation: 1, Epoch: 1}
	cache := New(scope, map[string]ed25519.PublicKey{"authority-1": pub})
	accountID, profileID, resourceID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	policy := protocol.Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: 1, ValidUntil: now.Add(time.Second).UnixMilli(), Permissions: []protocol.Permission{{
		AccountID: accountID, ProfileID: profileID, ResourceID: resourceID, SessionEpoch: 7, Actions: []string{"read", "media"},
	}}}
	applySnapshot(t, cache, private, scope, policy, now)
	lease, err := protocol.SignEnvelope(private, "authority-1", protocol.Claims{
		Version: 1, Kind: "lease", Issuer: scope.Issuer, Audience: "voice-node", Environment: scope.Environment,
		NodeID: scope.NodeID, SpaceID: scope.SpaceID, Generation: scope.Generation, Epoch: scope.Epoch,
		Revision: policy.Revision, IssuedAt: now.UnixMilli(), ExpiresAt: now.Add(600 * time.Millisecond).UnixMilli(), Hash: protocol.SnapshotDigest(policy),
	})
	require.NoError(t, err)
	require.NoError(t, cache.AcceptLease(lease, now))
	require.NoError(t, cache.Authorize(accountID, profileID, resourceID, 7, "media", now, 250*time.Millisecond))
	require.ErrorIs(t, cache.Authorize(accountID, profileID, resourceID, 8, "media", now, 0), ErrPermissionDenied)
	require.ErrorIs(t, cache.Authorize(accountID, profileID, resourceID, 7, "write", now, 0), ErrPermissionDenied)
	require.ErrorIs(t, cache.Authorize(accountID, profileID, resourceID, 7, "media", now.Add(400*time.Millisecond), 250*time.Millisecond), ErrAuthorityExpired)
	require.ErrorIs(t, cache.Authorize(accountID, profileID, resourceID, 7, "media", now, 251*time.Millisecond), ErrClockUncertain)
}

func TestExpiredLeaseCannotReviveAfterClockRollbackOrExactReplay(t *testing.T) {
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	scope := Scope{Issuer: "master", Environment: "sandbox", NodeID: uuid.NewString(), SpaceID: uuid.NewString(), Generation: 1, Epoch: 1}
	cache := New(scope, map[string]ed25519.PublicKey{"authority-1": pub})
	policy := snapshot(1, now.Add(time.Second), 1)
	policy.Permissions[0].Actions = []string{"media"}
	permission := policy.Permissions[0]
	applySnapshot(t, cache, private, scope, policy, now)
	lease, err := protocol.SignEnvelope(private, "authority-1", protocol.Claims{
		Version: 1, Kind: "lease", Issuer: scope.Issuer, Audience: "voice-node", Environment: scope.Environment,
		NodeID: scope.NodeID, SpaceID: scope.SpaceID, Generation: scope.Generation, Epoch: scope.Epoch,
		Revision: policy.Revision, IssuedAt: now.Add(-time.Second).UnixMilli(), ExpiresAt: now.Add(100 * time.Millisecond).UnixMilli(), Hash: protocol.SnapshotDigest(policy),
	})
	require.NoError(t, err)
	require.NoError(t, cache.AcceptLease(lease, now))
	authorize := func(at time.Time) error {
		return cache.Authorize(permission.AccountID, permission.ProfileID, permission.ResourceID, permission.SessionEpoch, "media", at, 0)
	}
	require.NoError(t, authorize(now))
	require.ErrorIs(t, authorize(now.Add(101*time.Millisecond)), ErrAuthorityExpired)
	rollback := now.Add(-500 * time.Millisecond)
	require.ErrorIs(t, authorize(rollback), ErrAuthorityExpired, "a wall-clock rollback cannot undo an observed expiry")
	require.ErrorIs(t, cache.AcceptLease(lease, rollback), ErrAuthorityExpired, "exact replay cannot renew the elapsed local deadline")
	require.ErrorIs(t, authorize(rollback), ErrAuthorityExpired)
}

func snapshot(revision int64, validUntil time.Time, count int) protocol.Snapshot {
	permissions := make([]protocol.Permission, count)
	for i := range permissions {
		permissions[i] = protocol.Permission{AccountID: uuid.NewString(), ProfileID: uuid.NewString(), ResourceID: uuid.NewString(), SessionEpoch: 1, Actions: []string{"read"}}
	}
	return protocol.Snapshot{Version: 1, Complete: true, PageCount: max(1, (count+protocol.SnapshotPagePermissionLimit-1)/protocol.SnapshotPagePermissionLimit), Revision: revision, ValidUntil: validUntil.UnixMilli(), Permissions: permissions}
}

func applySnapshot(t *testing.T, cache *Cache, private ed25519.PrivateKey, scope Scope, snapshot protocol.Snapshot, now time.Time) {
	t.Helper()
	require.NoError(t, cache.StageManifest(signedManifest(t, private, scope, snapshot, now), now))
	for _, page := range signedPages(t, private, scope, snapshot, now) {
		require.NoError(t, cache.StagePage(page, now))
	}
}

func signedManifest(t *testing.T, private ed25519.PrivateKey, scope Scope, snapshot protocol.Snapshot, now time.Time) protocol.Envelope {
	t.Helper()
	envelope, err := protocol.SignSnapshotManifest(private, "authority-1", scope, protocol.ManifestFor(snapshot), now)
	require.NoError(t, err)
	return envelope
}

func signedPages(t *testing.T, private ed25519.PrivateKey, scope Scope, snapshot protocol.Snapshot, now time.Time) []protocol.Envelope {
	t.Helper()
	pages, err := protocol.SignSnapshotPages(private, "authority-1", scope, snapshot, now)
	require.NoError(t, err)
	return pages
}
