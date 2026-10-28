package registry

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrRosterUpdatePending         = errors.New("session roster update is pending")
	ErrHostTransferRequiresNextRev = errors.New("host transfer requires the next roster revision")
)

func (s *Store) acceptSessionRosterUpdate(ctx context.Context, p SessionPrincipal, sessionID uuid.UUID, input SessionRosterUpdateInput, requestHash, rosterHash []byte) (SessionOperation, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return SessionOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var existingHash []byte
	var existingSession uuid.UUID
	err = tx.QueryRow(ctx, `SELECT request_hash,session_id FROM gis_session_operations
		WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`,
		p.ApplicationID, p.EnvironmentID, input.OperationID).Scan(&existingHash, &existingSession)
	if err == nil {
		if !equalBytes(existingHash, requestHash) || existingSession != sessionID {
			return SessionOperation{}, ErrIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return SessionOperation{}, err
		}
		return s.getOperation(ctx, p, input.OperationID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return SessionOperation{}, err
	}

	var status string
	var currentRevision int64
	var currentMembers []uuid.UUID
	var currentHost, currentReceipt *uuid.UUID
	var currentHash []byte
	var leaseValid bool
	err = tx.QueryRow(ctx, `SELECT session_status,roster_revision,members,host_profile_id,roster_hash,
		roster_receipt_id,COALESCE(lease_expires_at>clock_timestamp(),false)
		FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3 FOR UPDATE`,
		sessionID, p.ApplicationID, p.EnvironmentID).
		Scan(&status, &currentRevision, &currentMembers, &currentHost, &currentHash, &currentReceipt, &leaseValid)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionOperation{}, ErrSessionNotFound
	}
	if err != nil {
		return SessionOperation{}, err
	}
	if status != "active" {
		return SessionOperation{}, ErrSessionNotFound
	}
	if !leaseValid {
		return SessionOperation{}, ErrRosterLeaseExpired
	}
	if len(currentHash) == 0 {
		currentHash, err = sessionRosterHash(currentRevision, currentMembers, currentHost)
		if err != nil {
			return SessionOperation{}, err
		}
	}
	var pendingCount int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM gis_session_operations WHERE session_id=$1
		AND operation_kind='roster' AND status='pending'`, sessionID).Scan(&pendingCount); err != nil {
		return SessionOperation{}, err
	}
	operationStatus, stage, outcome := "pending", "roster_chat_pending", "pending"
	switch {
	case input.RosterRevision < currentRevision:
		operationStatus, stage, outcome = "succeeded", "stale", "stale"
	case input.RosterRevision == currentRevision:
		if !equalBytes(rosterHash, currentHash) {
			return SessionOperation{}, ErrIdempotencyConflict
		}
		if pendingCount != 0 {
			return SessionOperation{}, ErrRosterUpdatePending
		}
		operationStatus, stage, outcome = "succeeded", "replayed", "replayed"
	default:
		if pendingCount != 0 {
			return SessionOperation{}, ErrRosterUpdatePending
		}
		if !sameOptionalUUID(input.HostProfileID, currentHost) && input.RosterRevision != currentRevision+1 {
			return SessionOperation{}, ErrHostTransferRequiresNextRev
		}
	}

	if _, err = tx.Exec(ctx, `INSERT INTO gis_session_operations
		(operation_id,session_id,application_id,environment_id,operation_kind,request_hash,status,stage,terminalized_at)
		VALUES($1,$2,$3,$4,'roster',$5,$6,$7,CASE WHEN $6='succeeded' THEN clock_timestamp() ELSE NULL END)`, input.OperationID, sessionID,
		p.ApplicationID, p.EnvironmentID, requestHash, operationStatus, stage); err != nil {
		return SessionOperation{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO gis_session_roster_updates
		(operation_id,session_id,application_id,environment_id,request_hash,roster_revision,
		 roster_hash,members,host_profile_id,outcome)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, input.OperationID, sessionID,
		p.ApplicationID, p.EnvironmentID, requestHash, input.RosterRevision, rosterHash,
		input.Members, input.HostProfileID, outcome); err != nil {
		return SessionOperation{}, err
	}

	if outcome == "pending" {
		_, err = tx.Exec(ctx, `UPDATE gis_sessions SET roster_revision=$2,roster_complete=true,
			members=$3,host_profile_id=$4,roster_hash=$5,roster_receipt_id=$6,
			lease_expires_at=clock_timestamp()+interval '60 seconds',updated_at=now()
			WHERE id=$1`, sessionID, input.RosterRevision, input.Members, input.HostProfileID,
			rosterHash, input.OperationID)
		if err != nil {
			return SessionOperation{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionOperation{}, err
	}
	return s.getOperation(ctx, p, input.OperationID)
}

func sameOptionalUUID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
