package registry

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrSessionNotFound = errors.New("session not found")
var ErrSessionOperationAmbiguous = errors.New("session operation ID is ambiguous without app/environment scope")
var ErrSessionLeaseLost = errors.New("session operation lease was lost or expired")
var ErrSessionKeyTombstoned = errors.New("session external key is permanently tombstoned")

type SessionOperationKey struct {
	ApplicationID uuid.UUID
	EnvironmentID uuid.UUID
	OperationID   uuid.UUID
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
	HostProfileID  *uuid.UUID  `json:"host_profile_id,omitempty"`
}

type SessionRosterUpdateInput struct {
	OperationID    uuid.UUID   `json:"operation_id"`
	RosterRevision int64       `json:"roster_revision"`
	RosterComplete bool        `json:"roster_complete"`
	Members        []uuid.UUID `json:"members"`
	HostProfileID  *uuid.UUID  `json:"host_profile_id,omitempty"`
}

type SessionOperation struct {
	OperationID             uuid.UUID              `json:"operation_id"`
	SessionID               uuid.UUID              `json:"session_id"`
	Status                  string                 `json:"status"`
	SessionStatus           string                 `json:"session_status"`
	Stage                   string                 `json:"stage"`
	ChatID                  *uuid.UUID             `json:"chat_id"`
	VoiceRoomID             *uuid.UUID             `json:"voice_room_id"`
	ChatOwnerSessionID      *uuid.UUID             `json:"chat_owner_session_id"`
	ChatCreateReceiptID     *uuid.UUID             `json:"chat_create_receipt_id"`
	ChatRosterReceiptID     *uuid.UUID             `json:"chat_roster_receipt_id"`
	VoiceProvisionReceiptID *uuid.UUID             `json:"voice_provision_receipt_id"`
	RoleGrantReceiptID      *uuid.UUID             `json:"role_grant_receipt_id"`
	ActiveEventID           *uuid.UUID             `json:"active_event_id"`
	VoiceCloseReceiptID     *uuid.UUID             `json:"voice_close_receipt_id"`
	RoleRevokeReceiptID     *uuid.UUID             `json:"role_revoke_receipt_id"`
	RosterRevision          int64                  `json:"roster_revision"`
	RosterReceiptID         *uuid.UUID             `json:"roster_receipt_id"`
	RosterLeaseExpiresAt    *time.Time             `json:"roster_lease_expires_at"`
	LastError               *SessionOperationError `json:"last_error"`
}

// SessionOperationError reports the latest durable failure for an operation.
// RetryAfter is whole seconds rounded up to next_attempt_at when the caller may
// retry; permanent failures return nil even while GIS retries internal cleanup.
type SessionOperationError struct {
	Code       string `json:"code"`
	Outcome    string `json:"outcome"`
	RetryAfter *int   `json:"retry_after"`
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
	if err = lockScopedExternalKey(ctx, tx, principal.ApplicationID, principal.EnvironmentID, in.ExternalKey); err != nil {
		return SessionOperation{}, err
	}
	var tombstoned bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gis_session_external_key_tombstones WHERE application_id=$1 AND environment_id=$2 AND kind=$3 AND external_key=$4)`, principal.ApplicationID, principal.EnvironmentID, in.Kind, in.ExternalKey).Scan(&tombstoned); err != nil {
		return SessionOperation{}, err
	}
	if tombstoned {
		return SessionOperation{}, ErrSessionKeyTombstoned
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
			rosterHash, hashErr := sessionRosterHash(in.RosterRevision, in.Members, in.HostProfileID)
			if hashErr != nil {
				return SessionOperation{}, hashErr
			}
			_, err = tx.Exec(ctx, `INSERT INTO gis_sessions(id,application_id,environment_id,kind,external_key,parent_party_key,display_name,session_status,stage,roster_revision,roster_complete,members,host_profile_id,roster_hash,lease_expires_at,roster_receipt_id,chat_id,chat_owner_session_id,chat_create_receipt_id,chat_roster_receipt_id) VALUES($1,$2,$3,$4,$5,$6,NULL,'provisioning','accepted',$7,true,$8,$9,$10,clock_timestamp()+interval '60 seconds',$11,$12,$13,$14,$15)`, id, principal.ApplicationID, principal.EnvironmentID, in.Kind, in.ExternalKey, in.ParentPartyKey, in.RosterRevision, in.Members, in.HostProfileID, rosterHash, in.OperationID, chatID, chatOwner, chatCreate, chatRoster)
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
	rosterHash, err := sessionRosterHash(in.RosterRevision, in.Members, in.HostProfileID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO gis_sessions(id,application_id,environment_id,kind,external_key,parent_party_key,display_name,session_status,stage,roster_revision,roster_complete,members,host_profile_id,roster_hash,lease_expires_at,roster_receipt_id,chat_owner_session_id) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,'provisioning','accepted',$8,true,$9,$10,$11,clock_timestamp()+interval '60 seconds',$12,$1)`, id, p.ApplicationID, p.EnvironmentID, in.Kind, in.ExternalKey, in.ParentPartyKey, in.DisplayName, in.RosterRevision, in.Members, in.HostProfileID, rosterHash, in.OperationID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO gis_session_operations(operation_id,session_id,application_id,environment_id,operation_kind,request_hash,status,stage) VALUES($1,$2,$3,$4,'create',$5,'pending','accepted')`, in.OperationID, id, p.ApplicationID, p.EnvironmentID, hash)
	return err
}

func (s *Store) getOperation(ctx context.Context, p SessionPrincipal, operationID uuid.UUID) (SessionOperation, error) {
	var o SessionOperation
	var errorCode, errorOutcome *string
	var retryAfter *int
	err := s.Pool.QueryRow(ctx, `SELECT o.operation_id,o.session_id,o.status,s.session_status,o.stage,s.chat_id,s.voice_room_id,s.chat_owner_session_id,s.chat_create_receipt_id,s.chat_roster_receipt_id,s.voice_provision_receipt_id,s.role_grant_receipt_id,o.active_event_id,COALESCE(o.voice_close_receipt_id,s.voice_close_receipt_id),COALESCE(o.role_revoke_receipt_id,s.role_revoke_receipt_id),s.roster_revision,s.roster_receipt_id,s.lease_expires_at,o.error_code,COALESCE(o.error_outcome,'unknown'),CASE WHEN o.error_code IS NOT NULL AND o.status='pending' AND o.error_outcome='unknown' THEN GREATEST(0,CEIL(EXTRACT(EPOCH FROM (o.next_attempt_at-now()))))::integer ELSE NULL END FROM gis_session_operations o JOIN gis_sessions s ON s.id=o.session_id WHERE o.application_id=$1 AND o.environment_id=$2 AND o.operation_id=$3`, p.ApplicationID, p.EnvironmentID, operationID).Scan(&o.OperationID, &o.SessionID, &o.Status, &o.SessionStatus, &o.Stage, &o.ChatID, &o.VoiceRoomID, &o.ChatOwnerSessionID, &o.ChatCreateReceiptID, &o.ChatRosterReceiptID, &o.VoiceProvisionReceiptID, &o.RoleGrantReceiptID, &o.ActiveEventID, &o.VoiceCloseReceiptID, &o.RoleRevokeReceiptID, &o.RosterRevision, &o.RosterReceiptID, &o.RosterLeaseExpiresAt, &errorCode, &errorOutcome, &retryAfter)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionOperation{}, ErrSessionNotFound
	}
	if err == nil && errorCode != nil {
		o.LastError = &SessionOperationError{Code: *errorCode, Outcome: *errorOutcome, RetryAfter: retryAfter}
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

func saveOwnerReceiptTx(ctx context.Context, tx pgx.Tx, key SessionOperationKey, ownerOp uuid.UUID, stage string, reqHash []byte, r SessionOwnerReceipt) error {
	_, err := tx.Exec(ctx, `INSERT INTO gis_session_owner_receipts
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

// PurgeExpiredSessionData removes terminal match/fleet content when DB time is
// at or beyond terminalized_at + 30 days. Match history shares the same
// exclusive deadline. Party rows are intentionally never selected because
// child matches may still depend on the party resource and membership.
func (s *Store) PurgeExpiredSessionData(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	opRows, err := tx.Query(ctx, `SELECT application_id,environment_id,operation_id FROM gis_session_operations
		WHERE terminalized_at + interval '30 days' <= clock_timestamp()
		ORDER BY terminalized_at,operation_id LIMIT $1 FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return 0, err
	}
	var expiredOperations []SessionOperationKey
	for opRows.Next() {
		var key SessionOperationKey
		if err := opRows.Scan(&key.ApplicationID, &key.EnvironmentID, &key.OperationID); err != nil {
			opRows.Close()
			return 0, err
		}
		expiredOperations = append(expiredOperations, key)
	}
	if err := opRows.Err(); err != nil {
		opRows.Close()
		return 0, err
	}
	opRows.Close()
	for _, key := range expiredOperations {
		if _, err = tx.Exec(ctx, `DELETE FROM gis_session_owner_receipts WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, key.ApplicationID, key.EnvironmentID, key.OperationID); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM gis_session_roster_updates WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, key.ApplicationID, key.EnvironmentID, key.OperationID); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM gis_session_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, key.ApplicationID, key.EnvironmentID, key.OperationID); err != nil {
			return 0, err
		}
	}
	rows, err := tx.Query(ctx, `SELECT s.id,s.application_id,s.environment_id,s.kind,s.external_key FROM gis_sessions s
		WHERE session_status IN ('closed','failed') AND kind IN ('match','fleet')
		AND s.terminalized_at + interval '30 days' <= clock_timestamp()
		AND (s.kind <> 'match' OR s.history_expires_at <= clock_timestamp())
		AND NOT EXISTS (SELECT 1 FROM gis_session_operations o WHERE o.session_id=s.id)
		ORDER BY s.terminalized_at,s.id LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	type expiredSession struct {
		id, app, env uuid.UUID
		kind, key    string
	}
	var expired []expiredSession
	for rows.Next() {
		var item expiredSession
		if err := rows.Scan(&item.id, &item.app, &item.env, &item.kind, &item.key); err != nil {
			rows.Close()
			return 0, err
		}
		expired = append(expired, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	purged := 0
	for _, item := range expired {
		if err = lockScopedExternalKey(ctx, tx, item.app, item.env, item.key); err != nil {
			return 0, err
		}
		// Another worker or a concurrent request may have changed the session
		// after candidate selection; recheck under the key lock before purging.
		err = tx.QueryRow(ctx, `SELECT id,application_id,environment_id,kind,external_key FROM gis_sessions
			WHERE id=$1 AND session_status IN ('closed','failed') AND kind IN ('match','fleet')
			AND terminalized_at+interval '30 days'<=clock_timestamp()
			AND (kind<>'match' OR history_expires_at<=clock_timestamp())
			AND NOT EXISTS (SELECT 1 FROM gis_session_operations WHERE session_id=gis_sessions.id)
			FOR UPDATE`, item.id).Scan(&item.id, &item.app, &item.env, &item.kind, &item.key)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM gis_session_keep_group_operations WHERE session_id=$1 AND application_id=$2 AND environment_id=$3`, item.id, item.app, item.env); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM gis_session_consent_receipts WHERE session_id=$1 AND application_id=$2 AND environment_id=$3`, item.id, item.app, item.env); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO gis_session_external_key_tombstones(application_id,environment_id,kind,external_key)
			VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, item.app, item.env, item.kind, item.key); err != nil {
			return 0, err
		}
		mappingOperationID := deterministicOwnerID(item.app, item.env, item.id, "chat_mapping", "")
		if _, err = tx.Exec(ctx, `DELETE FROM game_resource_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, item.app, item.env, mappingOperationID); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `UPDATE game_resource_mappings SET status='tombstoned',updated_at=clock_timestamp() WHERE application_id=$1 AND environment_id=$2 AND external_key=$3`, item.app, item.env, item.key); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM gis_session_outbox WHERE session_id=$1`, item.id); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM gis_sessions WHERE id=$1`, item.id); err != nil {
			return 0, err
		}
		purged++
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return purged, nil
}

func lockScopedExternalKey(ctx context.Context, tx pgx.Tx, appID, environmentID uuid.UUID, externalKey string) error {
	scope := appID.String() + ":" + environmentID.String() + ":" + externalKey
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, scope)
	return err
}

func (s *Store) scheduleRetry(ctx context.Context, key SessionOperationKey, owner uuid.UUID, err error) error {
	_, e := s.Pool.Exec(ctx, `UPDATE gis_session_operations SET lease_owner=NULL,lease_until=NULL,
		stage_retry_count=stage_retry_count+1,
		next_attempt_at=now()+LEAST(30, (1 << LEAST(stage_retry_count,5))) * interval '1 second',
		error_code=$4,updated_at=now() WHERE application_id=$1 AND environment_id=$2
		AND operation_id=$3 AND status='pending' AND lease_owner=$5 AND lease_until>clock_timestamp()`,
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
