package roomlifecycle

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// This file freezes RED-E from R22.2-VOICE-DB-PLAN.md. It deliberately models
// process restart by constructing a new pool and store over the same database.
// It contains no production failpoint or external-system adapter.

func r22RestartLifecycleStore(t *testing.T, fixture r22StoreFixture) *PostgresLifecycleStore {
	t.Helper()
	pool, err := pgxpool.New(fixture.ctx, fixture.pool.Config().ConnString())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(fixture.ctx))
	return NewPostgresLifecycleStore(pool)
}

func r22InstallTerminalCommitFailure(t *testing.T, fixture r22StoreFixture) func() {
	t.Helper()
	_, err := fixture.pool.Exec(fixture.ctx, `
CREATE FUNCTION r22_fail_terminal_operation_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION USING ERRCODE='P0001', MESSAGE='r22 forced failure after terminal operation update'; END $$;
CREATE TRIGGER r22_fail_terminal_operation_update_trigger
AFTER UPDATE ON voice_lifecycle_operations
FOR EACH ROW WHEN (OLD.state='decided' AND NEW.state='completed')
EXECUTE FUNCTION r22_fail_terminal_operation_update()`)
	require.NoError(t, err)
	return func() {
		_, dropErr := fixture.pool.Exec(fixture.ctx, `
DROP TRIGGER r22_fail_terminal_operation_update_trigger ON voice_lifecycle_operations;
DROP FUNCTION r22_fail_terminal_operation_update()`)
		require.NoError(t, dropErr)
	}
}

func r22CrashPrepareJoinCompletion(t *testing.T, fixture r22StoreFixture) (LifecycleDecision, LifecycleOperation, LifecycleCompletion) {
	t.Helper()
	decision := fixture.decision(LifecycleMethodJoin)
	operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	r22ApplyAllEffects(t, fixture, decision)
	claim := r22ClaimOperation(t, fixture)
	receipt := r22JoinReceipt(fixture, decision, 1)
	completion := LifecycleCompletion{
		ActorProfileID: decision.ActorProfileID,
		OperationID:    decision.OperationID,
		WorkerID:       fixture.workerID,
		ExpectedFence:  claim.Fence,
		Receipt:        receipt,
		Outbox: []LifecycleOutboxRecord{
			r22TestOutbox(fixture, operation, 0, "joined", 1),
		},
		CompletedAt: r22StoreTime.Add(2 * time.Minute),
		ReplayUntil: r22StoreTime.Add(26 * time.Hour),
	}
	return decision, operation, completion
}

func TestPostgresLifecycleCrashHarness_E01_DecisionFailureRollsBackAndSameKeyRetries(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL crash harness requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22crashe01")
	grantExpiry := r22StoreTime.Add(time.Hour)
	fixture.seedMembership(t, &grantExpiry)
	decision := fixture.decision(LifecycleMethodSelfMove)
	before := r22AllTableSnapshots(t, fixture)
	drop := r22InstallFailureAfterInsert(t, fixture, "voice_media_epoch_denials")

	_, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.Error(t, err)
	require.Equal(t, before, r22AllTableSnapshots(t, fixture),
		"failure after the final decision insert must roll back room, operation, effects, and denial")

	drop()
	restarted := r22RestartLifecycleStore(t, fixture)
	decided, err := restarted.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err, "the same operation UUID must remain fresh after rollback")
	r22RequireOperationDecision(t, decision, decided)
	require.Len(t, r22AllTableSnapshots(t, fixture)["voice_lifecycle_effects"], 2)
	require.Len(t, r22AllTableSnapshots(t, fixture)["voice_media_epoch_denials"], 1)
}

func TestPostgresLifecycleCrashHarness_E02_E03_DecisionSurvivesRestartWithExactPlanAndFence(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL crash harness requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22crashe02")
	grantExpiry := r22StoreTime.Add(time.Hour)
	fixture.seedMembership(t, &grantExpiry)
	decision := fixture.decision(LifecycleMethodSelfMove)
	want, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	wantEffects, err := fixture.store.ListOperationEffects(fixture.ctx, decision.ActorProfileID, decision.OperationID)
	require.NoError(t, err)
	wantRows := r22AllTableSnapshots(t, fixture)

	restarted := r22RestartLifecycleStore(t, fixture)
	got, found, err := restarted.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
	require.NoError(t, err)
	require.True(t, found)
	r22RequireSameImmutableOperation(t, want, got)
	require.Equal(t, decision.RedisOwnerToken, got.RedisOwnerToken)
	gotEffects, err := restarted.ListOperationEffects(fixture.ctx, decision.ActorProfileID, decision.OperationID)
	require.NoError(t, err)
	require.Equal(t, wantEffects, gotEffects)
	_, err = restarted.AdvanceLatestGrantExpiry(
		fixture.ctx, fixture.subjectProfileID, fixture.sourceMediaEpoch, r22StoreTime.Add(2*time.Hour))
	require.ErrorIs(t, err, ErrSubjectTransitionActive)
	require.Equal(t, wantRows, r22AllTableSnapshots(t, fixture),
		"E03 stores no Redis-pending phase; its database cut is exactly E02")
}

func TestPostgresLifecycleCrashHarness_E04_EnsureRoomObservationReplaysExactTargetAfterRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL crash harness requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22crashe04")
	decision := fixture.decision(LifecycleMethodJoin)
	_, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	first, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, LifecycleEffectEnsureRoom, first.Effect.Kind)

	// The fake EnsureRoom succeeded externally, but the process stopped before
	// MarkEffectApplied. PostgreSQL therefore contains only the original claim.
	restarted := r22RestartLifecycleStore(t, fixture)
	worker := uuid.New()
	replayed, ok, err := restarted.ClaimNextEffect(fixture.ctx, worker, first.LeaseUntil, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.Greater(t, replayed.Fence, first.Fence)
	require.True(t, sameEffectObservation(first.Effect, replayed.Effect),
		"recovery may change only lease state, never the deterministic EnsureRoom request")
	require.NoError(t, restarted.MarkEffectApplied(fixture.ctx, LifecycleEffectApplied{
		EffectID: replayed.Effect.EffectID, WorkerID: worker, ExpectedFence: replayed.Fence,
		Observed: replayed.Effect, AppliedAt: first.LeaseUntil.Add(time.Minute),
	}))
}

func TestPostgresLifecycleCrashHarness_E05_EjectObservationReplaysOnlyOldGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL crash harness requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22crashe05")
	grantExpiry := r22StoreTime.Add(time.Hour)
	fixture.seedMembership(t, &grantExpiry)
	decision := fixture.decision(LifecycleMethodLeave)
	_, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	first, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, LifecycleEffectEjectParticipant, first.Effect.Kind)
	require.Equal(t, &fixture.sourceMediaEpoch, first.Effect.MediaEpoch)
	wantIdentity := "profile:" + fixture.subjectProfileID.String() + ":media:" + fixture.sourceMediaEpoch.String()
	require.Equal(t, &wantIdentity, first.Effect.ParticipantIdentity)

	restarted := r22RestartLifecycleStore(t, fixture)
	worker := uuid.New()
	replayed, ok, err := restarted.ClaimNextEffect(fixture.ctx, worker, first.LeaseUntil, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, sameEffectObservation(first.Effect, replayed.Effect))
	require.Equal(t, &fixture.sourceMediaEpoch, replayed.Effect.MediaEpoch)
	require.Equal(t, &wantIdentity, replayed.Effect.ParticipantIdentity)
	require.NoError(t, restarted.MarkEffectApplied(fixture.ctx, LifecycleEffectApplied{
		EffectID: replayed.Effect.EffectID, WorkerID: worker, ExpectedFence: replayed.Fence,
		Observed: replayed.Effect, AppliedAt: first.LeaseUntil.Add(time.Minute),
	}))
}

func TestPostgresLifecycleCrashHarness_E06_ExpiredOwnerCannotPersistAfterRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL crash harness requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22crashe06")
	decision := fixture.decision(LifecycleMethodJoin)
	_, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	oldClaim := r22ClaimOperation(t, fixture)

	restarted := r22RestartLifecycleStore(t, fixture)
	newWorker := uuid.New()
	newClaim, ok, err := restarted.ClaimNextOperation(fixture.ctx, newWorker, oldClaim.LeaseUntil, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.Greater(t, newClaim.Fence, oldClaim.Fence)
	before := r22AllTableSnapshots(t, fixture)
	require.ErrorIs(t, fixture.store.RenewOperationLease(fixture.ctx, LifecycleOperationLease{
		ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
		WorkerID: fixture.workerID, ExpectedFence: oldClaim.Fence, LeaseUntil: oldClaim.LeaseUntil.Add(2 * time.Minute),
	}), ErrStaleLease)
	require.ErrorIs(t, fixture.store.QuarantineOperation(fixture.ctx, LifecycleOperationQuarantine{
		ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
		WorkerID: fixture.workerID, ExpectedFence: oldClaim.Fence, Class: "late_worker", At: oldClaim.LeaseUntil,
	}), ErrStaleLease)
	_, err = fixture.store.CompleteOperation(fixture.ctx, LifecycleCompletion{
		ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
		WorkerID: fixture.workerID, ExpectedFence: oldClaim.Fence, Receipt: r22JoinReceipt(fixture, decision, 1),
		CompletedAt: oldClaim.LeaseUntil.Add(time.Minute), ReplayUntil: oldClaim.LeaseUntil.Add(25 * time.Hour),
	})
	require.ErrorIs(t, err, ErrStaleLease)
	require.Equal(t, before, r22AllTableSnapshots(t, fixture))
}

func TestPostgresLifecycleCrashHarness_E07_AppliedEffectsRecoverToExactTerminalMutation(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL crash harness requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22crashe07")
	decision, operation, completion := r22CrashPrepareJoinCompletion(t, fixture)
	before := r22AllTableSnapshots(t, fixture)
	_, found, err := fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
	require.NoError(t, err)
	require.False(t, found)
	room, found, err := fixture.store.LoadRoomSnapshot(fixture.ctx, *operation.DestinationRoomID)
	require.NoError(t, err)
	require.True(t, found)
	require.Zero(t, room.RosterVersion)

	restarted := r22RestartLifecycleStore(t, fixture)
	eligibility, err := restarted.CheckGrantEligibility(fixture.ctx, fixture.subjectProfileID, *operation.DestinationMediaEpoch)
	require.NoError(t, err)
	require.False(t, eligibility.Eligible)
	require.Equal(t, before, r22AllTableSnapshots(t, fixture))
	completed, err := restarted.CompleteOperation(fixture.ctx, completion)
	require.NoError(t, err)
	r22RequireCompletedReceipt(t, fixture, completed, completion.Receipt)
	r22RequireTerminalMembership(t, fixture, fixture.subjectProfileID, *operation.DestinationRoomID,
		*operation.DestinationMediaEpoch, decision.Authority, completion.CompletedAt)
}

func TestPostgresLifecycleCrashHarness_E08_TerminalTriggerAbortsWholeTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL crash harness requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22crashe08")
	_, _, completion := r22CrashPrepareJoinCompletion(t, fixture)
	before := r22AllTableSnapshots(t, fixture)
	drop := r22InstallTerminalCommitFailure(t, fixture)

	_, err := fixture.store.CompleteOperation(fixture.ctx, completion)
	require.Error(t, err)
	require.Equal(t, before, r22AllTableSnapshots(t, fixture),
		"failure after membership, roster, outbox, and terminal UPDATE must roll the transaction back")

	drop()
	restarted := r22RestartLifecycleStore(t, fixture)
	completed, err := restarted.CompleteOperation(fixture.ctx, completion)
	require.NoError(t, err, "removing the test-owned trigger is the positive control")
	require.Equal(t, LifecycleOperationCompleted, completed.State)
}

func TestPostgresLifecycleCrashHarness_E09_E10_E14_E18_TerminalStateReplaysWithoutMutation(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL crash harness requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22crashe09")
	decision, operation, completion := r22CrashPrepareJoinCompletion(t, fixture)
	completed, err := fixture.store.CompleteOperation(fixture.ctx, completion)
	require.NoError(t, err)
	want := r22AllTableSnapshots(t, fixture)

	restarted := r22RestartLifecycleStore(t, fixture)
	reloaded, found, err := restarted.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, completed.ReceiptBytes, reloaded.ReceiptBytes)
	require.Equal(t, completed.ReceiptHash, reloaded.ReceiptHash)
	r22RequireSameImmutableOperation(t, completed, reloaded)
	replayed, err := restarted.CompleteOperation(fixture.ctx, completion)
	require.NoError(t, err)
	require.Equal(t, completed.ReceiptBytes, replayed.ReceiptBytes)
	require.Equal(t, want, r22AllTableSnapshots(t, fixture),
		"terminal replay after restart must not repeat membership, effects, roster, or outbox writes")
	require.Len(t, want["voice_room_memberships"], 1)
	require.Len(t, want["voice_event_outbox"], 1)

	// E18 storage evidence: the completed operation and membership retain the
	// authority component epochs and the exact media generation. The authority
	// invalidation consumer itself belongs to R22.5.
	membership, found, err := restarted.LoadMembership(fixture.ctx, fixture.subjectProfileID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, decision.Authority.SpaceAccessEpoch, reloaded.Authority.SpaceAccessEpoch)
	require.Equal(t, decision.Authority.SubjectRolePolicyEpoch, reloaded.Authority.SubjectRolePolicyEpoch)
	require.Equal(t, operation.DestinationMediaEpoch, &membership.MediaEpoch)
}

func TestPostgresLifecycleCrashHarness_E11_AcknowledgedButUnmarkedOutboxReclaimsStableEnvelope(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL crash harness requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22crashe11")
	_, _, completion := r22CrashPrepareJoinCompletion(t, fixture)
	_, err := fixture.store.CompleteOperation(fixture.ctx, completion)
	require.NoError(t, err)
	firstWorker := uuid.New()
	first, ok, err := fixture.store.ClaimNextOutbox(fixture.ctx, firstWorker, r22StoreTime.Add(3*time.Minute), time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	// Conceptual NATS ack occurred here; R22.2 intentionally writes nothing.
	restarted := r22RestartLifecycleStore(t, fixture)
	secondWorker := uuid.New()
	replayed, ok, err := restarted.ClaimNextOutbox(fixture.ctx, secondWorker, first.LeaseUntil, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.Greater(t, replayed.Fence, first.Fence)
	require.Equal(t, first.Outbox.EventID, replayed.Outbox.EventID)
	require.Equal(t, first.Outbox.Subject, replayed.Outbox.Subject)
	require.Equal(t, first.Outbox.SchemaVersion, replayed.Outbox.SchemaVersion)
	require.Equal(t, first.Outbox.PayloadBytes, replayed.Outbox.PayloadBytes)
	require.Equal(t, first.Outbox.PayloadHash, replayed.Outbox.PayloadHash)
	require.NoError(t, restarted.MarkOutboxDelivered(fixture.ctx, LifecycleOutboxDelivered{
		EventID: replayed.Outbox.EventID, WorkerID: secondWorker, ExpectedFence: replayed.Fence,
		DeliveredAt: first.LeaseUntil.Add(time.Minute),
	}))
}

func TestPostgresLifecycleCrashHarness_E12_E13_DenialAndEligibilitySurviveRestartBoundary(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL crash harness requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22crashe12")
	grantExpiry := r22StoreTime.Add(time.Hour)
	fixture.seedMembership(t, &grantExpiry)
	decision := fixture.decision(LifecycleMethodLeave)
	_, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	effect, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, LifecycleEffectEjectParticipant, effect.Effect.Kind)
	require.NoError(t, fixture.store.MarkEffectApplied(fixture.ctx, LifecycleEffectApplied{
		EffectID: effect.Effect.EffectID, WorkerID: fixture.workerID, ExpectedFence: effect.Fence,
		Observed: effect.Effect, AppliedAt: r22StoreTime.Add(time.Minute),
	}))

	restarted := r22RestartLifecycleStore(t, fixture)
	denial, found, err := restarted.LookupMediaEpochDenial(fixture.ctx, fixture.sourceMediaEpoch)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, fixture.sourceMediaEpoch, denial.MediaEpoch)
	require.Equal(t, grantExpiry, denial.GrantExpiresAt)
	require.Equal(t, grantExpiry.Add(decision.AcceptedClockSkew), denial.DenyUntil)
	wantIdentity := "profile:" + fixture.subjectProfileID.String() + ":media:" + fixture.sourceMediaEpoch.String()
	require.Equal(t, wantIdentity, denial.ParticipantIdentity)
	_, found, err = restarted.LookupMediaEpochDenial(fixture.ctx, uuid.New())
	require.NoError(t, err)
	require.False(t, found, "a denial must match only the old media epoch")

	deleted, err := restarted.DeleteExpiredObservedDenials(fixture.ctx, denial.DenyUntil, 10)
	require.NoError(t, err)
	require.Zero(t, deleted, "the exact expiry boundary cannot delete without an absence observation")
	_, found, err = restarted.LookupMediaEpochDenial(fixture.ctx, fixture.sourceMediaEpoch)
	require.NoError(t, err)
	require.True(t, found)
	eligibility, err := restarted.CheckGrantEligibility(fixture.ctx, fixture.subjectProfileID, fixture.sourceMediaEpoch)
	require.NoError(t, err)
	require.False(t, eligibility.Eligible,
		"a lost or delayed webhook cannot make the nonterminal old generation eligible")
}

func TestPostgresLifecycleCrashHarness_E15_ClosedPostgresFailsEveryDurableAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL crash harness requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22crashe15")
	pool, err := pgxpool.New(fixture.ctx, fixture.pool.Config().ConnString())
	require.NoError(t, err)
	require.NoError(t, pool.Ping(fixture.ctx), "healthy store is the positive control")
	store := NewPostgresLifecycleStore(pool)
	require.NoError(t, store.CheckSchema(fixture.ctx))
	pool.Close()

	decision := fixture.decision(LifecycleMethodJoin)
	leave := fixture.decision(LifecycleMethodLeave)
	roomID, mediaEpoch := uuid.New(), uuid.New()
	roster, spaceEpoch, roleEpoch := int64(1), int64(41), int64(42)
	digest := r22StoreDigest(0x20)
	receipt := LifecycleReceipt{
		OperationID: decision.OperationID, ActorProfileID: decision.ActorProfileID,
		SubjectProfileID: decision.SubjectProfileID, SpaceID: decision.SpaceID,
		Method: LifecycleMethodJoin, Outcome: LifecycleOutcomeJoined,
		DestinationVoiceRoomID: decision.DestinationVoiceRoomID, RoomID: &roomID,
		DestinationRosterVersion: &roster, MediaEpoch: &mediaEpoch,
		SpaceAccessEpoch: &spaceEpoch, RolePolicyEpoch: &roleEpoch, AuthorizationDigest: &digest,
	}
	now := r22StoreTime
	workerID, effectID, eventID := uuid.New(), uuid.New(), uuid.New()
	tests := []struct {
		name string
		call func() error
	}{
		{"CheckSchema", func() error { return store.CheckSchema(fixture.ctx) }},
		{"LoadOperation", func() error {
			_, _, err := store.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
			return err
		}},
		{"DecideOperation", func() error { _, err := store.DecideOperation(fixture.ctx, decision); return err }},
		{"CompleteNoOp", func() error {
			_, err := store.CompleteNoOp(fixture.ctx, LifecycleNoOpDecision{Decision: leave, CompletedAt: now, ReplayUntil: now.Add(24 * time.Hour)})
			return err
		}},
		{"ClaimNextOperation", func() error {
			_, _, err := store.ClaimNextOperation(fixture.ctx, workerID, now, time.Minute)
			return err
		}},
		{"RenewOperationLease", func() error {
			return store.RenewOperationLease(fixture.ctx, LifecycleOperationLease{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID, WorkerID: workerID, ExpectedFence: 1, LeaseUntil: now.Add(time.Minute)})
		}},
		{"QuarantineOperation", func() error {
			return store.QuarantineOperation(fixture.ctx, LifecycleOperationQuarantine{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID, WorkerID: workerID, ExpectedFence: 1, Class: "closed", At: now})
		}},
		{"ListOperationEffects", func() error {
			_, err := store.ListOperationEffects(fixture.ctx, decision.ActorProfileID, decision.OperationID)
			return err
		}},
		{"ClaimNextEffect", func() error { _, _, err := store.ClaimNextEffect(fixture.ctx, workerID, now, time.Minute); return err }},
		{"RenewEffectLease", func() error {
			return store.RenewEffectLease(fixture.ctx, LifecycleEffectLease{EffectID: effectID, WorkerID: workerID, ExpectedFence: 1, LeaseUntil: now.Add(time.Minute)})
		}},
		{"MarkEffectApplied", func() error {
			return store.MarkEffectApplied(fixture.ctx, LifecycleEffectApplied{EffectID: effectID, WorkerID: workerID, ExpectedFence: 1, AppliedAt: now})
		}},
		{"MarkEffectRetry", func() error {
			return store.MarkEffectRetry(fixture.ctx, LifecycleEffectRetry{EffectID: effectID, Retry: LifecycleRetry{WorkerID: workerID, ExpectedFence: 1, ErrorClass: "closed", ErrorAt: now, NextAttemptAt: now.Add(time.Minute)}})
		}},
		{"QuarantineEffect", func() error {
			return store.QuarantineEffect(fixture.ctx, LifecycleEffectQuarantine{EffectID: effectID, WorkerID: workerID, ExpectedFence: 1, Class: "closed", At: now})
		}},
		{"CompleteOperation", func() error {
			_, err := store.CompleteOperation(fixture.ctx, LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID, WorkerID: workerID, ExpectedFence: 1, Receipt: receipt, CompletedAt: now, ReplayUntil: now.Add(24 * time.Hour)})
			return err
		}},
		{"LoadMembership", func() error { _, _, err := store.LoadMembership(fixture.ctx, decision.SubjectProfileID); return err }},
		{"LoadRoomSnapshot", func() error { _, _, err := store.LoadRoomSnapshot(fixture.ctx, roomID); return err }},
		{"CheckGrantEligibility", func() error {
			_, err := store.CheckGrantEligibility(fixture.ctx, decision.SubjectProfileID, mediaEpoch)
			return err
		}},
		{"AdvanceLatestGrantExpiry", func() error {
			_, err := store.AdvanceLatestGrantExpiry(fixture.ctx, decision.SubjectProfileID, mediaEpoch, now.Add(time.Hour))
			return err
		}},
		{"LookupMediaEpochDenial", func() error { _, _, err := store.LookupMediaEpochDenial(fixture.ctx, mediaEpoch); return err }},
		{"MarkDenialAbsenceObserved", func() error { return store.MarkDenialAbsenceObserved(fixture.ctx, mediaEpoch, now) }},
		{"DeleteExpiredObservedDenials", func() error { _, err := store.DeleteExpiredObservedDenials(fixture.ctx, now, 1); return err }},
		{"ClaimNextOutbox", func() error { _, _, err := store.ClaimNextOutbox(fixture.ctx, workerID, now, time.Minute); return err }},
		{"RenewOutboxLease", func() error {
			return store.RenewOutboxLease(fixture.ctx, LifecycleOutboxLease{EventID: eventID, WorkerID: workerID, ExpectedFence: 1, LeaseUntil: now.Add(time.Minute)})
		}},
		{"MarkOutboxDelivered", func() error {
			return store.MarkOutboxDelivered(fixture.ctx, LifecycleOutboxDelivered{EventID: eventID, WorkerID: workerID, ExpectedFence: 1, DeliveredAt: now})
		}},
		{"MarkOutboxRetry", func() error {
			return store.MarkOutboxRetry(fixture.ctx, LifecycleOutboxRetry{EventID: eventID, Retry: LifecycleRetry{WorkerID: workerID, ExpectedFence: 1, ErrorClass: "closed", ErrorAt: now, NextAttemptAt: now.Add(time.Minute)}})
		}},
		{"QuarantineOutbox", func() error {
			return store.QuarantineOutbox(fixture.ctx, LifecycleOutboxQuarantine{EventID: eventID, WorkerID: workerID, ExpectedFence: 1, Class: "closed", At: now})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.ErrorIs(t, test.call(), ErrUnavailable)
		})
	}
}

func TestPostgresLifecycleCrashHarness_E16_QuarantineFenceSurvivesRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL crash harness requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22crashe16")
	decision := fixture.decision(LifecycleMethodJoin)
	_, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	claim := r22ClaimOperation(t, fixture)
	quarantinedAt := r22StoreTime.Add(time.Minute)
	require.NoError(t, fixture.store.QuarantineOperation(fixture.ctx, LifecycleOperationQuarantine{
		ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
		WorkerID: fixture.workerID, ExpectedFence: claim.Fence,
		Class: "redis_divergence", Detail: "later comparator fixture", At: quarantinedAt,
	}))

	restarted := r22RestartLifecycleStore(t, fixture)
	operation, found, err := restarted.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, LifecycleOperationQuarantined, operation.State)
	require.Equal(t, claim.Fence, operation.LeaseFence)
	require.Nil(t, operation.LeaseOwner)
	require.Nil(t, operation.LeaseUntil)
	require.Equal(t, "redis_divergence", *operation.QuarantineClass)
	require.Equal(t, "later comparator fixture", *operation.QuarantineDetail)
	require.Equal(t, quarantinedAt, *operation.QuarantinedAt)
	other := fixture.decision(LifecycleMethodJoin)
	other.OperationID = uuid.New()
	_, err = restarted.DecideOperation(fixture.ctx, other)
	require.ErrorIs(t, err, ErrSubjectTransitionActive)
}

func TestPostgresLifecycleCrashHarness_LaterSliceSeams(t *testing.T) {
	seams := []struct {
		id    string
		owner string
	}{
		{"E03_RedisPendingHook", "R22.3; E02 already proves the identical PostgreSQL cut"},
		{"E10_ResponseReplay", "R22.3/R22.4; E09 already proves immutable terminal PostgreSQL replay"},
		{"E14_RedisLossFallback", "R22.3; this slice intentionally has no Redis fallback"},
		{"E16_PGRedisDivergenceComparison", "R22.3; the separate E16 test proves only durable quarantine/fence state"},
		{"E17_AuthorityBeforeStoreSequencing", "R22.4; this source-disabled store has no coordinator call site"},
		{"E18_AuthorityInvalidationConsumer", "R22.5; E09/E18 proves only retained epochs and media identity"},
	}
	for _, seam := range seams {
		t.Run(seam.id, func(t *testing.T) {
			t.Skip("later-slice seam: " + seam.owner)
		})
	}
}

var _ LifecycleStore = (*PostgresLifecycleStore)(nil)
