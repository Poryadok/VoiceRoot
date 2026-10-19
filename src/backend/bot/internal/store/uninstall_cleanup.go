package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *BotStore) pendingUninstallCleanup(ctx context.Context, tx pgx.Tx, bot, space uuid.UUID) (bool, error) {
	var pending bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bot_uninstall_operations o JOIN bot_uninstall_installations t ON t.operation_id=o.operation_id WHERE t.bot_id=$1 AND t.space_id=$2 AND o.state<>'COMPLETE')`, bot, space).Scan(&pending)
	return pending, err
}

// Only explicitly unfinished uninstall targets participate in discovery. This
// is not a scan/promote of arbitrary canceled installation history.
func (s *BotStore) retainedUninstallTarget(ctx context.Context, tx pgx.Tx, bot, space uuid.UUID) (*BotDeletionTarget, error) {
	rows, err := tx.Query(ctx, `SELECT t.scope_bytes,t.scope_sha256,o.operation_id,o.actor_profile_id,t.target_id FROM bot_uninstall_operations o JOIN bot_uninstall_installations t ON t.operation_id=o.operation_id WHERE t.bot_id=$1 AND t.space_id=$2 AND o.state<>'COMPLETE' FOR UPDATE OF o,t`, bot, space)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result *BotDeletionTarget
	for rows.Next() {
		if result != nil {
			return nil, ErrInstallationConflict
		}
		var raw, digest []byte
		var operation, actor, target uuid.UUID
		if err = rows.Scan(&raw, &digest, &operation, &actor, &target); err != nil {
			return nil, err
		}
		sum := sha256.Sum256(raw)
		if !bytes.Equal(sum[:], digest) {
			return nil, ErrInstallationConflict
		}
		var source BotDeletionTarget
		if json.Unmarshal(raw, &source) != nil {
			return nil, ErrInstallationConflict
		}
		canonical, e := json.Marshal(source)
		if e != nil || !bytes.Equal(canonical, raw) || source.OperationID != operation || source.TargetID != target || source.BotID != bot || source.SpaceID != space || source.ActorProfileID != actor {
			return nil, ErrInstallationConflict
		}
		source.ScopeBytes, source.ScopeSHA256 = raw, digest
		result = &source
	}
	return result, rows.Err()
}

// Atomically retain the local native ACK and retire live installation state.
// The separate Role/Messaging obligations survive accepted Empty cancellation.
func (s *BotStore) SaveUninstallNativeAcknowledgement(ctx context.Context, claim *BotDeletion, effect uuid.UUID, request, observation []byte) error {
	if claim == nil || claim.Purpose != "space_uninstall" || claim.OperationID == uuid.Nil || claim.ClaimGeneration <= 0 || len(request) == 0 || len(request) > 1024*1024 || len(observation) == 0 || len(observation) > 1024*1024 {
		return ErrInstallationConflict
	}
	effects, err := s.LifecycleEffects(ctx, claim)
	if err != nil {
		return err
	}
	var target *BotDeletionTarget
	for _, e := range effects {
		if e.EffectID == effect && e.Kind == "SPACE_CLEANUP" && e.ChatID == nil {
			copy := e.Target
			target = &copy
		}
	}
	if target == nil {
		return ErrInstallationConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = s.lockInstallation(ctx, tx, target.BotID, target.SpaceID, false); err != nil {
		return err
	}
	if err = s.lockLifecycleClaim(ctx, tx, claim); err != nil {
		return err
	}
	var savedRequest, savedReceipt []byte
	err = tx.QueryRow(ctx, `SELECT e.request_bytes,e.receipt_bytes FROM bot_uninstall_effects e JOIN bot_uninstall_installations t ON t.target_id=e.target_id WHERE e.effect_id=$1 AND t.operation_id=$2 AND e.kind='SPACE_CLEANUP' FOR UPDATE OF e`, effect, claim.OperationID).Scan(&savedRequest, &savedReceipt)
	if err != nil {
		return err
	}
	if !bytes.Equal(savedRequest, request) {
		return ErrInstallationConflict
	}
	if savedReceipt != nil {
		if !bytes.Equal(savedReceipt, observation) {
			return ErrInstallationConflict
		}
		return tx.Commit(ctx)
	}
	intent, err := scanInstallationIntent(tx.QueryRow(ctx, `SELECT `+intentColumns+` FROM bot_installation_intents WHERE installation_id=$1 FOR UPDATE`, target.InstallationID))
	if err != nil {
		return err
	}
	if intent.BotID != target.BotID || intent.SpaceID != target.SpaceID || intent.ActorProfileID != target.ActorProfileID || intent.OperationID != target.InstallOperationID || intent.CancelOperationID != target.CancelOperationID || intent.Generation != target.BotGeneration {
		return ErrInstallationConflict
	}
	if err = s.completeInstallationCancellationTx(ctx, tx, intent); err != nil {
		return err
	}
	sum := sha256.Sum256(observation)
	_, err = tx.Exec(ctx, `UPDATE bot_uninstall_effects SET receipt_bytes=$2,receipt_sha256=$3,state='COMPLETE' WHERE effect_id=$1`, effect, observation, sum[:])
	if err != nil {
		return err
	}
	// Local row waits may outlive the worker lease. Recheck after every effect.
	if err = s.lockLifecycleClaim(ctx, tx, claim); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Purpose selects separate physical journals. Global deletion's uniqueness and
// terminal authority are never reused for an ordinary Space uninstall.
func lifecycleQuery(claim *BotDeletion, query string) string {
	if claim != nil && claim.Purpose == "space_uninstall" {
		return strings.ReplaceAll(query, "bot_deletion_", "bot_uninstall_")
	}
	return query
}

func (s *BotStore) ensureUninstallCleanupTx(ctx context.Context, tx pgx.Tx, i *InstallationIntent, a DeletionAdmission) error {
	var prior uuid.UUID
	err := tx.QueryRow(ctx, `SELECT operation_id FROM bot_uninstall_operations WHERE installation_id=$1 FOR UPDATE`, i.InstallationID).Scan(&prior)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if a.AccountID == uuid.Nil || a.ProfileID == uuid.Nil || a.SessionEpoch <= 0 || a.AccountType == "" || a.Method != "/voice.bot.v1.BotService/UninstallBotFromSpace" || a.RequestID == "" || a.RequestHash == "" || len(a.RequestBytes) == 0 || !time.Now().Before(a.ExpiresAt) {
		return ErrDeletionOwner
	}
	op := &BotDeletion{Purpose: "space_uninstall", OperationID: uuid.New(), BotID: i.BotID, OwnerAccountID: a.AccountID, ActorProfileID: i.ActorProfileID, State: "PENDING"}
	decision := struct {
		OperationID, BotID, OwnerAccountID, ActorProfileID uuid.UUID
		Purpose                                            string
		Admission                                          *DeletionAdmission
	}{op.OperationID, op.BotID, op.OwnerAccountID, op.ActorProfileID, op.Purpose, &a}
	op.DecisionBytes, err = json.Marshal(decision)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(op.DecisionBytes)
	op.DecisionSHA256 = sum[:]
	_, err = tx.Exec(ctx, `INSERT INTO bot_uninstall_operations(operation_id,bot_id,installation_id,owner_account_id,actor_profile_id,decision_bytes,decision_sha256,state) VALUES($1,$2,$3,$4,$5,$6,$7,'PENDING')`, op.OperationID, op.BotID, i.InstallationID, op.OwnerAccountID, op.ActorProfileID, op.DecisionBytes, op.DecisionSHA256)
	if err != nil {
		return err
	}
	target, err := s.snapshotDeletionTarget(ctx, tx, op, i.SpaceID)
	if err != nil {
		return err
	}
	if target.InstallationID != i.InstallationID || target.CancelOperationID != i.CancelOperationID {
		return ErrInstallationConflict
	}
	if err = saveLifecycleTarget(ctx, tx, target, op.Purpose); err != nil {
		return err
	}
	if !time.Now().Before(a.ExpiresAt) || ctx.Err() != nil {
		return ErrDeletionOwner
	}
	return nil
}

func (s *BotStore) UninstallCleanup(ctx context.Context, installation uuid.UUID) (*BotDeletion, error) {
	d, err := scanBotDeletion(s.Pool.QueryRow(ctx, `SELECT `+deletionColumns+` FROM bot_uninstall_operations WHERE installation_id=$1`, installation))
	if err == nil {
		d.Purpose = "space_uninstall"
	}
	return d, err
}

func (s *BotStore) ClaimUninstallCleanup(ctx context.Context, operation uuid.UUID) (*BotDeletion, error) {
	d, err := scanBotDeletion(s.Pool.QueryRow(ctx, `UPDATE bot_uninstall_operations SET state='RUNNING',claim_generation=claim_generation+1,attempts=attempts+1,claimed_until=clock_timestamp()+interval '30 seconds' WHERE operation_id=$1 AND state<>'COMPLETE' AND (claimed_until IS NULL OR claimed_until<=clock_timestamp()) AND claim_generation<9223372036854775807 AND attempts<9223372036854775807 RETURNING `+deletionColumns, operation))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDeletionClaimLost
	}
	if err == nil {
		d.Purpose = "space_uninstall"
	}
	return d, err
}

func (s *BotStore) ClaimNextUninstallCleanup(ctx context.Context) (*BotDeletion, error) {
	d, err := scanBotDeletion(s.Pool.QueryRow(ctx, `UPDATE bot_uninstall_operations SET state='RUNNING',claim_generation=claim_generation+1,attempts=attempts+1,claimed_until=clock_timestamp()+interval '30 seconds' WHERE operation_id=(SELECT operation_id FROM bot_uninstall_operations WHERE state<>'COMPLETE' AND next_attempt_at<=clock_timestamp() AND (claimed_until IS NULL OR claimed_until<=clock_timestamp()) AND claim_generation<9223372036854775807 AND attempts<9223372036854775807 ORDER BY next_attempt_at,operation_id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING `+deletionColumns))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDeletionClaimLost
	}
	if err == nil {
		d.Purpose = "space_uninstall"
	}
	return d, err
}

func (s *BotStore) completeUninstallCleanup(ctx context.Context, claim *BotDeletion) error {
	effects, err := s.LifecycleEffects(ctx, claim)
	if err != nil {
		return err
	}
	if len(effects) < 2 {
		return ErrInstallationConflict
	}
	target := effects[0].Target
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// Preserve the existing Space -> Bot -> intent order. Other journal writers
	// take only the root lock; they never acquire a Space lock below it.
	if _, err = s.lockInstallation(ctx, tx, target.BotID, target.SpaceID, false); err != nil {
		return err
	}
	var generation int64
	var state string
	var current bool
	err = tx.QueryRow(ctx, `SELECT claim_generation,state FROM bot_uninstall_operations WHERE operation_id=$1 FOR UPDATE`, claim.OperationID).Scan(&generation, &state)
	if err != nil {
		return err
	}
	if generation != claim.ClaimGeneration || state != "RUNNING" {
		return ErrDeletionClaimLost
	}
	if err = tx.QueryRow(ctx, `SELECT claimed_until>clock_timestamp() FROM bot_uninstall_operations WHERE operation_id=$1`, claim.OperationID).Scan(&current); err != nil {
		return err
	}
	if !current {
		return ErrDeletionClaimLost
	}
	var pending bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bot_uninstall_effects e JOIN bot_uninstall_installations t ON t.target_id=e.target_id WHERE t.operation_id=$1 AND e.state<>'COMPLETE')`, claim.OperationID).Scan(&pending)
	if err != nil {
		return err
	}
	if pending {
		return ErrDeletionPending
	}
	intent, err := scanInstallationIntent(tx.QueryRow(ctx, `SELECT `+intentColumns+` FROM bot_installation_intents WHERE installation_id=$1 FOR UPDATE`, target.InstallationID))
	if err != nil {
		return err
	}
	if intent.BotID != target.BotID || intent.SpaceID != target.SpaceID || intent.ActorProfileID != target.ActorProfileID || intent.OperationID != target.InstallOperationID || intent.CancelOperationID != target.CancelOperationID || intent.Generation != target.BotGeneration {
		return ErrInstallationConflict
	}
	if err = s.completeInstallationCancellationTx(ctx, tx, intent); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `UPDATE bot_uninstall_operations SET state='COMPLETE',completed_at=clock_timestamp(),claimed_until=NULL,last_error='' WHERE operation_id=$1 AND claim_generation=$2 AND claimed_until>clock_timestamp()`, claim.OperationID, claim.ClaimGeneration)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrDeletionClaimLost
	}
	return tx.Commit(ctx)
}
