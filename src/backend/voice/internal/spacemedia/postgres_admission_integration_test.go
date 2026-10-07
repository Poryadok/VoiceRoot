package spacemedia

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
	"voice/backend/pkg/integrationtest"
	"voice/backend/voice/internal/gameprovision"
	"voice/backend/voice/internal/store"
)

func TestPostgresAdmissionOutboxWaitsForProjectionAndReplaysTheSameEvent(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	store := NewPostgresAdmissionStore(pool)
	require.NoError(t, store.CheckSchema(ctx))
	a := admissionFixture()
	require.NoError(t, prepareAdmission(ctx, store, &a))

	item, err := store.ClaimOutbox(ctx, time.Second)
	require.NoError(t, err)
	require.Nil(t, item, "PREPARED event intent is not publishable")
	require.NoError(t, store.Fence(ctx, a))
	item, err = store.ClaimOutbox(ctx, time.Second)
	require.NoError(t, err)
	require.Nil(t, item, "FENCED event intent is not publishable")

	// Tuple-only legacy cleanup cannot release an operation-owned fence.
	require.NoError(t, gameprovision.NewPostgresAccountVoiceFenceStore(pool).Release(ctx, a.AccountID, a.ProfileID, a.RoomID))
	var fenceOperation uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT admission_operation_id FROM voice_account_voice_fences WHERE account_id=$1`, a.AccountID).Scan(&fenceOperation))
	require.Equal(t, a.OperationID, fenceOperation)

	require.NoError(t, store.MarkCommitted(ctx, a.OperationID, a.Generation))
	item, err = store.ClaimOutbox(ctx, time.Second)
	require.NoError(t, err)
	require.Nil(t, item, "COMMITTED event intent is withheld until Redis projection is confirmed")
	require.NoError(t, store.MarkProjectionApplied(ctx, a.OperationID, a.Generation))

	first, err := store.ClaimOutbox(ctx, time.Second)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, a.Events[0].ID, first.ID)
	require.Equal(t, a.Events[0].Payload, first.Payload)
	require.NoError(t, store.ReleaseOutboxLease(ctx, *first))
	second, err := store.ClaimOutbox(ctx, time.Second)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, first.Payload, second.Payload)
	require.NotEqual(t, first.LeaseToken, second.LeaseToken)
	require.NoError(t, store.MarkOutboxDelivered(ctx, *second))
	second, err = store.ClaimOutbox(ctx, time.Second)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, a.Events[1].ID, second.ID)
}

func TestPostgresAdmissionRejectsChangedRetryAndPersistsRecoveryState(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	store := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	require.NoError(t, prepareAdmission(ctx, store, &a))
	changed := a
	changed.Events = append([]AdmissionEvent(nil), a.Events...)
	changed.Events[0].Payload = []byte("different fixed payload")
	require.ErrorIs(t, store.Prepare(ctx, changed), ErrAdmissionConflict)
	require.NoError(t, store.Fence(ctx, a))
	require.NoError(t, store.MarkCommitted(ctx, a.OperationID, a.Generation))
	rows, err := store.RecoveryRows(ctx, 20)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, AdmissionCommitted, rows[0].State)
	require.False(t, rows[0].ProjectionApplied)
	require.True(t, rows[0].CanJoin)
	require.True(t, rows[0].CanSubscribe)
	require.NoError(t, store.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
	require.NoError(t, store.ConfirmProjection(ctx, a.OperationID, a.Generation))
	activeRows, err := store.ProjectionRows(ctx, uuid.Nil, 20, true)
	require.NoError(t, err)
	require.Len(t, activeRows, 1, "durable active creator rows remain enumerable even if Redis loses its call index")
	require.Equal(t, a.OperationID, activeRows[0].OperationID)
	rows, err = store.RecoveryRows(ctx, 20)
	require.NoError(t, err)
	require.Empty(t, rows, "confirmed active projections are not pending recovery rows")
	require.NoError(t, store.BeginRoomLeave(ctx, a.OperationID, a.Generation))
	require.ErrorIs(t, store.ConfirmProjection(ctx, a.OperationID, a.Generation), ErrAdmissionConflict,
		"a stale Redis participant cannot retain token/roster authority after PostgreSQL starts revocation")
	rows, err = store.RecoveryRows(ctx, 20)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "REVOKING", rows[0].ParticipantState)
	require.True(t, rows[0].ProjectionApplied)
}

func TestPostgresAdmissionFenceRequiresExactOperationRelease(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	store := NewPostgresAdmissionStore(pool)
	first := admissionFixture()
	require.NoError(t, prepareAdmission(ctx, store, &first))
	require.NoError(t, store.Fence(ctx, first))
	second := admissionFixture()
	second.AccountID = first.AccountID
	require.NoError(t, prepareAdmission(ctx, store, &second))
	require.ErrorIs(t, store.Fence(ctx, second), ErrAdmissionConflict)

	require.NoError(t, store.MarkAborting(ctx, first.OperationID, first.Generation))
	require.NoError(t, store.ReleaseFence(ctx, first))
	require.NoError(t, store.MarkCleanupCompleted(ctx, first.OperationID, first.Generation))
	require.NoError(t, store.Fence(ctx, second))
	var got uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT admission_operation_id FROM voice_account_voice_fences WHERE account_id=$1`, first.AccountID).Scan(&got))
	require.Equal(t, second.OperationID, got)
}

func TestPostgresRoomHeadSerializesJoinersAndReopensOnlyAfterFinalLeave(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	admissions := NewPostgresAdmissionStore(pool)
	creator := admissionFixture()
	require.NoError(t, prepareAdmission(ctx, admissions, &creator))

	joiner := admissionFixture()
	joiner.VoiceRoomID = creator.VoiceRoomID
	joiner.SpaceID = creator.SpaceID
	joiner.RoomID = creator.RoomID
	joiner.RoomGeneration = creator.RoomGeneration
	joiner.CreatedRoom = false
	joiner.Events = []AdmissionEvent{{ID: uuid.New(), Subject: "voice.member_joined", Payload: []byte("joiner")}}
	_, err := admissions.ClaimRoomHead(ctx, joiner.VoiceRoomID, joiner.SpaceID.String(), joiner.RoomID, joiner.OperationID, false)
	require.ErrorIs(t, err, ErrRoomHeadPending, "a second caller cannot create a competing incarnation while the creator is pending")

	finalizeAdmission := func(a Admission) {
		t.Helper()
		require.NoError(t, admissions.Fence(ctx, a))
		require.NoError(t, admissions.MarkCommitted(ctx, a.OperationID, a.Generation))
		require.NoError(t, admissions.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
	}
	finalizeAdmission(creator)
	require.NoError(t, prepareAdmission(ctx, admissions, &joiner))
	finalizeAdmission(joiner)

	var started int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_media_admission_outbox WHERE voice_room_id=$1 AND room_generation=$2 AND subject='voice.call_started'`, creator.VoiceRoomID, creator.RoomGeneration).Scan(&started))
	require.Equal(t, 1, started, "only the creator owns call_started for an incarnation")

	require.NoError(t, admissions.BeginRoomLeave(ctx, creator.OperationID, creator.Generation))
	require.NoError(t, admissions.CompleteRoomLeave(ctx, creator))
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2`, creator.VoiceRoomID, creator.RoomGeneration).Scan(&state))
	require.Equal(t, "OPEN", state, "one participant leaving cannot release the room claim while another remains")

	require.NoError(t, admissions.BeginRoomLeave(ctx, joiner.OperationID, joiner.Generation))
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2`, creator.VoiceRoomID, creator.RoomGeneration).Scan(&state))
	require.Equal(t, "CLOSING", state)
	next := admissionFixture()
	next.VoiceRoomID = creator.VoiceRoomID
	next.SpaceID = creator.SpaceID
	_, err = admissions.ClaimRoomHead(ctx, next.VoiceRoomID, next.SpaceID.String(), next.RoomID, next.OperationID, true)
	require.ErrorIs(t, err, ErrRoomHeadPending, "a new incarnation waits for exact old-generation cleanup")
	require.NoError(t, admissions.CompleteRoomLeave(ctx, joiner))
	newHead, err := admissions.ClaimRoomHead(ctx, next.VoiceRoomID, next.SpaceID.String(), next.RoomID, next.OperationID, true)
	require.NoError(t, err)
	require.Equal(t, creator.RoomGeneration+1, newHead.RoomGeneration)
	require.NotEqual(t, creator.RoomID, newHead.RoomID)
}

func TestPostgresRoomHeadConcurrentCreatorsAndStaleLeaveCannotLeak(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	admissions := NewPostgresAdmissionStore(pool)
	first, second := admissionFixture(), admissionFixture()
	second.VoiceRoomID, second.SpaceID = first.VoiceRoomID, first.SpaceID
	candidates := []Admission{first, second}
	type claimResult struct {
		candidate Admission
		head      store.SpaceMediaRoomHead
		err       error
	}
	results := make(chan claimResult, len(candidates))
	var wg sync.WaitGroup
	for _, candidate := range candidates {
		candidate := candidate
		wg.Add(1)
		go func() {
			defer wg.Done()
			head, err := admissions.ClaimRoomHead(ctx, candidate.VoiceRoomID, candidate.SpaceID.String(), candidate.RoomID, candidate.OperationID, true)
			results <- claimResult{candidate: candidate, head: head, err: err}
		}()
	}
	wg.Wait()
	close(results)
	var creator Admission
	var creatorHead store.SpaceMediaRoomHead
	claims, pending := 0, 0
	for result := range results {
		if result.err == nil {
			claims++
			creator, creatorHead = result.candidate, result.head
		} else {
			require.ErrorIs(t, result.err, ErrRoomHeadPending)
			pending++
		}
	}
	require.Equal(t, 1, claims)
	require.Equal(t, 1, pending)
	creator.RoomGeneration = creatorHead.RoomGeneration
	creator.CreatedRoom = true
	require.NoError(t, admissions.Prepare(ctx, creator))
	require.NoError(t, admissions.Fence(ctx, creator))
	require.NoError(t, admissions.MarkCommitted(ctx, creator.OperationID, creator.Generation))
	require.NoError(t, admissions.MarkProjectionApplied(ctx, creator.OperationID, creator.Generation))
	joiner := admissionFixture()
	joiner.VoiceRoomID, joiner.SpaceID, joiner.RoomID = creator.VoiceRoomID, creator.SpaceID, creator.RoomID
	joiner.RoomGeneration, joiner.CreatedRoom = creator.RoomGeneration, false
	joiner.Events = []AdmissionEvent{{ID: uuid.New(), Subject: "voice.member_joined", Payload: []byte("joiner")}}
	ready, err := admissions.ClaimRoomHead(ctx, joiner.VoiceRoomID, joiner.SpaceID.String(), joiner.RoomID, joiner.OperationID, false)
	require.NoError(t, err)
	require.True(t, ready.Ready)
	require.Equal(t, creator.RoomID, ready.RoomID)
	require.Equal(t, creator.RoomGeneration, ready.RoomGeneration)

	// Closing the final participant ends only its exact generation; delayed
	// creator cleanup cannot affect a later incarnation.
	require.NoError(t, admissions.BeginRoomLeave(ctx, creator.OperationID, creator.Generation))
	require.NoError(t, admissions.CompleteRoomLeave(ctx, creator))
	newCandidate := admissionFixture()
	newCandidate.VoiceRoomID, newCandidate.SpaceID = creator.VoiceRoomID, creator.SpaceID
	newHead, err := admissions.ClaimRoomHead(ctx, newCandidate.VoiceRoomID, newCandidate.SpaceID.String(), newCandidate.RoomID, newCandidate.OperationID, true)
	require.NoError(t, err)
	require.Equal(t, creator.RoomGeneration+1, newHead.RoomGeneration)
	require.ErrorIs(t, admissions.AbandonRoomClaim(ctx, creator.VoiceRoomID, creator.RoomGeneration, creator.OperationID), ErrRoomHeadPending)
	require.NoError(t, admissions.ConfirmRoomHeadOpen(ctx, store.SpaceMediaAdmission{
		OperationID: newHead.CreatorOperationID, RoomGeneration: newHead.RoomGeneration,
		VoiceRoomID: newHead.VoiceRoomID, RoomID: newHead.RoomID, CreatedRoom: true,
	}))
}

func TestPostgresRoomHeadRecoversCrashBetweenClaimAndPrepare(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	admissions := NewPostgresAdmissionStore(pool)
	claim := admissionFixture()
	head, err := admissions.ClaimRoomHead(ctx, claim.VoiceRoomID, claim.SpaceID.String(), claim.RoomID, claim.OperationID, true)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE voice_space_media_room_heads SET created_at=clock_timestamp()-interval '1 minute' WHERE voice_room_id=$1 AND room_generation=$2`, claim.VoiceRoomID, head.RoomGeneration)
	require.NoError(t, err)
	require.NoError(t, admissions.RecoverOrphanRoomHeads(ctx))
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2`, claim.VoiceRoomID, head.RoomGeneration).Scan(&state))
	require.Equal(t, "ENDED", state)
	next := admissionFixture()
	next.VoiceRoomID, next.SpaceID = claim.VoiceRoomID, claim.SpaceID
	reopened, err := admissions.ClaimRoomHead(ctx, next.VoiceRoomID, next.SpaceID.String(), next.RoomID, next.OperationID, true)
	require.NoError(t, err)
	require.Equal(t, head.RoomGeneration+1, reopened.RoomGeneration)
}

func TestPostgresLastLeaveSerializesAfterPreparedAdmissionAndDrainsExactFence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	admissions := NewPostgresAdmissionStore(pool)
	creator := admissionFixture()
	require.NoError(t, prepareAdmission(ctx, admissions, &creator))
	require.NoError(t, admissions.Fence(ctx, creator))
	require.NoError(t, admissions.MarkCommitted(ctx, creator.OperationID, creator.Generation))
	require.NoError(t, admissions.MarkProjectionApplied(ctx, creator.OperationID, creator.Generation))

	joiner := admissionFixture()
	joiner.VoiceRoomID, joiner.SpaceID, joiner.RoomID = creator.VoiceRoomID, creator.SpaceID, creator.RoomID
	joiner.RoomGeneration, joiner.CreatedRoom = creator.RoomGeneration, false
	joiner.Events = []AdmissionEvent{{ID: uuid.New(), Subject: "voice.member_joined", Payload: []byte("pending join")}}
	// Prepare the second participant, then hold the canonical room head while
	// both Fence and the final leave queue on PostgreSQL's actual row lock. The
	// observed wait state makes the chosen Fence-first serialization explicit.
	require.NoError(t, admissions.Prepare(ctx, joiner))
	headLock, err := pool.Begin(ctx)
	require.NoError(t, err)
	var state string
	require.NoError(t, headLock.QueryRow(ctx, `SELECT state FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, creator.VoiceRoomID, creator.RoomGeneration).Scan(&state))
	require.Equal(t, "OPEN", state)
	fenceStarted := make(chan struct{})
	fenceResult := make(chan error, 1)
	go func() {
		close(fenceStarted)
		fenceResult <- admissions.Fence(ctx, joiner)
	}()
	<-fenceStarted
	waitForPostgresLockWait(t, ctx, pool, "SELECT room_generation,space_id,call_room_id,creator_operation_id,state FROM voice_space_media_room_heads")
	leaveStarted := make(chan struct{})
	leaveResult := make(chan error, 1)
	go func() {
		close(leaveStarted)
		leaveResult <- admissions.BeginRoomLeave(ctx, creator.OperationID, creator.Generation)
	}()
	<-leaveStarted
	waitForPostgresLockWait(t, ctx, pool, "SELECT state,room_generation FROM voice_space_media_room_heads")
	require.NoError(t, headLock.Commit(ctx))
	require.NoError(t, <-fenceResult)
	require.NoError(t, <-leaveResult)

	var joinerState, participantState, headState string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state,participant_state FROM voice_space_media_admissions WHERE operation_id=$1`, joiner.OperationID).Scan(&joinerState, &participantState))
	require.Equal(t, "ABORTING", joinerState)
	require.Equal(t, "PREPARED", participantState, "a fenced pending join is never promoted after final leave starts")
	require.ErrorIs(t, admissions.MarkCommitted(ctx, joiner.OperationID, joiner.Generation), ErrRoomHeadPending)
	require.ErrorIs(t, admissions.ConfirmProjection(ctx, joiner.OperationID, joiner.Generation), ErrAdmissionConflict)
	var undelivered int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_media_admission_outbox WHERE operation_id=$1 AND delivered_at IS NULL`, joiner.OperationID).Scan(&undelivered))
	require.Equal(t, 1, undelivered, "the member event remains durable but unpublished while cleanup is pending")

	// Exact operation cleanup removes only its own fence; neither stale creator
	// cleanup nor the pending join may release another participant's fence.
	require.NoError(t, admissions.BeginRoomLeave(ctx, creator.OperationID, creator.Generation))
	require.NoError(t, admissions.CompleteRoomLeave(ctx, creator))
	require.NoError(t, admissions.BeginRoomLeave(ctx, joiner.OperationID, joiner.Generation))
	require.NoError(t, admissions.CompleteRoomLeave(ctx, joiner))
	require.NoError(t, admissions.MarkCleanupCompleted(ctx, joiner.OperationID, joiner.Generation))
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2`, creator.VoiceRoomID, creator.RoomGeneration).Scan(&headState))
	require.Equal(t, "ENDED", headState)
	var fences int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_account_voice_fences WHERE account_id IN ($1,$2)`, creator.AccountID, joiner.AccountID).Scan(&fences))
	require.Zero(t, fences)
	next := admissionFixture()
	next.VoiceRoomID, next.SpaceID = creator.VoiceRoomID, creator.SpaceID
	nextHead, err := admissions.ClaimRoomHead(ctx, next.VoiceRoomID, next.SpaceID.String(), next.RoomID, next.OperationID, true)
	require.NoError(t, err)
	require.Equal(t, creator.RoomGeneration+1, nextHead.RoomGeneration)
	require.NotEqual(t, creator.RoomID, nextHead.RoomID)
	next.AccountID, next.ProfileID = joiner.AccountID, joiner.ProfileID
	next.RoomGeneration = nextHead.RoomGeneration
	require.NoError(t, admissions.Prepare(ctx, next))
	require.NoError(t, admissions.Fence(ctx, next))
	require.NoError(t, admissions.CompleteRoomLeave(ctx, joiner), "stale cleanup is idempotent and cannot release a newer exact-operation fence")
	var currentFence uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT admission_operation_id FROM voice_account_voice_fences WHERE account_id=$1`, next.AccountID).Scan(&currentFence))
	require.Equal(t, next.OperationID, currentFence)
}

func TestPostgresLastLeaveWinsBeforePendingPrepareAndFence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	admissions := NewPostgresAdmissionStore(pool)
	creator := admissionFixture()
	require.NoError(t, prepareAdmission(ctx, admissions, &creator))
	require.NoError(t, admissions.Fence(ctx, creator))
	require.NoError(t, admissions.MarkCommitted(ctx, creator.OperationID, creator.Generation))
	require.NoError(t, admissions.MarkProjectionApplied(ctx, creator.OperationID, creator.Generation))
	joiner := admissionFixture()
	joiner.VoiceRoomID, joiner.SpaceID, joiner.RoomID = creator.VoiceRoomID, creator.SpaceID, creator.RoomID
	joiner.RoomGeneration, joiner.CreatedRoom = creator.RoomGeneration, false
	joiner.Events = []AdmissionEvent{{ID: uuid.New(), Subject: "voice.member_joined", Payload: []byte("denied join")}}

	// The leave is the first waiter on the real room-head row lock. Once it
	// commits CLOSING, a delayed Prepare and Fence must have no durable effects.
	headLock, err := pool.Begin(ctx)
	require.NoError(t, err)
	var state string
	require.NoError(t, headLock.QueryRow(ctx, `SELECT state FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, creator.VoiceRoomID, creator.RoomGeneration).Scan(&state))
	leaveStarted := make(chan struct{})
	leaveResult := make(chan error, 1)
	go func() {
		close(leaveStarted)
		leaveResult <- admissions.BeginRoomLeave(ctx, creator.OperationID, creator.Generation)
	}()
	<-leaveStarted
	waitForPostgresLockWait(t, ctx, pool, "SELECT state,room_generation FROM voice_space_media_room_heads")
	prepareStarted := make(chan struct{})
	prepareResult := make(chan error, 1)
	go func() {
		close(prepareStarted)
		prepareResult <- admissions.Prepare(ctx, joiner)
	}()
	<-prepareStarted
	waitForPostgresLockWait(t, ctx, pool, "SELECT room_generation,space_id,call_room_id,creator_operation_id,state FROM voice_space_media_room_heads")
	require.NoError(t, headLock.Commit(ctx))
	require.NoError(t, <-leaveResult)
	require.ErrorIs(t, <-prepareResult, ErrRoomHeadPending)
	require.ErrorIs(t, admissions.Fence(ctx, joiner), ErrRoomHeadPending)
	var admissionRows, outboxRows, joinerFences int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_media_admissions WHERE operation_id=$1`, joiner.OperationID).Scan(&admissionRows))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_media_admission_outbox WHERE operation_id=$1`, joiner.OperationID).Scan(&outboxRows))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_account_voice_fences WHERE account_id=$1`, joiner.AccountID).Scan(&joinerFences))
	require.Zero(t, admissionRows)
	require.Zero(t, outboxRows)
	require.Zero(t, joinerFences)
	require.NoError(t, admissions.CompleteRoomLeave(ctx, creator))
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2`, creator.VoiceRoomID, creator.RoomGeneration).Scan(&state))
	require.Equal(t, "ENDED", state)
	next := admissionFixture()
	next.VoiceRoomID, next.SpaceID = creator.VoiceRoomID, creator.SpaceID
	nextHead, err := admissions.ClaimRoomHead(ctx, next.VoiceRoomID, next.SpaceID.String(), next.RoomID, next.OperationID, true)
	require.NoError(t, err)
	require.Equal(t, creator.RoomGeneration+1, nextHead.RoomGeneration)
	require.NotEqual(t, creator.RoomID, nextHead.RoomID)
}

func waitForPostgresLockWait(t *testing.T, ctx context.Context, pool *pgxpool.Pool, queryMarker string) {
	t.Helper()
	var lastErr error
	require.Eventually(t, func() bool {
		var waiting bool
		lastErr = pool.QueryRow(ctx, `SELECT EXISTS (
 SELECT 1 FROM pg_stat_activity WHERE pid <> pg_backend_pid() AND wait_event_type='Lock' AND query LIKE '%' || $1 || '%')`, queryMarker).Scan(&waiting)
		return lastErr == nil && waiting
	}, 4*time.Second, 10*time.Millisecond, "expected a PostgreSQL transaction to wait on the held room-head row lock; last query error: %v", lastErr)
}

func admissionFixture() Admission {
	return Admission{
		OperationID: uuid.New(), Generation: uuid.NewString(), AccountID: uuid.New(), ProfileID: uuid.New(),
		SpaceID: uuid.New(), RoomID: uuid.NewString(), VoiceRoomID: uuid.NewString(), RoomGeneration: 1, CreatedRoom: true, Identity: "space-media-test-identity",
		SessionEpoch: 3, AccessEpoch: 4, PolicyEpoch: 5, CanJoin: true, CanSubscribe: true,
		Events: []AdmissionEvent{
			{ID: uuid.New(), Subject: "voice.call_started", Payload: []byte("fixed call event")},
			{ID: uuid.New(), Subject: "voice.member_joined", Payload: []byte("fixed member event")},
		},
	}
}

func prepareAdmission(ctx context.Context, admissions *PostgresAdmissionStore, a *Admission) error {
	head, err := admissions.ClaimRoomHead(ctx, a.VoiceRoomID, a.SpaceID.String(), a.RoomID, a.OperationID, a.CreatedRoom)
	if err != nil {
		return err
	}
	if head.RoomID != a.RoomID || head.SpaceID != a.SpaceID.String() {
		return ErrAdmissionConflict
	}
	a.RoomGeneration = head.RoomGeneration
	return admissions.Prepare(ctx, *a)
}

func startSpaceMediaAdmissionPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	migrations := filepath.Join(root, "src", "backend", "migrations", "voice_db")
	pool := integrationtest.StartPostgres(t, ctx, "voice_space_media_admission", filepath.Join(migrations, "000001_room_lifecycle.up.sql"))
	for _, name := range []string{"000002_redis_divergence", "000003_matchmaking_membership", "000004_game_session_rooms", "000005_game_session_close", "000006_game_session_roster_lease", "000007_t17_sdk_conversion_fence", "000008_account_voice_fence", "000009_space_lifecycle", "000010_space_media_admission"} {
		body, err := os.ReadFile(filepath.Join(migrations, name+".up.sql"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err)
	}
	return pool
}
