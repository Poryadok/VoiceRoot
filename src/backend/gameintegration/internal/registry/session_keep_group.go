package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/principal"
)

var (
	ErrSessionKeepGroupDenied   = errors.New("keep-group operation denied")
	ErrSessionKeepGroupConflict = errors.New("keep-group operation conflicts with saved request")
)

type SessionKeepGroupInput struct {
	OperationID    uuid.UUID
	SessionID      uuid.UUID
	AccountID      uuid.UUID
	ProfileID      uuid.UUID
	RosterRevision int64
}

type SessionKeepGroupReceipt struct {
	OperationID     uuid.UUID   `json:"operation_id"`
	ReceiptID       uuid.UUID   `json:"receipt_id"`
	SessionID       uuid.UUID   `json:"session_id"`
	ChatID          uuid.UUID   `json:"chat_id"`
	DestinationKey  string      `json:"destination_key"`
	RosterRevision  int64       `json:"roster_revision"`
	PolicyRevision  int64       `json:"policy_revision"`
	AddedProfiles   []uuid.UUID `json:"added_profiles"`
	ChatReceiptID   uuid.UUID   `json:"chat_receipt_id"`
	ChatRequestHash string      `json:"chat_request_hash"`
	Replayed        bool        `json:"replayed"`
}

type sessionKeepGroupWork struct {
	Input          SessionKeepGroupInput
	RequestHash    []byte
	ApplicationID  uuid.UUID
	EnvironmentID  uuid.UUID
	DestinationKey string
	PolicyRevision int64
	TargetKind     string
	ChatID         uuid.UUID
	DisplayName    string
	Members        []uuid.UUID
	CreateOp       uuid.UUID
	CreateReceipt  uuid.UUID
	CreateHash     string
	MembersOp      uuid.UUID
	MembersReceipt uuid.UUID
	MembersHash    string
}

func (o *SessionOrchestrator) KeepSessionGroup(ctx context.Context, input SessionKeepGroupInput) (SessionKeepGroupReceipt, error) {
	if o == nil || o.Store == nil || o.Store.Pool == nil {
		return SessionKeepGroupReceipt{}, errors.New("session store unavailable")
	}
	work, receipt, err := o.Store.prepareSessionKeepGroup(ctx, input)
	if err != nil {
		return SessionKeepGroupReceipt{}, err
	}
	if receipt != nil {
		receipt.Replayed = true
		return *receipt, nil
	}
	if work.TargetKind == "new_chat" && work.CreateReceipt == uuid.Nil {
		if o.Owners.CreateChat == nil {
			return SessionKeepGroupReceipt{}, errors.New("managed Chat creation owner unavailable")
		}
		if err := o.Store.validateSessionKeepGroupWork(ctx, work); err != nil {
			return SessionKeepGroupReceipt{}, err
		}
		request := SessionOwnerRequest{OperationID: work.CreateOp, ApplicationID: work.ApplicationID, EnvironmentID: work.EnvironmentID,
			SessionID: work.Input.SessionID, ExternalKey: "game-keep-group/" + work.Input.SessionID.String(), DisplayName: work.DisplayName}
		message, err := BuildSessionOwnerProto("chat_create", request)
		if err != nil {
			return SessionKeepGroupReceipt{}, err
		}
		request.Proto = message
		request.RequestHash, err = principal.RequestHash(message)
		if err != nil {
			return SessionKeepGroupReceipt{}, err
		}
		ownerReceipt, err := o.Owners.CreateChat(ctx, request)
		if err != nil {
			return SessionKeepGroupReceipt{}, err
		}
		if ownerReceipt.ResourceID == uuid.Nil || ownerReceipt.ReceiptID != request.OperationID || ownerReceipt.RequestHash != request.RequestHash {
			return SessionKeepGroupReceipt{}, errors.New("invalid managed Chat creation receipt")
		}
		work.ChatID, work.CreateReceipt, work.CreateHash = ownerReceipt.ResourceID, ownerReceipt.ReceiptID, ownerReceipt.RequestHash
		if err := o.Store.recordSessionKeepGroupChat(ctx, work); err != nil {
			return SessionKeepGroupReceipt{}, err
		}
	}
	if o.Owners.SyncChatRoster == nil {
		return SessionKeepGroupReceipt{}, errors.New("managed Chat membership owner unavailable")
	}
	if err := o.Store.validateSessionKeepGroupWork(ctx, work); err != nil {
		return SessionKeepGroupReceipt{}, err
	}
	stage := "chat_roster"
	if work.TargetKind == "party_chat" {
		stage = "chat_add_members"
	}
	request := SessionOwnerRequest{OperationID: work.MembersOp, ApplicationID: work.ApplicationID, EnvironmentID: work.EnvironmentID,
		SessionID: work.Input.SessionID, ChatID: work.ChatID, Members: append([]uuid.UUID(nil), work.Members...)}
	message, err := BuildSessionOwnerProto(stage, request)
	if err != nil {
		return SessionKeepGroupReceipt{}, err
	}
	request.Proto = message
	request.RequestHash, err = principal.RequestHash(message)
	if err != nil {
		return SessionKeepGroupReceipt{}, err
	}
	ownerReceipt, err := o.Owners.SyncChatRoster(ctx, request)
	if err != nil {
		return SessionKeepGroupReceipt{}, err
	}
	if ownerReceipt.ResourceID != work.ChatID || ownerReceipt.ReceiptID != request.OperationID || ownerReceipt.RequestHash != request.RequestHash {
		return SessionKeepGroupReceipt{}, errors.New("invalid managed Chat membership receipt")
	}
	work.MembersReceipt, work.MembersHash = ownerReceipt.ReceiptID, ownerReceipt.RequestHash
	return o.Store.completeSessionKeepGroup(ctx, work)
}

func (s *Store) prepareSessionKeepGroup(ctx context.Context, input SessionKeepGroupInput) (sessionKeepGroupWork, *SessionKeepGroupReceipt, error) {
	if s == nil || s.Pool == nil || input.OperationID == uuid.Nil || input.SessionID == uuid.Nil || input.AccountID == uuid.Nil || input.ProfileID == uuid.Nil || input.RosterRevision <= 0 {
		return sessionKeepGroupWork{}, nil, ErrSessionKeepGroupDenied
	}
	requestBytes, err := json.Marshal(struct {
		Version int `json:"version"`
		SessionKeepGroupInput
	}{1, input})
	if err != nil {
		return sessionKeepGroupWork{}, nil, err
	}
	requestHash := sha256.Sum256(requestBytes)
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return sessionKeepGroupWork{}, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 31455))`, input.OperationID.String()); err != nil {
		return sessionKeepGroupWork{}, nil, err
	}
	var savedHash, receiptBytes []byte
	var createReceiptID, memberReceiptID *uuid.UUID
	var savedChatID *uuid.UUID
	var createRequestHash, memberRequestHash *string
	var operationStatus string
	var work sessionKeepGroupWork
	err = tx.QueryRow(ctx, `SELECT request_hash,application_id,environment_id,session_id,account_id,profile_id,roster_revision,destination_key,policy_revision,target_kind,chat_id,display_name,member_profiles,create_operation_id,create_receipt_id,create_request_hash,member_operation_id,member_receipt_id,member_request_hash,status,receipt_bytes
FROM gis_session_keep_group_operations WHERE operation_id=$1 FOR UPDATE`, input.OperationID).Scan(
		&savedHash, &work.ApplicationID, &work.EnvironmentID, &work.Input.SessionID, &work.Input.AccountID, &work.Input.ProfileID,
		&work.Input.RosterRevision, &work.DestinationKey, &work.PolicyRevision, &work.TargetKind, &savedChatID, &work.DisplayName,
		&work.Members, &work.CreateOp, &createReceiptID, &createRequestHash, &work.MembersOp, &memberReceiptID, &memberRequestHash,
		&operationStatus, &receiptBytes)
	if err == nil {
		if savedChatID != nil {
			work.ChatID = *savedChatID
		}
		if !equalBytes(savedHash, requestHash[:]) {
			return sessionKeepGroupWork{}, nil, ErrSessionKeepGroupConflict
		}
		if createReceiptID != nil {
			work.CreateReceipt = *createReceiptID
		}
		if createRequestHash != nil {
			work.CreateHash = *createRequestHash
		}
		if memberReceiptID != nil {
			work.MembersReceipt = *memberReceiptID
		}
		if memberRequestHash != nil {
			work.MembersHash = *memberRequestHash
		}
		work.Input.OperationID, work.RequestHash = input.OperationID, requestHash[:]
		if len(receiptBytes) != 0 {
			var receipt SessionKeepGroupReceipt
			if err := json.Unmarshal(receiptBytes, &receipt); err != nil {
				return sessionKeepGroupWork{}, nil, errors.New("stored keep-group receipt is invalid")
			}
			if err := tx.Commit(ctx); err != nil {
				return sessionKeepGroupWork{}, nil, err
			}
			return sessionKeepGroupWork{}, &receipt, nil
		}
		if err := tx.Commit(ctx); err != nil {
			return sessionKeepGroupWork{}, nil, err
		}
		return work, nil, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sessionKeepGroupWork{}, nil, err
	}
	work = sessionKeepGroupWork{Input: input, RequestHash: requestHash[:]}
	var kind, externalKey string
	var parentPartyKey *string
	var leaseValid, complete bool
	var members []uuid.UUID
	var displayName *string
	err = tx.QueryRow(ctx, `SELECT application_id,environment_id,kind,external_key,parent_party_key,display_name,roster_complete,members,session_status='active' AND lease_expires_at>clock_timestamp()
FROM gis_sessions WHERE id=$1 FOR UPDATE`, input.SessionID).Scan(&work.ApplicationID, &work.EnvironmentID, &kind, &externalKey, &parentPartyKey, &displayName, &complete, &members, &leaseValid)
	if err != nil || !complete || !leaseValid || !containsUUID(members, input.ProfileID) {
		return sessionKeepGroupWork{}, nil, ErrSessionKeepGroupDenied
	}
	var actorBinding bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_bindings WHERE application_id=$1 AND environment_id=$2 AND account_id=$3 AND profile_id=$4 AND status='active')`,
		work.ApplicationID, work.EnvironmentID, input.AccountID, input.ProfileID).Scan(&actorBinding); err != nil || !actorBinding {
		return sessionKeepGroupWork{}, nil, ErrSessionKeepGroupDenied
	}
	var currentRevision int64
	if err := tx.QueryRow(ctx, `SELECT roster_revision FROM gis_sessions WHERE id=$1 AND roster_revision=$2`, input.SessionID, input.RosterRevision).Scan(&currentRevision); err != nil {
		return sessionKeepGroupWork{}, nil, ErrSessionKeepGroupDenied
	}
	if displayName == nil || strings.TrimSpace(*displayName) == "" {
		work.DisplayName = "Game group"
	} else {
		work.DisplayName = strings.TrimSpace(*displayName)
	}
	work.DestinationKey = sessionConsentDestination(kind, externalKey, parentPartyKey, input.SessionID)
	if err := tx.QueryRow(ctx, `SELECT e.revision FROM environments e JOIN applications a ON a.id=e.application_id AND a.status IN ('sandbox','active') WHERE e.id=$1 AND e.application_id=$2 AND e.status='active'`, work.EnvironmentID, work.ApplicationID).Scan(&work.PolicyRevision); err != nil || work.PolicyRevision <= 0 {
		return sessionKeepGroupWork{}, nil, ErrSessionKeepGroupDenied
	}
	if err := tx.QueryRow(ctx, `SELECT COALESCE(array_agg(c.profile_id ORDER BY c.profile_id),ARRAY[]::uuid[])
FROM gis_session_consent_receipts c
JOIN environments e ON e.id=c.environment_id AND e.application_id=c.application_id AND e.status='active'
JOIN applications a ON a.id=c.application_id AND a.status IN ('sandbox','active')
JOIN player_bindings b ON b.application_id=c.application_id AND b.environment_id=c.environment_id AND b.account_id=c.account_id AND b.profile_id=c.profile_id AND b.status='active'
WHERE c.session_id=$1 AND c.roster_revision=$2 AND c.destination_key=$3 AND c.policy_revision=e.revision AND e.revision=$4`,
		input.SessionID, input.RosterRevision, work.DestinationKey, work.PolicyRevision).Scan(&work.Members); err != nil || len(work.Members) == 0 {
		return sessionKeepGroupWork{}, nil, ErrSessionKeepGroupDenied
	}
	work.TargetKind = "new_chat"
	if kind == "party" {
		work.TargetKind = "party_chat"
		err = tx.QueryRow(ctx, `SELECT chat_id FROM gis_sessions WHERE id=$1 AND session_status='active' AND chat_id IS NOT NULL`, input.SessionID).Scan(&work.ChatID)
	} else if parentPartyKey != nil && *parentPartyKey != "" {
		work.TargetKind = "party_chat"
		err = tx.QueryRow(ctx, `SELECT chat_id FROM gis_sessions WHERE application_id=$1 AND environment_id=$2 AND kind='party' AND external_key=$3 AND session_status='active' AND chat_id IS NOT NULL`,
			work.ApplicationID, work.EnvironmentID, *parentPartyKey).Scan(&work.ChatID)
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return sessionKeepGroupWork{}, nil, err
	}
	if work.TargetKind == "party_chat" && work.ChatID == uuid.Nil {
		return sessionKeepGroupWork{}, nil, ErrSessionKeepGroupDenied
	}
	work.CreateOp = deterministicOwnerID(work.ApplicationID, work.EnvironmentID, input.SessionID, "keep_group_chat_create", "")
	work.MembersOp = deterministicOwnerID(work.ApplicationID, work.EnvironmentID, input.SessionID, "keep_group_chat_members", input.OperationID.String())
	_, err = tx.Exec(ctx, `INSERT INTO gis_session_keep_group_operations(operation_id,request_hash,application_id,environment_id,session_id,account_id,profile_id,roster_revision,destination_key,policy_revision,target_kind,chat_id,display_name,member_profiles,create_operation_id,member_operation_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, input.OperationID, requestHash[:], work.ApplicationID, work.EnvironmentID, input.SessionID,
		input.AccountID, input.ProfileID, input.RosterRevision, work.DestinationKey, work.PolicyRevision, work.TargetKind, nilUUID(work.ChatID), work.DisplayName, work.Members, work.CreateOp, work.MembersOp)
	if err != nil {
		return sessionKeepGroupWork{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return sessionKeepGroupWork{}, nil, err
	}
	return work, nil, nil
}

func (s *Store) validateSessionKeepGroupWork(ctx context.Context, work sessionKeepGroupWork) error {
	var current bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(
SELECT 1 FROM gis_sessions s JOIN environments e ON e.id=s.environment_id AND e.application_id=s.application_id
JOIN applications a ON a.id=s.application_id AND a.status IN ('sandbox','active')
WHERE s.id=$1 AND s.application_id=$2 AND s.environment_id=$3 AND s.session_status='active' AND s.roster_complete
AND s.roster_revision=$4 AND s.lease_expires_at>clock_timestamp() AND e.status='active' AND e.revision=$5
AND NOT EXISTS(SELECT 1 FROM unnest($6::uuid[]) p(profile_id) WHERE NOT EXISTS(
SELECT 1 FROM gis_session_consent_receipts c JOIN player_bindings b ON b.application_id=c.application_id AND b.environment_id=c.environment_id AND b.account_id=c.account_id AND b.profile_id=c.profile_id AND b.status='active'
WHERE c.session_id=s.id AND c.profile_id=p.profile_id AND c.roster_revision=s.roster_revision AND c.destination_key=$7 AND c.policy_revision=e.revision)))`,
		work.Input.SessionID, work.ApplicationID, work.EnvironmentID, work.Input.RosterRevision, work.PolicyRevision, work.Members, work.DestinationKey).Scan(&current)
	if err != nil || !current {
		return ErrSessionKeepGroupDenied
	}
	return nil
}

func (s *Store) recordSessionKeepGroupChat(ctx context.Context, work sessionKeepGroupWork) error {
	_, err := s.Pool.Exec(ctx, `UPDATE gis_session_keep_group_operations SET chat_id=$2,create_receipt_id=$3,create_request_hash=$4,updated_at=clock_timestamp()
	WHERE operation_id=$1 AND status='pending' AND target_kind='new_chat' AND request_hash=$5`, work.Input.OperationID, work.ChatID, nilUUID(work.CreateReceipt), nilString(work.CreateHash), work.RequestHash)
	return err
}

func (s *Store) completeSessionKeepGroup(ctx context.Context, work sessionKeepGroupWork) (SessionKeepGroupReceipt, error) {
	receipt := SessionKeepGroupReceipt{OperationID: work.Input.OperationID, ReceiptID: work.Input.OperationID, SessionID: work.Input.SessionID,
		ChatID: work.ChatID, DestinationKey: work.DestinationKey, RosterRevision: work.Input.RosterRevision, PolicyRevision: work.PolicyRevision,
		AddedProfiles: append([]uuid.UUID(nil), work.Members...), ChatReceiptID: work.MembersReceipt, ChatRequestHash: work.MembersHash}
	bytes, err := json.Marshal(receipt)
	if err != nil {
		return SessionKeepGroupReceipt{}, err
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE gis_session_keep_group_operations SET chat_id=$2,create_receipt_id=$3,create_request_hash=$4,member_receipt_id=$5,member_request_hash=$6,status='completed',receipt_bytes=$7::jsonb,updated_at=clock_timestamp()
	WHERE operation_id=$1 AND status='pending' AND request_hash=$8`, work.Input.OperationID, work.ChatID, nilUUID(work.CreateReceipt), nilString(work.CreateHash), work.MembersReceipt, work.MembersHash, bytes, work.RequestHash)
	if err != nil {
		return SessionKeepGroupReceipt{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionKeepGroupReceipt{}, errors.New("keep-group receipt changed while completing")
	}
	return receipt, nil
}

func sessionConsentDestination(kind, externalKey string, parentPartyKey *string, sessionID uuid.UUID) string {
	if kind == "party" {
		return "party/" + externalKey
	}
	if parentPartyKey != nil && *parentPartyKey != "" {
		return "party/" + *parentPartyKey
	}
	return "session/" + sessionID.String()
}

func nilUUID(value uuid.UUID) any {
	if value == uuid.Nil {
		return nil
	}
	return value
}

func nilString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (w sessionKeepGroupWork) String() string {
	return fmt.Sprintf("session keep-group %s (%s)", w.Input.SessionID, w.Input.OperationID)
}
