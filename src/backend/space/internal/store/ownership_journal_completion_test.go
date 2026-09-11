package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
)

func ownershipJournalCompletionMigrationSQL(t *testing.T, direction string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000012_ownership_journal_completion."+direction+".sql"))
	require.NoError(t, err)
	return string(raw)
}

func ownershipJournalCompletionStoreFixture(t *testing.T) *SpaceStore {
	t.Helper()
	st := ownershipJournalCommitStoreFixture(t)
	applyR22SpaceEpochMigration(t, context.Background(), st)
	_, err := st.Pool.Exec(context.Background(), ownershipJournalCompletionMigrationSQL(t, "up"))
	require.NoError(t, err)
	return st
}

type ownershipCommitCompleter interface {
	CompleteOwnershipCommit(context.Context, OwnershipBinding, *rolev1.OwnershipTransferReceipt) (*OwnershipJournal, error)
}

type ownershipAbortCompleter interface {
	CompleteOwnershipAbort(context.Context, OwnershipBinding, *rolev1.OwnershipTransferReceipt) (*OwnershipJournal, error)
}

type ownershipTerminalEvidenceAccessor interface {
	OwnershipTerminalReceiptEvidence() (receiptBytes []byte, receiptHash [32]byte, ok bool)
}

func invokeOwnershipTerminal(st *SpaceStore, ctx context.Context, methodName string, binding OwnershipBinding, receipt *rolev1.OwnershipTransferReceipt) (*OwnershipJournal, error) {
	switch methodName {
	case "CompleteOwnershipCommit":
		completer, ok := any(st).(ownershipCommitCompleter)
		if !ok {
			return nil, errors.New("CompleteOwnershipCommit typed API is missing")
		}
		return completer.CompleteOwnershipCommit(ctx, binding, receipt)
	case "CompleteOwnershipAbort":
		completer, ok := any(st).(ownershipAbortCompleter)
		if !ok {
			return nil, errors.New("CompleteOwnershipAbort typed API is missing")
		}
		return completer.CompleteOwnershipAbort(ctx, binding, receipt)
	default:
		return nil, errors.New("unknown ownership terminal method")
	}
}

func ownershipTerminalReceipt(binding OwnershipBinding, prepared *rolev1.OwnershipTransferReceipt, state rolev1.OwnershipTransferState, wrapperUnknown []byte) *rolev1.OwnershipTransferReceipt {
	var intent *rolev1.OwnershipTransferIntent
	if prepared != nil {
		intent = proto.Clone(prepared.GetIntent()).(*rolev1.OwnershipTransferIntent)
	} else {
		intent = &rolev1.OwnershipTransferIntent{
			ProtocolVersion: 2, SpaceId: binding.SpaceID.String(),
			OldOwnerProfileId: binding.ActorProfileID.String(), NewOwnerProfileId: binding.NewOwnerProfileID.String(),
			OperationId: binding.OperationID.String(),
		}
	}
	owner := binding.ActorProfileID
	if state == rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED {
		owner = binding.NewOwnerProfileID
	}
	receipt := &rolev1.OwnershipTransferReceipt{Intent: intent, State: state, CurrentOwnerProfileId: owner.String()}
	receipt.ProtoReflect().SetUnknown(append([]byte(nil), wrapperUnknown...))
	return receipt
}

func seedOwnershipCommitDecided(t *testing.T, st *SpaceStore) (OwnershipBinding, *rolev1.OwnershipTransferReceipt, *OwnershipJournal) {
	t.Helper()
	binding := seedOwnershipProofConfirmed(t, st)
	prepared := ownershipRolePreparedReceiptFixture(binding)
	_, err := st.MarkOwnershipPrepared(context.Background(), binding, prepared)
	require.NoError(t, err)
	journal, err := st.DecideOwnershipCommit(context.Background(), binding)
	require.NoError(t, err)
	return binding, prepared, journal
}

func seedOwnershipAbortDecided(t *testing.T, st *SpaceStore, prepared bool) (OwnershipBinding, *rolev1.OwnershipTransferReceipt, *OwnershipJournal) {
	t.Helper()
	var binding OwnershipBinding
	var preparedReceipt *rolev1.OwnershipTransferReceipt
	if prepared {
		binding = seedOwnershipProofConfirmed(t, st)
		preparedReceipt = ownershipRolePreparedReceiptFixture(binding)
		_, err := st.MarkOwnershipPrepared(context.Background(), binding, preparedReceipt)
		require.NoError(t, err)
	} else {
		binding = seedOwnershipJournalBinding(t, st)
		_, err := st.ReserveOwnership(context.Background(), binding)
		require.NoError(t, err)
	}
	journal, err := st.DecideOwnershipAbort(context.Background(), binding)
	require.NoError(t, err)
	return binding, preparedReceipt, journal
}

func requireTerminalEvidence(t *testing.T, st *SpaceStore, binding OwnershipBinding, state string, receipt *rolev1.OwnershipTransferReceipt) {
	t.Helper()
	wantBytes := deterministicProtoBytes(t, receipt)
	wantHash := sha256.Sum256(wantBytes)
	var gotState string
	var gotBytes, gotHash []byte
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT state,role_terminal_receipt_bytes,role_terminal_receipt_hash FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&gotState, &gotBytes, &gotHash))
	require.Equal(t, state, gotState)
	require.Equal(t, wantBytes, gotBytes)
	require.Equal(t, wantHash[:], gotHash)
	var restored rolev1.OwnershipTransferReceipt
	require.NoError(t, proto.Unmarshal(gotBytes, &restored))
	require.Equal(t, receipt.ProtoReflect().GetUnknown(), restored.ProtoReflect().GetUnknown())
	require.Equal(t, receipt.GetIntent().ProtoReflect().GetUnknown(), restored.GetIntent().ProtoReflect().GetUnknown())
}

func ownershipTerminalEffectCounts(t *testing.T, st *SpaceStore, journal *OwnershipJournal) (int, int, int) {
	t.Helper()
	var audits, ready, outbox int
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT
		(SELECT count(*) FROM audit_log WHERE id=$1 AND space_id=$2 AND actor_profile_id=$3 AND action='ownership_transferred' AND target_type='profile' AND target_id=$4 AND details='{}'::jsonb),
		(SELECT count(*) FROM ownership_outbox WHERE event_id=$5 AND operation_id=$6 AND space_id=$2 AND previous_owner_profile_id=$3 AND new_owner_profile_id=$4 AND event_type='space.updated' AND ready),
		(SELECT count(*) FROM ownership_outbox WHERE operation_id=$6)`, journal.AuditID, journal.Binding.SpaceID, journal.Binding.ActorProfileID, journal.Binding.NewOwnerProfileID, journal.EventID, journal.Binding.OperationID).Scan(&audits, &ready, &outbox))
	return audits, ready, outbox
}

func ownershipTerminalSnapshot(t *testing.T, st *SpaceStore, binding OwnershipBinding) (string, []byte, []byte, int, int, int) {
	t.Helper()
	var state string
	var receiptBytes, receiptHash []byte
	var audits, ready, outbox int
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT j.state,j.role_terminal_receipt_bytes,j.role_terminal_receipt_hash,
		(SELECT count(*) FROM audit_log a WHERE a.id=j.audit_id),
		(SELECT count(*) FROM ownership_outbox o WHERE o.event_id=j.event_id AND o.ready),
		(SELECT count(*) FROM ownership_outbox o WHERE o.operation_id=j.operation_id)
		FROM ownership_journal j WHERE j.operation_id=$1`, binding.OperationID).Scan(&state, &receiptBytes, &receiptHash, &audits, &ready, &outbox))
	return state, receiptBytes, receiptHash, audits, ready, outbox
}

func seedOwnershipTerminalState(t *testing.T, st *SpaceStore, state string) (OwnershipBinding, *rolev1.OwnershipTransferReceipt) {
	t.Helper()
	var binding OwnershipBinding
	var prepared *rolev1.OwnershipTransferReceipt
	var err error
	switch state {
	case "reserved":
		binding = seedOwnershipJournalBinding(t, st)
		_, err = st.ReserveOwnership(context.Background(), binding)
	case "proof_confirmed":
		binding = seedOwnershipProofConfirmed(t, st)
	case "prepared":
		binding = seedOwnershipProofConfirmed(t, st)
		prepared = ownershipRolePreparedReceiptFixture(binding)
		_, err = st.MarkOwnershipPrepared(context.Background(), binding, prepared)
	case "commit_decided", "completed":
		binding, prepared, _ = seedOwnershipCommitDecided(t, st)
		if state == "completed" {
			_, err = invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipCommit", binding, ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, nil))
		}
	case "abort_decided", "aborted":
		binding, prepared, _ = seedOwnershipAbortDecided(t, st, true)
		if state == "aborted" {
			_, err = invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipAbort", binding, ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED, nil))
		}
	default:
		t.Fatalf("unsupported ownership state %q", state)
	}
	require.NoError(t, err)
	return binding, prepared
}

func TestOwnershipJournalCompletion_FinalizedReceiptPersistsExactTerminalHistoryAcrossRestart(t *testing.T) {
	st := ownershipJournalCompletionStoreFixture(t)
	binding, prepared, decided := seedOwnershipCommitDecided(t, st)
	unknown := protowire.AppendVarint(protowire.AppendTag(nil, 120, protowire.VarintType), 41)
	receipt := ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, unknown)

	completed, err := invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipCommit", binding, receipt)
	require.NoError(t, err)
	require.Equal(t, "completed", completed.State)
	require.Equal(t, decided.AuditID, completed.AuditID)
	require.Equal(t, decided.EventID, completed.EventID)
	assertOwnershipPreparedReceipt(t, completed, prepared)
	requireTerminalEvidence(t, st, binding, "completed", receipt)
	audits, ready, outbox := ownershipTerminalEffectCounts(t, st, completed)
	require.Equal(t, []int{1, 1, 1}, []int{audits, ready, outbox})

	reader, err := pgxpool.NewWithConfig(context.Background(), st.Pool.Config())
	require.NoError(t, err)
	t.Cleanup(reader.Close)
	reloaded, err := (&SpaceStore{Pool: reader}).LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, err)
	require.Equal(t, "completed", reloaded.State)
	require.Equal(t, completed.AuditID, reloaded.AuditID)
	require.Equal(t, completed.EventID, reloaded.EventID)
	requireTerminalEvidence(t, &SpaceStore{Pool: reader}, binding, "completed", receipt)
}

func TestOwnershipJournalCompletion_AbortedReceiptWithAndWithoutPrepareHasNoSuccessEffects(t *testing.T) {
	for _, withPrepare := range []bool{true, false} {
		t.Run(map[bool]string{true: "after_prepare", false: "before_prepare"}[withPrepare], func(t *testing.T) {
			st := ownershipJournalCompletionStoreFixture(t)
			binding, prepared, decided := seedOwnershipAbortDecided(t, st, withPrepare)
			receipt := ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED, protowire.AppendString(protowire.AppendTag(nil, 121, protowire.BytesType), "terminal-wrapper"))
			aborted, err := invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipAbort", binding, receipt)
			require.NoError(t, err)
			require.Equal(t, "aborted", aborted.State)
			if withPrepare {
				assertOwnershipPreparedReceipt(t, aborted, prepared)
			} else {
				require.Nil(t, aborted.RoleReceipt)
			}
			requireTerminalEvidence(t, st, binding, "aborted", receipt)
			audits, ready, outbox := ownershipTerminalEffectCounts(t, st, decided)
			require.Equal(t, []int{0, 0, 0}, []int{audits, ready, outbox})
			var owner uuid.UUID
			require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT owner_profile_id FROM spaces WHERE id=$1`, binding.SpaceID).Scan(&owner))
			require.Equal(t, binding.ActorProfileID, owner)
		})
	}
}

func TestOwnershipJournalCompletion_AbortRequiresSameTransactionLiveOldOwnerGuard(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "changed", true: "missing"}[missing], func(t *testing.T) {
			st := ownershipJournalCompletionStoreFixture(t)
			binding, prepared, decided := seedOwnershipAbortDecided(t, st, true)
			if missing {
				_, err := st.Pool.Exec(context.Background(), `DELETE FROM spaces WHERE id=$1`, binding.SpaceID)
				require.NoError(t, err)
			} else {
				_, err := st.Pool.Exec(context.Background(), `UPDATE spaces SET owner_profile_id=$2 WHERE id=$1`, binding.SpaceID, uuid.New())
				require.NoError(t, err)
			}
			receipt := ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED, nil)
			got, err := invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipAbort", binding, receipt)
			require.Error(t, err)
			require.Nil(t, got)
			var state string
			var terminalBytes []byte
			require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT state,role_terminal_receipt_bytes FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&state, &terminalBytes))
			require.Equal(t, "abort_decided", state)
			require.Nil(t, terminalBytes)
			audits, ready, outbox := ownershipTerminalEffectCounts(t, st, decided)
			require.Equal(t, []int{0, 0, 0}, []int{audits, ready, outbox})
		})
	}
}

func TestOwnershipJournalCompletion_AbortOwnerGuardLinearizesBeforeTerminalWrite(t *testing.T) {
	st := ownershipJournalCompletionStoreFixture(t)
	binding, prepared, _ := seedOwnershipAbortDecided(t, st, true)
	blocker, err := st.Pool.Begin(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = blocker.Rollback(context.Background()) })
	_, err = blocker.Exec(context.Background(), `SELECT 1 FROM spaces WHERE id=$1 FOR UPDATE`, binding.SpaceID)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	workerPool := r22SecondSpacePool(t, ctx, st.Pool, "r20_abort_owner_guard")
	result := make(chan ownershipDecisionResult, 1)
	go func() {
		journal, callErr := invokeOwnershipTerminal(&SpaceStore{Pool: workerPool}, ctx, "CompleteOwnershipAbort", binding, ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED, nil))
		result <- ownershipDecisionResult{journal: journal, err: callErr}
	}()
	r22WaitForSpaceLock(t, ctx, st.Pool, "r20_abort_owner_guard")
	_, err = blocker.Exec(ctx, `UPDATE spaces SET owner_profile_id=$2 WHERE id=$1`, binding.SpaceID, uuid.New())
	require.NoError(t, err)
	require.NoError(t, blocker.Commit(ctx))
	got := waitOwnershipDecision(t, result)
	require.Error(t, got.err)
	require.Nil(t, got.journal)
	var state string
	var terminalBytes []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,role_terminal_receipt_bytes FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&state, &terminalBytes))
	require.Equal(t, "abort_decided", state)
	require.Nil(t, terminalBytes)
}

func TestOwnershipJournalCompletion_RejectsInvalidTerminalReceiptsBeforeDatabaseAccess(t *testing.T) {
	binding := ownershipJournalBindingFixture()
	valid := ownershipTerminalReceipt(binding, nil, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, nil)
	type invalidReceiptCase struct {
		receipt *rolev1.OwnershipTransferReceipt
		want    error
	}
	cases := map[string]invalidReceiptCase{
		"nil":                   {nil, ErrOwnershipRoleReceiptInvalid},
		"unspecified":           {ownershipTerminalReceipt(binding, nil, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_UNSPECIFIED, nil), ErrOwnershipRoleReceiptInvalid},
		"prepared":              {ownershipTerminalReceipt(binding, nil, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_PREPARED, nil), ErrOwnershipRoleReceiptInvalid},
		"wrong protocol":        {proto.Clone(valid).(*rolev1.OwnershipTransferReceipt), ErrOwnershipRoleReceiptInvalid},
		"malformed space":       {proto.Clone(valid).(*rolev1.OwnershipTransferReceipt), ErrOwnershipRoleReceiptInvalid},
		"wrong space":           {proto.Clone(valid).(*rolev1.OwnershipTransferReceipt), ErrOwnershipConflict},
		"wrong old owner":       {proto.Clone(valid).(*rolev1.OwnershipTransferReceipt), ErrOwnershipConflict},
		"wrong new owner":       {proto.Clone(valid).(*rolev1.OwnershipTransferReceipt), ErrOwnershipConflict},
		"wrong operation":       {proto.Clone(valid).(*rolev1.OwnershipTransferReceipt), ErrOwnershipConflict},
		"wrong current owner":   {proto.Clone(valid).(*rolev1.OwnershipTransferReceipt), ErrOwnershipRoleReceiptInvalid},
		"missing current owner": {proto.Clone(valid).(*rolev1.OwnershipTransferReceipt), ErrOwnershipRoleReceiptInvalid},
	}
	cases["wrong protocol"].receipt.Intent.ProtocolVersion = 1
	cases["malformed space"].receipt.Intent.SpaceId = "not-a-uuid"
	cases["wrong space"].receipt.Intent.SpaceId = uuid.NewString()
	cases["wrong old owner"].receipt.Intent.OldOwnerProfileId = uuid.NewString()
	cases["wrong new owner"].receipt.Intent.NewOwnerProfileId = uuid.NewString()
	cases["wrong operation"].receipt.Intent.OperationId = uuid.NewString()
	cases["wrong current owner"].receipt.CurrentOwnerProfileId = uuid.NewString()
	cases["missing current owner"].receipt.CurrentOwnerProfileId = ""
	for name, testCase := range cases {
		for _, method := range []string{"CompleteOwnershipCommit", "CompleteOwnershipAbort"} {
			t.Run(method+"/"+name, func(t *testing.T) {
				got, err := invokeOwnershipTerminal(&SpaceStore{}, context.Background(), method, binding, testCase.receipt)
				require.ErrorIs(t, err, testCase.want)
				require.Nil(t, got)
				require.NotContains(t, err.Error(), "pool not configured", "receipt validation must precede Pool access")
			})
		}
	}
	invalidBinding := binding
	invalidBinding.OperationID = uuid.Nil
	got, err := invokeOwnershipTerminal(&SpaceStore{}, context.Background(), "CompleteOwnershipCommit", invalidBinding, valid)
	require.ErrorIs(t, err, ErrOwnershipBindingInvalid)
	require.Nil(t, got)
	require.NotContains(t, err.Error(), "pool not configured")
}

func TestOwnershipJournalCompletion_RequiresExactPreparedIntentIncludingUnknownOrdering(t *testing.T) {
	st := ownershipJournalCompletionStoreFixture(t)
	binding := seedOwnershipProofConfirmed(t, st)
	prepared := ownershipRolePreparedReceiptFixture(binding)
	unknownA := protowire.AppendString(protowire.AppendTag(nil, 100, protowire.BytesType), "first")
	unknownB := protowire.AppendVarint(protowire.AppendTag(nil, 101, protowire.VarintType), 9)
	prepared.Intent.ProtoReflect().SetUnknown(append(unknownA, unknownB...))
	_, err := st.MarkOwnershipPrepared(context.Background(), binding, prepared)
	require.NoError(t, err)
	_, err = st.DecideOwnershipCommit(context.Background(), binding)
	require.NoError(t, err)

	for name, mutate := range map[string]func(*rolev1.OwnershipTransferReceipt){
		"drop": func(r *rolev1.OwnershipTransferReceipt) { r.Intent.ProtoReflect().SetUnknown(nil) },
		"change": func(r *rolev1.OwnershipTransferReceipt) {
			r.Intent.ProtoReflect().SetUnknown(append(unknownA, protowire.AppendVarint(protowire.AppendTag(nil, 101, protowire.VarintType), 10)...))
		},
		"add": func(r *rolev1.OwnershipTransferReceipt) {
			r.Intent.ProtoReflect().SetUnknown(append(r.Intent.ProtoReflect().GetUnknown(), unknownA...))
		},
		"reorder": func(r *rolev1.OwnershipTransferReceipt) {
			r.Intent.ProtoReflect().SetUnknown(append(append([]byte(nil), unknownB...), unknownA...))
		},
	} {
		t.Run(name, func(t *testing.T) {
			receipt := ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, nil)
			mutate(receipt)
			got, callErr := invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipCommit", binding, receipt)
			require.Error(t, callErr)
			require.Nil(t, got)
			var state string
			require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT state FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&state))
			require.Equal(t, "commit_decided", state)
		})
	}
}

func TestOwnershipJournalCompletion_AbortBeforePrepareRejectsUnknownIntent(t *testing.T) {
	st := ownershipJournalCompletionStoreFixture(t)
	binding, _, _ := seedOwnershipAbortDecided(t, st, false)
	receipt := ownershipTerminalReceipt(binding, nil, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED, nil)
	receipt.Intent.ProtoReflect().SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 100, protowire.VarintType), 1))
	got, err := invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipAbort", binding, receipt)
	require.Error(t, err)
	require.Nil(t, got)
	var state string
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT state FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&state))
	require.Equal(t, "abort_decided", state)
}

func TestOwnershipJournalCompletion_ReloadRejectsSelfConsistentSemanticallyInvalidTerminalReceipt(t *testing.T) {
	for _, corruption := range []string{"binding", "state", "current_owner", "prepared_intent", "canonical_intent"} {
		t.Run(corruption, func(t *testing.T) {
			st := ownershipJournalCompletionStoreFixture(t)
			var binding OwnershipBinding
			var prepared *rolev1.OwnershipTransferReceipt
			var receipt *rolev1.OwnershipTransferReceipt
			if corruption == "canonical_intent" {
				binding, _, _ = seedOwnershipAbortDecided(t, st, false)
				receipt = ownershipTerminalReceipt(binding, nil, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED, nil)
				_, err := invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipAbort", binding, receipt)
				require.NoError(t, err)
			} else {
				binding, prepared, _ = seedOwnershipCommitDecided(t, st)
				receipt = ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, nil)
				_, err := invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipCommit", binding, receipt)
				require.NoError(t, err)
			}

			corrupt := proto.Clone(receipt).(*rolev1.OwnershipTransferReceipt)
			switch corruption {
			case "binding":
				corrupt.Intent.OperationId = uuid.NewString()
			case "state":
				corrupt.State = rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED
			case "current_owner":
				corrupt.CurrentOwnerProfileId = uuid.NewString()
			case "prepared_intent":
				corrupt.Intent.ProtoReflect().SetUnknown(nil)
			case "canonical_intent":
				corrupt.Intent.ProtoReflect().SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 127, protowire.VarintType), 1))
			}
			corruptBytes := deterministicProtoBytes(t, corrupt)
			corruptHash := sha256.Sum256(corruptBytes)

			_, err := st.Pool.Exec(context.Background(), `ALTER TABLE ownership_journal DISABLE TRIGGER USER`)
			require.NoError(t, err)
			triggersDisabled := true
			t.Cleanup(func() {
				if triggersDisabled {
					_, enableErr := st.Pool.Exec(context.Background(), `ALTER TABLE ownership_journal ENABLE TRIGGER USER`)
					require.NoError(t, enableErr)
				}
			})
			_, err = st.Pool.Exec(context.Background(), `UPDATE ownership_journal SET
				role_terminal_receipt_bytes=$2,role_terminal_receipt_hash=$3 WHERE operation_id=$1`,
				binding.OperationID, corruptBytes, corruptHash[:])
			require.NoError(t, err, "test fixture must persist a hash-consistent corrupt receipt")
			_, err = st.Pool.Exec(context.Background(), `ALTER TABLE ownership_journal ENABLE TRIGGER USER`)
			require.NoError(t, err)
			triggersDisabled = false
			var storedBytes, storedHash []byte
			require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT role_terminal_receipt_bytes,role_terminal_receipt_hash
				FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&storedBytes, &storedHash))
			require.Equal(t, corruptBytes, storedBytes)
			require.Equal(t, corruptHash[:], storedHash)

			reader, err := pgxpool.NewWithConfig(context.Background(), st.Pool.Config())
			require.NoError(t, err)
			t.Cleanup(reader.Close)
			loaded, loadErr := (&SpaceStore{Pool: reader}).LoadOwnership(context.Background(), binding.OperationID)
			require.Error(t, loadErr, "terminal receipt semantic corruption must fail closed on reload")
			require.Nil(t, loaded)
		})
	}
}

func TestOwnershipJournalCompletion_ExactWrapperReplayIsReadOnlyAndChangedWrapperConflicts(t *testing.T) {
	for _, terminal := range []string{"commit", "abort"} {
		t.Run(terminal, func(t *testing.T) {
			st := ownershipJournalCompletionStoreFixture(t)
			var binding OwnershipBinding
			var prepared *rolev1.OwnershipTransferReceipt
			method := "CompleteOwnershipCommit"
			roleState := rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED
			if terminal == "commit" {
				binding, prepared, _ = seedOwnershipCommitDecided(t, st)
			} else {
				binding, prepared, _ = seedOwnershipAbortDecided(t, st, true)
				method = "CompleteOwnershipAbort"
				roleState = rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED
			}
			unknown := protowire.AppendString(protowire.AppendTag(nil, 123, protowire.BytesType), "accepted-wrapper")
			receipt := ownershipTerminalReceipt(binding, prepared, roleState, unknown)
			first, err := invokeOwnershipTerminal(st, context.Background(), method, binding, receipt)
			require.NoError(t, err)
			beforeBytes := deterministicProtoBytes(t, receipt)
			beforeHash := sha256.Sum256(beforeBytes)
			beforeAudits, beforeReady, beforeOutbox := ownershipTerminalEffectCounts(t, st, first)

			replay, err := invokeOwnershipTerminal(st, context.Background(), method, binding, proto.Clone(receipt).(*rolev1.OwnershipTransferReceipt))
			require.NoError(t, err)
			require.Equal(t, first, replay)
			audits, ready, outbox := ownershipTerminalEffectCounts(t, st, replay)
			require.Equal(t, []int{beforeAudits, beforeReady, beforeOutbox}, []int{audits, ready, outbox})

			for name, mutate := range map[string]func(*rolev1.OwnershipTransferReceipt){
				"wrapper": func(changed *rolev1.OwnershipTransferReceipt) {
					changed.ProtoReflect().SetUnknown(protowire.AppendString(protowire.AppendTag(nil, 123, protowire.BytesType), "changed-wrapper"))
				},
				"intent body": func(changed *rolev1.OwnershipTransferReceipt) {
					changed.Intent.ProtoReflect().SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 124, protowire.VarintType), 1))
				},
			} {
				t.Run(name, func(t *testing.T) {
					changed := proto.Clone(receipt).(*rolev1.OwnershipTransferReceipt)
					mutate(changed)
					got, callErr := invokeOwnershipTerminal(st, context.Background(), method, binding, changed)
					require.Error(t, callErr)
					require.Nil(t, got)
					var storedBytes, storedHash []byte
					require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT role_terminal_receipt_bytes,role_terminal_receipt_hash FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&storedBytes, &storedHash))
					require.Equal(t, beforeBytes, storedBytes)
					require.Equal(t, beforeHash[:], storedHash)
				})
			}
		})
	}
}

func TestOwnershipJournalCompletion_OppositeAndPrematureTransitionsCannotCreateTerminalEvidence(t *testing.T) {
	states := []string{"reserved", "proof_confirmed", "prepared", "commit_decided", "abort_decided", "completed", "aborted"}
	actions := []struct {
		name, method, decision, terminal string
		roleState                        rolev1.OwnershipTransferState
	}{
		{"commit", "CompleteOwnershipCommit", "commit_decided", "completed", rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED},
		{"abort", "CompleteOwnershipAbort", "abort_decided", "aborted", rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED},
	}
	for _, state := range states {
		for _, action := range actions {
			t.Run(state+"/"+action.name, func(t *testing.T) {
				st := ownershipJournalCompletionStoreFixture(t)
				binding, prepared := seedOwnershipTerminalState(t, st, state)
				receipt := ownershipTerminalReceipt(binding, prepared, action.roleState, nil)
				beforeState, beforeBytes, beforeHash, beforeAudits, beforeReady, beforeOutbox := ownershipTerminalSnapshot(t, st, binding)

				got, err := invokeOwnershipTerminal(st, context.Background(), action.method, binding, receipt)
				if state == action.decision || state == action.terminal {
					require.NoError(t, err)
					require.Equal(t, action.terminal, got.State)
					if state == action.terminal {
						afterState, afterBytes, afterHash, afterAudits, afterReady, afterOutbox := ownershipTerminalSnapshot(t, st, binding)
						require.Equal(t, []any{beforeState, beforeBytes, beforeHash, beforeAudits, beforeReady, beforeOutbox}, []any{afterState, afterBytes, afterHash, afterAudits, afterReady, afterOutbox}, "terminal replay must be read-only")
					}
					return
				}
				require.ErrorIs(t, err, ErrOwnershipStateTransition)
				require.Nil(t, got)
				afterState, afterBytes, afterHash, afterAudits, afterReady, afterOutbox := ownershipTerminalSnapshot(t, st, binding)
				require.Equal(t, []any{beforeState, beforeBytes, beforeHash, beforeAudits, beforeReady, beforeOutbox}, []any{afterState, afterBytes, afterHash, afterAudits, afterReady, afterOutbox})
			})
		}
	}
}

func TestOwnershipJournalCompletion_ConcurrentIdenticalAndOppositeCallsHaveOneHistory(t *testing.T) {
	for _, terminal := range []string{"commit", "abort"} {
		t.Run(terminal, func(t *testing.T) {
			st := ownershipJournalCompletionStoreFixture(t)
			var binding OwnershipBinding
			var prepared *rolev1.OwnershipTransferReceipt
			var decided *OwnershipJournal
			method := "CompleteOwnershipCommit"
			oppositeMethod := "CompleteOwnershipAbort"
			roleState := rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED
			oppositeState := rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED
			wantState := "completed"
			wantEffects := []int{1, 1, 1}
			if terminal == "commit" {
				binding, prepared, decided = seedOwnershipCommitDecided(t, st)
			} else {
				binding, prepared, decided = seedOwnershipAbortDecided(t, st, true)
				method, oppositeMethod = oppositeMethod, method
				roleState, oppositeState = oppositeState, roleState
				wantState = "aborted"
				wantEffects = []int{0, 0, 0}
			}

			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			blocker, err := st.Pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = blocker.Rollback(context.Background()) }()
			_, err = blocker.Exec(ctx, `SELECT 1 FROM ownership_journal WHERE operation_id=$1 FOR UPDATE`, binding.OperationID)
			require.NoError(t, err)

			start := make(chan struct{})
			results := make(chan ownershipDecisionResult, 3)
			var launched sync.WaitGroup
			launched.Add(3)
			pools := make([]*pgxpool.Pool, 3)
			for index := range pools {
				pools[index] = r22SecondSpacePool(t, ctx, st.Pool, "r20_terminal_"+terminal+"_race_"+string(rune('a'+index)))
			}
			for index := 0; index < 3; index++ {
				index := index
				go func() {
					launched.Done()
					<-start
					callMethod, callState := method, roleState
					if index == 2 {
						callMethod, callState = oppositeMethod, oppositeState
					}
					journal, callErr := invokeOwnershipTerminal(&SpaceStore{Pool: pools[index]}, ctx, callMethod, binding, ownershipTerminalReceipt(binding, prepared, callState, nil))
					results <- ownershipDecisionResult{journal: journal, err: callErr}
				}()
			}
			launched.Wait()
			close(start)
			r22WaitForSpaceLock(t, ctx, st.Pool, "r20_terminal_"+terminal+"_race_a")
			require.NoError(t, blocker.Commit(ctx))

			success := 0
			for i := 0; i < 3; i++ {
				result := waitOwnershipDecision(t, results)
				if result.err == nil {
					success++
					require.Equal(t, wantState, result.journal.State)
				} else {
					require.ErrorIs(t, result.err, ErrOwnershipStateTransition)
					require.Nil(t, result.journal)
				}
			}
			require.Equal(t, 2, success, "both identical completion calls replay one history; opposite completion fails")
			audits, readyCount, outbox := ownershipTerminalEffectCounts(t, st, decided)
			require.Equal(t, wantEffects, []int{audits, readyCount, outbox})
		})
	}
}

func TestOwnershipJournalCompletion_AmbiguousCommitResponseReloadsDurableTerminalOutcome(t *testing.T) {
	st := ownershipJournalCompletionStoreFixture(t)
	binding, prepared, decided := seedOwnershipCommitDecided(t, st)
	proxy := startOwnershipCommitAmbiguityProxy(t, st.Pool.Config().ConnConfig.Host, st.Pool.Config().ConnConfig.Port)
	ambiguousStore := &SpaceStore{Pool: ownershipCommitAmbiguityPool(t, st.Pool, proxy)}
	proxy.arm()
	result := make(chan ownershipDecisionResult, 1)
	go func() {
		journal, callErr := invokeOwnershipTerminal(ambiguousStore, context.Background(), "CompleteOwnershipCommit", binding, ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, nil))
		result <- ownershipDecisionResult{journal: journal, err: callErr}
	}()
	select {
	case <-proxy.persisted:
	case <-time.After(5 * time.Second):
		t.Fatal("PostgreSQL did not durably commit terminal completion before the response-loss boundary")
	}
	proxy.dropCommitResponse()
	call := waitOwnershipDecision(t, result)
	require.NoError(t, call.err)
	require.Equal(t, "completed", call.journal.State)
	require.Equal(t, decided.AuditID, call.journal.AuditID)
	require.Equal(t, decided.EventID, call.journal.EventID)
	requireTerminalEvidence(t, st, binding, "completed", ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, nil))
	audits, ready, outbox := ownershipTerminalEffectCounts(t, st, decided)
	require.Equal(t, []int{1, 1, 1}, []int{audits, ready, outbox})
}

func TestOwnershipJournalCompletion_InjectedStatementFailuresRollbackEveryTerminalEffect(t *testing.T) {
	for _, failure := range []struct{ name, sql string }{
		{"audit", `CREATE FUNCTION r20_fail_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'r20 audit failure'; END $$; CREATE TRIGGER r20_fail_audit BEFORE INSERT ON audit_log FOR EACH ROW EXECUTE FUNCTION r20_fail_audit()`},
		{"outbox", `CREATE FUNCTION r20_fail_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.ready THEN RAISE EXCEPTION 'r20 outbox failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER r20_fail_outbox BEFORE UPDATE ON ownership_outbox FOR EACH ROW EXECUTE FUNCTION r20_fail_outbox()`},
		{"journal", `CREATE FUNCTION r20_fail_journal() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.state='completed' THEN RAISE EXCEPTION 'r20 journal failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER r20_fail_journal BEFORE UPDATE ON ownership_journal FOR EACH ROW EXECUTE FUNCTION r20_fail_journal()`},
	} {
		t.Run(failure.name, func(t *testing.T) {
			st := ownershipJournalCompletionStoreFixture(t)
			binding, prepared, decided := seedOwnershipCommitDecided(t, st)
			_, err := st.Pool.Exec(context.Background(), failure.sql)
			require.NoError(t, err)
			got, err := invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipCommit", binding, ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, nil))
			require.Error(t, err)
			require.Nil(t, got)
			var state string
			var terminalBytes []byte
			require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT state,role_terminal_receipt_bytes FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&state, &terminalBytes))
			require.Equal(t, "commit_decided", state)
			require.Nil(t, terminalBytes)
			audits, ready, outbox := ownershipTerminalEffectCounts(t, st, decided)
			require.Equal(t, []int{0, 0, 1}, []int{audits, ready, outbox})
		})
	}
}

func TestOwnershipJournalCompletion_ScannerReturnsDefensiveTerminalEvidenceCopies(t *testing.T) {
	st := ownershipJournalCompletionStoreFixture(t)
	binding, prepared, _ := seedOwnershipCommitDecided(t, st)
	receipt := ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, []byte{0xc0, 0x07, 0x01})
	_, err := invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipCommit", binding, receipt)
	require.NoError(t, err)
	first, err := st.LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, err)
	second, err := st.LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, err)
	firstAccessor, ok := any(first).(ownershipTerminalEvidenceAccessor)
	require.True(t, ok, "OwnershipJournal must expose typed terminal evidence")
	secondAccessor, ok := any(second).(ownershipTerminalEvidenceAccessor)
	require.True(t, ok, "OwnershipJournal must expose typed terminal evidence")
	firstBytes, firstHash, firstOK := firstAccessor.OwnershipTerminalReceiptEvidence()
	secondBytes, secondHash, secondOK := secondAccessor.OwnershipTerminalReceiptEvidence()
	require.True(t, firstOK)
	require.True(t, secondOK)
	original := append([]byte(nil), firstBytes...)
	firstBytes[0] ^= 0xff
	require.True(t, bytes.Equal(original, secondBytes))
	require.Equal(t, firstHash, secondHash)
}
