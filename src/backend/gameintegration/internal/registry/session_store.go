package registry

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrSessionNotFound = errors.New("session not found")

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

func (s *Store) claim(ctx context.Context, op uuid.UUID) (uuid.UUID, string, string, uuid.UUID, error) {
	owner := uuid.New()
	var session uuid.UUID
	var kind, stage string
	err := s.Pool.QueryRow(ctx, `UPDATE gis_session_operations SET lease_owner=$2,lease_until=now()+interval '5 seconds',attempts=attempts+1,updated_at=now() WHERE operation_id=$1 AND status='pending' AND (lease_until IS NULL OR lease_until<now()) RETURNING session_id,operation_kind,stage`, op, owner).Scan(&session, &kind, &stage)
	return session, kind, stage, owner, err
}

func (s *Store) saveOwnerReceipt(ctx context.Context, op, ownerOp uuid.UUID, stage string, reqHash []byte, r SessionOwnerReceipt) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO gis_session_owner_receipts(operation_id,stage,owner_operation_id,owner_request_hash,resource_id,receipt_id,receipt_bytes) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(operation_id,stage) DO NOTHING`, op, stage, ownerOp, reqHash, r.ResourceID, r.ReceiptID, []byte(r.ReceiptID.String()))
	return err
}

func (s *Store) due(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.Pool.Query(ctx, `SELECT operation_id FROM gis_session_operations WHERE status='pending' AND next_attempt_at<=now() AND (lease_until IS NULL OR lease_until<now()) ORDER BY created_at LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) scheduleRetry(ctx context.Context, op, owner uuid.UUID, err error) error {
	_, e := s.Pool.Exec(ctx, `UPDATE gis_session_operations SET lease_owner=NULL,lease_until=NULL,
		stage_retry_count=stage_retry_count+1,
		next_attempt_at=now()+LEAST(30, (1 << LEAST(stage_retry_count,5))) * interval '1 second',
		error_code=$3,updated_at=now() WHERE operation_id=$1 AND lease_owner=$2`, op, owner, safeOwnerError(err))
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
