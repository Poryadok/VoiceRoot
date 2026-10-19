package ordinarycall

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/retirementproof"
)

type cutoverAuthorityFixture struct{}

func (cutoverAuthorityFixture) CheckMaintenance(context.Context, retirementproof.CutBinding) error {
	return nil
}

func ordinaryCutFixture() retirementproof.CutBinding {
	d := retirementproof.Hash("unit-fixture", []byte("not an installed resource authority"))
	return retirementproof.CutBinding{EnvironmentID: uuid.New(), AuthorityIncarnation: uuid.New(), CutoverOperationID: uuid.New(), CoverageEpoch: 1,
		PlanHash: d, DatabaseIdentity: d, SourceCatalogHash: d, SchemaContractHash: d, WriterManifestHash: d,
		ProviderManifestHash: d, CredentialManifestHash: d, ClosedAdmissionBarrierID: uuid.New(), ClosedAdmissionBarrierHash: d, IssuerImplementationManifestHash: d}
}

func TestOrdinaryCutoverPrepareSealsHistoryAndClosesAtomically(t *testing.T) {
	pool := ordinaryPostgres(t)
	ctx := context.Background()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "voice_db", "000025_ordinary_cutover_drain.up.sql")
	ddl, err := os.ReadFile(path)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(ddl))
	require.NoError(t, err)
	down, err := os.ReadFile(filepath.Join(filepath.Dir(path), "000025_ordinary_cutover_drain.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.NoError(t, err, "empty maintenance journal supports a real DOWN/UP roundtrip")
	_, err = pool.Exec(ctx, string(ddl))
	require.NoError(t, err)
	store := NewPostgresStore(pool, storageTestEncoder)
	c := &Coordinator{Journal: store, Media: &mediaFake{}, Projection: &projectionFake{}, Now: func() time.Time { return testNow }}
	pending, first, _ := pendingGroup(t)
	room, err := c.Start(ctx, pending.Intent, nil)
	require.NoError(t, err)
	survivor := identity()
	join := uuid.New()
	room, err = c.Admit(ctx, room.Intent.RoomID, join, func(r *Room) error { return r.BeginJoin(join, survivor, r.TransactionTime) }, nil)
	require.NoError(t, err)
	oldGeneration := room.Members[first.ProfileID.String()].Generation
	room, err = c.Change(ctx, room.Intent.RoomID, func(r *Room) error { return r.DecideLeave(first.ProfileID, r.TransactionTime) })
	require.NoError(t, err)
	rejoin := uuid.New()
	room, err = c.Admit(ctx, room.Intent.RoomID, rejoin, func(r *Room) error { return r.BeginJoin(rejoin, first, r.TransactionTime) }, nil)
	require.NoError(t, err)
	require.NotEmpty(t, room.RetiredMembers)
	newGeneration := room.Members[first.ProfileID.String()].Generation
	require.NotEqual(t, oldGeneration, newGeneration)
	expected := CutoverExpectedRoom{WorkID: uuid.New(), RoomID: room.Intent.RoomID, Incarnation: room.Intent.OperationID, Version: room.Version}
	cut := ordinaryCutFixture()
	owner := &CutoverDrainOwner{Store: store, Authority: cutoverAuthorityFixture{}}
	ref, err := owner.PrepareCutoverDrain(ctx, cut, expected)
	require.NoError(t, err)
	work, err := owner.ReadOwnerWork(ctx, ref)
	require.NoError(t, err)
	require.Equal(t, "NATIVE_ORDINARY", work.SourceKind)
	require.Equal(t, uuid.Nil, work.SpaceID, "ordinary work must not fabricate a Space")
	require.NotEmpty(t, work.UnknownIssuanceMarkers, "historical tokens need protected source confinement")
	require.True(t, bytes.Contains(work.SealedTargetManifestBytes, []byte(oldGeneration.String())))
	require.True(t, bytes.Contains(work.SealedTargetManifestBytes, []byte(newGeneration.String())))
	require.True(t, bytes.Contains(work.SealedTargetManifestBytes, []byte(rejoin.String())))
	missingMarker := work
	missingMarker.UnknownIssuanceMarkers = missingMarker.UnknownIssuanceMarkers[:len(missingMarker.UnknownIssuanceMarkers)-1]
	_, err = validateCutoverManifest(missingMarker)
	require.ErrorIs(t, err, ErrConflict, "a complete target manifest cannot authorize an omitted historical marker")
	// Self-consistent hashes do not make an unrecognized native schema safe.
	// A reader which silently round-trips a smaller Go Room drops this history.
	var changedTargets []cutoverTarget
	require.NoError(t, json.Unmarshal(work.SealedTargetManifestBytes, &changedTargets))
	for i := range changedTargets {
		if changedTargets[i].Kind != "room" {
			continue
		}
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(changedTargets[i].Bytes, &fields))
		fields["unrecognized_issuance_history"] = json.RawMessage(`{"old_provider":"unmapped"}`)
		changedTargets[i].Bytes, err = json.Marshal(fields)
		require.NoError(t, err)
	}
	unknownSchema := work
	unknownSchema.SealedTargetManifestBytes, err = json.Marshal(changedTargets)
	require.NoError(t, err)
	unknownSchema.SealedTargetManifestHash = retirementproof.Hash(retirementproof.TargetDomain, unknownSchema.SealedTargetManifestBytes)
	unknownSchema.UnknownIssuanceMarkers = cutoverUnknownMarkers(changedTargets)
	_, err = validateCutoverManifest(unknownSchema)
	require.ErrorIs(t, err, ErrConflict, "self-consistent unsupported historical fields must remain unavailable")
	closed, err := store.Load(ctx, room.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StageClosing, closed.Stage)
	require.Equal(t, room.RosterVersion+2, closed.RosterVersion)
	replay, err := owner.PrepareCutoverDrain(ctx, cut, expected)
	require.NoError(t, err)
	require.Equal(t, ref, replay)
	changed := expected
	changed.Incarnation = uuid.New()
	_, err = owner.PrepareCutoverDrain(ctx, cut, changed)
	require.ErrorIs(t, err, ErrConflict)
	_, err = pool.Exec(ctx, `UPDATE voice_ordinary_cutover_works SET work_bytes='changed' WHERE work_id=$1`, ref.WorkID)
	require.Error(t, err, "retained work is immutable even to the test's database owner")
	connection, err := pool.Acquire(ctx)
	require.NoError(t, err)
	_, downErr := connection.Exec(ctx, string(down))
	_, rollbackErr := connection.Exec(ctx, "ROLLBACK")
	connection.Release()
	require.ErrorContains(t, downErr, "requires reconciliation before rollback")
	require.NoError(t, rollbackErr)
	retained, err := owner.ReadOwnerWork(ctx, ref)
	require.NoError(t, err)
	require.Equal(t, work, retained, "refused DOWN preserves the entire work and exact target rows")

	// A native outbox failure must roll back BOTH the seal and close decision.
	second, _, _ := pendingGroup(t)
	active, err := c.Start(ctx, second.Intent, nil)
	require.NoError(t, err)
	failing := NewPostgresStore(pool, func(r *Room, event Event) (EncodedEvent, error) {
		if event.Kind == EventParticipantLeft {
			return EncodedEvent{}, errors.New("fixture outbox failure")
		}
		return storageTestEncoder(r, event)
	})
	failedWork := CutoverExpectedRoom{WorkID: uuid.New(), RoomID: active.Intent.RoomID, Incarnation: active.Intent.OperationID, Version: active.Version}
	_, err = (&CutoverDrainOwner{Store: failing, Authority: cutoverAuthorityFixture{}}).PrepareCutoverDrain(ctx, cut, failedWork)
	require.ErrorContains(t, err, "fixture outbox failure")
	var exists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_ordinary_cutover_works WHERE work_id=$1)`, failedWork.WorkID).Scan(&exists))
	require.False(t, exists)
	stillActive, err := store.Load(ctx, active.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StageActive, stillActive.Stage)
	_, err = (&CutoverDrainOwner{Store: store}).PrepareCutoverDrain(ctx, cut, failedWork)
	require.ErrorIs(t, err, ErrUnavailable, "serving credentials have no permissive authority fallback")
	// The membership table is a lookup index, never permission to synthesize a
	// native member absent from the authoritative aggregate. Preserve a conflicting
	// row and the ACTIVE room rather than closing and blessing an unknown tuple.
	orphanProfile, orphanAccount, orphanGeneration, orphanOperation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO voice_ordinary_memberships(room_id,profile_id,account_id,membership_generation,fence_operation_id,media_identity,state,session_epoch)
VALUES($1,$2,$3,$4,$5,$6,'JOINED',1)`, active.Intent.RoomID, orphanProfile, orphanAccount, orphanGeneration, orphanOperation, "ov:"+orphanProfile.String()+":"+orphanGeneration.String())
	require.NoError(t, err)
	_, err = owner.PrepareCutoverDrain(ctx, cut, failedWork)
	require.ErrorIs(t, err, ErrConflict, "unowned lookup rows need authoritative reconciliation, not a drain receipt")
	stillActive, err = store.Load(ctx, active.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StageActive, stillActive.Stage)
	require.Equal(t, active.Version, stillActive.Version)
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_ordinary_memberships WHERE room_id=$1 AND profile_id=$2 AND membership_generation=$3)`, active.Intent.RoomID, orphanProfile, orphanGeneration).Scan(&exists))
	require.True(t, exists)
	// A stale row for an existing profile is even easier to hide: ordinary
	// syncMemberships would overwrite its old tuple. Refuse BEFORE that repair.
	_, err = pool.Exec(ctx, `DELETE FROM voice_ordinary_memberships WHERE room_id=$1 AND profile_id=$2`, active.Intent.RoomID, orphanProfile)
	require.NoError(t, err)
	staleGeneration := uuid.New()
	profile := active.Intent.Initiator.ProfileID
	_, err = pool.Exec(ctx, `UPDATE voice_ordinary_memberships SET membership_generation=$3,media_identity=$4 WHERE room_id=$1 AND profile_id=$2`, active.Intent.RoomID, profile, staleGeneration, "ov:"+profile.String()+":"+staleGeneration.String())
	require.NoError(t, err)
	_, err = owner.PrepareCutoverDrain(ctx, cut, failedWork)
	require.ErrorIs(t, err, ErrConflict, "native index repair must not erase unresolved predecessor evidence")
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_ordinary_memberships WHERE room_id=$1 AND profile_id=$2 AND membership_generation=$3)`, active.Intent.RoomID, profile, staleGeneration).Scan(&exists))
	require.True(t, exists)
	stillActive, err = store.Load(ctx, active.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, active.Version, stillActive.Version)
	require.Equal(t, active.RosterVersion, stillActive.RosterVersion)
}
