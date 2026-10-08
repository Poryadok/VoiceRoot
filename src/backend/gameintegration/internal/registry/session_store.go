package registry

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrSessionNotFound = errors.New("session not found")
var ErrSessionOperationAmbiguous = errors.New("session operation ID is ambiguous without app/environment scope")

type SessionOperationKey struct {
	ApplicationID uuid.UUID
	EnvironmentID uuid.UUID
	OperationID uuid.UUID
}

type SessionPrincipal struct {
	ApplicationID uuid.UUID
	EnvironmentID uuid.UUID
	Scopes        []string
}

type CreateSessionInput struct {
	OperationID    uuid.UUID   `json:"operation_id"`
	Kind           string      `json:"kind"`
	ExternalKey    string      `json:"external_key"`
	ParentPartyKey string      `json:"parent_party_key,omitempty"`
	DisplayName    string      `json:"display_name,omitempty"`
	RosterRevision int64       `json:"roster_revision"`
	RosterComplete bool        `json:"roster_complete"`
	Members        []uuid.UUID `json:"members"`
}

type SessionOperation struct {
	OperationID             uuid.UUID  `json:"operation_id"`
	SessionID               uuid.UUID  `json:"session_id"`
	Status                  string     `json:"status"`
	SessionStatus           string     `json:"session_status"`
	Stage                   string     `json:"stage"`
	ChatID                  *uuid.UUID `json:"chat_id"`
	VoiceRoomID             *uuid.UUID `json:"voice_room_id"`
	ChatOwnerSessionID      *uuid.UUID `json:"chat_owner_session_id"`
	ChatCreateReceiptID     *uuid.UUID `json:"chat_create_receipt_id"`
	ChatRosterReceiptID     *uuid.UUID `json:"chat_roster_receipt_id"`
	VoiceProvisionReceiptID *uuid.UUID `json:"voice_provision_receipt_id"`
	RoleGrantReceiptID      *uuid.UUID `json:"role_grant_receipt_id"`
	ActiveEventID           *uuid.UUID `json:"active_event_id"`
	VoiceCloseReceiptID     *uuid.UUID `json:"voice_close_receipt_id"`
	RoleRevokeReceiptID     *uuid.UUID `json:"role_revoke_receipt_id"`
}

func (s *Store) accept(ctx context.Context, principal SessionPrincipal, in CreateSessionInput, hash []byte) (SessionOperation, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return SessionOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var oldHash []byte
	var oldSession uuid.UUID
	err = tx.QueryRow(ctx, `SELECT request_hash, session_id FROM gis_session_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, principal.ApplicationID, principal.EnvironmentID, in.OperationID).Scan(&oldHash, &oldSession)
	if err == nil {
		if !equalBytes(oldHash, hash) {
			return SessionOperation{}, ErrIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return SessionOperation{}, err
		}
		return s.getOperation(ctx, principal, in.OperationID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return SessionOperation{}, err
	}
	if in.Kind == "match" || in.Kind == "fleet" {
		if in.ParentPartyKey != "" {
			var parentID uuid.UUID
			var chatID, chatOwner, chatCreate, chatRoster *uuid.UUID
			var parentMembers []uuid.UUID
			err = tx.QueryRow(ctx, `SELECT id,chat_id,chat_owner_session_id,chat_create_receipt_id,chat_roster_receipt_id,members FROM gis_sessions WHERE application_id=$1 AND environment_id=$2 AND kind='party' AND external_key=$3 AND session_status='active' FOR SHARE`, principal.ApplicationID, principal.EnvironmentID, in.ParentPartyKey).Scan(&parentID, &chatID, &chatOwner, &chatCreate, &chatRoster, &parentMembers)
			if err != nil {
				return SessionOperation{}, err
			}
			if !isSubset(in.Members, parentMembers) {
				return SessionOperation{}, errors.New("child roster is not a subset of party roster")
			}
			id := uuid.New()
			_, err = tx.Exec(ctx, `INSERT INTO gis_sessions(id,application_id,environment_id,kind,external_key,parent_party_key,display_name,session_status,stage,roster_revision,roster_complete,members,chat_id,chat_owner_session_id,chat_create_receipt_id,chat_roster_receipt_id) VALUES($1,$2,$3,$4,$5,$6,NULL,'provisioning','accepted',$7,true,$8,$9,$10,$11,$12)`, id, principal.ApplicationID, principal.EnvironmentID, in.Kind, in.ExternalKey, in.ParentPartyKey, in.RosterRevision, in.Members, chatID, chatOwner, chatCreate, chatRoster)
			if err != nil {
				return SessionOperation{}, err
			}
			_, err = tx.Exec(ctx, `INSERT INTO gis_session_operations(operation_id,session_id,application_id,environment_id,operation_kind,request_hash,status,stage) VALUES($1,$2,$3,$4,'create',$5,'pending','accepted')`, in.OperationID, id, principal.ApplicationID, principal.EnvironmentID, hash)
			if err != nil {
				return SessionOperation{}, err
			}
		} else {
			if in.DisplayName == "" {
				return SessionOperation{}, errors.New("display_name required")
			}
			if err = s.insertOwnedSession(ctx, tx, principal, in, hash); err != nil {
				return SessionOperation{}, err
			}
		}
	} else {
		if in.DisplayName == "" {
			return SessionOperation{}, errors.New("display_name required")
		}
		if err = s.insertOwnedSession(ctx, tx, principal, in, hash); err != nil {
			return SessionOperation{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionOperation{}, err
	}
	return s.getOperation(ctx, principal, in.OperationID)
}

func (s *Store) insertOwnedSession(ctx context.Context, tx pgx.Tx, p SessionPrincipal, in CreateSessionInput, hash []byte) error {
	id := uuid.New()
	_, err := tx.Exec(ctx, `INSERT INTO gis_sessions(id,application_id,environment_id,kind,external_key,parent_party_key,display_name,session_status,stage,roster_revision,roster_complete,members,chat_owner_session_id) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,'provisioning','accepted',$8,true,$9,$1)`, id, p.ApplicationID, p.EnvironmentID, in.Kind, in.ExternalKey, in.ParentPartyKey, in.DisplayName, in.RosterRevision, in.Members)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO gis_session_operations(operation_id,session_id,application_id,environment_id,operation_kind,request_hash,status,stage) VALUES($1,$2,$3,$4,'create',$5,'pending','accepted')`, in.OperationID, id, p.ApplicationID, p.EnvironmentID, hash)
	return err
}

func (s *Store) getOperation(ctx context.Context, p SessionPrincipal, operationID uuid.UUID) (SessionOperation, error) {
	var o SessionOperation
	err := s.Pool.QueryRow(ctx, `SELECT o.operation_id,o.session_id,o.status,s.session_status,o.stage,s.chat_id,s.voice_room_id,s.chat_owner_session_id,s.chat_create_receipt_id,s.chat_roster_receipt_id,s.voice_provision_receipt_id,s.role_grant_receipt_id,o.active_event_id,COALESCE(o.voice_close_receipt_id,s.voice_close_receipt_id),COALESCE(o.role_revoke_receipt_id,s.role_revoke_receipt_id) FROM gis_session_operations o JOIN gis_sessions s ON s.id=o.session_id WHERE o.application_id=$1 AND o.environment_id=$2 AND o.operation_id=$3`, p.ApplicationID, p.EnvironmentID, operationID).Scan(&o.OperationID, &o.SessionID, &o.Status, &o.SessionStatus, &o.Stage, &o.ChatID, &o.VoiceRoomID, &o.ChatOwnerSessionID, &o.ChatCreateReceiptID, &o.ChatRosterReceiptID, &o.VoiceProvisionReceiptID, &o.RoleGrantReceiptID, &o.ActiveEventID, &o.VoiceCloseReceiptID, &o.RoleRevokeReceiptID)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionOperation{}, ErrSessionNotFound
	}
	return o, err
}

func (s *Store) operationKeyForID(ctx context.Context, operationID uuid.UUID) (SessionOperationKey, error) {
	rows, err := s.Pool.Query(ctx, `SELECT application_id,environment_id FROM gis_session_operations
		WHERE operation_id=$1 ORDER BY application_id,environment_id`, operationID)
	if err != nil {
		return SessionOperationKey{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err = rows.Err(); err != nil {
			return SessionOperationKey{}, err
		}
		return SessionOperationKey{}, ErrSessionNotFound
	}
	key := SessionOperationKey{OperationID: operationID}
	if err = rows.Scan(&key.ApplicationID, &key.EnvironmentID); err != nil {
		return SessionOperationKey{}, err
	}
	if rows.Next() {
		return SessionOperationKey{}, ErrSessionOperationAmbiguous
	}
	if err = rows.Err(); err != nil {
		return SessionOperationKey{}, err
	}
	return key, nil
}

func (s *Store) claim(ctx context.Context, operationID uuid.UUID) (uuid.UUID, string, string, uuid.UUID, error) {
	key, err := s.operationKeyForID(ctx, operationID)
	if err != nil {
		return uuid.Nil, "", "", uuid.Nil, err
	}
	return s.claimScoped(ctx, key)
}

func (s *Store) claimScoped(ctx context.Context, key SessionOperationKey) (uuid.UUID, string, string, uuid.UUID, error) {
	if key.ApplicationID == uuid.Nil || key.EnvironmentID == uuid.Nil || key.OperationID == uuid.Nil {
		return uuid.Nil, "", "", uuid.Nil, errors.New("invalid session operation key")
	}
	owner := uuid.New()
	var session uuid.UUID
	var kind, stage string
	err := s.Pool.QueryRow(ctx, `UPDATE gis_session_operations
		SET lease_owner=$4,lease_until=now()+interval '5 seconds',attempts=attempts+1,updated_at=now()
		WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3 AND status='pending'
		AND (lease_until IS NULL OR lease_until<now())
		RETURNING session_id,operation_kind,stage`,
		key.ApplicationID, key.EnvironmentID, key.OperationID, owner).Scan(&session, &kind, &stage)
	return session, kind, stage, owner, err
}

func (s *Store) saveOwnerReceipt(ctx context.Context, key SessionOperationKey, ownerOp uuid.UUID, stage string, reqHash []byte, r SessionOwnerReceipt) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO gis_session_owner_receipts
		(application_id,environment_id,operation_id,stage,owner_operation_id,owner_request_hash,resource_id,receipt_id,receipt_bytes)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT(application_id,environment_id,operation_id,stage) DO NOTHING`,
		key.ApplicationID, key.EnvironmentID, key.OperationID, stage, ownerOp, reqHash, r.ResourceID, r.ReceiptID, []byte(r.ReceiptID.String()))
	return err
}

func (s *Store) due(ctx context.Context) ([]SessionOperationKey, error) {
	rows, err := s.Pool.Query(ctx, `SELECT application_id,environment_id,operation_id FROM gis_session_operations
		WHERE status='pending' AND next_attempt_at<=now() AND (lease_until IS NULL OR lease_until<now())
		ORDER BY created_at LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []SessionOperationKey
	for rows.Next() {
		var key SessionOperationKey
		if err := rows.Scan(&key.ApplicationID, &key.EnvironmentID, &key.OperationID); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *Store) scheduleRetry(ctx context.Context, key SessionOperationKey, owner uuid.UUID, err error) error {
	_, e := s.Pool.Exec(ctx, `UPDATE gis_session_operations SET lease_owner=NULL,lease_until=NULL,
		stage_retry_count=stage_retry_count+1,
		next_attempt_at=now()+LEAST(30, (1 << LEAST(stage_retry_count,5))) * interval '1 second',
		error_code=$4,updated_at=now() WHERE application_id=$1 AND environment_id=$2
		AND operation_id=$3 AND lease_owner=$5`,
		key.ApplicationID, key.EnvironmentID, key.OperationID, safeOwnerError(err), owner)
	return e
}

func isSubset(a, b []uuid.UUID) bool {
	set := make(map[uuid.UUID]bool, len(b))
	for _, id := range b {
		set[id] = true
	}
	for _, id := range a {
		if !set[id] {
			return false
		}
	}
	return true
}
