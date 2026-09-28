package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrGameSessionGrantInvalid  = errors.New("invalid game-session grant request")
	ErrGameSessionGrantConflict = errors.New("game-session grant operation conflicts with durable state")
	ErrGameSessionGrantRevoked  = errors.New("game-session grants are revoked")
)

type GameSessionGrantOutcome string

const (
	GameSessionGrantApplied GameSessionGrantOutcome = "APPLIED"
	GameSessionGrantReplay  GameSessionGrantOutcome = "REPLAYED"
	GameSessionGrantStale   GameSessionGrantOutcome = "STALE"
	GameSessionGrantRevoked GameSessionGrantOutcome = "REVOKED"
)

type GameSessionGrantApply struct {
	ApplicationID  uuid.UUID
	EnvironmentID  uuid.UUID
	SessionID      uuid.UUID
	VoiceRoomID    uuid.UUID
	OperationID    uuid.UUID
	RosterRevision int64
	ProfileIDs     []uuid.UUID
	RequestSHA256  [32]byte
}

type GameSessionGrantReceipt struct {
	ReceiptID               uuid.UUID
	OperationID             uuid.UUID
	ApplicationID           uuid.UUID
	EnvironmentID           uuid.UUID
	SessionID               uuid.UUID
	RequestSHA256           [32]byte
	RosterRevision          int64
	AppliedProfileSetSHA256 [32]byte
	Outcome                 GameSessionGrantOutcome
}

type gameSessionGrantScope struct {
	ApplicationID uuid.UUID
	EnvironmentID uuid.UUID
	SessionID     uuid.UUID
}

func (r GameSessionGrantApply) scope() gameSessionGrantScope {
	return gameSessionGrantScope{ApplicationID: r.ApplicationID, EnvironmentID: r.EnvironmentID, SessionID: r.SessionID}
}

func (s *RoleStore) ApplyGameSessionGrants(ctx context.Context, request GameSessionGrantApply) (GameSessionGrantReceipt, error) {
	if err := validateGameSessionGrantApply(request); err != nil {
		return GameSessionGrantReceipt{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return GameSessionGrantReceipt{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockGrantOperation(ctx, tx, request.OperationID); err != nil {
		return GameSessionGrantReceipt{}, err
	}
	prior, found, err := getGrantOperation(ctx, tx, request.OperationID)
	if err != nil {
		return GameSessionGrantReceipt{}, err
	}
	if found {
		if prior.Outcome == GameSessionGrantRevoked || prior.ApplicationID != request.ApplicationID || prior.EnvironmentID != request.EnvironmentID || prior.SessionID != request.SessionID || prior.operationKind != "apply" || prior.RequestSHA256 != request.RequestSHA256 {
			return GameSessionGrantReceipt{}, ErrGameSessionGrantConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return GameSessionGrantReceipt{}, err
		}
		return prior.GameSessionGrantReceipt, nil
	}
	if err := lockGrantSession(ctx, tx, request.scope()); err != nil {
		return GameSessionGrantReceipt{}, err
	}
	current, err := loadGrantSession(ctx, tx, request.scope(), true)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return GameSessionGrantReceipt{}, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		current = gameSessionGrantSession{RosterRevision: 0, ProfileSetSHA256: gameSessionProfileSetHash(nil), Status: "active"}
		_, err = tx.Exec(ctx, `INSERT INTO game_session_grant_sessions
			(application_id,environment_id,session_id,voice_room_id,roster_revision,profile_set_sha256,status)
			VALUES ($1,$2,$3,$4,0,$5,'active')`, request.ApplicationID, request.EnvironmentID, request.SessionID,
			request.VoiceRoomID, current.ProfileSetSHA256[:])
		if err != nil {
			return GameSessionGrantReceipt{}, err
		}
	} else if current.Status == "revoked" {
		return GameSessionGrantReceipt{}, ErrGameSessionGrantRevoked
	} else if current.VoiceRoomID != request.VoiceRoomID {
		return GameSessionGrantReceipt{}, ErrGameSessionGrantConflict
	}

	desiredHash := gameSessionProfileSetHash(request.ProfileIDs)
	outcome := GameSessionGrantApplied
	receiptRevision := request.RosterRevision
	appliedHash := desiredHash
	switch {
	case request.RosterRevision < current.RosterRevision:
		outcome = GameSessionGrantStale
		receiptRevision = current.RosterRevision
		appliedHash = current.ProfileSetSHA256
	case request.RosterRevision == current.RosterRevision:
		if !bytes.Equal(desiredHash[:], current.ProfileSetSHA256[:]) {
			return GameSessionGrantReceipt{}, ErrGameSessionGrantConflict
		}
		outcome = GameSessionGrantReplay
		receiptRevision = current.RosterRevision
		appliedHash = current.ProfileSetSHA256
	default:
		if _, err := tx.Exec(ctx, `DELETE FROM game_session_grants
			WHERE application_id=$1 AND environment_id=$2 AND session_id=$3`, request.ApplicationID, request.EnvironmentID, request.SessionID); err != nil {
			return GameSessionGrantReceipt{}, err
		}
		for _, profileID := range request.ProfileIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO game_session_grants
				(application_id,environment_id,session_id,voice_room_id,profile_id,permission,roster_revision)
				VALUES ($1,$2,$3,$4,$5,'VOICE_JOIN',$6)`, request.ApplicationID, request.EnvironmentID,
				request.SessionID, request.VoiceRoomID, profileID, request.RosterRevision); err != nil {
				return GameSessionGrantReceipt{}, err
			}
		}
		_, err := tx.Exec(ctx, `UPDATE game_session_grant_sessions SET voice_room_id=$4,roster_revision=$5,
			profile_set_sha256=$6,status='active',updated_at=clock_timestamp()
			WHERE application_id=$1 AND environment_id=$2 AND session_id=$3`, request.ApplicationID,
			request.EnvironmentID, request.SessionID, request.VoiceRoomID, request.RosterRevision, desiredHash[:])
		if err != nil {
			return GameSessionGrantReceipt{}, err
		}
	}
	receipt := GameSessionGrantReceipt{
		ReceiptID: uuid.New(), OperationID: request.OperationID,
		ApplicationID: request.ApplicationID, EnvironmentID: request.EnvironmentID, SessionID: request.SessionID,
		RequestSHA256: request.RequestSHA256, RosterRevision: receiptRevision,
		AppliedProfileSetSHA256: appliedHash, Outcome: outcome,
	}
	if err := insertGrantOperation(ctx, tx, receipt, "apply"); err != nil {
		return GameSessionGrantReceipt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return GameSessionGrantReceipt{}, err
	}
	return receipt, nil
}

func (s *RoleStore) RevokeGameSessionGrants(ctx context.Context, applicationID, environmentID, sessionID, operationID uuid.UUID, requestSHA256 [32]byte) (GameSessionGrantReceipt, error) {
	if applicationID == uuid.Nil || environmentID == uuid.Nil || sessionID == uuid.Nil || operationID == uuid.Nil {
		return GameSessionGrantReceipt{}, ErrGameSessionGrantInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return GameSessionGrantReceipt{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockGrantOperation(ctx, tx, operationID); err != nil {
		return GameSessionGrantReceipt{}, err
	}
	prior, found, err := getGrantOperation(ctx, tx, operationID)
	if err != nil {
		return GameSessionGrantReceipt{}, err
	}
	if found {
		if prior.ApplicationID != applicationID || prior.EnvironmentID != environmentID || prior.SessionID != sessionID || prior.operationKind != "revoke" || prior.RequestSHA256 != requestSHA256 {
			return GameSessionGrantReceipt{}, ErrGameSessionGrantConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return GameSessionGrantReceipt{}, err
		}
		return prior.GameSessionGrantReceipt, nil
	}
	scope := gameSessionGrantScope{ApplicationID: applicationID, EnvironmentID: environmentID, SessionID: sessionID}
	if err := lockGrantSession(ctx, tx, scope); err != nil {
		return GameSessionGrantReceipt{}, err
	}
	current, err := loadGrantSession(ctx, tx, scope, true)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return GameSessionGrantReceipt{}, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		emptyHash := gameSessionProfileSetHash(nil)
		_, err = tx.Exec(ctx, `INSERT INTO game_session_grant_sessions
			(application_id,environment_id,session_id,voice_room_id,roster_revision,profile_set_sha256,status)
			VALUES ($1,$2,$3,NULL,0,$4,'revoked')`, applicationID, environmentID, sessionID, emptyHash[:])
		if err != nil {
			return GameSessionGrantReceipt{}, err
		}
		current = gameSessionGrantSession{ProfileSetSHA256: emptyHash, Status: "revoked"}
	} else if current.Status != "revoked" {
		if _, err := tx.Exec(ctx, `DELETE FROM game_session_grants
			WHERE application_id=$1 AND environment_id=$2 AND session_id=$3`, applicationID, environmentID, sessionID); err != nil {
			return GameSessionGrantReceipt{}, err
		}
		emptyHash := gameSessionProfileSetHash(nil)
		_, err = tx.Exec(ctx, `UPDATE game_session_grant_sessions SET profile_set_sha256=$4,status='revoked',updated_at=clock_timestamp()
			WHERE application_id=$1 AND environment_id=$2 AND session_id=$3`, applicationID, environmentID, sessionID, emptyHash[:])
		if err != nil {
			return GameSessionGrantReceipt{}, err
		}
		current.ProfileSetSHA256 = emptyHash
	}
	receipt := GameSessionGrantReceipt{
		ReceiptID: uuid.New(), OperationID: operationID, ApplicationID: applicationID,
		EnvironmentID: environmentID, SessionID: sessionID, RequestSHA256: requestSHA256,
		RosterRevision: current.RosterRevision, AppliedProfileSetSHA256: current.ProfileSetSHA256,
		Outcome: GameSessionGrantRevoked,
	}
	if err := insertGrantOperation(ctx, tx, receipt, "revoke"); err != nil {
		return GameSessionGrantReceipt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return GameSessionGrantReceipt{}, err
	}
	return receipt, nil
}

func (s *RoleStore) CheckGameSessionGrant(ctx context.Context, applicationID, environmentID, sessionID, voiceRoomID, profileID uuid.UUID) (bool, error) {
	if applicationID == uuid.Nil || environmentID == uuid.Nil || sessionID == uuid.Nil || voiceRoomID == uuid.Nil || profileID == uuid.Nil {
		return false, ErrGameSessionGrantInvalid
	}
	var exists bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM game_session_grant_sessions s
		JOIN game_session_grants g USING (application_id,environment_id,session_id)
		WHERE s.application_id=$1 AND s.environment_id=$2 AND s.session_id=$3 AND s.voice_room_id=$4
		  AND s.status='active' AND g.voice_room_id=$4 AND g.profile_id=$5 AND g.permission='VOICE_JOIN'
	)`, applicationID, environmentID, sessionID, voiceRoomID, profileID).Scan(&exists)
	return exists, err
}

type gameSessionGrantSession struct {
	VoiceRoomID      uuid.UUID
	RosterRevision   int64
	ProfileSetSHA256 [32]byte
	Status           string
}

func loadGrantSession(ctx context.Context, tx pgx.Tx, scope gameSessionGrantScope, forUpdate bool) (gameSessionGrantSession, error) {
	query := `SELECT voice_room_id,roster_revision,profile_set_sha256,status FROM game_session_grant_sessions
		WHERE application_id=$1 AND environment_id=$2 AND session_id=$3`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var session gameSessionGrantSession
	var room *uuid.UUID
	var hash []byte
	err := tx.QueryRow(ctx, query, scope.ApplicationID, scope.EnvironmentID, scope.SessionID).Scan(&room, &session.RosterRevision, &hash, &session.Status)
	if err != nil {
		return gameSessionGrantSession{}, err
	}
	if room != nil {
		session.VoiceRoomID = *room
	}
	if len(hash) != len(session.ProfileSetSHA256) {
		return gameSessionGrantSession{}, errors.New("corrupt Role game-session profile set hash")
	}
	copy(session.ProfileSetSHA256[:], hash)
	return session, nil
}

type gameSessionGrantOperation struct {
	GameSessionGrantReceipt
	operationKind string
}

func getGrantOperation(ctx context.Context, tx pgx.Tx, operationID uuid.UUID) (gameSessionGrantOperation, bool, error) {
	var operation gameSessionGrantOperation
	var requestHash, profileSetHash []byte
	err := tx.QueryRow(ctx, `SELECT operation_kind,application_id,environment_id,session_id,request_sha256,
		receipt_id,roster_revision,applied_profile_set_sha256,outcome FROM game_session_grant_operations
		WHERE operation_id=$1`, operationID).Scan(&operation.operationKind, &operation.ApplicationID, &operation.EnvironmentID,
		&operation.SessionID, &requestHash, &operation.ReceiptID, &operation.RosterRevision, &profileSetHash, &operation.Outcome)
	if errors.Is(err, pgx.ErrNoRows) {
		return gameSessionGrantOperation{}, false, nil
	}
	if err != nil {
		return gameSessionGrantOperation{}, false, err
	}
	if len(requestHash) != 32 || len(profileSetHash) != 32 {
		return gameSessionGrantOperation{}, false, errors.New("corrupt Role game-session operation receipt hash")
	}
	copy(operation.RequestSHA256[:], requestHash)
	copy(operation.AppliedProfileSetSHA256[:], profileSetHash)
	operation.OperationID = operationID
	return operation, true, nil
}

func insertGrantOperation(ctx context.Context, tx pgx.Tx, receipt GameSessionGrantReceipt, kind string) error {
	_, err := tx.Exec(ctx, `INSERT INTO game_session_grant_operations
		(operation_id,operation_kind,application_id,environment_id,session_id,request_sha256,receipt_id,
		 roster_revision,applied_profile_set_sha256,outcome)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, receipt.OperationID, kind, receipt.ApplicationID,
		receipt.EnvironmentID, receipt.SessionID, receipt.RequestSHA256[:], receipt.ReceiptID,
		receipt.RosterRevision, receipt.AppliedProfileSetSHA256[:], string(receipt.Outcome))
	return err
}

func lockGrantOperation(ctx context.Context, tx pgx.Tx, operationID uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,13013))`, "role-grant-op/"+operationID.String())
	return err
}

func lockGrantSession(ctx context.Context, tx pgx.Tx, scope gameSessionGrantScope) error {
	key := fmt.Sprintf("%s/%s/%s", scope.ApplicationID, scope.EnvironmentID, scope.SessionID)
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,13014))`, "role-grant-session/"+key)
	return err
}

func validateGameSessionGrantApply(request GameSessionGrantApply) error {
	if request.ApplicationID == uuid.Nil || request.EnvironmentID == uuid.Nil || request.SessionID == uuid.Nil ||
		request.VoiceRoomID == uuid.Nil || request.OperationID == uuid.Nil || request.RosterRevision <= 0 || request.RosterRevision > math.MaxInt64 {
		return ErrGameSessionGrantInvalid
	}
	for index, profileID := range request.ProfileIDs {
		if profileID == uuid.Nil || (index > 0 && request.ProfileIDs[index-1].String() >= profileID.String()) {
			return ErrGameSessionGrantInvalid
		}
	}
	return nil
}

func gameSessionProfileSetHash(profiles []uuid.UUID) [32]byte {
	canonical := make([]byte, 0, len(profiles)*16)
	for _, profileID := range profiles {
		canonical = append(canonical, profileID[:]...)
	}
	return sha256.Sum256(canonical)
}
