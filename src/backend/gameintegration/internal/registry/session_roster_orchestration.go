package registry

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"voice/backend/pkg/principal"
)

func (o *SessionOrchestrator) advanceRoster(ctx context.Context, p SessionPrincipal, operationID, sessionID uuid.UUID, stage string, leaseOwner uuid.UUID, req SessionOwnerRequest) (SessionOperation, error) {
	key := SessionOperationKey{ApplicationID: p.ApplicationID, EnvironmentID: p.EnvironmentID, OperationID: operationID}
	var outcome, sessionStatus string
	var chatOwner uuid.UUID
	var leaseValid bool
	if err := o.Store.Pool.QueryRow(ctx, `SELECT u.outcome,s.session_status,
		COALESCE(s.chat_owner_session_id,'00000000-0000-0000-0000-000000000000'::uuid),
		COALESCE(s.lease_expires_at>clock_timestamp(),false)
		FROM gis_session_roster_updates u JOIN gis_sessions s ON s.id=u.session_id
		WHERE u.operation_id=$1 AND u.session_id=$2 AND u.application_id=$3 AND u.environment_id=$4`, operationID, sessionID, key.ApplicationID, key.EnvironmentID).
		Scan(&outcome, &sessionStatus, &chatOwner, &leaseValid); err != nil {
		return SessionOperation{}, err
	}
	if outcome != "pending" {
		return o.Store.getOperation(ctx, p, operationID)
	}
	if sessionStatus != "active" || !leaseValid {
		failureCode := "SESSION_TERMINAL"
		if sessionStatus == "active" && !leaseValid {
			failureCode = "ROSTER_LEASE_EXPIRED"
		}
		if err := o.failRosterOperation(ctx, key, leaseOwner, failureCode); err != nil {
			return SessionOperation{}, err
		}
		if failureCode != "ROSTER_LEASE_EXPIRED" {
			return o.Store.getOperation(ctx, p, operationID)
		}
		failureID := deterministicOwnerID(p.ApplicationID, p.EnvironmentID, sessionID, "roster_lease_expiry", fmt.Sprint(req.RosterRevision))
		if _, err := o.FailSession(ctx, p, sessionID, failureID, "ROSTER_LEASE_EXPIRED"); err != nil && !errors.Is(err, ErrSessionNotFound) {
			return SessionOperation{}, err
		}
		return o.Store.getOperation(ctx, p, operationID)
	}

	var ownerStage, nextStage string
	var call func(context.Context, SessionOwnerRequest) (SessionOwnerReceipt, error)
	switch stage {
	case "roster_chat_pending":
		if chatOwner != sessionID {
			advanced, err := o.advanceRosterStage(ctx, p, operationID, sessionID, leaseOwner, "roster_voice_pending", "")
			if errors.Is(err, ErrRosterLeaseExpired) {
				return o.advanceRoster(ctx, p, operationID, sessionID, stage, leaseOwner, req)
			}
			return advanced, err
		}
		ownerStage, nextStage, call = "chat_roster", "roster_voice_pending", o.Owners.SyncChatRoster
	case "roster_voice_pending":
		ownerStage, nextStage, call = "voice_roster", "roster_role_pending", o.Owners.ApplyVoiceRoster
	case "roster_role_pending":
		ownerStage, nextStage, call = "role_apply", "applied", o.Owners.ApplyRoleGrants
	default:
		return SessionOperation{}, fmt.Errorf("unknown roster operation stage %q", stage)
	}
	if call == nil {
		err := errors.New("session roster owner unavailable")
		_ = o.Store.scheduleRetry(ctx, key, leaseOwner, err)
		return SessionOperation{}, err
	}

	ownerOperationID := deterministicOwnerID(p.ApplicationID, p.EnvironmentID, sessionID,
		"t32_roster_"+ownerStage, fmt.Sprint(req.RosterRevision))
	req.OperationID = ownerOperationID
	message, err := BuildSessionOwnerProto(ownerStage, req)
	if err != nil {
		_ = o.Store.scheduleRetry(ctx, key, leaseOwner, err)
		return SessionOperation{}, fmt.Errorf("build %s owner request for app=%s env=%s session=%s operation=%s: %w",
			ownerStage, req.ApplicationID, req.EnvironmentID, req.SessionID, req.OperationID, err)
	}
	req.Proto = message
	req.RequestHash, err = principal.RequestHash(message)
	if err != nil {
		_ = o.Store.scheduleRetry(ctx, key, leaseOwner, err)
		return SessionOperation{}, err
	}
	hashBytes, err := principalHashBytes(req.RequestHash)
	if err != nil {
		_ = o.Store.scheduleRetry(ctx, key, leaseOwner, err)
		return SessionOperation{}, err
	}
	receipt, err := call(ctx, req)
	if err != nil {
		_ = o.Store.scheduleRetry(ctx, key, leaseOwner, err)
		return SessionOperation{}, err
	}
	if receipt.ResourceID == uuid.Nil || receipt.ReceiptID == uuid.Nil || receipt.RequestHash != req.RequestHash {
		err = errors.New("invalid roster owner receipt")
		_ = o.Store.scheduleRetry(ctx, key, leaseOwner, err)
		return SessionOperation{}, err
	}
	if err = o.recordRosterOwnerReceipt(ctx, key, sessionID, leaseOwner, ownerOperationID, ownerStage,
		hashBytes, receipt, nextStage); err != nil {
		if errors.Is(err, ErrRosterLeaseExpired) {
			return o.advanceRoster(ctx, p, operationID, sessionID, stage, leaseOwner, req)
		}
		return SessionOperation{}, err
	}
	return o.Store.getOperation(ctx, p, operationID)
}

func (o *SessionOrchestrator) advanceRosterStage(ctx context.Context, p SessionPrincipal, operationID, sessionID, leaseOwner uuid.UUID, nextStage, receiptStage string) (SessionOperation, error) {
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return SessionOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var activeAndLeased bool
	if err = tx.QueryRow(ctx, `SELECT session_status='active' AND COALESCE(lease_expires_at>clock_timestamp(),false)
		FROM gis_sessions WHERE id=$1 FOR UPDATE`, sessionID).Scan(&activeAndLeased); err != nil {
		return SessionOperation{}, err
	}
	if !activeAndLeased {
		return SessionOperation{}, ErrRosterLeaseExpired
	}
	if receiptStage != "" {
		return SessionOperation{}, errors.New("roster stage cannot advance without an owner receipt")
	}
	tag, err := tx.Exec(ctx, `UPDATE gis_session_operations SET stage=$3,stage_retry_count=0,
		lease_owner=NULL,lease_until=NULL,next_attempt_at=now(),error_code=NULL,error_outcome=NULL,updated_at=now()
		WHERE operation_id=$1 AND lease_owner=$2 AND status='pending' AND application_id=$4 AND environment_id=$5 AND lease_until>clock_timestamp()`, operationID, leaseOwner, nextStage, p.ApplicationID, p.EnvironmentID)
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, errors.New("session roster operation lease lost")
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionOperation{}, err
	}
	return o.Store.getOperation(ctx, p, operationID)
}

func (o *SessionOrchestrator) recordRosterOwnerReceipt(ctx context.Context, key SessionOperationKey, sessionID, leaseOwner, ownerOperationID uuid.UUID,
	stage string, requestHash []byte, receipt SessionOwnerReceipt, nextStage string) error {
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var leaseValid bool
	if err = tx.QueryRow(ctx, `SELECT COALESCE(lease_expires_at>clock_timestamp(),false)
		FROM gis_sessions WHERE id=$1 AND session_status='active' FOR UPDATE`, sessionID).Scan(&leaseValid); err != nil {
		return err
	}
	if !leaseValid {
		return ErrRosterLeaseExpired
	}
	if err = saveOwnerReceiptTx(ctx, tx, key, ownerOperationID, stage, requestHash, receipt); err != nil {
		return err
	}
	if stage == "chat_roster" {
		if err = replaceAcceptedBindingChatRoster(ctx, tx, key, sessionID); err != nil {
			return err
		}
	}
	var operationStatus, outcome string
	if nextStage == "applied" {
		operationStatus, outcome = "succeeded", "applied"
	} else {
		operationStatus, outcome = "pending", "pending"
	}
	tag, err := tx.Exec(ctx, `UPDATE gis_session_operations SET status=$3,stage=$4,stage_retry_count=0,
		lease_owner=NULL,lease_until=NULL,next_attempt_at=now(),error_code=NULL,error_outcome=NULL,updated_at=now()
		WHERE operation_id=$1 AND lease_owner=$2 AND status='pending' AND application_id=$5 AND environment_id=$6 AND lease_until>clock_timestamp()`, key.OperationID, leaseOwner, operationStatus, nextStage, key.ApplicationID, key.EnvironmentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("session roster operation lease lost")
	}
	if nextStage == "applied" {
		if _, err = tx.Exec(ctx, `UPDATE gis_session_roster_updates SET outcome=$2,updated_at=now() WHERE operation_id=$1 AND outcome='pending' AND application_id=$3 AND environment_id=$4`, key.OperationID, outcome, key.ApplicationID, key.EnvironmentID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (o *SessionOrchestrator) failRosterOperation(ctx context.Context, key SessionOperationKey, leaseOwner uuid.UUID, code string) error {
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `UPDATE gis_session_operations SET status='failed',stage='failed',lease_owner=NULL,
		lease_until=NULL,error_code=$3,error_outcome='permanent',updated_at=now()
		WHERE operation_id=$1 AND lease_owner=$2 AND status='pending' AND application_id=$4 AND environment_id=$5 AND lease_until>clock_timestamp()`, key.OperationID, leaseOwner, code, key.ApplicationID, key.EnvironmentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("session roster operation lease lost")
	}
	if _, err = tx.Exec(ctx, `UPDATE gis_session_roster_updates SET outcome='failed',updated_at=now() WHERE operation_id=$1 AND outcome='pending' AND application_id=$2 AND environment_id=$3`, key.OperationID, key.ApplicationID, key.EnvironmentID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
