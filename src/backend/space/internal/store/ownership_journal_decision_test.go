package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func ownershipAuthReceiptFixture(binding OwnershipBinding) OwnershipAuthReceipt {
	return OwnershipAuthReceipt{
		ReceiptID:         uuid.MustParse("66666666-6666-6666-6666-666666666666"),
		AccountID:         binding.AccountID,
		ProfileID:         binding.ActorProfileID,
		SpaceID:           binding.SpaceID,
		NewOwnerProfileID: binding.NewOwnerProfileID,
		OperationID:       binding.OperationID,
		SessionEpoch:      binding.SessionEpoch,
		ConsumedAt:        time.Date(2026, time.September, 10, 18, 19, 20, 123456000, time.UTC),
		VerifiedFactors:   []string{"password", "totp", "future_auth_factor"},
	}
}

func assertOwnershipAuthReceipt(t *testing.T, got *OwnershipJournal, want OwnershipAuthReceipt) {
	t.Helper()
	require.NotNil(t, got)
	require.NotNil(t, got.AuthReceipt)
	require.Equal(t, want, *got.AuthReceipt)
	require.Equal(t, time.UTC, got.AuthReceipt.ConsumedAt.Location())
}

func assertOwnershipJournalHasNoSuccessEffects(t *testing.T, st *SpaceStore, binding OwnershipBinding) {
	t.Helper()
	assertReservationHasNoPublicEffects(t, st, binding)
	var outboxExists bool
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT to_regclass('ownership_outbox') IS NOT NULL`).Scan(&outboxExists))
	require.False(t, outboxExists, "cycle2 must not introduce or write the ownership outbox")
}

func TestOwnershipJournalDecision_ConfirmPersistsExactAuthReceiptAcrossNewPool(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	binding := seedOwnershipJournalBinding(t, st)
	reserved, err := st.ReserveOwnership(context.Background(), binding)
	require.NoError(t, err)
	receipt := ownershipAuthReceiptFixture(binding)

	confirmed, err := st.ConfirmOwnershipProof(context.Background(), binding, receipt)
	require.NoError(t, err)
	require.Equal(t, "proof_confirmed", confirmed.State)
	require.Equal(t, reserved.AuditID, confirmed.AuditID)
	require.Equal(t, reserved.EventID, confirmed.EventID)
	assertOwnershipAuthReceipt(t, confirmed, receipt)

	replay, err := st.ConfirmOwnershipProof(context.Background(), binding, receipt)
	require.NoError(t, err)
	require.Equal(t, confirmed, replay)

	readerPool, err := pgxpool.NewWithConfig(context.Background(), st.Pool.Config())
	require.NoError(t, err)
	defer readerPool.Close()
	reader := &SpaceStore{Pool: readerPool}
	loaded, err := reader.LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, err)
	require.Equal(t, confirmed, loaded)

	// Returned factor slices are caller-owned snapshots, not aliases to store state.
	confirmed.AuthReceipt.VerifiedFactors[0] = "mutated"
	confirmed.AuthReceipt.VerifiedFactors = append(confirmed.AuthReceipt.VerifiedFactors, "appended")
	afterMutation, err := reader.LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, err)
	assertOwnershipAuthReceipt(t, afterMutation, receipt)
	assertOwnershipJournalHasNoSuccessEffects(t, reader, binding)
}

func TestOwnershipJournalDecision_InvalidOrMismatchedAuthReceiptLeavesReserved(t *testing.T) {
	mutations := map[string]struct {
		mutate func(*OwnershipAuthReceipt)
		want   error
	}{
		"receipt id missing":    {func(r *OwnershipAuthReceipt) { r.ReceiptID = uuid.Nil }, ErrOwnershipReceiptInvalid},
		"account mismatch":      {func(r *OwnershipAuthReceipt) { r.AccountID = uuid.New() }, ErrOwnershipConflict},
		"profile mismatch":      {func(r *OwnershipAuthReceipt) { r.ProfileID = uuid.New() }, ErrOwnershipConflict},
		"space mismatch":        {func(r *OwnershipAuthReceipt) { r.SpaceID = uuid.New() }, ErrOwnershipConflict},
		"new owner mismatch":    {func(r *OwnershipAuthReceipt) { r.NewOwnerProfileID = uuid.New() }, ErrOwnershipConflict},
		"operation mismatch":    {func(r *OwnershipAuthReceipt) { r.OperationID = uuid.New() }, ErrOwnershipConflict},
		"epoch mismatch":        {func(r *OwnershipAuthReceipt) { r.SessionEpoch++ }, ErrOwnershipConflict},
		"consumed time missing": {func(r *OwnershipAuthReceipt) { r.ConsumedAt = time.Time{} }, ErrOwnershipReceiptInvalid},
		"sub-microsecond time":  {func(r *OwnershipAuthReceipt) { r.ConsumedAt = r.ConsumedAt.Add(time.Nanosecond) }, ErrOwnershipReceiptInvalid},
	}
	for name, tc := range mutations {
		t.Run(name, func(t *testing.T) {
			st := ownershipJournalStoreFixture(t)
			binding := seedOwnershipJournalBinding(t, st)
			reserved, err := st.ReserveOwnership(context.Background(), binding)
			require.NoError(t, err)
			receipt := ownershipAuthReceiptFixture(binding)
			tc.mutate(&receipt)

			got, err := st.ConfirmOwnershipProof(context.Background(), binding, receipt)
			require.ErrorIs(t, err, tc.want)
			require.Nil(t, got)
			loaded, loadErr := st.LoadOwnership(context.Background(), binding.OperationID)
			require.NoError(t, loadErr)
			require.Equal(t, reserved, loaded)
			assertOwnershipJournalHasNoSuccessEffects(t, st, binding)
		})
	}
}

func TestOwnershipJournalDecision_ChangedReceiptReplayConflictsWithoutRewritingEvidence(t *testing.T) {
	mutations := map[string]func(*OwnershipAuthReceipt){
		"receipt id":    func(r *OwnershipAuthReceipt) { r.ReceiptID = uuid.New() },
		"account id":    func(r *OwnershipAuthReceipt) { r.AccountID = uuid.New() },
		"profile id":    func(r *OwnershipAuthReceipt) { r.ProfileID = uuid.New() },
		"space id":      func(r *OwnershipAuthReceipt) { r.SpaceID = uuid.New() },
		"new owner id":  func(r *OwnershipAuthReceipt) { r.NewOwnerProfileID = uuid.New() },
		"operation id":  func(r *OwnershipAuthReceipt) { r.OperationID = uuid.New() },
		"session epoch": func(r *OwnershipAuthReceipt) { r.SessionEpoch++ },
		"consumed time": func(r *OwnershipAuthReceipt) { r.ConsumedAt = r.ConsumedAt.Add(time.Microsecond) },
		"factor order": func(r *OwnershipAuthReceipt) {
			r.VerifiedFactors[0], r.VerifiedFactors[1] = r.VerifiedFactors[1], r.VerifiedFactors[0]
		},
		"factor body":  func(r *OwnershipAuthReceipt) { r.VerifiedFactors[1] = "backup_code" },
		"factor count": func(r *OwnershipAuthReceipt) { r.VerifiedFactors = r.VerifiedFactors[:2] },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			st := ownershipJournalStoreFixture(t)
			binding := seedOwnershipJournalBinding(t, st)
			_, err := st.ReserveOwnership(context.Background(), binding)
			require.NoError(t, err)
			receipt := ownershipAuthReceiptFixture(binding)
			confirmed, err := st.ConfirmOwnershipProof(context.Background(), binding, receipt)
			require.NoError(t, err)
			changed := receipt
			changed.VerifiedFactors = append([]string(nil), receipt.VerifiedFactors...)
			mutate(&changed)

			got, err := st.ConfirmOwnershipProof(context.Background(), binding, changed)
			require.ErrorIs(t, err, ErrOwnershipConflict)
			require.Nil(t, got)
			loaded, loadErr := st.LoadOwnership(context.Background(), binding.OperationID)
			require.NoError(t, loadErr)
			require.Equal(t, confirmed, loaded)
			assertOwnershipAuthReceipt(t, loaded, receipt)
		})
	}
}

func TestOwnershipJournalDecision_ChangedBindingAfterConfirmationCannotReplaceEvidence(t *testing.T) {
	tests := map[string]struct {
		mutate func(*OwnershipBinding)
		want   error
	}{
		"protocol version": {func(b *OwnershipBinding) { b.ProtocolVersion = 3 }, ErrOwnershipBindingInvalid},
		"operation id":     {func(b *OwnershipBinding) { b.OperationID = uuid.New() }, ErrOwnershipConflict},
		"space id":         {func(b *OwnershipBinding) { b.SpaceID = uuid.New() }, ErrOwnershipConflict},
		"account id":       {func(b *OwnershipBinding) { b.AccountID = uuid.New() }, ErrOwnershipConflict},
		"actor profile id": {func(b *OwnershipBinding) { b.ActorProfileID = uuid.New() }, ErrOwnershipConflict},
		"new owner id":     {func(b *OwnershipBinding) { b.NewOwnerProfileID = uuid.New() }, ErrOwnershipConflict},
		"session epoch":    {func(b *OwnershipBinding) { b.SessionEpoch++ }, ErrOwnershipConflict},
		"proof digest":     {func(b *OwnershipBinding) { b.ProofDigest = "cd" + b.ProofDigest[2:] }, ErrOwnershipConflict},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			st := ownershipJournalStoreFixture(t)
			binding := seedOwnershipJournalBinding(t, st)
			_, err := st.ReserveOwnership(context.Background(), binding)
			require.NoError(t, err)
			receipt := ownershipAuthReceiptFixture(binding)
			confirmed, err := st.ConfirmOwnershipProof(context.Background(), binding, receipt)
			require.NoError(t, err)
			changed := binding
			tc.mutate(&changed)

			got, err := st.ConfirmOwnershipProof(context.Background(), changed, receipt)
			require.ErrorIs(t, err, tc.want)
			require.Nil(t, got)
			loaded, loadErr := st.LoadOwnership(context.Background(), binding.OperationID)
			require.NoError(t, loadErr)
			require.Equal(t, confirmed, loaded)
		})
	}
}

func TestOwnershipJournalDecision_ConfirmAndAbortRequireExistingExactBinding(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	binding := seedOwnershipJournalBinding(t, st)
	receipt := ownershipAuthReceiptFixture(binding)

	got, err := st.ConfirmOwnershipProof(context.Background(), binding, receipt)
	require.ErrorIs(t, err, ErrOwnershipMissing)
	require.Nil(t, got)
	got, err = st.DecideOwnershipAbort(context.Background(), binding)
	require.ErrorIs(t, err, ErrOwnershipMissing)
	require.Nil(t, got)

	reserved, err := st.ReserveOwnership(context.Background(), binding)
	require.NoError(t, err)
	changed := binding
	changed.SessionEpoch++
	changedReceipt := ownershipAuthReceiptFixture(changed)
	got, err = st.ConfirmOwnershipProof(context.Background(), changed, changedReceipt)
	require.ErrorIs(t, err, ErrOwnershipConflict)
	require.Nil(t, got)
	got, err = st.DecideOwnershipAbort(context.Background(), changed)
	require.ErrorIs(t, err, ErrOwnershipConflict)
	require.Nil(t, got)
	loaded, loadErr := st.LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, loadErr)
	require.Equal(t, reserved, loaded)
}

func TestOwnershipJournalDecision_AbortBeforeReceiptIsIrreversible(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	binding := seedOwnershipJournalBinding(t, st)
	_, err := st.ReserveOwnership(context.Background(), binding)
	require.NoError(t, err)

	aborted, err := st.DecideOwnershipAbort(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, "abort_decided", aborted.State)
	require.Nil(t, aborted.AuthReceipt)
	replay, err := st.DecideOwnershipAbort(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, aborted, replay)

	late, err := st.ConfirmOwnershipProof(context.Background(), binding, ownershipAuthReceiptFixture(binding))
	require.ErrorIs(t, err, ErrOwnershipStateTransition)
	require.Nil(t, late)
	loaded, loadErr := st.LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, loadErr)
	require.Equal(t, aborted, loaded)
	assertOwnershipJournalHasNoSuccessEffects(t, st, binding)
}

func TestOwnershipJournalDecision_AbortAfterReceiptPreservesEvidenceAndCannotRevive(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	binding := seedOwnershipJournalBinding(t, st)
	_, err := st.ReserveOwnership(context.Background(), binding)
	require.NoError(t, err)
	receipt := ownershipAuthReceiptFixture(binding)
	_, err = st.ConfirmOwnershipProof(context.Background(), binding, receipt)
	require.NoError(t, err)

	aborted, err := st.DecideOwnershipAbort(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, "abort_decided", aborted.State)
	assertOwnershipAuthReceipt(t, aborted, receipt)
	late, err := st.ConfirmOwnershipProof(context.Background(), binding, receipt)
	require.ErrorIs(t, err, ErrOwnershipStateTransition)
	require.Nil(t, late)
	loaded, loadErr := st.LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, loadErr)
	require.Equal(t, aborted, loaded)
	assertOwnershipJournalHasNoSuccessEffects(t, st, binding)
}

type ownershipDecisionResult struct {
	journal *OwnershipJournal
	err     error
}

func ownershipDecisionNamedPool(t *testing.T, source *pgxpool.Pool, applicationName string) *pgxpool.Pool {
	t.Helper()
	config := source.Config()
	config.MaxConns = 1
	config.ConnConfig.RuntimeParams["application_name"] = applicationName
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	cleanupMutationLockPool(t, pool)
	require.NoError(t, pool.Ping(context.Background()))
	return pool
}

// waitOwnershipOperationBeforeSpace observes the worker holding the canonical
// two-int operation lock while waiting for the canonical bigint space lock.
// The timeout only bounds the DB observation; elapsed time is not the oracle.
func waitOwnershipOperationBeforeSpace(t *testing.T, ctx context.Context, observer *pgxpool.Pool, applicationName string) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var operationGranted, spaceWaiting bool
		err := observer.QueryRow(ctx, `SELECT
			EXISTS(SELECT 1 FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid
				WHERE a.application_name=$1 AND l.locktype='advisory' AND l.mode='ExclusiveLock'
				AND l.objsubid=2 AND l.granted),
			EXISTS(SELECT 1 FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid
				WHERE a.application_name=$1 AND l.locktype='advisory' AND l.mode='ExclusiveLock'
				AND l.objsubid=1 AND NOT l.granted)`, applicationName).Scan(&operationGranted, &spaceWaiting)
		require.NoError(t, err)
		if operationGranted && spaceWaiting {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("ownership transition did not hold operation lock before waiting for space lock: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitOwnershipDecision(t *testing.T, result <-chan ownershipDecisionResult) ownershipDecisionResult {
	t.Helper()
	select {
	case got := <-result:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("ownership decision did not finish")
		return ownershipDecisionResult{}
	}
}

func TestOwnershipJournalDecision_ConfirmAndAbortLockOperationBeforeSpace(t *testing.T) {
	for _, action := range []string{"confirm", "abort"} {
		t.Run(action, func(t *testing.T) {
			st := ownershipJournalStoreFixture(t)
			binding := seedOwnershipJournalBinding(t, st)
			reserved, err := st.ReserveOwnership(context.Background(), binding)
			require.NoError(t, err)

			blockerPool := independentMutationLockPool(t, st.Pool, 1)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			blocker, err := blockerPool.Begin(ctx)
			require.NoError(t, err)
			defer func() {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Second)
				defer cleanupCancel()
				_ = blocker.Rollback(cleanupCtx)
			}()
			_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(binding.SpaceID))
			require.NoError(t, err)

			applicationName := "r17-cycle2-" + action + "-" + uuid.NewString()
			worker := &SpaceStore{Pool: ownershipDecisionNamedPool(t, st.Pool, applicationName)}
			result := make(chan ownershipDecisionResult, 1)
			go func() {
				var journal *OwnershipJournal
				var callErr error
				if action == "confirm" {
					journal, callErr = worker.ConfirmOwnershipProof(ctx, binding, ownershipAuthReceiptFixture(binding))
				} else {
					journal, callErr = worker.DecideOwnershipAbort(ctx, binding)
				}
				result <- ownershipDecisionResult{journal: journal, err: callErr}
			}()

			waitOwnershipOperationBeforeSpace(t, ctx, st.Pool, applicationName)
			unchanged, loadErr := st.LoadOwnership(ctx, binding.OperationID)
			require.NoError(t, loadErr)
			require.Equal(t, reserved, unchanged, "waiting transition cannot write before both canonical locks")
			require.NoError(t, blocker.Commit(ctx))
			completed := waitOwnershipDecision(t, result)
			require.NoError(t, completed.err)
			if action == "confirm" {
				require.Equal(t, "proof_confirmed", completed.journal.State)
			} else {
				require.Equal(t, "abort_decided", completed.journal.State)
			}
		})
	}
}

func TestOwnershipJournalDecision_ConcurrentConfirmAndAbortNeverResurrect(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	binding := seedOwnershipJournalBinding(t, st)
	_, err := st.ReserveOwnership(context.Background(), binding)
	require.NoError(t, err)
	receipt := ownershipAuthReceiptFixture(binding)
	ready := make(chan struct{}, 2)
	start := make(chan struct{})
	results := make(chan ownershipDecisionResult, 2)
	workerCtx, cancelWorkers := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelWorkers()
	confirmStore := &SpaceStore{Pool: independentMutationLockPool(t, st.Pool, 2)}
	abortStore := &SpaceStore{Pool: independentMutationLockPool(t, st.Pool, 2)}
	go func() {
		ready <- struct{}{}
		<-start
		journal, callErr := confirmStore.ConfirmOwnershipProof(workerCtx, binding, receipt)
		results <- ownershipDecisionResult{journal: journal, err: callErr}
	}()
	go func() {
		ready <- struct{}{}
		<-start
		journal, callErr := abortStore.DecideOwnershipAbort(workerCtx, binding)
		results <- ownershipDecisionResult{journal: journal, err: callErr}
	}()
	<-ready
	<-ready
	close(start)
	first, second := waitOwnershipDecision(t, results), waitOwnershipDecision(t, results)
	for _, result := range []ownershipDecisionResult{first, second} {
		if result.err != nil {
			require.ErrorIs(t, result.err, ErrOwnershipStateTransition)
			require.Nil(t, result.journal)
		} else {
			require.NotNil(t, result.journal)
			require.Contains(t, []string{"proof_confirmed", "abort_decided"}, result.journal.State)
		}
	}

	loaded, loadErr := st.LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, loadErr)
	require.Equal(t, "abort_decided", loaded.State)
	if loaded.AuthReceipt != nil {
		assertOwnershipAuthReceipt(t, loaded, receipt)
	}
	late, lateErr := st.ConfirmOwnershipProof(context.Background(), binding, receipt)
	require.ErrorIs(t, lateErr, ErrOwnershipStateTransition)
	require.Nil(t, late)
	assertOwnershipJournalHasNoSuccessEffects(t, st, binding)
}

func TestOwnershipJournalDecision_AbortRejectsStatesOutsideCycle2WithoutMutation(t *testing.T) {
	for _, state := range []string{"commit_decided", "completed", "aborted"} {
		t.Run(state, func(t *testing.T) {
			st := ownershipJournalStoreFixture(t)
			binding := seedOwnershipJournalBinding(t, st)
			_, err := st.ReserveOwnership(context.Background(), binding)
			require.NoError(t, err)
			_, err = st.Pool.Exec(context.Background(), `UPDATE ownership_journal SET state=$2 WHERE operation_id=$1`, binding.OperationID, state)
			require.NoError(t, err)
			before, err := st.LoadOwnership(context.Background(), binding.OperationID)
			require.NoError(t, err)

			got, err := st.DecideOwnershipAbort(context.Background(), binding)
			require.ErrorIs(t, err, ErrOwnershipStateTransition)
			require.Nil(t, got)
			after, loadErr := st.LoadOwnership(context.Background(), binding.OperationID)
			require.NoError(t, loadErr)
			require.Equal(t, before, after)
		})
	}
}

func TestOwnershipJournalDecision_InvalidReceiptRejectedBeforeDatabaseAccess(t *testing.T) {
	binding := ownershipJournalBindingFixture()
	receipt := ownershipAuthReceiptFixture(binding)
	receipt.ConsumedAt = time.Time{}
	got, err := (&SpaceStore{}).ConfirmOwnershipProof(context.Background(), binding, receipt)
	require.ErrorIs(t, err, ErrOwnershipReceiptInvalid)
	require.Nil(t, got)
	require.False(t, errors.Is(err, ErrOwnershipMissing))
}
