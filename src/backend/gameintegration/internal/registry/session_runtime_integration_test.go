package registry

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSessionCloseAndPermanentFailureHaveOneTerminalizationWinner(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	owners := newSessionOwnerScript()
	p := SessionPrincipal{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"}}
	o := NewSessionOrchestrator(store, owners.adapters())
	created, err := o.CreateSession(ctx, p, CreateSessionInput{OperationID: uuid.New(), Kind: "party", ExternalKey: "terminal-race", DisplayName: "Raid", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{}})
	require.NoError(t, err)
	advanceSessionUntil(t, ctx, o, p, created.OperationID, "active")
	closeID, failureID := uuid.New(), uuid.New()
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	results := make(chan SessionOperation, 2)
	errs := make(chan error, 2)
	go func() {
		defer wg.Done()
		<-start
		op, e := o.CloseSession(ctx, p, created.SessionID, closeID)
		results <- op
		errs <- e
	}()
	go func() {
		defer wg.Done()
		<-start
		op, e := o.FailSession(ctx, p, created.SessionID, failureID, "OWNER_REJECTED")
		results <- op
		errs <- e
	}()
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var operations []SessionOperation
	for op := range results {
		operations = append(operations, op)
	}
	require.Len(t, operations, 2)
	var winner uuid.UUID
	var terminalKind string
	var finalStatus string
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT terminalization_operation_id,terminalization_kind,session_status FROM gis_sessions WHERE id=$1`, created.SessionID).Scan(&winner, &terminalKind, &finalStatus))
	require.Contains(t, []uuid.UUID{closeID, failureID}, winner)
	terminalState := "closed"
	if terminalKind == "failure" {
		terminalState = "failed"
	}
	advanceSessionUntil(t, ctx, o, p, operations[0].OperationID, terminalState)
	advanceSessionUntil(t, ctx, o, p, operations[1].OperationID, terminalState)
	for _,op:=range operations{if op.OperationID!=winner{_,e:=o.AdvanceOne(ctx,op.OperationID);require.NoError(t,e,"the coalesced loser must durably resolve after the winner commits")}}
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT session_status FROM gis_sessions WHERE id=$1`, created.SessionID).Scan(&finalStatus))
	if winner == closeID {
		require.Equal(t, "close", terminalKind)
		require.Equal(t, "closed", finalStatus)
	} else {
		require.Equal(t, "failure", terminalKind)
		require.Equal(t, "failed", finalStatus)
	}
	require.Len(t, owners.calls["voice_close"], 1, "the losing terminalization must not invoke another Voice close ID")
	require.Len(t, owners.calls["role_revoke"], 1, "the losing terminalization must not invoke another Role revoke ID")
	resolved := make([]SessionOperation, 0, 2)
	for _, op := range operations {
		value, e := o.GetOperation(ctx, p, op.OperationID)
		require.NoError(t, e)
		require.Equal(t,"succeeded",value.Status)
		resolved = append(resolved, value)
	}
	require.NotNil(t, resolved[0].VoiceCloseReceiptID)
	require.NotNil(t, resolved[1].VoiceCloseReceiptID)
	require.Equal(t, *resolved[0].VoiceCloseReceiptID, *resolved[1].VoiceCloseReceiptID)
	require.NotNil(t, resolved[0].RoleRevokeReceiptID)
	require.NotNil(t, resolved[1].RoleRevokeReceiptID)
	require.Equal(t, *resolved[0].RoleRevokeReceiptID, *resolved[1].RoleRevokeReceiptID)
	first, second := owners.calls["voice_close"][0], owners.calls["role_revoke"][0]
	require.Equal(t, winner, firstSessionTerminalizer(ctx, store, created.SessionID))
	require.NotEqual(t, uuid.Nil, first.OperationID)
	require.NotEqual(t, uuid.Nil, second.OperationID)
}

func firstSessionTerminalizer(ctx context.Context, store *Store, session uuid.UUID) uuid.UUID {
	var id uuid.UUID
	_ = store.Pool.QueryRow(ctx, `SELECT terminalization_operation_id FROM gis_sessions WHERE id=$1`, session).Scan(&id)
	return id
}

func TestSessionWorkerRestartReclaimsExpiredStageLease(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	owners := newSessionOwnerScript()
	p := SessionPrincipal{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"}}
	o := NewSessionOrchestrator(store, owners.adapters())
	accepted, err := o.CreateSession(ctx, p, CreateSessionInput{OperationID: uuid.New(), Kind: "party", ExternalKey: "restart-lease", DisplayName: "Raid", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{}})
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, `UPDATE gis_session_operations SET lease_owner=$2,lease_until=now()-interval '1 second' WHERE operation_id=$1`, accepted.OperationID, uuid.New())
	require.NoError(t, err)
	restarted := NewSessionOrchestrator(&Store{Pool: store.Pool}, owners.adapters())
	advanced, err := restarted.AdvanceOne(ctx, accepted.OperationID)
	require.NoError(t, err)
	require.Equal(t, "chat_ready", advanced.Stage)
	require.Len(t, owners.calls["chat_create"], 1)
}
