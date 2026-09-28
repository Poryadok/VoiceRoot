package registry

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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
	receipt := SessionOwnerReceipt{ResourceID: uuid.New(), ReceiptID: uuid.New(), RequestHash: request.RequestHash}
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
