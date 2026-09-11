package store

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
)

func ownershipCompletionMigrationBase(t *testing.T) *SpaceStore {
	t.Helper()
	st := ownershipJournalCommitStoreFixture(t)
	applyR22SpaceEpochMigration(t, context.Background(), st)
	return st
}

func requireOwnershipCompletionSchema(t *testing.T, pool *pgxpool.Pool, present bool) {
	t.Helper()
	var columns, constraints bool
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT
		(SELECT count(*)=2 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='ownership_journal' AND column_name IN ('role_terminal_receipt_bytes','role_terminal_receipt_hash')),
		(SELECT count(*)>=4 FROM information_schema.table_constraints WHERE table_schema=current_schema() AND table_name='ownership_journal' AND constraint_name IN (
			'ownership_journal_terminal_receipt_all_or_none','ownership_journal_terminal_receipt_state',
			'ownership_journal_completed_evidence','ownership_journal_aborted_evidence'))`).Scan(&columns, &constraints))
	require.Equal(t, present, columns)
	require.Equal(t, present, constraints)
}

func execOwnershipCompletionDown(t *testing.T, ctx context.Context, pool *pgxpool.Pool) error {
	t.Helper()
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, ownershipJournalCompletionMigrationSQL(t, "down"))
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if _, rollbackErr := conn.Exec(cleanup, "ROLLBACK"); rollbackErr != nil {
			_ = conn.Conn().Close(cleanup)
		}
	}
	return err
}

func TestOwnershipJournalCompletionMigration_AddsExactTerminalEvidenceAfter000011(t *testing.T) {
	st := ownershipCompletionMigrationBase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var beforeEpochTables bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT to_regclass('space_voice_access_epochs') IS NOT NULL AND to_regclass('space_voice_access_outbox') IS NOT NULL`).Scan(&beforeEpochTables))
	require.True(t, beforeEpochTables)
	_, err := st.Pool.Exec(ctx, ownershipJournalCompletionMigrationSQL(t, "up"))
	require.NoError(t, err)
	requireOwnershipCompletionSchema(t, st.Pool, true)
	var afterEpochTables bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT to_regclass('space_voice_access_epochs') IS NOT NULL AND to_regclass('space_voice_access_outbox') IS NOT NULL`).Scan(&afterEpochTables))
	require.True(t, afterEpochTables)
}

func TestOwnershipJournalCompletionMigration_PreterminalDownUpPreservesAllPriorEvidence(t *testing.T) {
	st := ownershipCompletionMigrationBase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	binding := seedOwnershipProofConfirmed(t, st)
	prepared := ownershipRolePreparedReceiptFixture(binding)
	_, err := st.MarkOwnershipPrepared(ctx, binding, prepared)
	require.NoError(t, err)
	before, err := st.DecideOwnershipCommit(ctx, binding)
	require.NoError(t, err)
	var epoch int64
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT access_epoch FROM space_voice_access_epochs WHERE space_id=$1`, binding.SpaceID).Scan(&epoch))

	_, err = st.Pool.Exec(ctx, ownershipJournalCompletionMigrationSQL(t, "up"))
	require.NoError(t, err)
	require.NoError(t, execOwnershipCompletionDown(t, ctx, st.Pool))
	requireOwnershipCompletionSchema(t, st.Pool, false)
	afterDown, err := st.LoadOwnership(ctx, binding.OperationID)
	require.NoError(t, err)
	require.Equal(t, before, afterDown)
	var ready bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT ready FROM ownership_outbox WHERE operation_id=$1`, binding.OperationID).Scan(&ready))
	require.False(t, ready)
	var afterEpoch int64
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT access_epoch FROM space_voice_access_epochs WHERE space_id=$1`, binding.SpaceID).Scan(&afterEpoch))
	require.Equal(t, epoch, afterEpoch)

	_, err = st.Pool.Exec(ctx, ownershipJournalCompletionMigrationSQL(t, "up"))
	require.NoError(t, err)
	requireOwnershipCompletionSchema(t, st.Pool, true)
	var terminalBytes []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT role_terminal_receipt_bytes FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&terminalBytes))
	require.Nil(t, terminalBytes)
}

func TestOwnershipJournalCompletionMigration_DownRefusesEveryTerminalEvidenceClass(t *testing.T) {
	for _, evidence := range []string{"completed", "aborted", "ready_outbox", "public_audit"} {
		t.Run(evidence, func(t *testing.T) {
			st := ownershipJournalCompletionStoreFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var binding OwnershipBinding
			var prepared *rolev1.OwnershipTransferReceipt
			var journal *OwnershipJournal
			if evidence == "aborted" {
				binding, prepared, _ = seedOwnershipAbortDecided(t, st, true)
				receipt := ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED, nil)
				_, err := invokeOwnershipTerminal(st, ctx, "CompleteOwnershipAbort", binding, receipt)
				require.NoError(t, err)
			} else {
				binding, prepared, journal = seedOwnershipCommitDecided(t, st)
				receipt := ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, nil)
				switch evidence {
				case "completed":
					_, err := invokeOwnershipTerminal(st, ctx, "CompleteOwnershipCommit", binding, receipt)
					require.NoError(t, err)
				case "ready_outbox":
					_, err := st.Pool.Exec(ctx, `UPDATE ownership_outbox SET ready=true WHERE operation_id=$1`, binding.OperationID)
					require.NoError(t, err)
				case "public_audit":
					_, err := st.Pool.Exec(ctx, `INSERT INTO audit_log(id,space_id,actor_profile_id,action,target_type,target_id,details) VALUES($1,$2,$3,'ownership_transferred','profile',$4,'{}')`, journal.AuditID, binding.SpaceID, binding.ActorProfileID, binding.NewOwnerProfileID)
					require.NoError(t, err)
				}
			}
			var beforeState string
			var beforeTerminal []byte
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,role_terminal_receipt_bytes FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&beforeState, &beforeTerminal))
			err := execOwnershipCompletionDown(t, ctx, st.Pool)
			require.Error(t, err)
			var refusal *pgconn.PgError
			require.ErrorAs(t, err, &refusal)
			require.Equal(t, "P0001", refusal.Code)
			requireOwnershipCompletionSchema(t, st.Pool, true)
			var afterState string
			var afterTerminal []byte
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,role_terminal_receipt_bytes FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&afterState, &afterTerminal))
			require.Equal(t, beforeState, afterState)
			require.Equal(t, beforeTerminal, afterTerminal)
		})
	}
}

func TestOwnershipJournalCompletionMigration_DownRefusesReceiptEvidenceIndependentOfTerminalState(t *testing.T) {
	st := ownershipJournalCompletionStoreFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	binding := seedOwnershipJournalBinding(t, st)
	_, err := st.ReserveOwnership(ctx, binding)
	require.NoError(t, err)
	receipt := ownershipTerminalReceipt(binding, nil, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED, nil)
	receiptBytes := deterministicProtoBytes(t, receipt)
	receiptHash := sha256.Sum256(receiptBytes)

	_, err = st.Pool.Exec(ctx, `ALTER TABLE ownership_journal DROP CONSTRAINT ownership_journal_terminal_receipt_state`)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE ownership_journal SET role_terminal_receipt_bytes=$2,role_terminal_receipt_hash=$3 WHERE operation_id=$1`, binding.OperationID, receiptBytes, receiptHash[:])
	require.NoError(t, err)
	var successEffects int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM ownership_outbox WHERE operation_id=$1 AND ready)+
		(SELECT count(*) FROM audit_log WHERE id=(SELECT audit_id FROM ownership_journal WHERE operation_id=$1))`, binding.OperationID).Scan(&successEffects))
	require.Zero(t, successEffects)

	err = execOwnershipCompletionDown(t, ctx, st.Pool)
	require.Error(t, err)
	var refusal *pgconn.PgError
	require.ErrorAs(t, err, &refusal)
	require.Equal(t, "P0001", refusal.Code)
	var state string
	var storedBytes, storedHash []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,role_terminal_receipt_bytes,role_terminal_receipt_hash FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&state, &storedBytes, &storedHash))
	require.Equal(t, "reserved", state)
	require.Equal(t, receiptBytes, storedBytes)
	require.Equal(t, receiptHash[:], storedHash)

	_, err = st.Pool.Exec(ctx, `ALTER TABLE ownership_journal DISABLE TRIGGER USER`)
	require.NoError(t, err)
	triggersDisabled := true
	t.Cleanup(func() {
		if triggersDisabled {
			_, enableErr := st.Pool.Exec(context.Background(), `ALTER TABLE ownership_journal ENABLE TRIGGER USER`)
			require.NoError(t, enableErr)
		}
	})
	_, err = st.Pool.Exec(ctx, `UPDATE ownership_journal SET role_terminal_receipt_bytes=NULL,role_terminal_receipt_hash=NULL WHERE operation_id=$1`, binding.OperationID)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `ALTER TABLE ownership_journal ENABLE TRIGGER USER`)
	require.NoError(t, err)
	triggersDisabled = false
	_, err = st.Pool.Exec(ctx, `ALTER TABLE ownership_journal ADD CONSTRAINT ownership_journal_terminal_receipt_state CHECK (
		role_terminal_receipt_bytes IS NULL OR state IN ('completed','aborted')
	)`)
	require.NoError(t, err)
	requireOwnershipCompletionSchema(t, st.Pool, true)
}

func waitCompletionDownRelationState(t *testing.T, ctx context.Context, observer *pgxpool.Pool, pid int32, granted []string, waiting string) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		allGranted := true
		for _, relation := range granted {
			var ok bool
			require.NoError(t, observer.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND relation=to_regclass($2) AND mode='AccessExclusiveLock' AND granted)`, pid, relation).Scan(&ok))
			allGranted = allGranted && ok
		}
		var isWaiting bool
		require.NoError(t, observer.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND relation=to_regclass($2) AND mode='AccessExclusiveLock' AND NOT granted)`, pid, waiting).Scan(&isWaiting))
		if allGranted && isWaiting {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("DOWN lock order not observed before %s: %v", waiting, ctx.Err())
		case <-ticker.C:
		}
	}
}

func startCompletionDown(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (int32, <-chan error, func()) {
	t.Helper()
	downSQL := ownershipJournalCompletionMigrationSQL(t, "down")
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	var pid int32
	require.NoError(t, conn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	result := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, downErr := conn.Exec(ctx, downSQL)
		if downErr != nil {
			rollbackCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			_, _ = conn.Exec(rollbackCtx, "ROLLBACK")
			cancel()
		}
		result <- downErr
	}()
	cleanup := func() {
		select {
		case <-done:
			conn.Release()
		case <-time.After(3 * time.Second):
			_ = conn.Conn().Close(context.Background())
			t.Error("completion DOWN worker did not terminate")
		}
	}
	return pid, result, cleanup
}

func TestOwnershipJournalCompletionMigration_DownUsesJournalOutboxAuditLockOrder(t *testing.T) {
	for _, blocker := range []struct {
		name, relation string
		granted        []string
	}{
		{"outbox", "ownership_outbox", []string{"ownership_journal"}},
		{"audit", "audit_log", []string{"ownership_journal", "ownership_outbox"}},
	} {
		t.Run(blocker.name, func(t *testing.T) {
			st := ownershipJournalCompletionStoreFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			lockTx, err := st.Pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = lockTx.Rollback(context.Background()) }()
			_, err = lockTx.Exec(ctx, `LOCK TABLE `+pgx.Identifier{blocker.relation}.Sanitize()+` IN ROW EXCLUSIVE MODE`)
			require.NoError(t, err)
			pid, result, cleanup := startCompletionDown(t, ctx, st.Pool)
			defer cleanup()
			waitCompletionDownRelationState(t, ctx, st.Pool, pid, blocker.granted, blocker.relation)
			require.NoError(t, lockTx.Rollback(ctx))
			require.NoError(t, <-result)
			requireOwnershipCompletionSchema(t, st.Pool, false)
		})
	}
}

func TestOwnershipJournalCompletionMigration_DownWaitsForUncommittedTerminalThenRefusesAndPreserves(t *testing.T) {
	st := ownershipJournalCompletionStoreFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	binding, prepared, journal := seedOwnershipCommitDecided(t, st)
	receipt := ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, []byte{0xc0, 0x07, 0x01})
	receiptBytes := deterministicProtoBytes(t, receipt)
	receiptHash := sha256.Sum256(receiptBytes)

	terminalTx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = terminalTx.Rollback(context.Background()) }()
	_, err = terminalTx.Exec(ctx, `SELECT 1 FROM ownership_journal WHERE operation_id=$1 FOR UPDATE`, binding.OperationID)
	require.NoError(t, err)
	_, err = terminalTx.Exec(ctx, `SELECT 1 FROM ownership_outbox WHERE operation_id=$1 FOR UPDATE`, binding.OperationID)
	require.NoError(t, err)
	_, err = terminalTx.Exec(ctx, `INSERT INTO audit_log(id,space_id,actor_profile_id,action,target_type,target_id,details) VALUES($1,$2,$3,'ownership_transferred','profile',$4,'{}')`, journal.AuditID, binding.SpaceID, binding.ActorProfileID, binding.NewOwnerProfileID)
	require.NoError(t, err)
	_, err = terminalTx.Exec(ctx, `UPDATE ownership_outbox SET ready=true WHERE operation_id=$1`, binding.OperationID)
	require.NoError(t, err)
	_, err = terminalTx.Exec(ctx, `UPDATE ownership_journal SET state='completed',role_terminal_receipt_bytes=$2,role_terminal_receipt_hash=$3 WHERE operation_id=$1`, binding.OperationID, receiptBytes, receiptHash[:])
	require.NoError(t, err)

	pid, result, cleanup := startCompletionDown(t, ctx, st.Pool)
	defer cleanup()
	waitCompletionDownRelationState(t, ctx, st.Pool, pid, nil, "ownership_journal")
	require.NoError(t, terminalTx.Commit(ctx))
	downErr := <-result
	require.Error(t, downErr)
	var refusal *pgconn.PgError
	require.ErrorAs(t, downErr, &refusal)
	require.Equal(t, "P0001", refusal.Code)
	requireOwnershipCompletionSchema(t, st.Pool, true)
	var state string
	var storedBytes, storedHash []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,role_terminal_receipt_bytes,role_terminal_receipt_hash FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&state, &storedBytes, &storedHash))
	require.Equal(t, "completed", state)
	require.Equal(t, receiptBytes, storedBytes)
	require.Equal(t, receiptHash[:], storedHash)
	audits, ready, outbox := ownershipTerminalEffectCounts(t, st, journal)
	require.Equal(t, []int{1, 1, 1}, []int{audits, ready, outbox})
}

func TestOwnershipJournalCompletionMigration_TerminalReceiptEvidenceIsImmutable(t *testing.T) {
	for _, terminal := range []string{"completed", "aborted"} {
		t.Run(terminal, func(t *testing.T) {
			st := ownershipJournalCompletionStoreFixture(t)
			var binding OwnershipBinding
			var prepared *rolev1.OwnershipTransferReceipt
			var journal *OwnershipJournal
			var receipt *rolev1.OwnershipTransferReceipt
			if terminal == "completed" {
				binding, prepared, journal = seedOwnershipCommitDecided(t, st)
				receipt = ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, nil)
				_, err := invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipCommit", binding, receipt)
				require.NoError(t, err)
			} else {
				binding, prepared, journal = seedOwnershipAbortDecided(t, st, true)
				receipt = ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED, nil)
				_, err := invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipAbort", binding, receipt)
				require.NoError(t, err)
			}
			beforeState, beforeBytes, beforeHash, beforeAudits, beforeReady, beforeOutbox := ownershipTerminalSnapshot(t, st, binding)
			replacement := proto.Clone(receipt).(*rolev1.OwnershipTransferReceipt)
			replacement.ProtoReflect().SetUnknown(protowire.AppendString(protowire.AppendTag(nil, 126, protowire.BytesType), "rewritten"))
			replacementBytes := deterministicProtoBytes(t, replacement)
			replacementHash := sha256.Sum256(replacementBytes)

			_, err := st.Pool.Exec(context.Background(), `UPDATE ownership_journal
				SET role_terminal_receipt_bytes=$2,role_terminal_receipt_hash=$3
				WHERE operation_id=$1`, binding.OperationID, replacementBytes, replacementHash[:])
			require.Error(t, err, "terminal receipt bytes and hash must be immutable even when rewritten consistently")
			afterState, afterBytes, afterHash, afterAudits, afterReady, afterOutbox := ownershipTerminalSnapshot(t, st, binding)
			require.Equal(t,
				[]any{beforeState, beforeBytes, beforeHash, beforeAudits, beforeReady, beforeOutbox},
				[]any{afterState, afterBytes, afterHash, afterAudits, afterReady, afterOutbox})
			audits, ready, outbox := ownershipTerminalEffectCounts(t, st, journal)
			require.Equal(t, []int{beforeAudits, beforeReady, beforeOutbox}, []int{audits, ready, outbox})
		})
	}
}

func TestOwnershipJournalCompletionMigration_TerminalOutcomeCannotBeRewritten(t *testing.T) {
	for _, transition := range []string{"completed_to_aborted", "aborted_to_completed"} {
		t.Run(transition, func(t *testing.T) {
			st := ownershipJournalCompletionStoreFixture(t)
			var binding OwnershipBinding
			var prepared *rolev1.OwnershipTransferReceipt
			if transition == "completed_to_aborted" {
				binding, prepared, _ = seedOwnershipCommitDecided(t, st)
				_, err := invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipCommit", binding,
					ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, nil))
				require.NoError(t, err)
			} else {
				binding, prepared, _ = seedOwnershipAbortDecided(t, st, true)
				_, err := invokeOwnershipTerminal(st, context.Background(), "CompleteOwnershipAbort", binding,
					ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED, nil))
				require.NoError(t, err)
			}
			beforeState, beforeBytes, beforeHash, beforeAudits, beforeReady, beforeOutbox := ownershipTerminalSnapshot(t, st, binding)
			var err error
			if transition == "completed_to_aborted" {
				receipt := ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED, nil)
				receiptBytes := deterministicProtoBytes(t, receipt)
				receiptHash := sha256.Sum256(receiptBytes)
				_, err = st.Pool.Exec(context.Background(), `UPDATE ownership_journal SET
					state='aborted',pending_audit_action=NULL,pending_audit_target_type=NULL,
					pending_audit_target_id=NULL,pending_audit_details=NULL,
					role_terminal_receipt_bytes=$2,role_terminal_receipt_hash=$3
					WHERE operation_id=$1`, binding.OperationID, receiptBytes, receiptHash[:])
			} else {
				receipt := ownershipTerminalReceipt(binding, prepared, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, nil)
				receiptBytes := deterministicProtoBytes(t, receipt)
				receiptHash := sha256.Sum256(receiptBytes)
				_, err = st.Pool.Exec(context.Background(), `UPDATE ownership_journal SET
					state='completed',pending_audit_action='ownership_transferred',pending_audit_target_type='profile',
					pending_audit_target_id=new_owner_profile_id,pending_audit_details='{}'::jsonb,
					role_terminal_receipt_bytes=$2,role_terminal_receipt_hash=$3
					WHERE operation_id=$1`, binding.OperationID, receiptBytes, receiptHash[:])
			}
			require.Error(t, err, "a terminal ownership outcome must never be rewritten to its opposite")
			afterState, afterBytes, afterHash, afterAudits, afterReady, afterOutbox := ownershipTerminalSnapshot(t, st, binding)
			require.Equal(t,
				[]any{beforeState, beforeBytes, beforeHash, beforeAudits, beforeReady, beforeOutbox},
				[]any{afterState, afterBytes, afterHash, afterAudits, afterReady, afterOutbox})
		})
	}
}
