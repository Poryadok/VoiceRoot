package registry

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/principal"
)

type sessionOwnerScript struct {
	blocked       map[string]bool
	loseNextReply map[string]bool
	calls         map[string][]SessionOwnerRequest
	receipts      map[sessionOwnerEffectKey]SessionOwnerReceipt
	sideEffects   map[sessionOwnerEffectKey]int
}

type sessionOwnerEffectKey struct {
	operationID uuid.UUID
	requestHash string
}

func newSessionOwnerScript() *sessionOwnerScript {
	return &sessionOwnerScript{
		blocked: make(map[string]bool), loseNextReply: make(map[string]bool),
		calls: make(map[string][]SessionOwnerRequest), receipts: make(map[sessionOwnerEffectKey]SessionOwnerReceipt),
		sideEffects: make(map[sessionOwnerEffectKey]int),
	}
}

func (s *sessionOwnerScript) result(stage string, request SessionOwnerRequest) (SessionOwnerReceipt, error) {
	s.calls[stage] = append(s.calls[stage], request)
	key := sessionOwnerEffectKey{operationID: request.OperationID, requestHash: request.RequestHash}
	if s.blocked[stage] {
		return SessionOwnerReceipt{}, errors.New("owner receipt not available yet")
	}
	if receipt, committed := s.receipts[key]; committed {
		return receipt, nil
	}
	receiptID := uuid.New()
	if stage == "chat_create" {
		// ProvisionManagedChat defines receipt_id as the durable operation UUID.
		receiptID = request.OperationID
	}
	receipt := SessionOwnerReceipt{ResourceID: uuid.New(), ReceiptID: receiptID, RequestHash: request.RequestHash}
	s.receipts[key] = receipt
	s.sideEffects[key]++
	if s.loseNextReply[stage] {
		s.loseNextReply[stage] = false
		return SessionOwnerReceipt{}, errors.New("simulated transport loss after owner commit")
	}
	return receipt, nil
}

func (s *sessionOwnerScript) adapters() SessionOwnerAdapters {
	return SessionOwnerAdapters{
		CreateChat: func(_ context.Context, in SessionOwnerRequest) (SessionOwnerReceipt, error) {
			return s.result("chat_create", in)
		},
		SyncChatRoster: func(_ context.Context, in SessionOwnerRequest) (SessionOwnerReceipt, error) {
			return s.result("chat_roster", in)
		},
		ProvisionVoice: func(_ context.Context, in SessionOwnerRequest) (SessionOwnerReceipt, error) {
			return s.result("voice_provision", in)
		},
		ApplyRoleGrants: func(_ context.Context, in SessionOwnerRequest) (SessionOwnerReceipt, error) {
			return s.result("role_apply", in)
		},
		CloseVoice: func(_ context.Context, in SessionOwnerRequest) (SessionOwnerReceipt, error) {
			return s.result("voice_close", in)
		},
		RevokeRoleGrants: func(_ context.Context, in SessionOwnerRequest) (SessionOwnerReceipt, error) {
			return s.result("role_revoke", in)
		},
	}
}

func startT31SessionStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	migrationPath := filepath.Join("..", "..", "..", "migrations", "game_integration_db", "000012_t31_sessions.up.sql")
	migration, err := os.ReadFile(migrationPath)
	require.NoError(t, err, "T31 session persistence migration must be installed")
	_, err = pool.Exec(ctx, string(migration))
	require.NoError(t, err)
	return &Store{Pool: pool}, ctx
}

func TestSessionOrchestratorWaitsForEveryOwnerReceiptBeforeAtomicActiveOutbox(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	owners := newSessionOwnerScript()
	principal := SessionPrincipal{
		ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"},
	}
	orchestrator := NewSessionOrchestrator(store, owners.adapters())
	request := CreateSessionInput{
		OperationID: uuid.New(), Kind: "match", ExternalKey: "match-red-1", DisplayName: "Match one",
		RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{uuid.New()},
	}
	operation, err := orchestrator.CreateSession(ctx, principal, request)
	require.NoError(t, err)
	require.Equal(t, "pending", operation.Status)
	require.Equal(t, "provisioning", operation.SessionStatus)
	require.Equal(t, "accepted", operation.Stage)
	require.Empty(t, operation.ActiveEventID)

	replay, err := orchestrator.CreateSession(ctx, principal, request)
	require.NoError(t, err)
	require.Equal(t, operation.OperationID, replay.OperationID)
	require.Equal(t, operation.SessionID, replay.SessionID)
	changed := request
	changed.DisplayName = "Changed match"
	_, err = orchestrator.CreateSession(ctx, principal, changed)
	require.ErrorIs(t, err, ErrIdempotencyConflict)
	require.Empty(t, owners.calls, "same-ID body conflict must happen before any owner effect")

	for _, step := range []struct {
		owner string
		stage string
	}{
		{owner: "chat_create", stage: "accepted"},
		{owner: "chat_roster", stage: "chat_ready"},
		{owner: "voice_provision", stage: "roster_ready"},
		{owner: "role_apply", stage: "voice_ready"},
	} {
		owners.blocked[step.owner] = true
		_, err = orchestrator.AdvanceOne(ctx, operation.OperationID)
		require.Error(t, err, "an owner without its durable receipt cannot advance %s", step.stage)
		pending, getErr := orchestrator.GetOperation(ctx, principal, operation.OperationID)
		require.NoError(t, getErr)
		require.Equal(t, "pending", pending.Status)
		require.Equal(t, "provisioning", pending.SessionStatus)
		require.Equal(t, step.stage, pending.Stage)
		require.Empty(t, pending.ActiveEventID)
		outbox, outboxErr := orchestrator.GetActiveOutboxEvent(ctx, operation.SessionID)
		require.NoError(t, outboxErr)
		require.Nil(t, outbox, "active outbox event must not exist before every owner receipt is durable")

		owners.blocked[step.owner] = false
		if step.owner == "voice_provision" {
			owners.loseNextReply[step.owner] = true
			_, lostReplyErr := orchestrator.AdvanceOne(ctx, operation.OperationID)
			require.Error(t, lostReplyErr, "owner effect commits before its first reply is lost")
			stillPending, getErr := orchestrator.GetOperation(ctx, principal, operation.OperationID)
			require.NoError(t, getErr)
			require.Equal(t, step.stage, stillPending.Stage)
			require.Empty(t, stillPending.ActiveEventID)
			outbox, outboxErr := orchestrator.GetActiveOutboxEvent(ctx, operation.SessionID)
			require.NoError(t, outboxErr)
			require.Nil(t, outbox)
		}
		advanced, advanceErr := orchestrator.AdvanceOne(ctx, operation.OperationID)
		require.NoError(t, advanceErr)
		require.Equal(t, stageAfterOwnerReceipt(step.owner), advanced.Stage)
		requests := owners.calls[step.owner]
		wantCalls := 2
		if step.owner == "voice_provision" {
			wantCalls = 3 // pre-effect unavailable, committed/lost reply, receipt reconciliation
		}
		require.Len(t, requests, wantCalls, "the transient attempt and retry must reach one owner")
		for _, request := range requests[1:] {
			require.Equal(t, requests[0].OperationID, request.OperationID)
			require.Equal(t, requests[0].RequestHash, request.RequestHash)
		}
		if step.owner == "voice_provision" {
			key := sessionOwnerEffectKey{operationID: requests[1].OperationID, requestHash: requests[1].RequestHash}
			require.Equal(t, 1, owners.sideEffects[key], "lost reply retry must reconcile the existing receipt without repeating the effect")
			require.NotNil(t, advanced.VoiceProvisionReceiptID)
			require.Equal(t, owners.receipts[key].ReceiptID, *advanced.VoiceProvisionReceiptID)
		}
	}

	ready, err := orchestrator.GetOperation(ctx, principal, operation.OperationID)
	require.NoError(t, err)
	require.Equal(t, "pending", ready.Status)
	require.Equal(t, "grants_ready", ready.Stage)
	require.Empty(t, ready.ActiveEventID)

	active, err := orchestrator.AdvanceOne(ctx, operation.OperationID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", active.Status)
	require.Equal(t, "active", active.SessionStatus)
	require.Equal(t, "active", active.Stage)
	require.NotEmpty(t, active.ActiveEventID)
	outbox, err := orchestrator.GetActiveOutboxEvent(ctx, operation.SessionID)
	require.NoError(t, err)
	require.NotNil(t, outbox)
	require.Equal(t, active.ActiveEventID, outbox.EventID,
		"the operation's active state and its outbox event must be committed together")

	// A fresh service over the same Postgres pool models process restart and
	// proves both sides of the active/outbox commit are visible from durable state.
	restarted := NewSessionOrchestrator(&Store{Pool: store.Pool}, owners.adapters())
	statusAfterRestart, err := restarted.GetOperation(ctx, principal, operation.OperationID)
	require.NoError(t, err)
	require.Equal(t, active.ActiveEventID, statusAfterRestart.ActiveEventID)
	outboxAfterRestart, err := restarted.GetActiveOutboxEvent(ctx, operation.SessionID)
	require.NoError(t, err)
	require.NotNil(t, outboxAfterRestart)
	require.Equal(t, active.ActiveEventID, outboxAfterRestart.EventID)
}

func stageAfterOwnerReceipt(owner string) string {
	switch owner {
	case "chat_create":
		return "chat_ready"
	case "chat_roster":
		return "roster_ready"
	case "voice_provision":
		return "voice_ready"
	case "role_apply":
		return "grants_ready"
	default:
		return ""
	}
}

func TestParentedChildKeepsPartyChatAndReceiptsAfterClose(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	owners := newSessionOwnerScript()
	principal := SessionPrincipal{
		ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"},
	}
	orchestrator := NewSessionOrchestrator(store, owners.adapters())
	partyMembers := []uuid.UUID{uuid.New()}

	party, err := orchestrator.CreateSession(ctx, principal, CreateSessionInput{
		OperationID: uuid.New(), Kind: "party", ExternalKey: "party-shared", DisplayName: "Raid",
		RosterRevision: 1, RosterComplete: true, Members: partyMembers,
	})
	require.NoError(t, err)
	advanceSessionUntil(t, ctx, orchestrator, principal, party.OperationID, "active")
	party, err = orchestrator.GetOperation(ctx, principal, party.OperationID)
	require.NoError(t, err)
	require.NotEmpty(t, party.ChatID)
	require.NotEmpty(t, party.ChatOwnerSessionID)
	require.NotEmpty(t, party.ChatCreateReceiptID)
	require.NotEmpty(t, party.ChatRosterReceiptID)

	child, err := orchestrator.CreateSession(ctx, principal, CreateSessionInput{
		OperationID: uuid.New(), Kind: "match", ExternalKey: "match-child", ParentPartyKey: "party-shared",
		RosterRevision: 1, RosterComplete: true, Members: partyMembers,
	})
	require.NoError(t, err)
	advanceSessionUntil(t, ctx, orchestrator, principal, child.OperationID, "active")
	child, err = orchestrator.GetOperation(ctx, principal, child.OperationID)
	require.NoError(t, err)
	require.Equal(t, party.ChatID, child.ChatID)
	require.NotNil(t, child.ChatOwnerSessionID)
	require.Equal(t, party.SessionID, *child.ChatOwnerSessionID)
	require.Equal(t, party.ChatCreateReceiptID, child.ChatCreateReceiptID)
	require.Equal(t, party.ChatRosterReceiptID, child.ChatRosterReceiptID)
	require.NotEqual(t, party.SessionID, child.SessionID)
	require.Equal(t, 1, len(owners.calls["chat_create"]), "a child must not provision another Chat")
	require.Equal(t, 1, len(owners.calls["chat_roster"]), "a child roster must never be synchronized into the party Chat")

	closeOperation, err := orchestrator.CloseSession(ctx, principal, child.SessionID, uuid.New())
	require.NoError(t, err)
	advanceSessionUntil(t, ctx, orchestrator, principal, closeOperation.OperationID, "closed")
	closedChild, err := orchestrator.GetOperation(ctx, principal, child.OperationID)
	require.NoError(t, err)
	require.Equal(t, "closed", closedChild.SessionStatus)
	require.Equal(t, party.ChatID, closedChild.ChatID)
	require.NotNil(t, closedChild.ChatOwnerSessionID)
	require.Equal(t, party.SessionID, *closedChild.ChatOwnerSessionID)
	require.Equal(t, party.ChatCreateReceiptID, closedChild.ChatCreateReceiptID)
	require.Equal(t, party.ChatRosterReceiptID, closedChild.ChatRosterReceiptID)
	partyAfterClose, err := orchestrator.GetOperation(ctx, principal, party.OperationID)
	require.NoError(t, err)
	require.Equal(t, "active", partyAfterClose.SessionStatus)
	require.Equal(t, party.ChatID, partyAfterClose.ChatID)
}

func TestSessionOrchestratorPersistsChatMappingOnlyAfterUncertainCreateRetry(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	app, env := createBindingTestEnvironment(t, ctx, store)
	owners := newSessionOwnerScript()
	owners.loseNextReply["chat_create"] = true
	principalContext := SessionPrincipal{ApplicationID: app, EnvironmentID: env, Scopes: []string{"game.sessions.manage"}}
	orchestrator := NewSessionOrchestrator(store, owners.adapters())
	request := CreateSessionInput{
		OperationID: uuid.New(), Kind: "match", ExternalKey: "t30-chat-retry", DisplayName: "T30 retry",
		RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{uuid.New()},
	}
	accepted, err := orchestrator.CreateSession(ctx, principalContext, request)
	require.NoError(t, err)

	_, err = orchestrator.AdvanceOne(ctx, accepted.OperationID)
	require.Error(t, err, "Chat committed once but its response was lost")
	require.Len(t, owners.calls["chat_create"], 1)
	firstRequest := owners.calls["chat_create"][0]
	ownerKey := sessionOwnerEffectKey{operationID: firstRequest.OperationID, requestHash: firstRequest.RequestHash}
	require.Equal(t, 1, owners.sideEffects[ownerKey], "the lost response follows one durable Chat effect")
	firstReceipt := owners.receipts[ownerKey]
	require.NotZero(t, firstReceipt.ResourceID)
	require.NotZero(t, firstReceipt.ReceiptID)
	require.Equal(t, firstRequest.OperationID, firstReceipt.ReceiptID, "Chat receipt ID is its durable operation UUID")

	pending, err := orchestrator.GetOperation(ctx, principalContext, accepted.OperationID)
	require.NoError(t, err)
	require.Equal(t, "pending", pending.Status)
	require.Equal(t, "provisioning", pending.SessionStatus)
	require.Equal(t, "accepted", pending.Stage)
	require.Empty(t, pending.ChatID)
	require.Empty(t, pending.ChatCreateReceiptID)
	require.Empty(t, pending.ActiveEventID)
	var mappings, mappingOperations, ownerReceipts, outboxEvents int
	mappingOperationID := deterministicOwnerID(app, env, accepted.SessionID, "chat_mapping", "")
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM game_resource_mappings WHERE application_id=$1 AND environment_id=$2 AND external_key=$3`, app, env, request.ExternalKey).Scan(&mappings))
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM game_resource_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, app, env, mappingOperationID).Scan(&mappingOperations))
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_owner_receipts WHERE operation_id=$1 AND stage='chat_create'`, accepted.OperationID).Scan(&ownerReceipts))
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_outbox WHERE session_id=$1`, accepted.SessionID).Scan(&outboxEvents))
	require.Zero(t, mappings)
	require.Zero(t, mappingOperations)
	require.Zero(t, ownerReceipts, "GIS must not persist an unobserved Chat receipt before reconciling the owner reply")
	require.Zero(t, outboxEvents)

	advanced, err := orchestrator.AdvanceOne(ctx, accepted.OperationID)
	require.NoError(t, err)
	require.Equal(t, "chat_ready", advanced.Stage)
	require.Len(t, owners.calls["chat_create"], 2)
	retryRequest := owners.calls["chat_create"][1]
	require.Equal(t, firstRequest.OperationID, retryRequest.OperationID, "retry uses the deterministic Chat operation ID")
	require.Equal(t, firstRequest.RequestHash, retryRequest.RequestHash, "retry preserves the full protobuf request hash")
	firstBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(firstRequest.Proto)
	require.NoError(t, err)
	retryBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(retryRequest.Proto)
	require.NoError(t, err)
	require.Equal(t, firstBytes, retryBytes, "uncertain Chat RPC retry must use byte-identical full request bytes")
	computedHash, err := principal.RequestHash(firstRequest.Proto)
	require.NoError(t, err)
	require.Equal(t, firstRequest.RequestHash, computedHash)
	require.Equal(t, 1, owners.sideEffects[ownerKey], "retry reconciles the durable receipt without repeating Chat creation")

	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM game_resource_mappings WHERE application_id=$1 AND environment_id=$2 AND external_key=$3`, app, env, request.ExternalKey).Scan(&mappings))
	require.Equal(t, 1, mappings)
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM game_resource_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, app, env, mappingOperationID).Scan(&mappingOperations))
	require.Equal(t, 1, mappingOperations)
	requestHashBytes, err := principalHashBytes(firstRequest.RequestHash)
	require.NoError(t, err)
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_owner_receipts WHERE operation_id=$1 AND stage='chat_create' AND owner_operation_id=$2 AND owner_request_hash=$3 AND resource_id=$4 AND receipt_id=$5`, accepted.OperationID, firstRequest.OperationID, requestHashBytes, firstReceipt.ResourceID, firstReceipt.ReceiptID).Scan(&ownerReceipts))
	require.Equal(t, 1, ownerReceipts)
	var mappedResource, mappedChat, mappedChatOperation uuid.UUID
	var mappedHash string
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT resource_id,chat_id,chat_operation_id,chat_request_hash FROM game_resource_mappings WHERE application_id=$1 AND environment_id=$2 AND external_key=$3`, app, env, request.ExternalKey).Scan(&mappedResource, &mappedChat, &mappedChatOperation, &mappedHash))
	require.Equal(t, firstReceipt.ResourceID, mappedResource)
	require.Equal(t, firstReceipt.ResourceID, mappedChat)
	require.Equal(t, firstRequest.OperationID, mappedChatOperation)
	require.Equal(t, firstRequest.RequestHash, mappedHash)
	require.Equal(t, firstReceipt.ResourceID, advanced.ChatID)
	require.Equal(t, firstReceipt.ReceiptID, advanced.ChatCreateReceiptID)
	require.Empty(t, advanced.ActiveEventID)
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_outbox WHERE session_id=$1`, accepted.SessionID).Scan(&outboxEvents))
	require.Zero(t, outboxEvents, "the active event remains fenced until every owner receipt is persisted")
}

func TestSessionOrchestratorRejectsUnprovenChatReceiptBeforeMapping(t *testing.T) {
	for _, tc := range []struct {
		name    string
		receipt func(SessionOwnerRequest) SessionOwnerReceipt
	}{
		{name: "missing receipt ID", receipt: func(in SessionOwnerRequest) SessionOwnerReceipt {
			return SessionOwnerReceipt{ResourceID: uuid.New(), RequestHash: in.RequestHash}
		}},
		{name: "mismatched receipt ID", receipt: func(in SessionOwnerRequest) SessionOwnerReceipt {
			return SessionOwnerReceipt{ResourceID: uuid.New(), ReceiptID: uuid.New(), RequestHash: in.RequestHash}
		}},
		{name: "empty request hash", receipt: func(_ SessionOwnerRequest) SessionOwnerReceipt {
			return SessionOwnerReceipt{ResourceID: uuid.New(), ReceiptID: uuid.New()}
		}},
		{name: "mismatched request hash", receipt: func(_ SessionOwnerRequest) SessionOwnerReceipt {
			return SessionOwnerReceipt{ResourceID: uuid.New(), ReceiptID: uuid.New(), RequestHash: "sha256:" + strings.Repeat("0", 64)}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, ctx := startT31SessionStore(t)
			app, env := createBindingTestEnvironment(t, ctx, store)
			owners := newSessionOwnerScript()
			adapters := owners.adapters()
			adapters.CreateChat = func(_ context.Context, in SessionOwnerRequest) (SessionOwnerReceipt, error) {
				owners.calls["chat_create"] = append(owners.calls["chat_create"], in)
				return tc.receipt(in), nil
			}
			principalContext := SessionPrincipal{ApplicationID: app, EnvironmentID: env, Scopes: []string{"game.sessions.manage"}}
			orchestrator := NewSessionOrchestrator(store, adapters)
			request := CreateSessionInput{OperationID: uuid.New(), Kind: "match", ExternalKey: "t30-invalid-chat-receipt", DisplayName: "Invalid receipt", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{uuid.New()}}
			accepted, err := orchestrator.CreateSession(ctx, principalContext, request)
			require.NoError(t, err)
			_, err = orchestrator.AdvanceOne(ctx, accepted.OperationID)
			require.Error(t, err)
			pending, err := orchestrator.GetOperation(ctx, principalContext, accepted.OperationID)
			require.NoError(t, err)
			require.Equal(t, "pending", pending.Status)
			require.Equal(t, "provisioning", pending.SessionStatus)
			require.Equal(t, "accepted", pending.Stage)
			require.Empty(t, pending.ChatID)
			require.Empty(t, pending.ChatCreateReceiptID)
			require.Empty(t, pending.ActiveEventID)
			var count int
			require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM game_resource_mappings WHERE application_id=$1 AND environment_id=$2 AND external_key=$3`, app, env, request.ExternalKey).Scan(&count))
			require.Zero(t, count)
			mappingOperationID := deterministicOwnerID(app, env, accepted.SessionID, "chat_mapping", "")
			require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM game_resource_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, app, env, mappingOperationID).Scan(&count))
			require.Zero(t, count)
			require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_owner_receipts WHERE operation_id=$1 AND stage='chat_create'`, accepted.OperationID).Scan(&count))
			require.Zero(t, count)
			require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_outbox WHERE session_id=$1`, accepted.SessionID).Scan(&count))
			require.Zero(t, count)
		})
	}
}

func advanceSessionUntil(t *testing.T, ctx context.Context, orchestrator *SessionOrchestrator, principal SessionPrincipal, operationID uuid.UUID, sessionStatus string) {
	t.Helper()
	for attempt := 0; attempt < 12; attempt++ {
		operation, err := orchestrator.GetOperation(ctx, principal, operationID)
		require.NoError(t, err)
		if operation.SessionStatus == sessionStatus {
			return
		}
		_, err = orchestrator.AdvanceOne(ctx, operationID)
		require.NoError(t, err, "advance from durable stage %s", operation.Stage)
	}
	t.Fatalf("operation %s did not reach session_status %q", operationID, sessionStatus)
}
