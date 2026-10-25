package spacemedia

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/grpcsvc"
	"voice/backend/voice/internal/spacegatetest"
	"voice/backend/voice/internal/spacelifecycle"
	"voice/backend/voice/internal/store"
)

func TestSpaceAdmissionMissingGateRejectsStaleFencersBeforeAllocation(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	s := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	ctx = ownAdmission(ctx, &a)
	require.NoError(t, prepareAdmission(ctx, s, &a))
	require.NoError(t, s.Fence(ctx, a))
	require.NoError(t, s.MarkCommitted(ctx, a.OperationID, a.Generation))
	require.NoError(t, s.MarkAborting(ctx, a.OperationID, a.Generation))
	oldTarget, err := s.BeginRestrictiveRoomLeave(ctx, a)
	require.NoError(t, err)
	require.NoError(t, s.RelinquishAdmissionOwnership(ctx, a.OperationID))
	current, claimed, err := s.ClaimAdmissionRecovery(context.Background(), a)
	require.NoError(t, err)
	require.True(t, claimed)
	calls, client := newP3Redis(t)
	key := "p3:space_admission_dispatch:" + a.OperationID.String()
	for _, mode := range []string{"owned", "restrictive"} {
		t.Run(mode, func(t *testing.T) {
			require.NoError(t, client.Del(ctx, key).Err())
			if mode == "owned" {
				_, err = s.FenceOwnedRoomLeave(context.Background(), a.Ownership, a.OperationID, calls)
			} else {
				_, err = s.FenceRestrictiveRoomLeave(context.Background(), oldTarget, calls)
			}
			require.ErrorIs(t, err, ErrAdmissionConflict)
			require.Zero(t, client.Exists(ctx, key).Val(), "a stale fencer must reject before generic RESTORING allocation")
		})
	}
	fresh := store.WithSpaceAdmissionOwnership(context.Background(), current.Ownership)
	require.NoError(t, s.FenceAbortedAdmission(fresh, a.OperationID, calls))
}

func waitForAdmissionLeaseExpiry(t *testing.T, ctx context.Context, pool *pgxpool.Pool, op uuid.UUID) {
	t.Helper()
	require.Eventually(t, func() bool {
		var expired bool
		err := pool.QueryRow(ctx, `SELECT lease_until<=clock_timestamp() FROM voice_space_media_admission_owners WHERE operation_id=$1`, op).Scan(&expired)
		return err == nil && expired
	}, 4*time.Second, 10*time.Millisecond, "observe actual PostgreSQL deadline, not a scheduler sleep")
}

func TestSpaceAdmissionOwnerLockWaitCannotCarryPrelockFreshness(t *testing.T) {
	for _, mode := range []string{"mutation", "renewal"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			pool := startSpaceMediaAdmissionPostgres(t, ctx)
			s := NewPostgresAdmissionStore(pool)
			a := admissionFixture()
			ctx = ownAdmission(ctx, &a)
			require.NoError(t, prepareAdmission(ctx, s, &a))
			_, err := pool.Exec(ctx, `UPDATE voice_space_media_admission_owners SET lease_until=clock_timestamp()+interval '2 seconds' WHERE operation_id=$1`, a.OperationID)
			require.NoError(t, err)
			blocker, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer blocker.Rollback(context.Background())
			var op uuid.UUID
			require.NoError(t, blocker.QueryRow(ctx, `SELECT operation_id FROM voice_space_media_admission_owners WHERE operation_id=$1 FOR UPDATE`, a.OperationID).Scan(&op))
			result := make(chan error, 1)
			go func() {
				if mode == "mutation" {
					result <- s.MarkAborting(ctx, a.OperationID, a.Generation)
				} else {
					result <- s.RenewAdmissionOwnership(ctx, a.OperationID)
				}
			}()
			waitForPostgresLockWait(t, ctx, pool, "voice_space_media_admission_owners")
			var beforeDeadline bool
			require.NoError(t, pool.QueryRow(ctx, `SELECT clock_timestamp()<lease_until FROM voice_space_media_admission_owners WHERE operation_id=$1`, a.OperationID).Scan(&beforeDeadline))
			require.True(t, beforeDeadline, "the candidate observed the owner before expiry and is now blocked on unchanged row")
			waitForAdmissionLeaseExpiry(t, ctx, pool, a.OperationID)
			require.NoError(t, blocker.Commit(ctx))
			require.ErrorIs(t, <-result, ErrAdmissionConflict, "freshness must be evaluated after acquiring owner lock")
			var state string
			require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM voice_space_media_admissions WHERE operation_id=$1`, a.OperationID).Scan(&state))
			require.Equal(t, AdmissionPrepared, state)
			var expired bool
			require.NoError(t, pool.QueryRow(ctx, `SELECT lease_until<=clock_timestamp() FROM voice_space_media_admission_owners WHERE operation_id=$1`, a.OperationID).Scan(&expired))
			require.True(t, expired, "a renewal blocked across expiry cannot resurrect the lease")
		})
	}
}

func TestSpaceAdmissionAlreadyLockedTransactionMayFinishAfterExpiry(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	s := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	ctx = ownAdmission(ctx, &a)
	require.NoError(t, prepareAdmission(ctx, s, &a))
	_, err := pool.Exec(ctx, `UPDATE voice_space_media_admission_owners SET lease_until=clock_timestamp()+interval '2 seconds' WHERE operation_id=$1`, a.OperationID)
	require.NoError(t, err)
	blocker, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer blocker.Rollback(context.Background())
	var op uuid.UUID
	require.NoError(t, blocker.QueryRow(ctx, `SELECT operation_id FROM voice_space_media_admissions WHERE operation_id=$1 FOR UPDATE`, a.OperationID).Scan(&op))
	result := make(chan error, 1)
	go func() { result <- s.MarkAborting(ctx, a.OperationID, a.Generation) }()
	waitForPostgresLockWait(t, ctx, pool, "UPDATE voice_space_media_admissions SET state='ABORTING'")
	waitForAdmissionLeaseExpiry(t, ctx, pool, a.OperationID)
	recovered := make(chan error, 1)
	go func() {
		_, claimed, err := s.ClaimAdmissionRecovery(context.Background(), a)
		if err == nil && !claimed {
			err = ErrAdmissionConflict
		}
		recovered <- err
	}()
	waitForPostgresLockWait(t, ctx, pool, "pg_advisory_xact_lock")
	select {
	case err := <-recovered:
		t.Fatalf("takeover crossed the still-locked transaction: %v", err)
	default:
	}
	require.NoError(t, blocker.Commit(ctx))
	require.NoError(t, <-result, "a transaction already checked under retained exclusive owner lock remains ordered before takeover")
	require.NoError(t, <-recovered)
	require.ErrorIs(t, s.MarkAborting(ctx, a.OperationID, a.Generation), ErrAdmissionConflict, "no authority carries to a subsequent transaction")
}

func TestSpaceAdmissionCancelledDownstreamWaitRollsBackAuthority(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	s := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	ctx = ownAdmission(ctx, &a)
	require.NoError(t, prepareAdmission(ctx, s, &a))
	blocker, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer blocker.Rollback(context.Background())
	var op uuid.UUID
	require.NoError(t, blocker.QueryRow(ctx, `SELECT operation_id FROM voice_space_media_admissions WHERE operation_id=$1 FOR UPDATE`, a.OperationID).Scan(&op))
	attempt, cancel := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() { result <- s.MarkAborting(attempt, a.OperationID, a.Generation) }()
	waitForPostgresLockWait(t, ctx, pool, "UPDATE voice_space_media_admissions SET state='ABORTING'")
	cancel()
	require.ErrorIs(t, <-result, context.Canceled)
	require.NoError(t, blocker.Commit(ctx))
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM voice_space_media_admissions WHERE operation_id=$1`, a.OperationID).Scan(&state))
	require.Equal(t, AdmissionPrepared, state)
	require.NoError(t, s.RenewAdmissionOwnership(ctx, a.OperationID), "cancelled transaction released its owner lock")
}

func TestSpaceAdmissionOwnedDenyLoaderCannotCreateLiveGate(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	s := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	ctx = ownAdmission(ctx, &a)
	require.NoError(t, prepareAdmission(ctx, s, &a))
	require.NoError(t, s.Fence(ctx, a))
	require.NoError(t, s.MarkCommitted(ctx, a.OperationID, a.Generation))
	calls, client := newP3Redis(t)
	key := "p3:space_admission_dispatch:" + a.OperationID.String()
	require.NoError(t, client.Del(ctx, key).Err())
	_, err := s.FenceOwnedRoomLeave(ctx, a.Ownership, a.OperationID, calls)
	require.ErrorIs(t, err, ErrAdmissionConflict)
	require.Zero(t, client.Exists(ctx, key).Val(), "a deny-only API may not allocate or publish a LIVE gate")
}

func TestSpaceAdmissionCapturedWrongTupleHasNoNativeEffects(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	s := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	ctx = ownAdmission(ctx, &a)
	require.NoError(t, prepareAdmission(ctx, s, &a))
	require.NoError(t, s.Fence(ctx, a))
	require.NoError(t, s.MarkCommitted(ctx, a.OperationID, a.Generation))
	require.NoError(t, s.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
	calls := store.NewMemoryCallStore()
	c := &Coordinator{Calls: calls, Store: calls, Admissions: s, ProjectionGates: spacegatetest.New()}
	target := spacelifecycle.CapturedTarget{OperationID: a.OperationID.String(), Generation: a.Generation, SpaceID: a.SpaceID.String(), AccountID: a.AccountID.String(), ProfileID: a.ProfileID.String(), RoomID: a.RoomID, VoiceRoomID: a.VoiceRoomID, RoomGeneration: a.RoomGeneration, Identity: a.Identity}
	snapshot := func() string {
		var result string
		err := pool.QueryRow(ctx, `SELECT jsonb_build_object('admission',to_jsonb(a),'head',to_jsonb(h),'roster_outbox',(SELECT jsonb_agg(to_jsonb(r) ORDER BY event_id) FROM voice_roster_outbox r WHERE r.room_id::text=a.room_id),'admission_outbox',(SELECT jsonb_agg(to_jsonb(o) ORDER BY event_id) FROM voice_space_media_admission_outbox o WHERE o.operation_id=a.operation_id))::text FROM voice_space_media_admissions a JOIN voice_space_media_room_heads h USING(voice_room_id,room_generation) WHERE a.operation_id=$1`, a.OperationID).Scan(&result)
		require.NoError(t, err)
		return result
	}
	before := snapshot()
	mutations := map[string]func(*spacelifecycle.CapturedTarget){"account": func(t *spacelifecycle.CapturedTarget) { t.AccountID = uuid.NewString() }, "profile": func(t *spacelifecycle.CapturedTarget) { t.ProfileID = uuid.NewString() }, "space": func(t *spacelifecycle.CapturedTarget) { t.SpaceID = uuid.NewString() }, "room": func(t *spacelifecycle.CapturedTarget) { t.RoomID = uuid.NewString() }, "voice-room": func(t *spacelifecycle.CapturedTarget) { t.VoiceRoomID = uuid.NewString() }, "room-generation": func(t *spacelifecycle.CapturedTarget) { t.RoomGeneration++ }, "identity": func(t *spacelifecycle.CapturedTarget) { t.Identity += "-wrong" }}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			wrong := target
			mutate(&wrong)
			require.ErrorIs(t, c.ResumeCapturedDrain(ctx, wrong), ErrAdmissionConflict)
			require.Equal(t, before, snapshot(), "a rejected captured tuple must leave participant/head/roster/outbox byte-equivalent")
		})
	}
}

type admissionDenyLostAck struct {
	key  string
	once sync.Once
	lost bool
}

func (h *admissionDenyLostAck) DialHook(next redis.DialHook) redis.DialHook          { return next }
func (h *admissionDenyLostAck) ProcessHook(next redis.ProcessHook) redis.ProcessHook { return next }
func (h *admissionDenyLostAck) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, commands []redis.Cmder) error {
		deny := false
		for _, command := range commands {
			args := command.Args()
			if command.Name() == "set" && len(args) > 2 && args[1] == h.key {
				var gate struct {
					Phase string
					Live  bool
				}
				if raw, ok := args[2].(string); ok && json.Unmarshal([]byte(raw), &gate) == nil && gate.Phase == "READY" && !gate.Live {
					deny = true
				}
			}
		}
		err := next(ctx, commands)
		if err != nil || !deny {
			return err
		}
		lost := false
		h.once.Do(func() { h.lost = true; lost = true })
		if lost {
			return errors.New("injected acknowledgement loss after actual DENY EXEC")
		}
		return nil
	}
}

func TestSpaceAdmissionUnknownSinkAcknowledgementCannotReleaseFence(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	s := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	ctx = ownAdmission(ctx, &a)
	require.NoError(t, prepareAdmission(ctx, s, &a))
	require.NoError(t, s.Fence(ctx, a))
	require.NoError(t, s.MarkAborting(ctx, a.OperationID, a.Generation))
	calls, client := newP3Redis(t)
	hook := &admissionDenyLostAck{key: "p3:space_admission_dispatch:" + a.OperationID.String()}
	client.AddHook(hook)
	require.ErrorContains(t, s.FenceAbortedAdmission(ctx, a.OperationID, calls), "acknowledgement loss")
	require.True(t, hook.lost)
	var gate struct {
		Phase string
		Live  bool
	}
	raw, err := client.Get(ctx, hook.key).Bytes()
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &gate))
	require.Equal(t, "READY", gate.Phase)
	require.False(t, gate.Live, "actual Redis committed DENY despite the lost response")
	var ack int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT dispatch_fenced_epoch FROM voice_space_media_admission_owners WHERE operation_id=$1`, a.OperationID).Scan(&ack))
	require.Zero(t, ack, "unknown sink response is not PG acknowledgement")
	require.ErrorIs(t, s.ReleaseFence(ctx, a), ErrAdmissionConflict)
	require.ErrorIs(t, s.MarkCleanupCompleted(ctx, a.OperationID, a.Generation), ErrAdmissionConflict)
	require.NoError(t, s.FenceAbortedAdmission(ctx, a.OperationID, calls))
	require.NoError(t, s.ReleaseFence(ctx, a))
	require.NoError(t, s.MarkCleanupCompleted(ctx, a.OperationID, a.Generation))
}

func TestSpaceAdmissionRestrictiveRevocationCannotAdoptTakeover(t *testing.T) {
	for _, boundary := range []string{"before-fence", "after-fence", "after-drain"} {
		t.Run(boundary, func(t *testing.T) {
			ctx := context.Background()
			pool := startSpaceMediaAdmissionPostgres(t, ctx)
			s := NewPostgresAdmissionStore(pool)
			a := admissionFixture()
			ctx = ownAdmission(ctx, &a)
			require.NoError(t, prepareAdmission(ctx, s, &a))
			require.NoError(t, s.Fence(ctx, a))
			require.NoError(t, s.MarkCommitted(ctx, a.OperationID, a.Generation))
			calls, client := newP3Redis(t)
			c := &Coordinator{ProjectionGates: calls, Admissions: s, Calls: calls, Store: calls, Media: &p3Media{}}
			forward, err := c.prepareProjection(ctx, a)
			require.NoError(t, err)
			shell := store.Call{SpaceProjectionTicket: forward, SpaceLifecycleGeneration: a.LifecycleGeneration, RoomID: a.RoomID, VoiceRoomID: a.VoiceRoomID, SpaceID: a.SpaceID.String(), LivekitRoomName: "voice-room-" + a.VoiceRoomID, SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, Status: callsv1.CallStatus_CALL_STATUS_UNSPECIFIED, States: map[string]store.ParticipantState{}}
			_, err = calls.CreateCall(ctx, shell)
			require.NoError(t, err)
			participant := store.SpaceMediaParticipant{ProjectionTicket: forward, LifecycleGeneration: a.LifecycleGeneration, RoomGeneration: a.RoomGeneration, CreatedRoom: a.CreatedRoom, AccountID: a.AccountID.String(), ProfileID: a.ProfileID.String(), AdmissionOperationID: a.OperationID.String(), Identity: a.Identity, Generation: a.Generation, Issued: store.SpaceMediaGrant{SessionEpoch: a.SessionEpoch, AccessEpoch: a.AccessEpoch, PolicyEpoch: a.PolicyEpoch, CanJoin: true, CanSubscribe: true}}
			_, err = calls.AdmitSpaceMediaParticipant(ctx, a.RoomID, participant, a.MaxParticipants)
			require.NoError(t, err)
			require.NoError(t, s.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
			// Explicit missing workflow authority cannot become a trusted ACL call.
			require.ErrorIs(t, s.BeginOwnedRoomLeave(context.Background(), store.SpaceAdmissionOwnership{}, a.OperationID, a.Generation), ErrAdmissionConflict)
			target, err := s.BeginRestrictiveRoomLeave(context.Background(), a)
			require.NoError(t, err)
			require.Equal(t, int64(1), target.OwnerEpoch)
			cleanupContext := func(ticket store.SpaceProjectionTicket) context.Context {
				ticket.SpaceID = a.SpaceID.String()
				ticket.RoomID = a.RoomID
				ticket.VoiceRoomID = a.VoiceRoomID
				ticket.RoomGeneration = a.RoomGeneration
				ticket.OperationID = a.OperationID.String()
				ticket.AdmissionGeneration = a.Generation
				ticket.AccountID = a.AccountID.String()
				ticket.ProfileID = a.ProfileID.String()
				ticket.Identity = a.Identity
				return store.WithSpaceCleanupTarget(context.Background(), ticket)
			}
			var oldCleanup context.Context
			if boundary != "before-fence" {
				ticket, err := s.FenceRestrictiveRoomLeave(context.Background(), target, calls)
				require.NoError(t, err)
				oldCleanup = cleanupContext(ticket)
			}
			drain := func(cleanup context.Context) {
				_, _, err := calls.BeginSpaceMediaRevocation(cleanup, a.RoomID, a.ProfileID.String(), a.Identity, a.Generation)
				require.NoError(t, err)
				require.NoError(t, c.Media.RemoveParticipant(cleanup, shell.LivekitRoomName, a.Identity))
				_, _, err = calls.CompleteSpaceMediaRevocation(cleanup, a.RoomID, a.ProfileID.String(), a.Identity, a.Generation)
				require.NoError(t, err)
			}
			if boundary == "after-drain" {
				drain(oldCleanup)
			}
			require.NoError(t, s.RelinquishAdmissionOwnership(ctx, a.OperationID))
			current, claimed, err := NewPostgresAdmissionStore(pool).ClaimAdmissionRecovery(context.Background(), a)
			require.NoError(t, err)
			require.True(t, claimed)
			require.Equal(t, int64(2), current.Ownership.Epoch)
			gateKey := "p3:space_admission_dispatch:" + a.OperationID.String()
			before, err := client.Get(ctx, gateKey).Result()
			require.NoError(t, err)
			_, err = s.FenceOwnedRoomLeave(context.Background(), a.Ownership, a.OperationID, calls)
			require.ErrorIs(t, err, ErrAdmissionConflict, "stale owned loader must reject before changing Redis")
			_, err = s.FenceRestrictiveRoomLeave(context.Background(), target, calls)
			require.ErrorIs(t, err, ErrAdmissionConflict, "restrictive target cannot adopt a newer epoch")
			after, err := client.Get(ctx, gateKey).Result()
			require.NoError(t, err)
			require.Equal(t, before, after)
			require.ErrorIs(t, s.CompleteOwnedRoomLeave(context.Background(), a.Ownership, a), ErrAdmissionConflict)
			require.ErrorIs(t, s.CompleteRestrictiveRoomLeave(context.Background(), target), ErrAdmissionConflict)
			fresh, err := s.BeginRestrictiveRoomLeave(context.Background(), a)
			require.NoError(t, err)
			require.Equal(t, int64(2), fresh.OwnerEpoch)
			ticket, err := s.FenceRestrictiveRoomLeave(context.Background(), fresh, calls)
			require.NoError(t, err)
			require.False(t, ticket.AdmissionLive)
			if oldCleanup != nil {
				_, _, err := calls.CompleteSpaceMediaRevocation(oldCleanup, a.RoomID, a.ProfileID.String(), a.Identity, a.Generation)
				require.ErrorIs(t, err, store.ErrSpaceMediaStaleGrant)
			}
			_, err = calls.CreateCall(ctx, shell)
			require.ErrorIs(t, err, store.ErrSpaceMediaStaleGrant, "captured forward callback cannot revive projection after restrictive DENY")
			if boundary != "after-drain" {
				drain(cleanupContext(ticket))
			}
			wrong := fresh
			wrong.Admission.AccountID = uuid.New()
			require.ErrorIs(t, s.CompleteRestrictiveRoomLeave(context.Background(), wrong), ErrAdmissionConflict)
			require.NoError(t, s.CompleteRestrictiveRoomLeave(context.Background(), fresh))
			require.NoError(t, client.Del(ctx, gateKey).Err())
			rebuilt, err := s.FenceRestrictiveRoomLeave(context.Background(), fresh, calls)
			require.NoError(t, err)
			require.False(t, rebuilt.AdmissionLive)
			require.NotEqual(t, ticket.AdmissionIncarnation, rebuilt.AdmissionIncarnation, "terminal restrictive reconstruction cannot reuse a lost nonce or mint forward authority")
			_, _, err = calls.CompleteSpaceMediaRevocation(cleanupContext(ticket), a.RoomID, a.ProfileID.String(), a.Identity, a.Generation)
			require.ErrorIs(t, err, store.ErrSpaceMediaStaleGrant)
			bad := a
			bad.AccountID = uuid.New()
			require.ErrorIs(t, s.CompleteOwnedRoomLeave(context.Background(), current.Ownership, bad), ErrAdmissionConflict, "even the current owner cannot acknowledge another terminal tuple")
			require.ErrorIs(t, s.BeginOwnedRoomLeave(context.Background(), a.Ownership, a.OperationID, a.Generation), ErrAdmissionConflict, "terminal idempotence cannot bypass replaced owner")
			require.ErrorIs(t, s.CompleteOwnedRoomLeave(context.Background(), a.Ownership, a), ErrAdmissionConflict)
			var fences int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_account_voice_fences WHERE admission_operation_id=$1`, a.OperationID).Scan(&fences))
			require.Zero(t, fences)
		})
	}
}

func TestSpaceAdmissionOrphanRecoveryRechecksRenewalAfterOwnerRowLock(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	s := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	ctx = ownAdmission(ctx, &a)
	_, err := s.ClaimRoomHead(ctx, a.VoiceRoomID, a.SpaceID.String(), a.RoomID, a.OperationID, true)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE voice_space_media_admission_owners SET lease_until=clock_timestamp()-interval '1 second' WHERE operation_id=$1`, a.OperationID)
	require.NoError(t, err)
	// Hold a fresh authoritative deadline uncommitted while the old MVCC
	// snapshot still appears expired. Production renewal no-revive is tested
	// separately; this fixture isolates recovery's required row-lock recheck.
	renewal, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer renewal.Rollback(ctx)
	_, err = renewal.Exec(ctx, `UPDATE voice_space_media_admission_owners SET lease_until=clock_timestamp()+interval '1 minute' WHERE operation_id=$1`, a.OperationID)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- NewPostgresAdmissionStore(pool).RecoverOrphanRoomHeads(context.Background()) }()
	completed := false
	require.Eventually(t, func() bool {
		select {
		case err := <-done:
			require.NoError(t, err)
			completed = true
			return true
		default:
		}
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%voice_space_media_admission_owners%')`).Scan(&waiting)
		require.NoError(t, err)
		return waiting
	}, 3*time.Second, 10*time.Millisecond, "recovery must either complete or actually wait on the ownership arbitration row")
	require.NoError(t, renewal.Commit(ctx))
	if !completed {
		require.NoError(t, <-done)
	}
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM voice_space_media_room_heads WHERE creator_operation_id=$1`, a.OperationID).Scan(&state))
	require.Equal(t, "OPEN", state, "a stale expiry snapshot cannot close a head after a fresher owner deadline wins")
}

// Independent store instances deliberately share only PostgreSQL. The live
// request is paused at a durable boundary; recovery runs to completion before
// that request resumes, so the interleaving does not depend on scheduler luck.
func TestSpaceAdmissionRecoveryDoesNotStealLiveWriter(t *testing.T) {
	for _, boundary := range []string{"PREPARED", "FENCED", "COMMITTED"} {
		t.Run(boundary, func(t *testing.T) {
			ctx := context.Background()
			pool := startSpaceMediaAdmissionPostgres(t, ctx)
			foreground := NewPostgresAdmissionStore(pool)
			recovery := NewPostgresAdmissionStore(pool)
			a := admissionFixture()
			ctx = ownAdmission(ctx, &a)
			require.NoError(t, prepareAdmission(ctx, foreground, &a))
			if boundary != "PREPARED" {
				require.NoError(t, foreground.Fence(ctx, a))
			}
			if boundary == "COMMITTED" {
				require.NoError(t, foreground.MarkCommitted(ctx, a.OperationID, a.Generation))
			}
			calls := store.NewMemoryCallStore()
			coordinator := &Coordinator{ProjectionGates: spacegatetest.New(), Store: calls,
				Calls: calls, Admissions: recovery, Access: fakeAccess{}, Grants: fakeGrants{},
				SessionEpochChecker: currentEpochChecker{}}
			require.NoError(t, coordinator.reconcileAdmissions(ctx))
			var state string
			require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM voice_space_media_admissions WHERE operation_id=$1`, a.OperationID).Scan(&state))
			require.Equal(t, boundary, state, "a recovery process cannot abort an admission while its foreground writer retains authority")
			if boundary == "PREPARED" {
				require.NoError(t, foreground.Fence(ctx, a))
			}
			require.NoError(t, foreground.MarkCommitted(ctx, a.OperationID, a.Generation))
			require.NoError(t, foreground.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
			require.NoError(t, foreground.ConfirmProjection(ctx, a.OperationID, a.Generation))
		})
	}
}

func TestSpaceAdmissionStalePreparedCandidateRollsForwardCurrentCommittedCreator(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	foreground := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	ctx = ownAdmission(ctx, &a)
	require.NoError(t, prepareAdmission(ctx, foreground, &a))
	stale := a
	stale.State = AdmissionPrepared
	require.NoError(t, foreground.Fence(ctx, a))
	require.NoError(t, foreground.MarkCommitted(ctx, a.OperationID, a.Generation))
	_, claimed, err := NewPostgresAdmissionStore(pool).ClaimAdmissionRecovery(context.Background(), stale)
	require.NoError(t, err)
	require.False(t, claimed, "even a stale PREPARED candidate cannot steal a healthy COMMITTED writer")
	_, err = pool.Exec(ctx, `UPDATE voice_space_media_admission_owners SET lease_until=clock_timestamp()-interval '1 second' WHERE operation_id=$1`, a.OperationID)
	require.NoError(t, err)
	calls, _ := newP3Redis(t)
	recovery := NewPostgresAdmissionStore(pool)
	coordinator := &Coordinator{ProjectionGates: calls, Admissions: recovery, Calls: calls, Store: calls, Media: &p3Media{},
		Access: fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{a.ProfileID.String(): {SpaceID: a.SpaceID.String(), Active: true, Member: true, AccessEpoch: a.AccessEpoch, CommunicationStateResolved: true}}},
		Grants: fakeGrants{byProfile: map[string]grpcsvc.CanonicalVoiceRoomGrants{a.ProfileID.String(): {PolicyEpoch: a.PolicyEpoch, CanJoin: true, CanSubscribe: true}}}, SessionEpochChecker: currentEpochChecker{}}
	require.NoError(t, coordinator.reconcileOwnedAdmission(context.Background(), stale), "takeover reads COMMITTED and restores the creator shell/participant, instead of applying the stale PREPARED abort branch")
	require.NoError(t, recovery.ConfirmProjection(ctx, a.OperationID, a.Generation))
	call, err := calls.GetCall(ctx, a.RoomID)
	require.NoError(t, err)
	require.Equal(t, a.OperationID.String(), call.SpaceMedia[a.ProfileID.String()].AdmissionOperationID)
	var roster int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT roster_version FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2`, a.VoiceRoomID, a.RoomGeneration).Scan(&roster))
	require.Equal(t, int64(1), roster)
	want := map[string][]byte{}
	for _, event := range a.Events {
		want[event.ID.String()] = event.Payload
	}
	for range a.Events {
		item, err := recovery.ClaimOutbox(ctx, ReconcileInterval)
		require.NoError(t, err)
		require.NotNil(t, item)
		payload, ok := want[item.ID.String()]
		require.True(t, ok)
		require.Equal(t, payload, item.Payload)
		delete(want, item.ID.String())
		require.NoError(t, recovery.MarkOutboxDelivered(ctx, *item))
	}
	require.Empty(t, want, "takeover retains the original event IDs and exact payload bytes")
}

func TestSpaceAdmissionSinkFencesPausedProjectionAfterTakeover(t *testing.T) {
	for _, boundary := range []string{"before-shell", "shell", "participant"} {
		t.Run(boundary, func(t *testing.T) {
			ctx := context.Background()
			pool := startSpaceMediaAdmissionPostgres(t, ctx)
			foreground := NewPostgresAdmissionStore(pool)
			a := admissionFixture()
			ctx = ownAdmission(ctx, &a)
			require.NoError(t, prepareAdmission(ctx, foreground, &a))
			require.NoError(t, foreground.Fence(ctx, a))
			require.NoError(t, foreground.MarkCommitted(ctx, a.OperationID, a.Generation))
			calls, client := newP3Redis(t)
			coordinator := &Coordinator{ProjectionGates: calls, Admissions: foreground, Calls: calls, Store: calls, Media: &p3Media{}}
			old, err := coordinator.prepareProjection(ctx, a)
			require.NoError(t, err)
			call := store.Call{SpaceProjectionTicket: old, SpaceLifecycleGeneration: a.LifecycleGeneration, RoomID: a.RoomID, VoiceRoomID: a.VoiceRoomID, SpaceID: a.SpaceID.String(), LivekitRoomName: "voice-room-" + a.VoiceRoomID, SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, Status: callsv1.CallStatus_CALL_STATUS_UNSPECIFIED, States: map[string]store.ParticipantState{}}
			participant := store.SpaceMediaParticipant{ProjectionTicket: old, LifecycleGeneration: a.LifecycleGeneration, RoomGeneration: a.RoomGeneration, CreatedRoom: a.CreatedRoom, AccountID: a.AccountID.String(), ProfileID: a.ProfileID.String(), AdmissionOperationID: a.OperationID.String(), Identity: a.Identity, Generation: a.Generation, Issued: store.SpaceMediaGrant{SessionEpoch: a.SessionEpoch, AccessEpoch: a.AccessEpoch, PolicyEpoch: a.PolicyEpoch, CanJoin: true, CanSubscribe: true}}
			if boundary != "before-shell" {
				_, err = calls.CreateCall(ctx, call)
				require.NoError(t, err)
			}
			if boundary == "participant" {
				_, err = calls.AdmitSpaceMediaParticipant(ctx, a.RoomID, participant, a.MaxParticipants)
				require.NoError(t, err)
			}
			_, err = pool.Exec(ctx, `UPDATE voice_space_media_admission_owners SET lease_until=clock_timestamp()-interval '1 second' WHERE operation_id=$1`, a.OperationID)
			require.NoError(t, err)
			recovery := NewPostgresAdmissionStore(pool)
			current, claimed, err := recovery.ClaimAdmissionRecovery(context.Background(), a)
			require.NoError(t, err)
			require.True(t, claimed)
			require.Equal(t, AdmissionCommitted, current.State)
			recoveryCtx := store.WithSpaceAdmissionOwnership(context.Background(), current.Ownership)
			require.NoError(t, recovery.MarkAborting(recoveryCtx, a.OperationID, a.Generation))
			require.ErrorIs(t, recovery.ReleaseFence(recoveryCtx, current), ErrAdmissionConflict, "no fence release before actual sink DENY acknowledgement")
			require.ErrorIs(t, recovery.MarkCleanupCompleted(recoveryCtx, a.OperationID, a.Generation), ErrAdmissionConflict, "cleanup cannot complete before sink fencing")
			coordinator.Admissions = recovery
			require.NoError(t, coordinator.abortAdmission(recoveryCtx, current))
			var remaining int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_account_voice_fences WHERE admission_operation_id=$1`, a.OperationID).Scan(&remaining))
			require.Zero(t, remaining)
			// Erase the terminal call so an existing/ended document cannot conceal a
			// missing sink fence. The suspended callback retains its original ticket.
			require.NoError(t, client.Del(ctx, "p3:call:"+a.RoomID).Err())
			spaceGate, err := calls.ReadReadySpaceGate(ctx, a.SpaceID.String())
			require.NoError(t, err)
			require.Equal(t, old.Incarnation, spaceGate.Incarnation, "the Space remains LIVE at the original nonce: only admission takeover fences this callback")
			_, err = calls.CreateCall(ctx, call)
			require.ErrorIs(t, err, store.ErrSpaceMediaStaleGrant)
			require.Zero(t, client.Exists(ctx, "p3:call:"+a.RoomID, "p3:session:"+a.ProfileID.String(), "p3:active_voice_room:"+a.VoiceRoomID).Val())
			// Loss/rebuild uses a new nonce and fresh terminal PG authority; it may
			// not reopen dispatch or let an old callback adopt a replacement gate.
			require.NoError(t, client.Del(ctx, "p3:space_admission_dispatch:"+a.OperationID.String()).Err())
			rebuilt, err := calls.EnsureAdmissionDispatch(recoveryCtx, a.OperationID.String(), recovery.ReadAdmissionDispatch)
			require.NoError(t, err)
			require.False(t, rebuilt.AdmissionLive)
			require.NotEqual(t, old.AdmissionIncarnation, rebuilt.AdmissionIncarnation)
			_, err = calls.CreateCall(ctx, call)
			require.ErrorIs(t, err, store.ErrSpaceMediaStaleGrant)
			require.Error(t, foreground.MarkProjectionApplied(ctx, a.OperationID, a.Generation), "retired head or owner denies stale completion")
		})
	}
}

func TestSpaceAdmissionExpiredOwnerCannotResumeOrBeatRecoveryCAS(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	live := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	ctx = ownAdmission(ctx, &a)
	require.NoError(t, prepareAdmission(ctx, live, &a))
	require.NoError(t, live.Fence(ctx, a))
	// Expire the DB deadline directly, without sleeping or changing process time.
	_, err := pool.Exec(ctx, `UPDATE voice_space_media_admission_owners SET lease_until=clock_timestamp()-interval '1 second' WHERE operation_id=$1`, a.OperationID)
	require.NoError(t, err)
	require.ErrorIs(t, live.RenewAdmissionOwnership(ctx, a.OperationID), ErrAdmissionConflict)
	require.ErrorIs(t, live.MarkCommitted(ctx, a.OperationID, a.Generation), ErrAdmissionConflict)
	type result struct {
		admission Admission
		claimed   bool
		err       error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			r, claimed, e := NewPostgresAdmissionStore(pool).ClaimAdmissionRecovery(context.Background(), a)
			results <- result{r, claimed, e}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	winners := 0
	for r := range results {
		require.NoError(t, r.err)
		if !r.claimed {
			continue
		}
		winners++
		require.Equal(t, int64(2), r.admission.Ownership.Epoch)
		require.NotEqual(t, a.Ownership.Token, r.admission.Ownership.Token)
		require.Equal(t, AdmissionAborting, r.admission.State, "takeover of unfinished prepare/fence is monotonic abort")
	}
	require.Equal(t, 1, winners, "exactly one independent recovery claimant owns the next epoch")
	require.ErrorIs(t, live.RenewAdmissionOwnership(ctx, a.OperationID), ErrAdmissionConflict)
	require.ErrorIs(t, live.MarkAborting(ctx, a.OperationID, a.Generation), ErrAdmissionConflict)
	require.ErrorIs(t, live.ReleaseFence(ctx, a), ErrAdmissionConflict)
	require.ErrorIs(t, live.MarkProjectionApplied(ctx, a.OperationID, a.Generation), ErrAdmissionConflict)
	require.ErrorIs(t, live.MarkCleanupCompleted(ctx, a.OperationID, a.Generation), ErrAdmissionConflict)
}

func TestSpaceAdmissionReplacedOwnerCannotFinishRevocation(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	s := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	ctx = ownAdmission(ctx, &a)
	require.NoError(t, prepareAdmission(ctx, s, &a))
	require.NoError(t, s.Fence(ctx, a))
	require.NoError(t, s.MarkCommitted(ctx, a.OperationID, a.Generation))
	require.NoError(t, s.MarkAborting(ctx, a.OperationID, a.Generation))
	require.NoError(t, s.FenceAbortedAdmission(ctx, a.OperationID, spacegatetest.New()))
	require.NoError(t, fixtureBeginOwnedLeave(ctx, s, a.OperationID, a.Generation))
	require.NoError(t, s.RelinquishAdmissionOwnership(ctx, a.OperationID))
	current, claimed, err := NewPostgresAdmissionStore(pool).ClaimAdmissionRecovery(context.Background(), a)
	require.NoError(t, err)
	require.True(t, claimed)
	require.ErrorIs(t, fixtureBeginOwnedLeave(ctx, s, a.OperationID, a.Generation), ErrAdmissionConflict, "an old recovery cannot resume the shared revocation path after takeover")
	require.ErrorIs(t, fixtureCompleteOwnedLeave(ctx, s, a), ErrAdmissionConflict, "a delayed old cleanup cannot release the account fence or mark completion")
	var fences int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_account_voice_fences WHERE admission_operation_id=$1`, a.OperationID).Scan(&fences))
	require.Equal(t, 1, fences)
	newCtx := store.WithSpaceAdmissionOwnership(context.Background(), current.Ownership)
	require.NoError(t, s.FenceAbortedAdmission(newCtx, a.OperationID, spacegatetest.New()))
	require.NoError(t, fixtureBeginOwnedLeave(newCtx, s, a.OperationID, a.Generation))
	require.NoError(t, fixtureCompleteOwnedLeave(newCtx, s, current))
	require.NoError(t, s.MarkCleanupCompleted(newCtx, a.OperationID, a.Generation))
}

func TestSpaceAdmissionHeadBeforePrepareUsesOwnershipNotAge(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	s := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	ctx = ownAdmission(ctx, &a)
	head, err := s.ClaimRoomHead(ctx, a.VoiceRoomID, a.SpaceID.String(), a.RoomID, a.OperationID, true)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE voice_space_media_room_heads SET created_at=clock_timestamp()-interval '1 day' WHERE creator_operation_id=$1`, a.OperationID)
	require.NoError(t, err)
	require.NoError(t, NewPostgresAdmissionStore(pool).RecoverOrphanRoomHeads(context.Background()))
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM voice_space_media_room_heads WHERE creator_operation_id=$1`, a.OperationID).Scan(&state))
	require.Equal(t, "OPEN", state, "an old timestamp never revokes a live owner")
	_, err = pool.Exec(ctx, `UPDATE voice_space_media_admission_owners SET lease_until=clock_timestamp()-interval '1 second' WHERE operation_id=$1`, a.OperationID)
	require.NoError(t, err)
	require.NoError(t, NewPostgresAdmissionStore(pool).RecoverOrphanRoomHeads(context.Background()))
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM voice_space_media_room_heads WHERE creator_operation_id=$1`, a.OperationID).Scan(&state))
	require.Equal(t, "ENDED", state)
	a.RoomGeneration = head.RoomGeneration
	a.LifecycleGeneration = head.LifecycleGeneration
	require.Error(t, s.Prepare(ctx, a), "a resumed expired creator cannot prepare after head cleanup")
}

func TestSpaceAdmissionRecoveryPagesPastHealthyOwners(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	s := NewPostgresAdmissionStore(pool)
	creator := admissionFixture()
	ctx = ownAdmission(ctx, &creator)
	require.NoError(t, prepareAdmission(ctx, s, &creator))
	require.NoError(t, s.Fence(ctx, creator))
	require.NoError(t, s.MarkCommitted(ctx, creator.OperationID, creator.Generation))
	require.NoError(t, s.MarkProjectionApplied(ctx, creator.OperationID, creator.Generation))
	var last Admission
	for range 258 {
		a := admissionFixture()
		ctx = ownAdmission(ctx, &a)
		a.RoomID, a.SpaceID, a.VoiceRoomID = creator.RoomID, creator.SpaceID, creator.VoiceRoomID
		a.CreatedRoom = false
		a.Events = a.Events[1:]
		require.NoError(t, prepareAdmission(ctx, s, &a))
		last = a
	}
	// The fixture controls DB deadlines, rather than sleeping or depending on
	// runner speed. Only the final admission is eligible, beyond the old page.
	_, err := pool.Exec(ctx, `UPDATE voice_space_media_admission_owners SET lease_until=clock_timestamp()+interval '1 minute'`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE voice_space_media_admission_owners SET lease_until=clock_timestamp()-interval '1 second' WHERE operation_id=$1`, last.OperationID)
	require.NoError(t, err)
	rows, err := NewPostgresAdmissionStore(pool).RecoveryRows(context.Background(), 256)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, last.OperationID, rows[0].OperationID, "healthy owners cannot occupy every page and starve an expired row")
}

func TestSpaceAdmissionRecoveryResumesAfterDrainBeforeCleanupMarker(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	s := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	ctx = ownAdmission(ctx, &a)
	require.NoError(t, prepareAdmission(ctx, s, &a))
	require.NoError(t, s.Fence(ctx, a))
	require.NoError(t, s.MarkCommitted(ctx, a.OperationID, a.Generation))
	calls, _ := newP3Redis(t)
	c := &Coordinator{ProjectionGates: calls, Admissions: s, Calls: calls, Store: calls, Media: &p3Media{}}
	ticket, err := c.prepareProjection(ctx, a)
	require.NoError(t, err)
	call, err := calls.CreateCall(ctx, store.Call{SpaceProjectionTicket: ticket, SpaceLifecycleGeneration: a.LifecycleGeneration, RoomID: a.RoomID, VoiceRoomID: a.VoiceRoomID, SpaceID: a.SpaceID.String(), LivekitRoomName: "voice-room-" + a.VoiceRoomID, SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, Status: callsv1.CallStatus_CALL_STATUS_UNSPECIFIED, States: map[string]store.ParticipantState{}})
	require.NoError(t, err)
	call, err = calls.AdmitSpaceMediaParticipant(ctx, call.RoomID, store.SpaceMediaParticipant{ProjectionTicket: ticket, LifecycleGeneration: a.LifecycleGeneration, RoomGeneration: a.RoomGeneration, CreatedRoom: true, AccountID: a.AccountID.String(), ProfileID: a.ProfileID.String(), AdmissionOperationID: a.OperationID.String(), Identity: a.Identity, Generation: a.Generation, Issued: store.SpaceMediaGrant{SessionEpoch: a.SessionEpoch, AccessEpoch: a.AccessEpoch, PolicyEpoch: a.PolicyEpoch, CanJoin: true, CanSubscribe: true}}, a.MaxParticipants)
	require.NoError(t, err)
	require.NoError(t, s.MarkAborting(ctx, a.OperationID, a.Generation))
	require.NoError(t, s.FenceAbortedAdmission(ctx, a.OperationID, calls))
	_, completed, err := c.ejectParticipant(ctx, call, call.SpaceMedia[a.ProfileID.String()])
	require.NoError(t, err)
	require.True(t, completed)
	var ended, pending bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT participant_state='ENDED',cleanup_completed_at IS NULL FROM voice_space_media_admissions WHERE operation_id=$1`, a.OperationID).Scan(&ended, &pending))
	require.True(t, ended)
	require.True(t, pending)
	require.NoError(t, s.RelinquishAdmissionOwnership(ctx, a.OperationID))
	current, claimed, err := NewPostgresAdmissionStore(pool).ClaimAdmissionRecovery(context.Background(), a)
	require.NoError(t, err)
	require.True(t, claimed, "ENDED projection is not a completed ABORTING cleanup")
	newCtx := store.WithSpaceAdmissionOwnership(context.Background(), current.Ownership)
	require.NoError(t, c.abortAdmission(newCtx, current))
	require.NoError(t, pool.QueryRow(ctx, `SELECT cleanup_completed_at IS NULL FROM voice_space_media_admissions WHERE operation_id=$1`, a.OperationID).Scan(&pending))
	require.False(t, pending)
	item, err := s.ClaimOutbox(ctx, ReconcileInterval)
	require.NoError(t, err)
	require.Nil(t, item, "an aborted unconfirmed admission never publishes its frozen intents")
}
