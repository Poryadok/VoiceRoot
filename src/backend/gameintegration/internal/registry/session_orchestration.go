package registry

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/principal"
)

type SessionOwnerRequest struct {
	OperationID         uuid.UUID
	RequestHash         string
	ApplicationID       uuid.UUID
	EnvironmentID       uuid.UUID
	SessionID           uuid.UUID
	Kind                string
	ExternalKey         string
	DisplayName         string
	ChatID              uuid.UUID
	ChatOwnerSessionID  uuid.UUID
	ChatCreateReceiptID uuid.UUID
	RosterRevision      int64
	Members             []uuid.UUID
	VoiceRoomID         uuid.UUID
	Proto               proto.Message
}
type SessionOwnerReceipt struct {
	ResourceID  uuid.UUID
	ReceiptID   uuid.UUID
	RequestHash string
}
type SessionOwnerAdapters struct {
	CreateChat       func(context.Context, SessionOwnerRequest) (SessionOwnerReceipt, error)
	SyncChatRoster   func(context.Context, SessionOwnerRequest) (SessionOwnerReceipt, error)
	ProvisionVoice   func(context.Context, SessionOwnerRequest) (SessionOwnerReceipt, error)
	ApplyRoleGrants  func(context.Context, SessionOwnerRequest) (SessionOwnerReceipt, error)
	CloseVoice       func(context.Context, SessionOwnerRequest) (SessionOwnerReceipt, error)
	RevokeRoleGrants func(context.Context, SessionOwnerRequest) (SessionOwnerReceipt, error)
}
type SessionOrchestrator struct {
	Store  *Store
	Owners SessionOwnerAdapters
}
type SessionActiveOutboxEvent struct {
	EventID       *uuid.UUID
	ApplicationID uuid.UUID
	EnvironmentID uuid.UUID
	SessionID     uuid.UUID
	EventType     string
	Payload       []byte
}

func NewSessionOrchestrator(store *Store, owners SessionOwnerAdapters) *SessionOrchestrator {
	return &SessionOrchestrator{Store: store, Owners: owners}
}

func (o *SessionOrchestrator) CreateSession(ctx context.Context, p SessionPrincipal, in CreateSessionInput) (SessionOperation, error) {
	if o == nil || o.Store == nil || o.Store.Pool == nil {
		return SessionOperation{}, errors.New("session store unavailable")
	}
	if err := validateCreate(in); err != nil {
		return SessionOperation{}, err
	}
	hash, err := sessionRequestHash(p, in)
	if err != nil {
		return SessionOperation{}, err
	}
	return o.Store.accept(ctx, p, in, hash)
}
func (o *SessionOrchestrator) GetOperation(ctx context.Context, p SessionPrincipal, id uuid.UUID) (SessionOperation, error) {
	if o == nil || o.Store == nil {
		return SessionOperation{}, errors.New("session store unavailable")
	}
	return o.Store.getOperation(ctx, p, id)
}
func (o *SessionOrchestrator) GetActiveOutboxEvent(ctx context.Context, sessionID uuid.UUID) (*SessionActiveOutboxEvent, error) {
	var event SessionActiveOutboxEvent
	var eventID uuid.UUID
	err := o.Store.Pool.QueryRow(ctx, `SELECT event_id,application_id,environment_id,session_id,event_type,payload FROM gis_session_outbox WHERE session_id=$1 AND event_type='voice.game.sessions.active.v1'`, sessionID).Scan(&eventID, &event.ApplicationID, &event.EnvironmentID, &event.SessionID, &event.EventType, &event.Payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	event.EventID = &eventID
	return &event, nil
}

func (o *SessionOrchestrator) CloseSession(ctx context.Context, p SessionPrincipal, sessionID, operationID uuid.UUID) (SessionOperation, error) {
	if operationID == uuid.Nil || sessionID == uuid.Nil {
		return SessionOperation{}, errors.New("invalid close request")
	}
	var body struct {
		OperationKind string `json:"operation_kind"`
		Target        string `json:"target_session_id"`
		APIVersion    string `json:"api_version"`
		Scope         struct {
			ApplicationID string `json:"application_id"`
			EnvironmentID string `json:"environment_id"`
		} `json:"scope"`
		Request struct {
			OperationID string `json:"operation_id"`
		} `json:"request"`
	}
	body.OperationKind = "close"
	body.Target = sessionID.String()
	body.APIVersion = "v1"
	body.Scope.ApplicationID = p.ApplicationID.String()
	body.Scope.EnvironmentID = p.EnvironmentID.String()
	body.Request.OperationID = operationID.String()
	bytes, err := json.Marshal(body)
	if err != nil {
		return SessionOperation{}, err
	}
	bytes, err = jsoncanonicalizer.Transform(bytes)
	if err != nil {
		return SessionOperation{}, err
	}
	sum := sha256.Sum256(bytes)
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return SessionOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var oldHash []byte
	var oldSession uuid.UUID
	err = tx.QueryRow(ctx, `SELECT request_hash,session_id FROM gis_session_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, p.ApplicationID, p.EnvironmentID, operationID).Scan(&oldHash, &oldSession)
	if err == nil {
		if !equalBytes(oldHash, sum[:]) || oldSession != sessionID {
			return SessionOperation{}, ErrIdempotencyConflict
		}
		if e := tx.Commit(ctx); e != nil {
			return SessionOperation{}, e
		}
		return o.Store.getOperation(ctx, p, operationID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return SessionOperation{}, err
	}
	var status string
	var terminal *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT session_status,terminalization_operation_id FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3 FOR UPDATE`, sessionID, p.ApplicationID, p.EnvironmentID).Scan(&status, &terminal)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionOperation{}, ErrSessionNotFound
	}
	if err != nil {
		return SessionOperation{}, err
	}
	if status == "active" {
		var children int
		err = tx.QueryRow(ctx, `SELECT count(*) FROM gis_sessions WHERE parent_party_key=(SELECT external_key FROM gis_sessions WHERE id=$1) AND application_id=$2 AND environment_id=$3 AND session_status='active'`, sessionID, p.ApplicationID, p.EnvironmentID).Scan(&children)
		if err != nil {
			return SessionOperation{}, err
		}
		if children > 0 {
			return SessionOperation{}, errors.New("party has active children")
		}
	}
	if terminal == nil {
		terminal = &operationID
		_, err = tx.Exec(ctx, `UPDATE gis_sessions SET terminalization_operation_id=$2,terminalization_kind='close',terminalization_stage='voice_close_pending',session_status='closing',stage='voice_close_pending',updated_at=now() WHERE id=$1`, sessionID, operationID)
		if err != nil {
			return SessionOperation{}, err
		}
	}
	stage := "voice_close_pending"
	if *terminal != operationID {
		stage = "coalesced"
	}
	_, err = tx.Exec(ctx, `INSERT INTO gis_session_operations(operation_id,session_id,application_id,environment_id,operation_kind,request_hash,status,stage) VALUES($1,$2,$3,$4,'close',$5,'pending',$6)`, operationID, sessionID, p.ApplicationID, p.EnvironmentID, sum[:], stage)
	if err != nil {
		return SessionOperation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionOperation{}, err
	}
	return o.Store.getOperation(ctx, p, operationID)
}

// FailSession fences a permanently rejected create operation through the same
// session terminalization CAS used by explicit close.
func (o *SessionOrchestrator) FailSession(ctx context.Context, p SessionPrincipal, sessionID, operationID uuid.UUID, reason string) (SessionOperation, error) {
	if operationID == uuid.Nil || sessionID == uuid.Nil || strings.TrimSpace(reason) == "" {
		return SessionOperation{}, errors.New("invalid session failure")
	}
	var payload struct {
		Kind          string `json:"operation_kind"`
		Target        string `json:"target_session_id"`
		Version       string `json:"api_version"`
		ApplicationID string `json:"application_id"`
		EnvironmentID string `json:"environment_id"`
		OperationID   string `json:"operation_id"`
		Reason        string `json:"reason"`
	}
	payload.Kind = "failure"
	payload.Target = sessionID.String()
	payload.Version = "v1"
	payload.ApplicationID = p.ApplicationID.String()
	payload.EnvironmentID = p.EnvironmentID.String()
	payload.OperationID = operationID.String()
	payload.Reason = reason
	bytes, err := json.Marshal(payload)
	if err != nil {
		return SessionOperation{}, err
	}
	sum := sha256.Sum256(bytes)
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return SessionOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	var winner *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT session_status,terminalization_operation_id FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3 FOR UPDATE`, sessionID, p.ApplicationID, p.EnvironmentID).Scan(&status, &winner)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionOperation{}, ErrSessionNotFound
	}
	if err != nil {
		return SessionOperation{}, err
	}
	if status == "closed" || status == "failed" {
		return SessionOperation{}, errors.New("session already terminal")
	}
	if winner == nil {
		winner = &operationID
		_, err = tx.Exec(ctx, `UPDATE gis_sessions SET terminalization_operation_id=$2,terminalization_kind='failure',terminalization_stage='failure_voice_close_pending',session_status='closing',stage='failure_voice_close_pending',updated_at=now() WHERE id=$1`, sessionID, operationID)
		if err != nil {
			return SessionOperation{}, err
		}
	}
	stage := "failure_voice_close_pending"
	if *winner != operationID {
		stage = "coalesced"
	}
	_, err = tx.Exec(ctx, `INSERT INTO gis_session_operations(operation_id,session_id,application_id,environment_id,operation_kind,request_hash,status,stage,error_code) VALUES($1,$2,$3,$4,'failure',$5,'pending',$6,$7)`, operationID, sessionID, p.ApplicationID, p.EnvironmentID, sum[:], stage, reason)
	if err != nil {
		return SessionOperation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionOperation{}, err
	}
	return o.Store.getOperation(ctx, p, operationID)
}

func (o *SessionOrchestrator) AdvanceOne(ctx context.Context, operationID uuid.UUID) (SessionOperation, error) {
	if o == nil || o.Store == nil {
		return SessionOperation{}, errors.New("session store unavailable")
	}
	sessionID, kind, stage, leaseOwner, err := o.Store.claim(ctx, operationID)
	if err != nil {
		return SessionOperation{}, err
	}
	var p SessionPrincipal
	var req SessionOwnerRequest
	var sessionKind, external, display string
	var app, env, chat, chatOwner, chatReceipt, voice uuid.UUID
	var revision int64
	var members []uuid.UUID
	err = o.Store.Pool.QueryRow(ctx, `SELECT application_id,environment_id,kind,external_key,COALESCE(display_name,''),COALESCE(chat_id,'00000000-0000-0000-0000-000000000000'::uuid),COALESCE(chat_owner_session_id,'00000000-0000-0000-0000-000000000000'::uuid),COALESCE(chat_create_receipt_id,'00000000-0000-0000-0000-000000000000'::uuid),COALESCE(voice_room_id,'00000000-0000-0000-0000-000000000000'::uuid),roster_revision,members FROM gis_sessions WHERE id=$1`, sessionID).Scan(&app, &env, &sessionKind, &external, &display, &chat, &chatOwner, &chatReceipt, &voice, &revision, &members)
	if err != nil {
		return SessionOperation{}, err
	}
	p = SessionPrincipal{ApplicationID: app, EnvironmentID: env}
	var createInput CreateSessionInput
	err = o.Store.Pool.QueryRow(ctx, `SELECT operation_id FROM gis_session_operations WHERE session_id=$1 AND operation_kind='create' ORDER BY created_at LIMIT 1`, sessionID).Scan(&createInput.OperationID)
	if err != nil {
		return SessionOperation{}, err
	}
	req = SessionOwnerRequest{ApplicationID: app, EnvironmentID: env, SessionID: sessionID, Kind: sessionKind, ExternalKey: external, DisplayName: display, ChatID: chat, ChatOwnerSessionID: chatOwner, ChatCreateReceiptID: chatReceipt, RosterRevision: revision, Members: members, VoiceRoomID: voice}
	if stage == "active" {
		return o.finishActive(ctx, operationID, sessionID, p)
	}
	if strings.Contains(stage, "close") || strings.Contains(stage, "revoke") {
		return o.advanceTerminal(ctx, operationID, sessionID, stage, leaseOwner, req)
	}
	if stage == "coalesced" {
		return o.coalesceOperation(ctx, operationID, sessionID)
	}
	if kind == "close" {
		return SessionOperation{}, errors.New("close operation stage mismatch")
	}
	var ownerStage string
	discriminator := ""
	if stage == "roster_ready" || stage == "voice_ready" {
		discriminator = fmt.Sprint(revision)
	}
	var call func(context.Context, SessionOwnerRequest) (SessionOwnerReceipt, error)
	switch stage {
	case "accepted":
		if chatOwner != sessionID {
			return o.advanceNoOwner(ctx, operationID, sessionID, stage, "chat_ready")
		}
		ownerStage = "chat_create"
		call = o.Owners.CreateChat
	case "chat_ready":
		if chatOwner != sessionID {
			return o.advanceNoOwner(ctx, operationID, sessionID, stage, "roster_ready")
		}
		ownerStage = "chat_roster"
		call = o.Owners.SyncChatRoster
	case "roster_ready":
		ownerStage = "voice_provision"
		call = o.Owners.ProvisionVoice
	case "voice_ready":
		ownerStage = "role_apply"
		call = o.Owners.ApplyRoleGrants
	case "grants_ready":
		return o.finishActive(ctx, operationID, sessionID, p)
	default:
		return SessionOperation{}, fmt.Errorf("unknown session stage %q", stage)
	}
	req.OperationID = deterministicOwnerID(app, env, sessionID, ownerStage, discriminator)
	message, buildErr := BuildSessionOwnerProto(ownerStage, req)
	if buildErr != nil {
		return SessionOperation{}, buildErr
	}
	req.Proto = message
	req.RequestHash, err = principal.RequestHash(message)
	if err != nil {
		return SessionOperation{}, err
	}
	requestHashBytes, err := principalHashBytes(req.RequestHash)
	if err != nil {
		return SessionOperation{}, err
	}
	if call == nil {
		err = errors.New("session owner unavailable")
	} else {
		var receipt SessionOwnerReceipt
		receipt, err = call(ctx, req)
		if err == nil {
			if receipt.ResourceID == uuid.Nil || receipt.ReceiptID == uuid.Nil || receipt.RequestHash != req.RequestHash {
				err = errors.New("invalid owner receipt")
			} else {
				if err = o.Store.saveOwnerReceipt(ctx, operationID, req.OperationID, ownerStage, requestHashBytes, receipt); err == nil {
					err = o.recordResource(ctx, operationID, sessionID, ownerStage, leaseOwner, receipt)
				}
			}
		}
	}
	if err != nil {
		_ = o.Store.scheduleRetry(ctx, operationID, leaseOwner, err)
		return SessionOperation{}, err
	}
	return o.Store.getOperation(ctx, p, operationID)
}

func (o *SessionOrchestrator) recordResource(ctx context.Context, op, session uuid.UUID, stage string, owner uuid.UUID, r SessionOwnerReceipt) error {
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var next string
	switch stage {
	case "chat_create":
		next = "chat_ready"
		_, err = tx.Exec(ctx, `UPDATE gis_sessions SET chat_id=$2,chat_create_receipt_id=$3,updated_at=now() WHERE id=$1`, session, r.ResourceID, r.ReceiptID)
	case "chat_roster":
		next = "roster_ready"
		_, err = tx.Exec(ctx, `UPDATE gis_sessions SET chat_roster_receipt_id=$2,updated_at=now() WHERE id=$1`, session, r.ReceiptID)
	case "voice_provision":
		next = "voice_ready"
		_, err = tx.Exec(ctx, `UPDATE gis_sessions SET voice_room_id=$2,voice_provision_receipt_id=$3,updated_at=now() WHERE id=$1`, session, r.ResourceID, r.ReceiptID)
	case "role_apply":
		next = "grants_ready"
		_, err = tx.Exec(ctx, `UPDATE gis_sessions SET role_grant_receipt_id=$2,updated_at=now() WHERE id=$1`, session, r.ReceiptID)
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE gis_session_operations SET stage=$3,stage_retry_count=0,lease_owner=NULL,lease_until=NULL,next_attempt_at=now(),updated_at=now() WHERE operation_id=$1 AND lease_owner=$2`, op, owner, next)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE gis_sessions SET stage=$2,updated_at=now() WHERE id=$1`, session, next)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (o *SessionOrchestrator) advanceNoOwner(ctx context.Context, op, session uuid.UUID, stage, next string) (SessionOperation, error) {
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return SessionOperation{}, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `UPDATE gis_session_operations SET stage=$2,stage_retry_count=0,lease_owner=NULL,lease_until=NULL,next_attempt_at=now(),updated_at=now() WHERE operation_id=$1`, op, next)
	if err != nil {
		return SessionOperation{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE gis_sessions SET stage=$2,updated_at=now() WHERE id=$1`, session, next)
	if err != nil {
		return SessionOperation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionOperation{}, err
	}
	var p SessionPrincipal
	_ = o.Store.Pool.QueryRow(ctx, `SELECT application_id,environment_id FROM gis_sessions WHERE id=$1`, session).Scan(&p.ApplicationID, &p.EnvironmentID)
	return o.Store.getOperation(ctx, p, op)
}

func (o *SessionOrchestrator) finishActive(ctx context.Context, op, session uuid.UUID, p SessionPrincipal) (SessionOperation, error) {
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return SessionOperation{}, err
	}
	defer tx.Rollback(ctx)
	event := uuid.New()
	tag, err := tx.Exec(ctx, `UPDATE gis_session_operations SET status='succeeded',stage='active',active_event_id=$2,stage_retry_count=0,lease_owner=NULL,lease_until=NULL,updated_at=now() WHERE operation_id=$1 AND status='pending'`, op, event)
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, ErrIdempotencyConflict
	}
	tag, err = tx.Exec(ctx, `UPDATE gis_sessions SET session_status='active',stage='active',updated_at=now() WHERE id=$1 AND session_status='provisioning'`, session)
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, ErrIdempotencyConflict
	}
	var activeAt time.Time
	var kind string
	if err = tx.QueryRow(ctx, `SELECT kind,clock_timestamp() FROM gis_sessions WHERE id=$1`, session).Scan(&kind, &activeAt); err != nil {
		return SessionOperation{}, err
	}
	payload, marshalErr := json.Marshal(SessionActiveEvent{
		EventID: event, ApplicationID: p.ApplicationID, EnvironmentID: p.EnvironmentID,
		SessionID: session, OperationID: op, Kind: kind, ActiveAt: activeAt.UTC(),
	})
	if marshalErr != nil {
		return SessionOperation{}, marshalErr
	}
	digest := sha256.Sum256(payload)
	_, err = tx.Exec(ctx, `INSERT INTO gis_session_outbox(event_id,application_id,environment_id,session_id,event_type,payload,payload_bytes,payload_sha256,created_at)
		VALUES($1,$2,$3,$4,'voice.game.sessions.active.v1',$5::jsonb,$6,$7,$8)`, event, p.ApplicationID, p.EnvironmentID, session, string(payload), payload, digest[:], activeAt)
	if err != nil {
		return SessionOperation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionOperation{}, err
	}
	return o.Store.getOperation(ctx, p, op)
}

func (o *SessionOrchestrator) advanceTerminal(ctx context.Context, op, session uuid.UUID, stage string, leaseOwner uuid.UUID, req SessionOwnerRequest) (SessionOperation, error) {
	var p SessionPrincipal
	var app, env uuid.UUID
	var sessionStage, terminalKind string
	var terminal uuid.UUID
	err := o.Store.Pool.QueryRow(ctx, `SELECT application_id,environment_id,terminalization_operation_id,terminalization_kind,terminalization_stage FROM gis_sessions WHERE id=$1`, session).Scan(&app, &env, &terminal, &terminalKind, &sessionStage)
	if err != nil {
		return SessionOperation{}, err
	}
	p = SessionPrincipal{ApplicationID: app, EnvironmentID: env}
	if terminal != op {
		return o.coalesceOperation(ctx, op, session)
	}
	var adapter func(context.Context, SessionOwnerRequest) (SessionOwnerReceipt, error)
	var ownerStage, next string
	failureTerminalization := strings.HasPrefix(stage, "failure_")
	baseStage := strings.TrimPrefix(stage, "failure_")
	switch baseStage {
	case "voice_close_pending":
		adapter = o.Owners.CloseVoice
		ownerStage = "terminalize_voice_close"
		next = "role_revoke_pending"
		if failureTerminalization {
			next = "failure_role_revoke_pending"
		}
	case "role_revoke_pending":
		adapter = o.Owners.RevokeRoleGrants
		ownerStage = "terminalize_role_revoke"
		next = "closed"
	default:
		return SessionOperation{}, fmt.Errorf("unknown terminal stage %s", stage)
	}
	req.OperationID = deterministicOwnerID(app, env, session, ownerStage, terminal.String())
	message, err := BuildSessionOwnerProto(ownerStage, req)
	if err != nil {
		return SessionOperation{}, err
	}
	req.Proto = message
	req.RequestHash, err = principal.RequestHash(message)
	if err != nil {
		return SessionOperation{}, err
	}
	requestHashBytes, err := principalHashBytes(req.RequestHash)
	if err != nil {
		return SessionOperation{}, err
	}
	if adapter == nil {
		return SessionOperation{}, errors.New("terminal owner unavailable")
	}
	receipt, err := adapter(ctx, req)
	if err != nil {
		_ = o.Store.scheduleRetry(ctx, op, leaseOwner, err)
		return SessionOperation{}, err
	}
	if receipt.ReceiptID == uuid.Nil || receipt.ResourceID == uuid.Nil || receipt.RequestHash != req.RequestHash {
		return SessionOperation{}, errors.New("invalid terminal owner receipt")
	}
	if err = o.Store.saveOwnerReceipt(ctx, op, req.OperationID, ownerStage, requestHashBytes, receipt); err != nil {
		return SessionOperation{}, err
	}
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return SessionOperation{}, err
	}
	defer tx.Rollback(ctx)
	if baseStage == "voice_close_pending" {
		_, err = tx.Exec(ctx, `UPDATE gis_sessions SET terminalization_stage=$3,voice_close_receipt_id=$4,updated_at=now() WHERE id=$1 AND terminalization_operation_id=$2`, session, op, next, receipt.ReceiptID)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE gis_session_operations SET stage=$2,stage_retry_count=0,lease_owner=NULL,lease_until=NULL,updated_at=now() WHERE operation_id=$1 AND lease_owner=$3`, op, next, leaseOwner)
		}
		if err != nil {
			return SessionOperation{}, err
		}
	} else {
		finalStage := "closed"
		if terminalKind == "failure" {
			finalStage = "failed"
		}
		_, err = tx.Exec(ctx, `UPDATE gis_sessions SET terminalization_stage=$3,session_status=$3,stage=$3,role_revoke_receipt_id=$4,updated_at=now() WHERE id=$1 AND terminalization_operation_id=$2`, session, op, finalStage, receipt.ReceiptID)
		if err == nil {
			status := "succeeded"
			_, err = tx.Exec(ctx, `UPDATE gis_session_operations SET status=$2,stage=$3,stage_retry_count=0,voice_close_receipt_id=(SELECT receipt_id FROM gis_session_owner_receipts WHERE operation_id=$1 AND stage='terminalize_voice_close'),role_revoke_receipt_id=$4,lease_owner=NULL,lease_until=NULL,updated_at=now() WHERE operation_id=$1 AND lease_owner=$5`, op, status, finalStage, receipt.ReceiptID, leaseOwner)
			if err == nil && terminalKind == "failure" {
				_, err = tx.Exec(ctx, `UPDATE gis_session_operations SET status='failed',stage='failed',voice_close_receipt_id=(SELECT voice_close_receipt_id FROM gis_sessions WHERE id=$1),role_revoke_receipt_id=(SELECT role_revoke_receipt_id FROM gis_sessions WHERE id=$1),updated_at=now() WHERE session_id=$1 AND operation_kind='create'`, session)
			}
		}
		if err != nil {
			return SessionOperation{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionOperation{}, err
	}
	return o.Store.getOperation(ctx, p, op)
}

func (o *SessionOrchestrator) coalesceOperation(ctx context.Context, op, session uuid.UUID) (SessionOperation, error) {
	var p SessionPrincipal
	var winner uuid.UUID
	var sessionStatus string
	err := o.Store.Pool.QueryRow(ctx, `SELECT application_id,environment_id,terminalization_operation_id,session_status FROM gis_sessions WHERE id=$1`, session).Scan(&p.ApplicationID, &p.EnvironmentID, &winner, &sessionStatus)
	if err != nil {
		return SessionOperation{}, err
	}
	if winner == uuid.Nil {
		return SessionOperation{}, errors.New("terminalization winner missing")
	}
	if sessionStatus == "closed" || sessionStatus == "failed" {
		_, err = o.Store.Pool.Exec(ctx, `UPDATE gis_session_operations SET status='succeeded',stage=$2,voice_close_receipt_id=(SELECT voice_close_receipt_id FROM gis_sessions WHERE id=$3),role_revoke_receipt_id=(SELECT role_revoke_receipt_id FROM gis_sessions WHERE id=$3),lease_owner=NULL,lease_until=NULL WHERE operation_id=$1`, op, sessionStatus, session)
		if err != nil {
			return SessionOperation{}, err
		}
	} else {
		_, err = o.Store.Pool.Exec(ctx, `UPDATE gis_session_operations SET stage='coalesced',lease_owner=NULL,lease_until=NULL,next_attempt_at=now()+interval '1 second' WHERE operation_id=$1`, op)
		if err != nil {
			return SessionOperation{}, err
		}
	}
	return o.Store.getOperation(ctx, p, op)
}

func validateCreate(in CreateSessionInput) error {
	if in.OperationID == uuid.Nil || in.RosterRevision <= 0 || !in.RosterComplete || in.Members == nil {
		return errors.New("invalid session request")
	}
	if in.Kind != "party" && in.Kind != "match" && in.Kind != "fleet" {
		return errors.New("invalid session kind")
	}
	if in.ExternalKey == "" || len(in.ExternalKey) > 255 {
		return errors.New("invalid external key")
	}
	if in.Kind == "party" && in.ParentPartyKey != "" {
		return errors.New("party cannot have parent")
	}
	if in.ParentPartyKey != "" && in.DisplayName != "" {
		return errors.New("parented child cannot set display_name")
	}
	if in.ParentPartyKey == "" && strings.TrimSpace(in.DisplayName) == "" {
		return errors.New("display_name required")
	}
	for i, id := range in.Members {
		if id == uuid.Nil || i > 0 && in.Members[i-1].String() >= id.String() {
			return errors.New("members must be sorted unique UUIDs")
		}
	}
	return nil
}

func ValidateCreateSessionInput(in CreateSessionInput) error { return validateCreate(in) }

type sessionHashEnvelope struct {
	APIVersion string `json:"api_version"`
	Scope      struct {
		ApplicationID string `json:"application_id"`
		EnvironmentID string `json:"environment_id"`
	} `json:"scope"`
	Request CreateSessionInput `json:"request"`
}

func sessionRequestHash(p SessionPrincipal, in CreateSessionInput) ([]byte, error) {
	var value sessionHashEnvelope
	value.APIVersion = "v1"
	value.Scope.ApplicationID = p.ApplicationID.String()
	value.Scope.EnvironmentID = p.EnvironmentID.String()
	value.Request = in
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	b, err = jsoncanonicalizer.Transform(b)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(b)
	return sum[:], nil
}

var sessionURLNamespace = uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")

func deterministicOwnerID(app, env, session uuid.UUID, stage, discriminator string) uuid.UUID {
	name := "voice-gis-t31/v1\n" + app.String() + "\n" + env.String() + "\n" + session.String() + "\n" + stage + "\n" + discriminator
	return uuid.NewSHA1(sessionURLNamespace, []byte(name))
}
func safeOwnerError(error) string { return "OWNER_UNAVAILABLE" }
