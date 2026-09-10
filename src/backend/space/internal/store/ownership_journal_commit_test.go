package store

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
)

func seedOwnershipProofConfirmed(t *testing.T, st *SpaceStore) OwnershipBinding {
	t.Helper()
	binding := seedOwnershipJournalBinding(t, st)
	_, err := st.ReserveOwnership(context.Background(), binding)
	require.NoError(t, err)
	_, err = st.ConfirmOwnershipProof(context.Background(), binding, ownershipAuthReceiptFixture(binding))
	require.NoError(t, err)
	return binding
}

func ownershipJournalCommitStoreFixture(t *testing.T) *SpaceStore {
	t.Helper()
	st := ownershipJournalStoreFixture(t)
	migration, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000010_ownership_journal_commit.up.sql"))
	require.NoError(t, err)
	_, err = st.Pool.Exec(context.Background(), string(migration))
	require.NoError(t, err)
	return st
}

func ownershipRolePreparedReceiptFixture(binding OwnershipBinding) *rolev1.OwnershipTransferReceipt {
	intent := &rolev1.OwnershipTransferIntent{
		ProtocolVersion:   2,
		SpaceId:           binding.SpaceID.String(),
		OldOwnerProfileId: binding.ActorProfileID.String(),
		NewOwnerProfileId: binding.NewOwnerProfileID.String(),
		OperationId:       binding.OperationID.String(),
	}
	intentUnknown := protowire.AppendTag(nil, 100, protowire.BytesType)
	intentUnknown = protowire.AppendString(intentUnknown, "r14-intent-unknown")
	intent.ProtoReflect().SetUnknown(intentUnknown)
	receipt := &rolev1.OwnershipTransferReceipt{
		Intent: intent,
		State:  rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_PREPARED,
	}
	receiptUnknown := protowire.AppendTag(nil, 101, protowire.VarintType)
	receiptUnknown = protowire.AppendVarint(receiptUnknown, 17)
	receipt.ProtoReflect().SetUnknown(receiptUnknown)
	return receipt
}

func deterministicProtoBytes(t *testing.T, message proto.Message) []byte {
	t.Helper()
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(message)
	require.NoError(t, err)
	return raw
}

func assertOwnershipPreparedReceipt(t *testing.T, journal *OwnershipJournal, receipt *rolev1.OwnershipTransferReceipt) {
	t.Helper()
	require.NotNil(t, journal)
	require.NotNil(t, journal.RoleReceipt)
	wantReceipt := deterministicProtoBytes(t, receipt)
	wantIntent := deterministicProtoBytes(t, receipt.Intent)
	require.Equal(t, wantReceipt, journal.RoleReceipt.ReceiptBytes)
	require.Equal(t, sha256.Sum256(wantReceipt), journal.RoleReceipt.ReceiptHash)
	require.Equal(t, wantIntent, journal.RoleReceipt.IntentBytes)
	require.Equal(t, sha256.Sum256(wantIntent), journal.RoleReceipt.IntentHash)
	var restored rolev1.OwnershipTransferReceipt
	require.NoError(t, proto.Unmarshal(journal.RoleReceipt.ReceiptBytes, &restored))
	require.Equal(t, receipt.ProtoReflect().GetUnknown(), restored.ProtoReflect().GetUnknown())
	require.Equal(t, receipt.Intent.ProtoReflect().GetUnknown(), restored.Intent.ProtoReflect().GetUnknown())
}

func assertNoOwnershipCommitEffects(t *testing.T, st *SpaceStore, binding OwnershipBinding, expectedOwner uuid.UUID) {
	t.Helper()
	space, err := st.GetSpace(context.Background(), binding.SpaceID)
	require.NoError(t, err)
	require.Equal(t, expectedOwner, space.OwnerProfileID)
	page, err := st.ListAuditLogPage(context.Background(), binding.SpaceID, "", 10)
	require.NoError(t, err)
	require.Empty(t, page.Rows)
	var outboxCount int
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM ownership_outbox WHERE operation_id=$1`, binding.OperationID).Scan(&outboxCount))
	require.Zero(t, outboxCount)
}

func TestOwnershipJournalCommit_PreparedReceiptPersistsExactUnknownFieldsAndReplays(t *testing.T) {
	st := ownershipJournalCommitStoreFixture(t)
	binding := seedOwnershipProofConfirmed(t, st)
	receipt := ownershipRolePreparedReceiptFixture(binding)
	prepared, err := st.MarkOwnershipPrepared(context.Background(), binding, receipt)
	require.NoError(t, err)
	require.Equal(t, "prepared", prepared.State)
	assertOwnershipPreparedReceipt(t, prepared, receipt)

	replay, err := st.MarkOwnershipPrepared(context.Background(), binding, proto.Clone(receipt).(*rolev1.OwnershipTransferReceipt))
	require.NoError(t, err)
	require.Equal(t, prepared, replay)
	readerPool, err := pgxpool.NewWithConfig(context.Background(), st.Pool.Config())
	require.NoError(t, err)
	defer readerPool.Close()
	loaded, err := (&SpaceStore{Pool: readerPool}).LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, err)
	require.Equal(t, prepared, loaded)

	prepared.RoleReceipt.IntentBytes[0] ^= 0xff
	prepared.RoleReceipt.ReceiptBytes[0] ^= 0xff
	afterMutation, err := (&SpaceStore{Pool: readerPool}).LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, err)
	assertOwnershipPreparedReceipt(t, afterMutation, receipt)
	assertNoOwnershipCommitEffects(t, st, binding, binding.ActorProfileID)
}

func TestOwnershipJournalCommit_InvalidOrMismatchedPreparedReceiptCannotAdvance(t *testing.T) {
	tests := map[string]struct {
		mutate func(*rolev1.OwnershipTransferReceipt)
		want   error
	}{
		"missing intent":      {func(r *rolev1.OwnershipTransferReceipt) { r.Intent = nil }, ErrOwnershipRoleReceiptInvalid},
		"version one":         {func(r *rolev1.OwnershipTransferReceipt) { r.Intent.ProtocolVersion = 1 }, ErrOwnershipRoleReceiptInvalid},
		"version future":      {func(r *rolev1.OwnershipTransferReceipt) { r.Intent.ProtocolVersion = 3 }, ErrOwnershipRoleReceiptInvalid},
		"space mismatch":      {func(r *rolev1.OwnershipTransferReceipt) { r.Intent.SpaceId = uuid.NewString() }, ErrOwnershipConflict},
		"old owner mismatch":  {func(r *rolev1.OwnershipTransferReceipt) { r.Intent.OldOwnerProfileId = uuid.NewString() }, ErrOwnershipConflict},
		"new owner mismatch":  {func(r *rolev1.OwnershipTransferReceipt) { r.Intent.NewOwnerProfileId = uuid.NewString() }, ErrOwnershipConflict},
		"operation mismatch":  {func(r *rolev1.OwnershipTransferReceipt) { r.Intent.OperationId = uuid.NewString() }, ErrOwnershipConflict},
		"malformed space":     {func(r *rolev1.OwnershipTransferReceipt) { r.Intent.SpaceId = "not-a-uuid" }, ErrOwnershipRoleReceiptInvalid},
		"malformed old owner": {func(r *rolev1.OwnershipTransferReceipt) { r.Intent.OldOwnerProfileId = "not-a-uuid" }, ErrOwnershipRoleReceiptInvalid},
		"malformed new owner": {func(r *rolev1.OwnershipTransferReceipt) { r.Intent.NewOwnerProfileId = "not-a-uuid" }, ErrOwnershipRoleReceiptInvalid},
		"malformed operation": {func(r *rolev1.OwnershipTransferReceipt) { r.Intent.OperationId = "not-a-uuid" }, ErrOwnershipRoleReceiptInvalid},
		"unspecified state": {func(r *rolev1.OwnershipTransferReceipt) {
			r.State = rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_UNSPECIFIED
		}, ErrOwnershipRoleReceiptInvalid},
		"finalized state": {func(r *rolev1.OwnershipTransferReceipt) {
			r.State = rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED
		}, ErrOwnershipRoleReceiptInvalid},
		"aborted state": {func(r *rolev1.OwnershipTransferReceipt) {
			r.State = rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED
		}, ErrOwnershipRoleReceiptInvalid},
		"prepared claims new owner":       {func(r *rolev1.OwnershipTransferReceipt) { r.CurrentOwnerProfileId = r.Intent.NewOwnerProfileId }, ErrOwnershipRoleReceiptInvalid},
		"prepared claims old owner":       {func(r *rolev1.OwnershipTransferReceipt) { r.CurrentOwnerProfileId = r.Intent.OldOwnerProfileId }, ErrOwnershipRoleReceiptInvalid},
		"prepared claims unrelated owner": {func(r *rolev1.OwnershipTransferReceipt) { r.CurrentOwnerProfileId = uuid.NewString() }, ErrOwnershipRoleReceiptInvalid},
		"prepared claims malformed owner": {func(r *rolev1.OwnershipTransferReceipt) { r.CurrentOwnerProfileId = "not-a-uuid" }, ErrOwnershipRoleReceiptInvalid},
	}
	st := ownershipJournalCommitStoreFixture(t)
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			binding := seedOwnershipProofConfirmed(t, st)
			before, err := st.LoadOwnership(context.Background(), binding.OperationID)
			require.NoError(t, err)
			receipt := ownershipRolePreparedReceiptFixture(binding)
			tc.mutate(receipt)
			got, err := st.MarkOwnershipPrepared(context.Background(), binding, receipt)
			require.ErrorIs(t, err, tc.want)
			require.Nil(t, got)
			after, loadErr := st.LoadOwnership(context.Background(), binding.OperationID)
			require.NoError(t, loadErr)
			require.Equal(t, before, after)
		})
	}
}

func TestOwnershipJournalCommit_PreparedReplayRejectsEveryChangedReceiptField(t *testing.T) {
	st := ownershipJournalCommitStoreFixture(t)
	binding := seedOwnershipProofConfirmed(t, st)
	receipt := ownershipRolePreparedReceiptFixture(binding)
	prepared, err := st.MarkOwnershipPrepared(context.Background(), binding, receipt)
	require.NoError(t, err)
	tests := map[string]struct {
		mutate func(*rolev1.OwnershipTransferReceipt)
		want   error
	}{
		"protocol version": {func(r *rolev1.OwnershipTransferReceipt) { r.Intent.ProtocolVersion = 1 }, ErrOwnershipRoleReceiptInvalid},
		"space":            {func(r *rolev1.OwnershipTransferReceipt) { r.Intent.SpaceId = uuid.NewString() }, ErrOwnershipConflict},
		"old owner":        {func(r *rolev1.OwnershipTransferReceipt) { r.Intent.OldOwnerProfileId = uuid.NewString() }, ErrOwnershipConflict},
		"new owner":        {func(r *rolev1.OwnershipTransferReceipt) { r.Intent.NewOwnerProfileId = uuid.NewString() }, ErrOwnershipConflict},
		"operation":        {func(r *rolev1.OwnershipTransferReceipt) { r.Intent.OperationId = uuid.NewString() }, ErrOwnershipConflict},
		"state": {func(r *rolev1.OwnershipTransferReceipt) {
			r.State = rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED
		}, ErrOwnershipRoleReceiptInvalid},
		"current new owner":       {func(r *rolev1.OwnershipTransferReceipt) { r.CurrentOwnerProfileId = r.Intent.NewOwnerProfileId }, ErrOwnershipRoleReceiptInvalid},
		"current old owner":       {func(r *rolev1.OwnershipTransferReceipt) { r.CurrentOwnerProfileId = r.Intent.OldOwnerProfileId }, ErrOwnershipRoleReceiptInvalid},
		"current unrelated owner": {func(r *rolev1.OwnershipTransferReceipt) { r.CurrentOwnerProfileId = uuid.NewString() }, ErrOwnershipRoleReceiptInvalid},
		"current malformed owner": {func(r *rolev1.OwnershipTransferReceipt) { r.CurrentOwnerProfileId = "not-a-uuid" }, ErrOwnershipRoleReceiptInvalid},
		"intent unknown": {func(r *rolev1.OwnershipTransferReceipt) {
			unknown := protowire.AppendTag(nil, 120, protowire.BytesType)
			unknown = protowire.AppendString(unknown, "changed-intent")
			r.Intent.ProtoReflect().SetUnknown(append(r.Intent.ProtoReflect().GetUnknown(), unknown...))
		}, ErrOwnershipConflict},
		"receipt unknown": {func(r *rolev1.OwnershipTransferReceipt) {
			unknown := protowire.AppendTag(nil, 121, protowire.BytesType)
			unknown = protowire.AppendString(unknown, "changed-receipt")
			r.ProtoReflect().SetUnknown(append(r.ProtoReflect().GetUnknown(), unknown...))
		}, ErrOwnershipConflict},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			changed := proto.Clone(receipt).(*rolev1.OwnershipTransferReceipt)
			tc.mutate(changed)
			got, err := st.MarkOwnershipPrepared(context.Background(), binding, changed)
			require.ErrorIs(t, err, tc.want)
			require.Nil(t, got)
			loaded, loadErr := st.LoadOwnership(context.Background(), binding.OperationID)
			require.NoError(t, loadErr)
			require.Equal(t, prepared, loaded)
		})
	}
}

func TestOwnershipJournalCommit_DecisionRequiresBothReceiptsAndLiveOwnerMember(t *testing.T) {
	st := ownershipJournalCommitStoreFixture(t)
	reservedBinding := seedOwnershipJournalBinding(t, st)
	_, err := st.ReserveOwnership(context.Background(), reservedBinding)
	require.NoError(t, err)
	got, err := st.MarkOwnershipPrepared(context.Background(), reservedBinding, ownershipRolePreparedReceiptFixture(reservedBinding))
	require.ErrorIs(t, err, ErrOwnershipStateTransition, "Role PREPARED evidence cannot replace the missing Auth receipt")
	require.Nil(t, got)
	got, err = st.DecideOwnershipCommit(context.Background(), reservedBinding)
	require.ErrorIs(t, err, ErrOwnershipStateTransition)
	require.Nil(t, got)

	proofBinding := seedOwnershipProofConfirmed(t, st)
	got, err = st.DecideOwnershipCommit(context.Background(), proofBinding)
	require.ErrorIs(t, err, ErrOwnershipStateTransition)
	require.Nil(t, got)

	for _, failure := range []string{"owner changed", "target membership removed"} {
		t.Run(failure, func(t *testing.T) {
			binding := seedOwnershipProofConfirmed(t, st)
			_, err := st.MarkOwnershipPrepared(context.Background(), binding, ownershipRolePreparedReceiptFixture(binding))
			require.NoError(t, err)
			expectedOwner := binding.ActorProfileID
			if failure == "owner changed" {
				expectedOwner = uuid.New()
				_, err = st.Pool.Exec(context.Background(), `UPDATE spaces SET owner_profile_id=$2 WHERE id=$1`, binding.SpaceID, expectedOwner)
			} else {
				_, err = st.Pool.Exec(context.Background(), `DELETE FROM space_members WHERE space_id=$1 AND profile_id=$2`, binding.SpaceID, binding.NewOwnerProfileID)
			}
			require.NoError(t, err)
			got, err := st.DecideOwnershipCommit(context.Background(), binding)
			if failure == "owner changed" {
				require.ErrorIs(t, err, ErrNotSpaceOwner)
			} else {
				require.ErrorIs(t, err, ErrMemberNotFound)
			}
			require.Nil(t, got)
			loaded, loadErr := st.LoadOwnership(context.Background(), binding.OperationID)
			require.NoError(t, loadErr)
			require.Equal(t, "prepared", loaded.State)
			require.Nil(t, loaded.PendingAudit)
			assertNoOwnershipCommitEffects(t, st, binding, expectedOwner)
		})
	}
}

func TestOwnershipJournalCommit_MarkPreparedAndCommitLockOperationBeforeSpace(t *testing.T) {
	for _, action := range []string{"mark-prepared", "commit"} {
		t.Run(action, func(t *testing.T) {
			st := ownershipJournalCommitStoreFixture(t)
			binding := seedOwnershipProofConfirmed(t, st)
			before, err := st.LoadOwnership(context.Background(), binding.OperationID)
			require.NoError(t, err)
			if action == "commit" {
				before, err = st.MarkOwnershipPrepared(context.Background(), binding, ownershipRolePreparedReceiptFixture(binding))
				require.NoError(t, err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			blocker, err := independentMutationLockPool(t, st.Pool, 1).Begin(ctx)
			require.NoError(t, err)
			defer func() {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Second)
				defer cleanupCancel()
				_ = blocker.Rollback(cleanupCtx)
			}()
			_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(binding.SpaceID))
			require.NoError(t, err)

			applicationName := "r17-cycle3-" + action + "-" + uuid.NewString()
			worker := &SpaceStore{Pool: ownershipDecisionNamedPool(t, st.Pool, applicationName)}
			result := make(chan ownershipDecisionResult, 1)
			go func() {
				var journal *OwnershipJournal
				var callErr error
				if action == "mark-prepared" {
					journal, callErr = worker.MarkOwnershipPrepared(ctx, binding, ownershipRolePreparedReceiptFixture(binding))
				} else {
					journal, callErr = worker.DecideOwnershipCommit(ctx, binding)
				}
				result <- ownershipDecisionResult{journal: journal, err: callErr}
			}()

			waitOwnershipOperationBeforeSpace(t, ctx, st.Pool, applicationName)
			unchanged, loadErr := st.LoadOwnership(ctx, binding.OperationID)
			require.NoError(t, loadErr)
			require.Equal(t, before, unchanged)
			assertNoOwnershipCommitEffects(t, st, binding, binding.ActorProfileID)
			require.NoError(t, blocker.Commit(ctx))
			completed := waitOwnershipDecision(t, result)
			require.NoError(t, completed.err)
			if action == "mark-prepared" {
				require.Equal(t, "prepared", completed.journal.State)
			} else {
				require.Equal(t, "commit_decided", completed.journal.State)
			}
		})
	}
}

func TestOwnershipJournalCommit_AtomicCommitPersistsPendingArtifactsButKeepsThemInvisible(t *testing.T) {
	st := ownershipJournalCommitStoreFixture(t)
	binding := seedOwnershipProofConfirmed(t, st)
	receipt := ownershipRolePreparedReceiptFixture(binding)
	prepared, err := st.MarkOwnershipPrepared(context.Background(), binding, receipt)
	require.NoError(t, err)
	committed, err := st.DecideOwnershipCommit(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, "commit_decided", committed.State)
	require.Equal(t, prepared.AuditID, committed.PendingAudit.ID)
	require.Equal(t, binding.SpaceID, committed.PendingAudit.SpaceID)
	require.Equal(t, binding.ActorProfileID, committed.PendingAudit.ActorProfileID)
	require.Equal(t, "ownership_transferred", committed.PendingAudit.Action)
	require.Equal(t, "profile", committed.PendingAudit.TargetType)
	require.Equal(t, binding.NewOwnerProfileID, committed.PendingAudit.TargetID)
	require.JSONEq(t, `{}`, committed.PendingAudit.DetailsJSON)

	space, err := st.GetSpace(context.Background(), binding.SpaceID)
	require.NoError(t, err)
	require.Equal(t, binding.NewOwnerProfileID, space.OwnerProfileID)
	page, err := st.ListAuditLogPage(context.Background(), binding.SpaceID, "", 10)
	require.NoError(t, err)
	require.Empty(t, page.Rows)
	var eventID, operationID, spaceID, oldOwner, newOwner uuid.UUID
	var eventType string
	var ready bool
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT event_id,operation_id,space_id,previous_owner_profile_id,new_owner_profile_id,event_type,ready FROM ownership_outbox WHERE operation_id=$1`, binding.OperationID).Scan(&eventID, &operationID, &spaceID, &oldOwner, &newOwner, &eventType, &ready))
	require.Equal(t, prepared.EventID, eventID)
	require.Equal(t, binding.OperationID, operationID)
	require.Equal(t, binding.SpaceID, spaceID)
	require.Equal(t, binding.ActorProfileID, oldOwner)
	require.Equal(t, binding.NewOwnerProfileID, newOwner)
	require.Equal(t, "space.updated", eventType)
	require.False(t, ready)
	var dispatchable int
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM ownership_outbox WHERE ready=true`).Scan(&dispatchable))
	require.Zero(t, dispatchable)

	replay, err := st.DecideOwnershipCommit(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, committed, replay)
	var outboxCount int
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM ownership_outbox WHERE operation_id=$1`, binding.OperationID).Scan(&outboxCount))
	require.Equal(t, 1, outboxCount)

	laterOwner := uuid.New()
	_, err = st.Pool.Exec(context.Background(), `UPDATE spaces SET owner_profile_id=$2 WHERE id=$1`, binding.SpaceID, laterOwner)
	require.NoError(t, err)
	replay, err = st.DecideOwnershipCommit(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, committed, replay)
	space, err = st.GetSpace(context.Background(), binding.SpaceID)
	require.NoError(t, err)
	require.Equal(t, laterOwner, space.OwnerProfileID, "commit replay cannot rewrite a later live owner")
	aborted, err := st.DecideOwnershipAbort(context.Background(), binding)
	require.ErrorIs(t, err, ErrOwnershipStateTransition)
	require.Nil(t, aborted)
	space, err = st.GetSpace(context.Background(), binding.SpaceID)
	require.NoError(t, err)
	require.Equal(t, laterOwner, space.OwnerProfileID)
}

func TestOwnershipJournalCommit_OutboxFailureRollsBackOwnerDecisionAndPendingAudit(t *testing.T) {
	st := ownershipJournalCommitStoreFixture(t)
	binding := seedOwnershipProofConfirmed(t, st)
	_, err := st.MarkOwnershipPrepared(context.Background(), binding, ownershipRolePreparedReceiptFixture(binding))
	require.NoError(t, err)
	_, err = st.Pool.Exec(context.Background(), `CREATE FUNCTION fail_ownership_outbox_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected ownership outbox failure'; END $$; CREATE TRIGGER ownership_outbox_failure BEFORE INSERT ON ownership_outbox FOR EACH ROW EXECUTE FUNCTION fail_ownership_outbox_insert()`)
	require.NoError(t, err)
	got, err := st.DecideOwnershipCommit(context.Background(), binding)
	require.Error(t, err)
	require.Nil(t, got)
	loaded, loadErr := st.LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, loadErr)
	require.Equal(t, "prepared", loaded.State)
	require.Nil(t, loaded.PendingAudit)
	assertNoOwnershipCommitEffects(t, st, binding, binding.ActorProfileID)
}

func TestOwnershipJournalCommit_ConcurrentCommitAndAbortHaveOneDurableWinner(t *testing.T) {
	st := ownershipJournalCommitStoreFixture(t)
	binding := seedOwnershipProofConfirmed(t, st)
	_, err := st.MarkOwnershipPrepared(context.Background(), binding, ownershipRolePreparedReceiptFixture(binding))
	require.NoError(t, err)
	ready := make(chan struct{}, 2)
	start := make(chan struct{})
	results := make(chan ownershipDecisionResult, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	commitStore := &SpaceStore{Pool: independentMutationLockPool(t, st.Pool, 2)}
	abortStore := &SpaceStore{Pool: independentMutationLockPool(t, st.Pool, 2)}
	go func() {
		ready <- struct{}{}
		<-start
		j, e := commitStore.DecideOwnershipCommit(ctx, binding)
		results <- ownershipDecisionResult{journal: j, err: e}
	}()
	go func() {
		ready <- struct{}{}
		<-start
		j, e := abortStore.DecideOwnershipAbort(ctx, binding)
		results <- ownershipDecisionResult{journal: j, err: e}
	}()
	<-ready
	<-ready
	close(start)
	a, b := waitOwnershipDecision(t, results), waitOwnershipDecision(t, results)
	successes := 0
	for _, result := range []ownershipDecisionResult{a, b} {
		if result.err == nil {
			successes++
		} else {
			require.ErrorIs(t, result.err, ErrOwnershipStateTransition)
			require.Nil(t, result.journal)
		}
	}
	require.Equal(t, 1, successes)
	readerPool, err := pgxpool.NewWithConfig(context.Background(), st.Pool.Config())
	require.NoError(t, err)
	defer readerPool.Close()
	loaded, err := (&SpaceStore{Pool: readerPool}).LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, err)
	space, err := st.GetSpace(context.Background(), binding.SpaceID)
	require.NoError(t, err)
	if loaded.State == "commit_decided" {
		require.Equal(t, binding.NewOwnerProfileID, space.OwnerProfileID)
		require.NotNil(t, loaded.PendingAudit)
	} else {
		require.Equal(t, "abort_decided", loaded.State)
		require.Equal(t, binding.ActorProfileID, space.OwnerProfileID)
		assertNoOwnershipCommitEffects(t, st, binding, binding.ActorProfileID)
	}
}

func TestOwnershipJournalCommit_PreparedAbortHasNoSuccessEffects(t *testing.T) {
	st := ownershipJournalCommitStoreFixture(t)
	binding := seedOwnershipProofConfirmed(t, st)
	receipt := ownershipRolePreparedReceiptFixture(binding)
	_, err := st.MarkOwnershipPrepared(context.Background(), binding, receipt)
	require.NoError(t, err)
	aborted, err := st.DecideOwnershipAbort(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, "abort_decided", aborted.State)
	assertOwnershipPreparedReceipt(t, aborted, receipt)
	require.Nil(t, aborted.PendingAudit)
	assertNoOwnershipCommitEffects(t, st, binding, binding.ActorProfileID)
	committed, err := st.DecideOwnershipCommit(context.Background(), binding)
	require.ErrorIs(t, err, ErrOwnershipStateTransition)
	require.Nil(t, committed)
	after, loadErr := st.LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, loadErr)
	require.Equal(t, aborted, after)
}

// ownershipCommitAmbiguityProxy forwards one armed COMMIT through PostgreSQL,
// observes its successful CommandComplete, then drops that response. The
// transaction is durable while pgx must report an ambiguous transport error.
type ownershipCommitAmbiguityProxy struct {
	listener net.Listener
	upstream string

	mu        sync.Mutex
	armed     bool
	claimed   bool
	shutdown  chan struct{}
	drop      chan struct{}
	persisted chan struct{}

	shutdownOnce sync.Once
	dropOnce     sync.Once
}

func startOwnershipCommitAmbiguityProxy(t *testing.T, upstreamHost string, upstreamPort uint16) *ownershipCommitAmbiguityProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	p := &ownershipCommitAmbiguityProxy{
		listener:  listener,
		upstream:  net.JoinHostPort(upstreamHost, fmt.Sprintf("%d", upstreamPort)),
		shutdown:  make(chan struct{}),
		drop:      make(chan struct{}),
		persisted: make(chan struct{}),
	}
	go p.serve()
	t.Cleanup(p.abort)
	return p
}

func (p *ownershipCommitAmbiguityProxy) port() uint16 {
	return uint16(p.listener.Addr().(*net.TCPAddr).Port)
}

func (p *ownershipCommitAmbiguityProxy) arm() {
	p.mu.Lock()
	p.armed = true
	p.mu.Unlock()
}

func (p *ownershipCommitAmbiguityProxy) claimCommit() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.armed || p.claimed {
		return false
	}
	p.claimed = true
	return true
}

func (p *ownershipCommitAmbiguityProxy) dropCommitResponse() {
	p.dropOnce.Do(func() { close(p.drop) })
}

func (p *ownershipCommitAmbiguityProxy) abort() {
	p.dropCommitResponse()
	p.shutdownOnce.Do(func() { close(p.shutdown) })
	_ = p.listener.Close()
}

func (p *ownershipCommitAmbiguityProxy) serve() {
	for {
		client, err := p.listener.Accept()
		if err != nil {
			return
		}
		go p.serveConn(client)
	}
}

func (p *ownershipCommitAmbiguityProxy) serveConn(client net.Conn) {
	upstream, err := net.Dial("tcp", p.upstream)
	if err != nil {
		_ = client.Close()
		return
	}
	defer client.Close()
	defer upstream.Close()
	var interceptCommit atomic.Bool
	done := make(chan struct{}, 2)
	go func() {
		_ = proxyOwnershipCommitFrontend(client, upstream, p, &interceptCommit)
		done <- struct{}{}
	}()
	go func() {
		_ = proxyOwnershipCommitBackend(upstream, client, p, &interceptCommit)
		done <- struct{}{}
	}()
	<-done
}

func proxyOwnershipCommitFrontend(client, upstream net.Conn, p *ownershipCommitAmbiguityProxy, interceptCommit *atomic.Bool) error {
	length, startup, err := readOwnershipPostgresStartup(client)
	if err != nil {
		return err
	}
	if err := writeOwnershipPostgresStartup(upstream, length, startup); err != nil {
		return err
	}
	for {
		messageType, payload, err := readOwnershipPostgresPacket(client)
		if err != nil {
			return err
		}
		if messageType == 'Q' && strings.EqualFold(strings.TrimSpace(strings.TrimSuffix(string(payload), "\x00")), "commit") && p.claimCommit() {
			interceptCommit.Store(true)
		}
		if err := writeOwnershipPostgresPacket(upstream, messageType, payload); err != nil {
			return err
		}
	}
}

func proxyOwnershipCommitBackend(upstream, client net.Conn, p *ownershipCommitAmbiguityProxy, interceptCommit *atomic.Bool) error {
	for {
		messageType, payload, err := readOwnershipPostgresPacket(upstream)
		if err != nil {
			return err
		}
		if interceptCommit.Load() && messageType == 'C' && strings.EqualFold(strings.TrimSpace(strings.TrimSuffix(string(payload), "\x00")), "commit") {
			close(p.persisted)
			select {
			case <-p.drop:
			case <-p.shutdown:
			}
			return io.ErrUnexpectedEOF
		}
		if err := writeOwnershipPostgresPacket(client, messageType, payload); err != nil {
			return err
		}
	}
}

func readOwnershipPostgresStartup(conn net.Conn) (uint32, []byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return 0, nil, err
	}
	length := binary.BigEndian.Uint32(header[:])
	if length < 4 || length > 16<<20 {
		return 0, nil, fmt.Errorf("invalid PostgreSQL startup packet length %d", length)
	}
	payload := make([]byte, length-4)
	_, err := io.ReadFull(conn, payload)
	return length, payload, err
}

func writeOwnershipPostgresStartup(conn net.Conn, length uint32, payload []byte) error {
	packet := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(packet[:4], length)
	copy(packet[4:], payload)
	return writeOwnershipPostgresAll(conn, packet)
}

func readOwnershipPostgresPacket(conn net.Conn) (byte, []byte, error) {
	var header [5]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return 0, nil, err
	}
	length := binary.BigEndian.Uint32(header[1:])
	if length < 4 || length > 64<<20 {
		return 0, nil, fmt.Errorf("invalid PostgreSQL packet length %d", length)
	}
	payload := make([]byte, length-4)
	_, err := io.ReadFull(conn, payload)
	return header[0], payload, err
}

func writeOwnershipPostgresPacket(conn net.Conn, messageType byte, payload []byte) error {
	packet := make([]byte, 5+len(payload))
	packet[0] = messageType
	binary.BigEndian.PutUint32(packet[1:5], uint32(len(payload)+4))
	copy(packet[5:], payload)
	return writeOwnershipPostgresAll(conn, packet)
}

func writeOwnershipPostgresAll(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func ownershipCommitAmbiguityPool(t *testing.T, source *pgxpool.Pool, proxy *ownershipCommitAmbiguityProxy) *pgxpool.Pool {
	t.Helper()
	config := source.Config()
	config.MaxConns = 1
	config.MinConns = 0
	config.ConnConfig.Host = "127.0.0.1"
	config.ConnConfig.Port = proxy.port()
	config.ConnConfig.TLSConfig = nil
	config.ConnConfig.Fallbacks = nil
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	t.Cleanup(proxy.abort)
	require.NoError(t, pool.Ping(context.Background()))
	return pool
}

func TestOwnershipJournalCommit_AmbiguousCommitErrorRecoversDurableDecisionByReload(t *testing.T) {
	st := ownershipJournalCommitStoreFixture(t)
	binding := seedOwnershipProofConfirmed(t, st)
	_, err := st.MarkOwnershipPrepared(context.Background(), binding, ownershipRolePreparedReceiptFixture(binding))
	require.NoError(t, err)
	proxy := startOwnershipCommitAmbiguityProxy(t, st.Pool.Config().ConnConfig.Host, st.Pool.Config().ConnConfig.Port)
	ambiguousStore := &SpaceStore{Pool: ownershipCommitAmbiguityPool(t, st.Pool, proxy)}
	proxy.arm()
	result := make(chan ownershipDecisionResult, 1)
	go func() {
		journal, callErr := ambiguousStore.DecideOwnershipCommit(context.Background(), binding)
		result <- ownershipDecisionResult{journal: journal, err: callErr}
	}()
	select {
	case <-proxy.persisted:
	case <-time.After(5 * time.Second):
		t.Fatal("PostgreSQL did not durably commit before the response-loss boundary")
	}
	proxy.dropCommitResponse()
	call := waitOwnershipDecision(t, result)
	require.ErrorIs(t, call.err, ErrOwnershipCommitAmbiguous)
	require.Nil(t, call.journal)

	readerPool, err := pgxpool.NewWithConfig(context.Background(), st.Pool.Config())
	require.NoError(t, err)
	defer readerPool.Close()
	recovered, err := (&SpaceStore{Pool: readerPool}).LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, err)
	require.Equal(t, "commit_decided", recovered.State)
	require.NotNil(t, recovered.PendingAudit)
	space, err := (&SpaceStore{Pool: readerPool}).GetSpace(context.Background(), binding.SpaceID)
	require.NoError(t, err)
	require.Equal(t, binding.NewOwnerProfileID, space.OwnerProfileID)
}

func TestOwnershipJournalCommit_InvalidPreparedReceiptRejectedBeforeDatabaseAccess(t *testing.T) {
	binding := ownershipJournalBindingFixture()
	got, err := (&SpaceStore{}).MarkOwnershipPrepared(context.Background(), binding, nil)
	require.ErrorIs(t, err, ErrOwnershipRoleReceiptInvalid)
	require.Nil(t, got)
	require.False(t, errors.Is(err, ErrOwnershipMissing))
}
