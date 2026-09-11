package store

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	rolev1 "voice.app/voice/role/v1"
)

type ownershipReadyOutboxReader interface {
	ReadReadyOwnershipOutbox(context.Context, int, func(eventID, operationID, spaceID, previousOwnerProfileID, newOwnerProfileID uuid.UUID, eventType string, createdAt time.Time) error) error
}

type readyOwnershipOutboxRow struct {
	EventID                uuid.UUID
	OperationID            uuid.UUID
	SpaceID                uuid.UUID
	PreviousOwnerProfileID uuid.UUID
	NewOwnerProfileID      uuid.UUID
	EventType              string
	CreatedAt              time.Time
}

func invokeReadyOwnershipOutbox(st *SpaceStore, ctx context.Context, limit int) ([]readyOwnershipOutboxRow, error) {
	reader, ok := any(st).(ownershipReadyOutboxReader)
	if !ok {
		return nil, errors.New("ReadReadyOwnershipOutbox typed API is missing")
	}
	rows := []readyOwnershipOutboxRow{}
	err := reader.ReadReadyOwnershipOutbox(ctx, limit, func(eventID, operationID, spaceID, previousOwnerProfileID, newOwnerProfileID uuid.UUID, eventType string, createdAt time.Time) error {
		rows = append(rows, readyOwnershipOutboxRow{
			EventID: eventID, OperationID: operationID, SpaceID: spaceID,
			PreviousOwnerProfileID: previousOwnerProfileID, NewOwnerProfileID: newOwnerProfileID,
			EventType: eventType, CreatedAt: createdAt,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func TestOwnershipOutbox_CommitDecisionIsInvisibleUntilMatchingTerminalReceipt(t *testing.T) {
	st := ownershipJournalCompletionStoreFixture(t)
	binding, prepared, decided := seedOwnershipCommitDecided(t, st)
	rows, err := invokeReadyOwnershipOutbox(st, context.Background(), 10)
	require.NoError(t, err)
	require.Empty(t, rows)
	var audits int
	var ready bool
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM audit_log WHERE id=$1),ready FROM ownership_outbox WHERE operation_id=$2`, decided.AuditID, binding.OperationID).Scan(&audits, &ready))
	require.Zero(t, audits)
	require.False(t, ready)

	_, err = invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipCommit", binding, ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, nil))
	require.NoError(t, err)
	rows, err = invokeReadyOwnershipOutbox(st, context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, decided.EventID, rows[0].EventID)
	require.Equal(t, binding.OperationID, rows[0].OperationID)
	require.Equal(t, binding.SpaceID, rows[0].SpaceID)
	require.Equal(t, binding.ActorProfileID, rows[0].PreviousOwnerProfileID)
	require.Equal(t, binding.NewOwnerProfileID, rows[0].NewOwnerProfileID)
	require.Equal(t, "space.updated", rows[0].EventType)
	require.False(t, rows[0].CreatedAt.IsZero())
}

func TestOwnershipOutbox_CompletionPublishesAuditReadyAndTerminalStateInOneSnapshot(t *testing.T) {
	st := ownershipJournalCompletionStoreFixture(t)
	binding, prepared, journal := seedOwnershipCommitDecided(t, st)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	const gate int64 = 72020012
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, gate)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION r20_pause_terminal() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.state='completed' THEN PERFORM pg_advisory_xact_lock(72020012); END IF; RETURN NEW; END $$; CREATE TRIGGER r20_pause_terminal BEFORE UPDATE ON ownership_journal FOR EACH ROW EXECUTE FUNCTION r20_pause_terminal()`)
	require.NoError(t, err)

	workerPool := r22SecondSpacePool(t, ctx, st.Pool, "r20_atomic_terminal")
	result := make(chan ownershipDecisionResult, 1)
	go func() {
		completed, callErr := invokeOwnershipTerminal(&SpaceStore{Pool: workerPool}, ctx, "CompleteOwnershipCommit", binding, ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, nil))
		result <- ownershipDecisionResult{journal: completed, err: callErr}
	}()
	r22WaitForSpaceLock(t, ctx, st.Pool, "r20_atomic_terminal")

	readTuple := func() (string, bool, bool) {
		var state string
		var audit, ready bool
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT j.state,
			EXISTS(SELECT 1 FROM audit_log a WHERE a.id=j.audit_id),
			EXISTS(SELECT 1 FROM ownership_outbox o WHERE o.event_id=j.event_id AND o.ready)
			FROM ownership_journal j WHERE j.operation_id=$1`, binding.OperationID).Scan(&state, &audit, &ready))
		return state, audit, ready
	}
	state, audit, ready := readTuple()
	require.Equal(t, []any{"commit_decided", false, false}, []any{state, audit, ready})
	require.NoError(t, blocker.Commit(ctx))
	completed := waitOwnershipDecision(t, result)
	require.NoError(t, completed.err)
	state, audit, ready = readTuple()
	require.Equal(t, []any{"completed", true, true}, []any{state, audit, ready})
	audits, readyCount, outbox := ownershipTerminalEffectCounts(t, st, journal)
	require.Equal(t, []int{1, 1, 1}, []int{audits, readyCount, outbox})
}

func TestOwnershipOutbox_AbortHasOnlyAtomicTerminalEvidenceAndNoSuccessEffect(t *testing.T) {
	st := ownershipJournalCompletionStoreFixture(t)
	binding, prepared, journal := seedOwnershipAbortDecided(t, st, true)
	_, err := invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipAbort", binding, ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED, nil))
	require.NoError(t, err)
	var state string
	var audit, ready bool
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT j.state,
		EXISTS(SELECT 1 FROM audit_log a WHERE a.id=j.audit_id),
		EXISTS(SELECT 1 FROM ownership_outbox o WHERE o.event_id=j.event_id AND o.ready)
		FROM ownership_journal j WHERE j.operation_id=$1`, binding.OperationID).Scan(&state, &audit, &ready))
	require.Equal(t, []any{"aborted", false, false}, []any{state, audit, ready})
	rows, err := invokeReadyOwnershipOutbox(st, context.Background(), 10)
	require.NoError(t, err)
	require.Empty(t, rows)
	audits, readyCount, outbox := ownershipTerminalEffectCounts(t, st, journal)
	require.Equal(t, []int{0, 0, 0}, []int{audits, readyCount, outbox})
}

func TestOwnershipOutbox_ReadyRowsHaveStableIdentityDeterministicOrderAndLimit(t *testing.T) {
	st := ownershipJournalCompletionStoreFixture(t)
	type expected struct {
		eventID, operationID uuid.UUID
		createdAt            time.Time
	}
	want := make([]expected, 0, 3)
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for index := 0; index < 3; index++ {
		binding, prepared, journal := seedOwnershipCommitDecided(t, st)
		_, err := st.Pool.Exec(context.Background(), `UPDATE ownership_outbox SET created_at=$2 WHERE operation_id=$1`, binding.OperationID, created)
		require.NoError(t, err)
		_, err = invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipCommit", binding, ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, nil))
		require.NoError(t, err)
		want = append(want, expected{journal.EventID, binding.OperationID, created})
	}
	slices.SortFunc(want, func(left, right expected) int {
		return bytes.Compare(left.eventID[:], right.eventID[:])
	})
	first, err := invokeReadyOwnershipOutbox(st, context.Background(), 2)
	require.NoError(t, err)
	second, err := invokeReadyOwnershipOutbox(st, context.Background(), 2)
	require.NoError(t, err)
	require.Len(t, first, 2)
	require.Len(t, second, 2)
	for index := range first {
		require.Equal(t, want[index].eventID, first[index].EventID)
		require.Equal(t, want[index].operationID, first[index].OperationID)
		require.Equal(t, want[index].createdAt, first[index].CreatedAt)
		require.Equal(t, first[index], second[index])
	}
}

func TestOwnershipOutbox_InvalidLimitAndDatabaseOutageFailClosed(t *testing.T) {
	st := ownershipJournalCompletionStoreFixture(t)
	for _, limit := range []int{-1, 0, 101} {
		rows, err := invokeReadyOwnershipOutbox(st, context.Background(), limit)
		require.Error(t, err)
		require.Nil(t, rows)
	}
	st.Pool.Close()
	rows, err := invokeReadyOwnershipOutbox(st, context.Background(), 10)
	require.Error(t, err)
	require.Nil(t, rows)
}
