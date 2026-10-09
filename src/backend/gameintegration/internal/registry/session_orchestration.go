package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

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
	var voiceRoomID, chatOwnerSessionID *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT session_status,terminalization_operation_id,voice_room_id,chat_owner_session_id FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3 FOR UPDATE`, sessionID, p.ApplicationID, p.EnvironmentID).Scan(&status, &terminal, &voiceRoomID, &chatOwnerSessionID)
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
		var createID uuid.UUID
		var createStatus, createStage, createErrorCode string
		var createLeaseOwner *uuid.UUID
		err = tx.QueryRow(ctx, `SELECT operation_id,status,stage,COALESCE(error_code,''),lease_owner FROM gis_session_operations
			WHERE session_id=$1 AND application_id=$2 AND environment_id=$3 AND operation_kind='create'
			ORDER BY created_at LIMIT 1 FOR UPDATE`, sessionID, p.ApplicationID, p.EnvironmentID).Scan(&createID, &createStatus, &createStage, &createErrorCode, &createLeaseOwner)
		if err != nil {
			return SessionOperation{}, err
		}
		stage := "voice_close_pending"
		sessionStatus := "closing"
		reconcileCreateStage := ""
		// A live/expired lease means an owner call may still be in flight. A
		// retry marker means the last call's outcome was uncertain. In either
		// case, closing must replay the same owner key before terminal cleanup.
		if createStatus == "pending" && (createLeaseOwner != nil || createErrorCode != "") {
			switch createStage {
			case "accepted":
				if chatOwnerSessionID != nil && *chatOwnerSessionID == sessionID {
					reconcileCreateStage = createStage
				}
			case "chat_ready":
				if chatOwnerSessionID != nil && *chatOwnerSessionID == sessionID {
					reconcileCreateStage = createStage
				}
			case "roster_ready", "voice_ready":
				reconcileCreateStage = createStage
			}
		}
		if reconcileCreateStage != "" {
			stage = "create_reconcile_pending:" + reconcileCreateStage
		} else if voiceRoomID == nil {
			stage, sessionStatus = "closed", "closed"
		}
		_, err = tx.Exec(ctx, `UPDATE gis_sessions SET terminalization_operation_id=$2,terminalization_kind='close',terminalization_stage=$3,session_status=$4,stage=$3,updated_at=now() WHERE id=$1`, sessionID, operationID, stage, sessionStatus)
		if err != nil {
			return SessionOperation{}, err
		}
		// The close operation takes custody of any claimed owner stage: it replays
		// the same owner key under its own lease before terminal cleanup.
		_, err = tx.Exec(ctx, `UPDATE gis_session_operations SET status='failed',stage='failed',error_code=COALESCE(error_code,'SESSION_CLOSED'),
			voice_close_receipt_id=(SELECT voice_close_receipt_id FROM gis_sessions WHERE id=$1),
			role_revoke_receipt_id=(SELECT role_revoke_receipt_id FROM gis_sessions WHERE id=$1),
			lease_owner=NULL,lease_until=NULL,updated_at=now()
			WHERE operation_id=$4 AND application_id=$2 AND environment_id=$3 AND operation_kind='create' AND status='pending'`,
			sessionID, p.ApplicationID, p.EnvironmentID, createID)
		if err != nil {
			return SessionOperation{}, err
		}
	}
	stage := "voice_close_pending"
	operationStatus := "pending"
	if *terminal == operationID {
		var savedStage string
		err = tx.QueryRow(ctx, `SELECT terminalization_stage FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3`, sessionID, p.ApplicationID, p.EnvironmentID).Scan(&savedStage)
		if err != nil {
			return SessionOperation{}, err
		}
		if savedStage == "closed" {
			stage, operationStatus = "closed", "succeeded"
		} else if strings.HasPrefix(savedStage, "create_reconcile_pending:") {
			stage = savedStage
		}
	}
	if *terminal != operationID {
		stage = "coalesced"
	}
	_, err = tx.Exec(ctx, `INSERT INTO gis_session_operations(operation_id,session_id,application_id,environment_id,operation_kind,request_hash,status,stage) VALUES($1,$2,$3,$4,'close',$5,$6,$7)`, operationID, sessionID, p.ApplicationID, p.EnvironmentID, sum[:], operationStatus, stage)
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
	var oldHash []byte
	var oldSession uuid.UUID
	err = tx.QueryRow(ctx, `SELECT request_hash,session_id FROM gis_session_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, p.ApplicationID, p.EnvironmentID, operationID).Scan(&oldHash, &oldSession)
	if err == nil {
		if !equalBytes(oldHash, sum[:]) || oldSession != sessionID {
			return SessionOperation{}, ErrIdempotencyConflict
		}
		if err = tx.Commit(ctx); err != nil {
			return SessionOperation{}, err
		}
		return o.Store.getOperation(ctx, p, operationID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return SessionOperation{}, err
	}
	var status string
	var winner *uuid.UUID
	var voiceRoomID *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT session_status,terminalization_operation_id,voice_room_id FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3 FOR UPDATE`, sessionID, p.ApplicationID, p.EnvironmentID).Scan(&status, &winner, &voiceRoomID)
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
		stage, sessionStatus := "failure_voice_close_pending", "closing"
		if voiceRoomID == nil {
			stage, sessionStatus = "failed", "failed"
		}
		_, err = tx.Exec(ctx, `UPDATE gis_sessions SET terminalization_operation_id=$2,terminalization_kind='failure',terminalization_stage=$3,session_status=$4,stage=$3,updated_at=now() WHERE id=$1`, sessionID, operationID, stage, sessionStatus)
		if err != nil {
			return SessionOperation{}, err
		}
		_, err = tx.Exec(ctx, `UPDATE gis_session_operations SET status='failed',stage='failed',error_code=$4,
			voice_close_receipt_id=(SELECT voice_close_receipt_id FROM gis_sessions WHERE id=$1),
			role_revoke_receipt_id=(SELECT role_revoke_receipt_id FROM gis_sessions WHERE id=$1),
			lease_owner=NULL,lease_until=NULL,updated_at=now()
			WHERE session_id=$1 AND application_id=$2 AND environment_id=$3 AND operation_kind='create' AND status='pending'`,
			sessionID, p.ApplicationID, p.EnvironmentID, reason)
		if err != nil {
			return SessionOperation{}, err
		}
	}
	stage := "failure_voice_close_pending"
	operationStatus := "pending"
	if *winner == operationID && voiceRoomID == nil {
		stage, operationStatus = "failed", "succeeded"
	}
	if *winner != operationID {
		stage = "coalesced"
	}
	_, err = tx.Exec(ctx, `INSERT INTO gis_session_operations(operation_id,session_id,application_id,environment_id,operation_kind,request_hash,status,stage,error_code) VALUES($1,$2,$3,$4,'failure',$5,$6,$7,$8)`, operationID, sessionID, p.ApplicationID, p.EnvironmentID, sum[:], operationStatus, stage, reason)
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
	key, err := o.Store.operationKeyForID(ctx, operationID)
	if err != nil {
		return SessionOperation{}, err
	}
	return o.AdvanceScoped(ctx, key)
}

// AdvanceScoped advances an operation using its complete application and
// environment namespace. UUID-only lookup is retained for compatibility, but
// rejects ambiguous IDs rather than selecting an arbitrary tenant.
func (o *SessionOrchestrator) AdvanceScoped(ctx context.Context, key SessionOperationKey) (SessionOperation, error) {
	if o == nil || o.Store == nil {
		return SessionOperation{}, errors.New("session store unavailable")
	}
	if key.ApplicationID == uuid.Nil || key.EnvironmentID == uuid.Nil || key.OperationID == uuid.Nil {
		return SessionOperation{}, errors.New("invalid session operation key")
	}
	scopePrincipal := SessionPrincipal{ApplicationID: key.ApplicationID, EnvironmentID: key.EnvironmentID}
	current, err := o.Store.getOperation(ctx, scopePrincipal, key.OperationID)
	if err != nil {
		return SessionOperation{}, err
	}
	if current.Status == "pending" && strings.HasPrefix(current.Stage, "create_reconcile_unresolved:") {
		// An owner operation conflict can conceal a prior effect. Keep the close
		// operation visible in durable custody without replaying a known immutable
		// rejection or dispatching a new owner operation.
		return current, nil
	}
	sessionID, kind, stage, leaseOwner, err := o.Store.claimScoped(ctx, key)
	if err != nil {
		return SessionOperation{}, err
	}
	var p SessionPrincipal
	var req SessionOwnerRequest
	var sessionKind, external, display string
	var app, env, chat, chatOwner, chatReceipt, voice uuid.UUID
	var revision int64
	var members []uuid.UUID
	err = o.Store.Pool.QueryRow(ctx, `SELECT application_id,environment_id,kind,external_key,COALESCE(display_name,''),COALESCE(chat_id,'00000000-0000-0000-0000-000000000000'::uuid),COALESCE(chat_owner_session_id,'00000000-0000-0000-0000-000000000000'::uuid),COALESCE(chat_create_receipt_id,'00000000-0000-0000-0000-000000000000'::uuid),COALESCE(voice_room_id,'00000000-0000-0000-0000-000000000000'::uuid),roster_revision,members FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3`, sessionID, key.ApplicationID, key.EnvironmentID).Scan(&app, &env, &sessionKind, &external, &display, &chat, &chatOwner, &chatReceipt, &voice, &revision, &members)
	if err != nil {
		return SessionOperation{}, err
	}
	p = SessionPrincipal{ApplicationID: app, EnvironmentID: env}
	var createInput CreateSessionInput
	err = o.Store.Pool.QueryRow(ctx, `SELECT operation_id FROM gis_session_operations WHERE session_id=$1 AND application_id=$2 AND environment_id=$3 AND operation_kind='create' ORDER BY created_at LIMIT 1`, sessionID, key.ApplicationID, key.EnvironmentID).Scan(&createInput.OperationID)
	if err != nil {
		return SessionOperation{}, err
	}
	req = SessionOwnerRequest{ApplicationID: app, EnvironmentID: env, SessionID: sessionID, Kind: sessionKind, ExternalKey: external, DisplayName: display, ChatID: chat, ChatOwnerSessionID: chatOwner, ChatCreateReceiptID: chatReceipt, RosterRevision: revision, Members: members, VoiceRoomID: voice}
	if strings.HasPrefix(stage, "create_reconcile_pending:") {
		return o.reconcileClosingCreateStage(ctx, key, leaseOwner, sessionID, p, req, createInput.OperationID, strings.TrimPrefix(stage, "create_reconcile_pending:"))
	}
	if strings.HasPrefix(stage, "create_reconcile_no_effect:") {
		return o.advanceNoEffectClosingCreateStage(ctx, key, leaseOwner, sessionID, req, strings.TrimPrefix(stage, "create_reconcile_no_effect:"))
	}
	if stage == "active" {
		return o.finishActive(ctx, key, sessionID, stage, leaseOwner, p)
	}
	if strings.Contains(stage, "close") || strings.Contains(stage, "revoke") {
		return o.advanceTerminal(ctx, key, sessionID, stage, leaseOwner, req)
	}
	if stage == "coalesced" {
		return o.coalesceOperation(ctx, key, sessionID, stage, leaseOwner)
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
			return o.advanceNoOwner(ctx, key, sessionID, stage, "chat_ready", leaseOwner)
		}
		ownerStage = "chat_create"
		call = o.Owners.CreateChat
	case "chat_ready":
		if chatOwner != sessionID {
			return o.advanceNoOwner(ctx, key, sessionID, stage, "roster_ready", leaseOwner)
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
		return o.finishActive(ctx, key, sessionID, stage, leaseOwner, p)
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
			if receipt.ResourceID == uuid.Nil || receipt.ReceiptID == uuid.Nil || receipt.RequestHash != req.RequestHash ||
				(ownerStage == "chat_create" && receipt.ReceiptID != req.OperationID) {
				err = errors.New("invalid owner receipt")
			} else {
				var mapping *ResourceMappingInput
				if ownerStage == "chat_create" {
					mapping = &ResourceMappingInput{
						ApplicationID: app, EnvironmentID: env,
						OperationID: deterministicOwnerID(app, env, sessionID, "chat_mapping", ""),
						ResourceKind: "chat", ExternalKey: external, ResourceID: receipt.ResourceID,
						ChatID: receipt.ResourceID, ChatOperationID: req.OperationID, ChatRequestHash: req.RequestHash,
					}
				}
				err = o.recordResource(ctx, key, sessionID, stage, ownerStage, req.OperationID, leaseOwner, receipt, requestHashBytes, mapping)
			}
		}
	}
	if err != nil {
		var permanent *PermanentOwnerRejection
		if errors.As(err, &permanent) && permanent != nil {
			if reason := permanentOwnerFailureCode(ownerStage, permanent.Category); reason != "" {
				failureID := deterministicOwnerID(app, env, sessionID, "create_failure", reason)
				if _, failureErr := o.FailSession(ctx, p, sessionID, failureID, reason); failureErr != nil {
					_ = o.Store.scheduleRetry(ctx, key, leaseOwner, failureErr)
					return SessionOperation{}, failureErr
				}
				return o.Store.getOperation(ctx, p, key.OperationID)
			}
		}
		_ = o.Store.scheduleRetry(ctx, key, leaseOwner, err)
		return SessionOperation{}, err
	}
	return o.Store.getOperation(ctx, p, key.OperationID)
}

func (o *SessionOrchestrator) reconcileClosingCreateStage(ctx context.Context, closeKey SessionOperationKey, closeLeaseOwner, sessionID uuid.UUID, p SessionPrincipal, req SessionOwnerRequest, createOperationID uuid.UUID, createStage string) (SessionOperation, error) {
	discriminator := ""
	var ownerStage string
	var call func(context.Context, SessionOwnerRequest) (SessionOwnerReceipt, error)
	switch createStage {
	case "accepted":
		if req.ChatOwnerSessionID != sessionID {
			return SessionOperation{}, errors.New("unexpected no-owner stage in close reconciliation")
		}
		ownerStage, call = "chat_create", o.Owners.CreateChat
	case "chat_ready":
		if req.ChatOwnerSessionID != sessionID {
			return SessionOperation{}, errors.New("unexpected no-owner stage in close reconciliation")
		}
		ownerStage, call = "chat_roster", o.Owners.SyncChatRoster
	case "roster_ready":
		ownerStage, call, discriminator = "voice_provision", o.Owners.ProvisionVoice, fmt.Sprint(req.RosterRevision)
	case "voice_ready":
		ownerStage, call, discriminator = "role_apply", o.Owners.ApplyRoleGrants, fmt.Sprint(req.RosterRevision)
	default:
		return SessionOperation{}, fmt.Errorf("unsupported create reconciliation stage %q", createStage)
	}
	req.OperationID = deterministicOwnerID(req.ApplicationID, req.EnvironmentID, sessionID, ownerStage, discriminator)
	message, err := BuildSessionOwnerProto(ownerStage, req)
	if err != nil {
		return SessionOperation{}, err
	}
	req.Proto = message
	req.RequestHash, err = principal.RequestHash(message)
	if err != nil {
		return SessionOperation{}, err
	}
	requestHash, err := principalHashBytes(req.RequestHash)
	if err != nil {
		return SessionOperation{}, err
	}
	if call == nil {
		err = errors.New("session owner unavailable")
	} else {
		var receipt SessionOwnerReceipt
		receipt, err = call(ctx, req)
		if err == nil && (receipt.ResourceID == uuid.Nil || receipt.ReceiptID == uuid.Nil || receipt.RequestHash != req.RequestHash ||
			(ownerStage == "chat_create" && receipt.ReceiptID != req.OperationID)) {
			err = errors.New("invalid owner receipt during close reconciliation")
		}
		if err == nil {
			return o.persistClosingCreateReceipt(ctx, closeKey, closeLeaseOwner, sessionID, p, createOperationID, ownerStage, createStage, req, requestHash, receipt)
		}
	}
	var permanent *PermanentOwnerRejection
	if errors.As(err, &permanent) && permanent != nil {
		if code := permanentOwnerFailureCode(ownerStage, permanent.Category); code != "" {
			return o.persistClosingCreateRejection(ctx, closeKey, closeLeaseOwner, sessionID, p, createStage, ownerStage, permanent.Category, code)
		}
	}
	_ = o.Store.scheduleRetry(ctx, closeKey, closeLeaseOwner, err)
	return SessionOperation{}, err
}

func closeRejectionProvesNoEffect(ownerStage, category string) bool {
	return ownerStage == "chat_roster" && category == "resource_missing" ||
		ownerStage == "role_apply" && category == "terminal_revoked"
}

func (o *SessionOrchestrator) persistClosingCreateRejection(ctx context.Context, key SessionOperationKey, leaseOwner, session uuid.UUID, p SessionPrincipal, createStage, ownerStage, category, code string) (SessionOperation, error) {
	noEffect := closeRejectionProvesNoEffect(ownerStage, category)
	marker := "create_reconcile_unresolved:" + createStage + ":" + category
	closeCode := "GIS_OWNER_UNRESOLVED_" + strings.TrimPrefix(code, "OWNER_")
	if noEffect {
		marker = "create_reconcile_no_effect:" + createStage + ":" + category
		closeCode = "GIS_OWNER_NO_EFFECT_" + strings.TrimPrefix(code, "OWNER_")
	}
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return SessionOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `UPDATE gis_session_operations SET stage=$5,error_code=$6,lease_owner=NULL,lease_until=NULL,
		next_attempt_at=CASE WHEN $7 THEN now() ELSE 'infinity'::timestamptz END,updated_at=now()
		WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3 AND lease_owner=$4
		AND stage=$8 AND status='pending' AND lease_until>clock_timestamp()`,
		key.ApplicationID, key.EnvironmentID, key.OperationID, leaseOwner, marker, closeCode, noEffect,
		"create_reconcile_pending:"+createStage)
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, ErrSessionLeaseLost
	}
	tag, err = tx.Exec(ctx, `UPDATE gis_sessions SET terminalization_stage=$4,stage=$4,updated_at=now()
		WHERE id=$1 AND application_id=$2 AND environment_id=$3 AND terminalization_operation_id=$5
		AND terminalization_stage=$6 AND session_status='closing'`,
		session, key.ApplicationID, key.EnvironmentID, marker, key.OperationID, "create_reconcile_pending:"+createStage)
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, ErrSessionLeaseLost
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionOperation{}, err
	}
	return o.Store.getOperation(ctx, p, key.OperationID)
}

func (o *SessionOrchestrator) advanceNoEffectClosingCreateStage(ctx context.Context, key SessionOperationKey, leaseOwner, session uuid.UUID, req SessionOwnerRequest, marker string) (SessionOperation, error) {
	parts := strings.Split(marker, ":")
	if len(parts) != 2 || !closeRejectionProvesNoEffect(ownerStageForCreateStage(parts[0]), parts[1]) {
		return SessionOperation{}, errors.New("invalid close no-effect marker")
	}
	ownerStage := ownerStageForCreateStage(parts[0])
	code := permanentOwnerFailureCode(ownerStage, parts[1])
	if code == "" {
		return SessionOperation{}, errors.New("invalid close no-effect rejection")
	}
	nextStage, nextStatus, sessionStatus := "voice_close_pending", "pending", "closing"
	if req.VoiceRoomID == uuid.Nil {
		nextStage, nextStatus, sessionStatus = "closed", "succeeded", "closed"
	}
	currentMarker := "create_reconcile_no_effect:" + marker
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return SessionOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `UPDATE gis_session_operations SET stage=$5,status=$6,error_code=$7,lease_owner=NULL,lease_until=NULL,
		next_attempt_at=now(),updated_at=now()
		WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3 AND lease_owner=$4
		AND stage=$8 AND status='pending' AND lease_until>clock_timestamp()`,
		key.ApplicationID, key.EnvironmentID, key.OperationID, leaseOwner, nextStage, nextStatus, code, currentMarker)
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, ErrSessionLeaseLost
	}
	tag, err = tx.Exec(ctx, `UPDATE gis_sessions SET terminalization_stage=$4,session_status=$5,stage=$4,updated_at=now()
		WHERE id=$1 AND application_id=$2 AND environment_id=$3 AND terminalization_operation_id=$6
		AND terminalization_stage=$7 AND session_status='closing'`,
		session, key.ApplicationID, key.EnvironmentID, nextStage, sessionStatus, key.OperationID, currentMarker)
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, ErrSessionLeaseLost
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionOperation{}, err
	}
	return o.Store.getOperation(ctx, SessionPrincipal{ApplicationID: key.ApplicationID, EnvironmentID: key.EnvironmentID}, key.OperationID)
}

func ownerStageForCreateStage(createStage string) string {
	switch createStage {
	case "accepted":
		return "chat_create"
	case "chat_ready":
		return "chat_roster"
	case "roster_ready":
		return "voice_provision"
	case "voice_ready":
		return "role_apply"
	default:
		return ""
	}
}

func (o *SessionOrchestrator) persistClosingCreateReceipt(ctx context.Context, closeKey SessionOperationKey, closeLeaseOwner, sessionID uuid.UUID, p SessionPrincipal, createOperationID uuid.UUID, ownerStage, createStage string, req SessionOwnerRequest, requestHash []byte, receipt SessionOwnerReceipt) (SessionOperation, error) {
	newVoiceRoomID := req.VoiceRoomID
	if ownerStage == "voice_provision" {
		newVoiceRoomID = receipt.ResourceID
	}
	nextStage, nextStatus, sessionStatus := "voice_close_pending", "pending", "closing"
	if newVoiceRoomID == uuid.Nil {
		nextStage, nextStatus, sessionStatus = "closed", "succeeded", "closed"
	}
	marker := "create_reconcile_pending:" + createStage
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return SessionOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `UPDATE gis_session_operations SET stage=$5,status=$6,stage_retry_count=0,
		lease_owner=NULL,lease_until=NULL,next_attempt_at=now(),error_code=NULL,updated_at=now()
		WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3 AND lease_owner=$4
		AND stage=$7 AND status='pending' AND lease_until>clock_timestamp()`,
		closeKey.ApplicationID, closeKey.EnvironmentID, closeKey.OperationID, closeLeaseOwner, nextStage, nextStatus, marker)
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, ErrSessionLeaseLost
	}
	createKey := SessionOperationKey{ApplicationID: closeKey.ApplicationID, EnvironmentID: closeKey.EnvironmentID, OperationID: createOperationID}
	if err = saveOwnerReceiptTx(ctx, tx, createKey, req.OperationID, ownerStage, requestHash, receipt); err != nil {
		return SessionOperation{}, err
	}
	if ownerStage == "chat_create" {
		_, err = o.Store.createResourceMappingTx(ctx, tx, ResourceMappingInput{
			ApplicationID: req.ApplicationID, EnvironmentID: req.EnvironmentID,
			OperationID: deterministicOwnerID(req.ApplicationID, req.EnvironmentID, sessionID, "chat_mapping", ""),
			ResourceKind: "chat", ExternalKey: req.ExternalKey, ResourceID: receipt.ResourceID,
			ChatID: receipt.ResourceID, ChatOperationID: req.OperationID, ChatRequestHash: req.RequestHash,
		})
		if err != nil {
			return SessionOperation{}, err
		}
	}
	var update string
	switch ownerStage {
	case "chat_create":
		update = `UPDATE gis_sessions SET chat_id=$4,chat_create_receipt_id=$5,terminalization_stage=$6,session_status=$7,stage=$6,updated_at=now()
			WHERE id=$1 AND application_id=$2 AND environment_id=$3 AND terminalization_operation_id=$8 AND terminalization_stage=$9 AND session_status='closing'`
	case "chat_roster":
		update = `UPDATE gis_sessions SET chat_roster_receipt_id=$5,terminalization_stage=$6,session_status=$7,stage=$6,updated_at=now()
			WHERE id=$1 AND application_id=$2 AND environment_id=$3 AND terminalization_operation_id=$8 AND terminalization_stage=$9 AND session_status='closing'`
	case "voice_provision":
		update = `UPDATE gis_sessions SET voice_room_id=$4,voice_provision_receipt_id=$5,terminalization_stage=$6,session_status=$7,stage=$6,updated_at=now()
			WHERE id=$1 AND application_id=$2 AND environment_id=$3 AND terminalization_operation_id=$8 AND terminalization_stage=$9 AND session_status='closing'`
	case "role_apply":
		update = `UPDATE gis_sessions SET role_grant_receipt_id=$5,terminalization_stage=$6,session_status=$7,stage=$6,updated_at=now()
			WHERE id=$1 AND application_id=$2 AND environment_id=$3 AND terminalization_operation_id=$8 AND terminalization_stage=$9 AND session_status='closing'`
	default:
		return SessionOperation{}, fmt.Errorf("unsupported reconciliation receipt stage %q", ownerStage)
	}
	tag, err = tx.Exec(ctx, update, sessionID, closeKey.ApplicationID, closeKey.EnvironmentID, receipt.ResourceID,
		receipt.ReceiptID, nextStage, sessionStatus, closeKey.OperationID, marker)
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, ErrSessionLeaseLost
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionOperation{}, err
	}
	return o.Store.getOperation(ctx, p, closeKey.OperationID)
}

func (o *SessionOrchestrator) recordResource(ctx context.Context, key SessionOperationKey, session uuid.UUID, currentStage, ownerStage string, ownerOperationID, owner uuid.UUID, r SessionOwnerReceipt, requestHash []byte, mapping *ResourceMappingInput) error {
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var next string
	switch ownerStage {
	case "chat_create":
		next = "chat_ready"
	case "chat_roster":
		next = "roster_ready"
	case "voice_provision":
		next = "voice_ready"
	case "role_apply":
		next = "grants_ready"
	default:
		return errors.New("unsupported owner resource stage")
	}
	tag, err := tx.Exec(ctx, `UPDATE gis_session_operations SET stage=$6,stage_retry_count=0,error_code=NULL,lease_owner=NULL,lease_until=NULL,next_attempt_at=now(),updated_at=now()
		WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3 AND lease_owner=$4 AND stage=$5
		AND status='pending' AND lease_until>clock_timestamp()`,
		key.ApplicationID, key.EnvironmentID, key.OperationID, owner, currentStage, next)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrSessionLeaseLost
	}
	if err = saveOwnerReceiptTx(ctx, tx, key, ownerOperationID, ownerStage, requestHash, r); err != nil {
		return err
	}
	if mapping != nil {
		if _, err = o.Store.createResourceMappingTx(ctx, tx, *mapping); err != nil {
			return err
		}
	}
	var resourceRows int64
	switch ownerStage {
	case "chat_create":
		tag, err = tx.Exec(ctx, `UPDATE gis_sessions SET chat_id=$4,chat_create_receipt_id=$5,updated_at=now() WHERE id=$1 AND application_id=$2 AND environment_id=$3 AND stage=$6 AND session_status='provisioning'`, session, key.ApplicationID, key.EnvironmentID, r.ResourceID, r.ReceiptID, currentStage)
	case "chat_roster":
		tag, err = tx.Exec(ctx, `UPDATE gis_sessions SET chat_roster_receipt_id=$4,updated_at=now() WHERE id=$1 AND application_id=$2 AND environment_id=$3 AND stage=$5 AND session_status='provisioning'`, session, key.ApplicationID, key.EnvironmentID, r.ReceiptID, currentStage)
	case "voice_provision":
		tag, err = tx.Exec(ctx, `UPDATE gis_sessions SET voice_room_id=$4,voice_provision_receipt_id=$5,updated_at=now() WHERE id=$1 AND application_id=$2 AND environment_id=$3 AND stage=$6 AND session_status='provisioning'`, session, key.ApplicationID, key.EnvironmentID, r.ResourceID, r.ReceiptID, currentStage)
	case "role_apply":
		tag, err = tx.Exec(ctx, `UPDATE gis_sessions SET role_grant_receipt_id=$4,updated_at=now() WHERE id=$1 AND application_id=$2 AND environment_id=$3 AND stage=$5 AND session_status='provisioning'`, session, key.ApplicationID, key.EnvironmentID, r.ReceiptID, currentStage)
	}
	if err != nil {
		return err
	}
	resourceRows = tag.RowsAffected()
	if resourceRows != 1 {
		return ErrSessionLeaseLost
	}
	stageTag, err := tx.Exec(ctx, `UPDATE gis_sessions SET stage=$4,updated_at=now() WHERE id=$1 AND application_id=$2 AND environment_id=$3 AND stage=$5 AND session_status='provisioning'`, session, key.ApplicationID, key.EnvironmentID, next, currentStage)
	if err != nil {
		return err
	}
	if stageTag.RowsAffected() != 1 {
		return ErrSessionLeaseLost
	}
	return tx.Commit(ctx)
}

func (o *SessionOrchestrator) advanceNoOwner(ctx context.Context, key SessionOperationKey, session uuid.UUID, stage, next string, owner uuid.UUID) (SessionOperation, error) {
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return SessionOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `UPDATE gis_session_operations SET stage=$6,stage_retry_count=0,error_code=NULL,lease_owner=NULL,lease_until=NULL,next_attempt_at=now(),updated_at=now()
		WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3 AND lease_owner=$4 AND stage=$5
		AND status='pending' AND lease_until>clock_timestamp()`, key.ApplicationID, key.EnvironmentID, key.OperationID, owner, stage, next)
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, ErrSessionLeaseLost
	}
	tag, err = tx.Exec(ctx, `UPDATE gis_sessions SET stage=$4,updated_at=now() WHERE id=$1 AND application_id=$2 AND environment_id=$3 AND stage=$5 AND session_status='provisioning'`, session, key.ApplicationID, key.EnvironmentID, next, stage)
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, ErrSessionLeaseLost
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionOperation{}, err
	}
	var p SessionPrincipal
	p = SessionPrincipal{ApplicationID: key.ApplicationID, EnvironmentID: key.EnvironmentID}
	return o.Store.getOperation(ctx, p, key.OperationID)
}

func (o *SessionOrchestrator) finishActive(ctx context.Context, key SessionOperationKey, session uuid.UUID, stage string, leaseOwner uuid.UUID, p SessionPrincipal) (SessionOperation, error) {
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return SessionOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	event := uuid.New()
	tag, err := tx.Exec(ctx, `UPDATE gis_session_operations SET status='succeeded',stage='active',active_event_id=$6,stage_retry_count=0,error_code=NULL,lease_owner=NULL,lease_until=NULL,updated_at=now()
		WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3 AND lease_owner=$4 AND stage=$5
		AND status='pending' AND lease_until>clock_timestamp()`, key.ApplicationID, key.EnvironmentID, key.OperationID, leaseOwner, stage, event)
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, ErrSessionLeaseLost
	}
	tag, err = tx.Exec(ctx, `UPDATE gis_sessions SET session_status='active',stage='active',updated_at=now() WHERE id=$1 AND application_id=$2 AND environment_id=$3 AND session_status='provisioning' AND stage=$4`, session, key.ApplicationID, key.EnvironmentID, stage)
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, ErrSessionLeaseLost
	}
	var activeAt time.Time
	var kind string
	if err = tx.QueryRow(ctx, `SELECT kind,clock_timestamp() FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3`, session, key.ApplicationID, key.EnvironmentID).Scan(&kind, &activeAt); err != nil {
		return SessionOperation{}, err
	}
	payload, marshalErr := json.Marshal(SessionActiveEvent{
		EventID: event, ApplicationID: p.ApplicationID, EnvironmentID: p.EnvironmentID,
		SessionID: session, OperationID: key.OperationID, Kind: kind, ActiveAt: activeAt.UTC(),
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
	return o.Store.getOperation(ctx, p, key.OperationID)
}

func (o *SessionOrchestrator) advanceTerminal(ctx context.Context, key SessionOperationKey, session uuid.UUID, stage string, leaseOwner uuid.UUID, req SessionOwnerRequest) (SessionOperation, error) {
	var app, env uuid.UUID
	var terminalKind string
	var terminal uuid.UUID
	err := o.Store.Pool.QueryRow(ctx, `SELECT application_id,environment_id,terminalization_operation_id,terminalization_kind FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3`, session, key.ApplicationID, key.EnvironmentID).Scan(&app, &env, &terminal, &terminalKind)
	if err != nil {
		return SessionOperation{}, err
	}
	if terminal != key.OperationID {
		return o.coalesceOperation(ctx, key, session, stage, leaseOwner)
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
		_ = o.Store.scheduleRetry(ctx, key, leaseOwner, err)
		return SessionOperation{}, err
	}
	if receipt.ReceiptID == uuid.Nil || receipt.ResourceID == uuid.Nil || receipt.RequestHash != req.RequestHash {
		return SessionOperation{}, errors.New("invalid terminal owner receipt")
	}
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return SessionOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	finalStage := next
	operationStatus := "pending"
	if baseStage == "role_revoke_pending" {
		finalStage = "closed"
		if terminalKind == "failure" {
			finalStage = "failed"
		}
		operationStatus = "succeeded"
	}
	tag, err := tx.Exec(ctx, `UPDATE gis_session_operations SET status=$6,stage=$7,stage_retry_count=0,
		error_code=CASE WHEN error_code LIKE 'GIS_OWNER_NO_EFFECT_%' THEN error_code ELSE NULL END,
		voice_close_receipt_id=CASE WHEN $8 THEN (SELECT receipt_id FROM gis_session_owner_receipts
			WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3 AND stage='terminalize_voice_close')
			ELSE voice_close_receipt_id END,
		role_revoke_receipt_id=CASE WHEN $8 THEN $9 ELSE role_revoke_receipt_id END,
		lease_owner=NULL,lease_until=NULL,updated_at=now()
		WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3 AND lease_owner=$4 AND stage=$5
		AND status='pending' AND lease_until>clock_timestamp()`, key.ApplicationID, key.EnvironmentID,
		key.OperationID, leaseOwner, stage, operationStatus, finalStage, baseStage == "role_revoke_pending", receipt.ReceiptID)
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, ErrSessionLeaseLost
	}
	if err = saveOwnerReceiptTx(ctx, tx, key, req.OperationID, ownerStage, requestHashBytes, receipt); err != nil {
		return SessionOperation{}, err
	}
	if baseStage == "voice_close_pending" {
		tag, err = tx.Exec(ctx, `UPDATE gis_sessions SET terminalization_stage=$4,voice_close_receipt_id=$5,updated_at=now()
			WHERE id=$1 AND application_id=$2 AND environment_id=$3 AND terminalization_operation_id=$6 AND terminalization_stage=$7`,
			session, key.ApplicationID, key.EnvironmentID, finalStage, receipt.ReceiptID, key.OperationID, stage)
	} else {
		tag, err = tx.Exec(ctx, `UPDATE gis_sessions SET terminalization_stage=$4,session_status=$4,stage=$4,
			role_revoke_receipt_id=$5,updated_at=now()
			WHERE id=$1 AND application_id=$2 AND environment_id=$3 AND terminalization_operation_id=$6 AND terminalization_stage=$7`,
			session, key.ApplicationID, key.EnvironmentID, finalStage, receipt.ReceiptID, key.OperationID, stage)
	}
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, ErrIdempotencyConflict
	}
	if baseStage == "role_revoke_pending" && terminalKind == "failure" {
		tag, err = tx.Exec(ctx, `UPDATE gis_session_operations SET status='failed',stage='failed',
			voice_close_receipt_id=(SELECT voice_close_receipt_id FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3),
			role_revoke_receipt_id=(SELECT role_revoke_receipt_id FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3),updated_at=now()
			WHERE session_id=$1 AND application_id=$2 AND environment_id=$3 AND operation_kind='create' AND status IN ('failed','succeeded')`,
			session, key.ApplicationID, key.EnvironmentID)
		if err != nil {
			return SessionOperation{}, err
		}
		if tag.RowsAffected() != 1 {
			return SessionOperation{}, ErrIdempotencyConflict
		}
	} else if baseStage == "role_revoke_pending" {
		_, err = tx.Exec(ctx, `UPDATE gis_session_operations SET
			voice_close_receipt_id=(SELECT voice_close_receipt_id FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3),
			role_revoke_receipt_id=(SELECT role_revoke_receipt_id FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3),updated_at=now()
			WHERE session_id=$1 AND application_id=$2 AND environment_id=$3 AND operation_kind='create' AND status IN ('failed','succeeded')`,
			session, key.ApplicationID, key.EnvironmentID)
		if err != nil {
			return SessionOperation{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionOperation{}, err
	}
	return o.Store.getOperation(ctx, SessionPrincipal{ApplicationID: app, EnvironmentID: env}, key.OperationID)
}


func (o *SessionOrchestrator) coalesceOperation(ctx context.Context, key SessionOperationKey, session uuid.UUID, stage string, leaseOwner uuid.UUID) (SessionOperation, error) {
	tx, err := o.Store.Pool.Begin(ctx)
	if err != nil {
		return SessionOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var p SessionPrincipal
	var winner uuid.UUID
	var sessionStatus string
	err = tx.QueryRow(ctx, `SELECT application_id,environment_id,terminalization_operation_id,session_status FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3 FOR SHARE`, session, key.ApplicationID, key.EnvironmentID).Scan(&p.ApplicationID, &p.EnvironmentID, &winner, &sessionStatus)
	if err != nil {
		return SessionOperation{}, err
	}
	if winner == uuid.Nil {
		return SessionOperation{}, errors.New("terminalization winner missing")
	}
	settled := sessionStatus == "closed" || sessionStatus == "failed"
	operationStatus, operationStage := "pending", "coalesced"
	if settled {
		operationStatus, operationStage = "succeeded", sessionStatus
	}
	tag, err := tx.Exec(ctx, `UPDATE gis_session_operations SET status=$6,stage=$7,
		voice_close_receipt_id=CASE WHEN $8 THEN (SELECT voice_close_receipt_id FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3) ELSE voice_close_receipt_id END,
		role_revoke_receipt_id=CASE WHEN $8 THEN (SELECT role_revoke_receipt_id FROM gis_sessions WHERE id=$1 AND application_id=$2 AND environment_id=$3) ELSE role_revoke_receipt_id END,
		lease_owner=NULL,lease_until=NULL,next_attempt_at=CASE WHEN $8 THEN next_attempt_at ELSE now()+interval '1 second' END
		WHERE application_id=$2 AND environment_id=$3 AND operation_id=$4 AND status='pending' AND stage=$5
		AND lease_owner=$9 AND lease_until>clock_timestamp()`, session, key.ApplicationID, key.EnvironmentID,
		key.OperationID, stage, operationStatus, operationStage, settled, leaseOwner)
	if err != nil {
		return SessionOperation{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionOperation{}, ErrSessionLeaseLost
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionOperation{}, err
	}
	return o.Store.getOperation(ctx, p, key.OperationID)
}

func validateCreate(in CreateSessionInput) error {
	if in.OperationID == uuid.Nil || in.RosterRevision <= 0 || in.RosterRevision > 9007199254740991 || !in.RosterComplete || in.Members == nil {
		return errors.New("invalid session request")
	}
	if in.Kind != "party" && in.Kind != "match" && in.Kind != "fleet" {
		return errors.New("invalid session kind")
	}
	if in.ExternalKey == "" || len(in.ExternalKey) > 255 || !utf8.ValidString(in.ExternalKey) {
		return errors.New("invalid external key")
	}
	if in.Kind == "party" && in.ParentPartyKey != "" {
		return errors.New("party cannot have parent")
	}
	if in.ParentPartyKey != "" && in.DisplayName != "" {
		return errors.New("parented child cannot set display_name")
	}
	if in.ParentPartyKey == "" && (strings.TrimSpace(in.DisplayName) == "" || !utf8.ValidString(in.DisplayName) || utf8.RuneCountInString(in.DisplayName) > 128) {
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
	if err := validateCreate(in); err != nil {
		return nil, err
	}
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

