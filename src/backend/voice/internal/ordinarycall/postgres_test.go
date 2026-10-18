package ordinarycall

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/voice/internal/gameprovision"
)

func TestPostgresConversionCapturesPendingNativeSourceWithoutRedisAndReplaysImmutableBinding(t *testing.T) {
	pool := ordinaryPostgres(t)
	ctx := context.Background()
	journal := NewPostgresStore(pool, storageTestEncoder)
	source, owner, op := pendingGroup(t)
	require.NoError(t, journal.Create(ctx, source))
	target := identity()
	// Construct with the real initiator instead of rewriting a captured member.
	targetRoom, err := NewPendingRoom(CreateIntent{OperationID: uuid.New(), RoomID: uuid.New(), ChatID: uuid.New(), Group: true,
		Initiator: Identity{AccountID: target.AccountID, ProfileID: uuid.New()}, MediaKind: 1}, testNow)
	require.NoError(t, err)
	require.NoError(t, journal.Create(ctx, targetRoom))
	store := gameprovision.NewPostgresStore(pool)
	store.EnableOrdinaryConversions()
	req := &callsv1.FenceSdkConversionRequest{OperationId: uuid.NewString(), BindingId: uuid.NewString(),
		SourceAccountId: owner.AccountID.String(), SourceActorId: uuid.NewString(), SourceProfileId: owner.ProfileID.String(),
		TargetAccountId: target.AccountID.String(), TargetProfileId: target.ProfileID.String(), FrozenAuthorityEpoch: 1, FreezeReceiptId: uuid.NewString()}
	reservation, err := store.ReserveSdkConversionFence(ctx, req, false, "", testNow)
	require.NoError(t, err)
	require.NotNil(t, reservation.NativeSource)
	require.True(t, reservation.Response.TargetSessionConflict, "another profile of the target account is busy before Redis exists")
	require.Equal(t, source.Intent.RoomID.String(), reservation.SourceRoom)
	require.Equal(t, source.Members[owner.ProfileID.String()].Generation, reservation.NativeSource.Generation)
	_, err = journal.Mutate(ctx, source.Intent.RoomID, func(r *Room) error {
		if err := r.RecordEnsureResult(op, true, testNow); err != nil {
			return err
		}
		return r.CommitAdmission(op, testNow)
	})
	require.ErrorIs(t, err, ErrConflict, "conversion won the shared barrier before finalization")
	_, err = journal.Mutate(ctx, source.Intent.RoomID, func(r *Room) error { return r.AbortAdmission(op, testNow) })
	require.NoError(t, err)
	replay, err := store.ReserveSdkConversionFence(ctx, req, false, "different-hint", testNow)
	require.NoError(t, err)
	require.Equal(t, reservation.NativeSource, replay.NativeSource)
	require.Equal(t, reservation.SourceRoom, replay.SourceRoom)
	_, err = pool.Exec(ctx, `UPDATE voice_sdk_conversion_fence_receipts SET ordinary_source_snapshot=NULL WHERE operation_id=$1`, req.OperationId)
	require.NoError(t, err)
	_, err = store.ReserveSdkConversionFence(ctx, req, false, "", testNow)
	require.ErrorIs(t, err, gameprovision.ErrConflict, "old pending receipt cannot infer an empty native source")
	mismatch := conversionRequest(Identity{AccountID: uuid.New(), ProfileID: owner.ProfileID}, target)
	_, err = store.ReserveSdkConversionFence(ctx, mismatch, false, "", testNow)
	require.ErrorIs(t, err, gameprovision.ErrConflict, "missing Redis cannot conceal a mismatched trusted account/profile")
}

func TestPostgresCommittedConversionSourceCleansExactJournalAndConfirmsProjection(t *testing.T) {
	pool := ordinaryPostgres(t)
	ctx := context.Background()
	journal := NewPostgresStore(pool, storageTestEncoder)
	c := &Coordinator{Journal: journal, Media: &mediaFake{}, Projection: &projectionFake{}, Now: func() time.Time { return testNow }}
	pending, owner, _ := pendingGroup(t)
	r, err := c.Start(ctx, pending.Intent, nil)
	require.NoError(t, err)
	target := identity()
	req := conversionRequest(owner, target)
	conversion := gameprovision.NewPostgresStore(pool)
	conversion.EnableOrdinaryConversions()
	reserved, err := conversion.ReserveSdkConversionFence(ctx, req, false, "", testNow)
	require.NoError(t, err)
	b := reserved.NativeSource
	require.NotNil(t, b)
	require.NoError(t, c.QuiesceConversionSource(ctx, MediaBinding{RoomID: b.RoomID, RoomOperationID: b.RoomOperationID, AccountID: b.AccountID,
		ProfileID: b.ProfileID, Generation: b.Generation, FenceOperationID: b.FenceOperationID, MediaIdentity: b.MediaIdentity}))
	final, err := journal.Load(ctx, r.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StageClosed, final.Stage)
	var fences int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_account_voice_fences WHERE account_id=$1`, owner.AccountID).Scan(&fences))
	require.Zero(t, fences)
	var projected uint64
	require.NoError(t, pool.QueryRow(ctx, `SELECT projection_version FROM voice_ordinary_rooms WHERE room_id=$1`, r.Intent.RoomID).Scan(&projected))
	require.Equal(t, final.Version, projected)
	completed, err := conversion.CompleteSdkConversionFence(ctx, uuid.MustParse(req.OperationId), time.Now().UTC())
	require.NoError(t, err)
	require.NotNil(t, completed.ObservedEjectionAt)
}

func conversionRequest(source, target Identity) *callsv1.FenceSdkConversionRequest {
	return &callsv1.FenceSdkConversionRequest{OperationId: uuid.NewString(), BindingId: uuid.NewString(), SourceActorId: uuid.NewString(),
		SourceAccountId: source.AccountID.String(), SourceProfileId: source.ProfileID.String(), TargetAccountId: target.AccountID.String(),
		TargetProfileId: target.ProfileID.String(), FrozenAuthorityEpoch: 1, FreezeReceiptId: uuid.NewString()}
}

func TestPostgresReversedConversionIdentitiesDoNotDeadlock(t *testing.T) {
	pool := ordinaryPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a, b := identity(), identity()
	first, second := gameprovision.NewPostgresStore(pool), gameprovision.NewPostgresStore(pool)
	first.EnableOrdinaryConversions()
	second.EnableOrdinaryConversions()
	start := make(chan struct{})
	result := make(chan error, 2)
	go func() {
		<-start
		_, err := first.ReserveSdkConversionFence(ctx, conversionRequest(a, b), false, "", testNow)
		result <- err
	}()
	go func() {
		<-start
		_, err := second.ReserveSdkConversionFence(ctx, conversionRequest(b, a), false, "", testNow)
		result <- err
	}()
	close(start)
	for range 2 {
		select {
		case err := <-result:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal("reversed identity barriers did not finish", ctx.Err())
		}
	}
}

// This encoder isolates database state/lease tests. It does not establish the
// real protobuf envelope contract, which has separate wire round-trip tests.
func storageTestEncoder(_ *Room, event Event) (EncodedEvent, error) {
	name := string(event.Kind)
	switch event.Kind {
	case EventIncoming, EventAccepted, EventDeclined, EventMissed, EventEnded:
		name = "call_" + name
	}
	return EncodedEvent{ID: event.ID, Subject: "voice." + name, Payload: []byte(event.ID.String())}, nil
}

func ordinaryPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	dir := filepath.Join(root, "src", "backend", "migrations", "voice_db")
	pool := integrationtest.StartPostgres(t, context.Background(), "voice_ordinary", filepath.Join(dir, "000001_room_lifecycle.up.sql"))
	for _, migration := range []string{
		"000002_redis_divergence", "000003_matchmaking_membership", "000004_game_session_rooms",
		"000005_game_session_close", "000006_game_session_roster_lease", "000007_t17_sdk_conversion_fence",
		"000008_account_voice_fence", "000009_space_lifecycle", "000010_space_media_admission",
		"000011_match_squad_operations", "000012_ordinary_calls",
	} {
		body, err := os.ReadFile(filepath.Join(dir, migration+".up.sql"))
		require.NoError(t, err)
		_, err = pool.Exec(context.Background(), string(body))
		require.NoError(t, err)
	}
	return pool
}

func TestPostgresCreateJournalsExactNonexpiringFencesBeforeSDK(t *testing.T) {
	pool := ordinaryPostgres(t)
	store := NewPostgresStore(pool, storageTestEncoder)
	ctx := context.Background()
	require.NoError(t, store.CheckSchema(ctx))
	room, owner, op := pendingGroup(t)
	require.NoError(t, store.Create(ctx, room))

	var operation uuid.UUID
	var generation string
	var expiry *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT admission_operation_id,admission_generation,reservation_expires_at
FROM voice_account_voice_fences WHERE account_id=$1`, owner.AccountID).Scan(&operation, &generation, &expiry))
	require.Equal(t, op, operation)
	require.Equal(t, room.Members[owner.ProfileID.String()].Generation.String(), generation)
	require.Nil(t, expiry)
	require.NoError(t, gameprovision.NewPostgresAccountVoiceFenceStore(pool).Release(ctx, owner.AccountID, owner.ProfileID, room.Intent.RoomID.String()))
	var remains bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_account_voice_fences WHERE account_id=$1)`, owner.AccountID).Scan(&remains))
	require.True(t, remains, "tuple-only legacy cleanup cannot erase journal ownership")
	loaded, err := store.Load(ctx, room.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StagePreparing, loaded.Stage)
	require.False(t, loaded.CanIssueToken(owner.ProfileID))
}

func pendingConversion(t *testing.T, pool *pgxpool.Pool, source, target Identity) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `INSERT INTO voice_sdk_conversion_fence_receipts(
operation_id,binding_id,source_account_id,source_actor_id,source_profile_id,target_account_id,target_profile_id,
frozen_authority_epoch,freeze_receipt_id,request_hash,receipt_id,state)
VALUES($1,$2,$3,$4,$5,$6,$7,1,$8,$9,$10,'pending')`, uuid.New(), uuid.New(), source.AccountID,
		uuid.New(), source.ProfileID, target.AccountID, target.ProfileID, uuid.New(), make([]byte, 32), uuid.New())
	require.NoError(t, err)
}

func TestPostgresConversionDeniesOtherProfileOfTargetAccountBeforeSDK(t *testing.T) {
	pool := ordinaryPostgres(t)
	store := NewPostgresStore(pool, storageTestEncoder)
	room, owner, _ := pendingGroup(t)
	target := Identity{AccountID: owner.AccountID, ProfileID: uuid.New()}
	pendingConversion(t, pool, identity(), target)
	require.ErrorIs(t, store.Create(context.Background(), room), ErrConflict)
	_, err := store.Load(context.Background(), room.Intent.RoomID)
	require.ErrorIs(t, err, ErrNotFound, "denied transaction must not leave a preparing journal")
	var count int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM voice_account_voice_fences`).Scan(&count))
	require.Zero(t, count)
}

func TestPostgresConversionBetweenPrepareAndFinalizeDeniesCommitButAllowsAbort(t *testing.T) {
	pool := ordinaryPostgres(t)
	store := NewPostgresStore(pool, storageTestEncoder)
	ctx := context.Background()
	room, owner, op := pendingGroup(t)
	require.NoError(t, store.Create(ctx, room))
	pendingConversion(t, pool, owner, identity())
	_, err := store.Mutate(ctx, room.Intent.RoomID, func(r *Room) error {
		if err := r.RecordEnsureResult(op, true, testNow); err != nil {
			return err
		}
		return r.CommitAdmission(op, testNow)
	})
	require.ErrorIs(t, err, ErrConflict)
	loaded, err := store.Load(ctx, room.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StagePreparing, loaded.Stage)
	require.Empty(t, loaded.Events)
	_, err = store.Mutate(ctx, room.Intent.RoomID, func(r *Room) error { return r.AbortAdmission(op, testNow) })
	require.NoError(t, err, "conversion fence must never prevent native cleanup")
	loaded, err = store.Load(ctx, room.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StageAborting, loaded.Stage)
}

func TestPostgresMediaAuthorizationBindsGenerationAndCurrentConversionBarrier(t *testing.T) {
	pool := ordinaryPostgres(t)
	store := NewPostgresStore(pool, storageTestEncoder)
	ctx := context.Background()
	r, owner, op := pendingGroup(t)
	require.NoError(t, store.Create(ctx, r))
	r, err := store.Mutate(ctx, r.Intent.RoomID, func(r *Room) error {
		if err := r.RecordEnsureResult(op, true, testNow); err != nil {
			return err
		}
		return r.CommitAdmission(op, testNow)
	})
	require.NoError(t, err)
	binding, err := r.MediaBinding(owner.ProfileID)
	require.NoError(t, err)
	name, err := store.AuthorizeMedia(ctx, binding)
	require.NoError(t, err)
	require.Equal(t, r.LiveKitRoomName, name)
	stale := binding
	stale.Generation = uuid.New()
	_, err = store.AuthorizeMedia(ctx, stale)
	require.ErrorIs(t, err, ErrStaleGeneration)
	pendingConversion(t, pool, owner, identity())
	_, err = store.AuthorizeMedia(ctx, binding)
	require.ErrorIs(t, err, ErrConflict)
	_, err = store.Mutate(ctx, r.Intent.RoomID, func(r *Room) error { return r.DecideEnd(owner.ProfileID, "hangup", testNow) })
	require.NoError(t, err, "the conversion barrier must allow native cleanup")
	_, err = store.AuthorizeMedia(ctx, binding)
	require.ErrorIs(t, err, ErrNotMember)
}

func TestPostgresTwoInstancesSerializeJoinAgainstEnd(t *testing.T) {
	pool := ordinaryPostgres(t)
	first, second := NewPostgresStore(pool, storageTestEncoder), NewPostgresStore(pool, storageTestEncoder)
	ctx := context.Background()
	room, owner, startOperation := pendingGroup(t)
	require.NoError(t, first.Create(ctx, room))
	room, err := first.Mutate(ctx, room.Intent.RoomID, func(r *Room) error {
		if err := r.RecordEnsureResult(startOperation, true, testNow); err != nil {
			return err
		}
		return r.CommitAdmission(startOperation, testNow)
	})
	require.NoError(t, err)
	joiner, op := identity(), uuid.New()
	start := make(chan struct{})
	errors := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		_, err := first.Mutate(ctx, room.Intent.RoomID, func(r *Room) error {
			if err := r.BeginJoin(op, joiner, testNow); err != nil {
				return err
			}
			return r.CommitAdmission(op, testNow)
		})
		errors <- err
	}()
	go func() {
		defer workers.Done()
		<-start
		_, err := second.Mutate(ctx, room.Intent.RoomID, func(r *Room) error {
			return r.DecideEnd(owner.ProfileID, "hangup", testNow)
		})
		errors <- err
	}()
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			require.ErrorIs(t, err, ErrAdmissionAborted)
		}
	}
	loaded, err := first.Load(ctx, room.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StageClosing, loaded.Stage)
	require.False(t, loaded.CanIssueToken(joiner.ProfileID))
	joined, left := 0, 0
	for _, event := range loaded.Events {
		if event.ProfileID == joiner.ProfileID {
			if event.Kind == EventParticipantJoined {
				joined++
			}
			if event.Kind == EventParticipantLeft {
				left++
			}
		}
	}
	require.Equal(t, joined, left, "no invented left for an admission that lost")
	require.LessOrEqual(t, joined, 1)
}

func TestPostgresOutboxWaitsForProjectionAndKeepsIdentityOnLeaseReplay(t *testing.T) {
	pool := ordinaryPostgres(t)
	store := NewPostgresStore(pool, storageTestEncoder)
	ctx := context.Background()
	room, _, operation := pendingGroup(t)
	require.NoError(t, store.Create(ctx, room))
	room, err := store.Mutate(ctx, room.Intent.RoomID, func(r *Room) error {
		if err := r.RecordEnsureResult(operation, true, testNow); err != nil {
			return err
		}
		return r.CommitAdmission(operation, testNow)
	})
	require.NoError(t, err)
	item, err := store.ClaimOutbox(ctx, 30*time.Second)
	require.NoError(t, err)
	require.Nil(t, item)
	require.NoError(t, store.MarkProjected(ctx, room.Intent.RoomID, room.Version))
	first, err := store.ClaimOutbox(ctx, 30*time.Second)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.NoError(t, store.ReleaseOutbox(ctx, *first, time.Time{}))
	second, err := store.ClaimOutbox(ctx, 30*time.Second)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, first.Payload, second.Payload)
	require.NotEqual(t, first.LeaseToken, second.LeaseToken)
	require.ErrorIs(t, store.MarkDelivered(ctx, *first), ErrStaleLease)
	require.NoError(t, store.MarkDelivered(ctx, *second))
}
