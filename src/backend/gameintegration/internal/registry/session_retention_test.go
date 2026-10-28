package registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func startT34RetentionStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	store, ctx := startT32SessionStore(t)
	for _, name := range []string{"000019_t34_session_consent.up.sql", "000020_t34_keep_group_operations.up.sql"} {
		body, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "game_integration_db", name))
		require.NoError(t, err)
		_, err = store.Pool.Exec(ctx, string(body))
		require.NoError(t, err)
	}
	return store, ctx
}

func TestSessionRetentionPurgesAtExclusiveBoundaryAndLeavesKeyTombstone(t *testing.T) {
	store, ctx := startT34RetentionStore(t)
	app, env := createBindingTestEnvironment(t, ctx, store)
	p := SessionPrincipal{ApplicationID: app, EnvironmentID: env, Scopes: []string{"game.sessions.manage"}}
	owners := newSessionOwnerScript()
	orchestrator := NewSessionOrchestrator(store, owners.adapters())
	request := CreateSessionInput{OperationID: uuid.New(), Kind: "match", ExternalKey: "retained-match-key",
		DisplayName: "retained content", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{uuid.New()}}
	created, err := orchestrator.CreateSession(ctx, p, request)
	require.NoError(t, err)
	require.NoError(t, advanceSessionToActive(t, ctx, orchestrator, created.OperationID))
	closed, err := orchestrator.CloseSession(ctx, p, created.SessionID, uuid.New())
	require.NoError(t, err)
	require.NoError(t, advanceSessionToTerminal(t, ctx, orchestrator, closed.OperationID, "closed"))

	var terminalAt string
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT terminalized_at::text FROM gis_sessions WHERE id=$1`, created.SessionID).Scan(&terminalAt))
	require.NotEmpty(t, terminalAt)
	_, err = store.Pool.Exec(ctx, `UPDATE gis_sessions SET terminalized_at=clock_timestamp()-interval '30 days'+interval '1 second',history_expires_at=clock_timestamp()+interval '1 second' WHERE id=$1`, created.SessionID)
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, `UPDATE gis_session_operations SET terminalized_at=clock_timestamp()-interval '30 days'+interval '1 second' WHERE session_id=$1`, created.SessionID)
	require.NoError(t, err)
	n, err := store.PurgeExpiredSessionData(ctx, 10)
	require.NoError(t, err)
	require.Zero(t, n, "purge must use DB time and preserve data just before the exclusive deadline")
	replayed, err := orchestrator.CloseSession(ctx, p, created.SessionID, closed.OperationID)
	require.NoError(t, err)
	require.Equal(t, closed.OperationID, replayed.OperationID, "terminal receipt remains retryable before its own cutoff")
	_, err = store.Pool.Exec(ctx, `UPDATE gis_sessions SET terminalized_at=clock_timestamp()-interval '30 days',history_expires_at=clock_timestamp()-interval '30 days' WHERE id=$1`, created.SessionID)
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, `UPDATE gis_session_operations SET terminalized_at=clock_timestamp()-interval '30 days' WHERE session_id=$1`, created.SessionID)
	require.NoError(t, err)
	n, err = store.PurgeExpiredSessionData(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, n, "equality with terminal_at + 30 days is expired")

	var tombstoneCount, sessionCount, operationCount, receiptCount, mappingOperationCount int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_external_key_tombstones WHERE application_id=$1 AND environment_id=$2 AND kind='match' AND external_key=$3`, app, env, request.ExternalKey).Scan(&tombstoneCount))
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_sessions WHERE id=$1`, created.SessionID).Scan(&sessionCount))
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_operations WHERE session_id=$1`, created.SessionID).Scan(&operationCount))
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_owner_receipts WHERE operation_id=$1`, created.OperationID).Scan(&receiptCount))
	mappingOperationID := deterministicOwnerID(app, env, created.SessionID, "chat_mapping", "")
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM game_resource_operations WHERE operation_id=$1`, mappingOperationID).Scan(&mappingOperationCount))
	require.Equal(t, 1, tombstoneCount)
	require.Zero(t, sessionCount)
	require.Zero(t, operationCount)
	require.Zero(t, receiptCount)
	require.Zero(t, mappingOperationCount)
	var mappingStatus string
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT status FROM game_resource_mappings WHERE application_id=$1 AND environment_id=$2 AND external_key=$3`, app, env, request.ExternalKey).Scan(&mappingStatus))
	require.Equal(t, "tombstoned", mappingStatus)
	newMapping := resourceMappingInput(app, env, "chat", request.ExternalKey, uuid.New())
	newMapping.ChatID = newMapping.ResourceID
	_, err = store.CreateResourceMapping(ctx, newMapping)
	require.ErrorIs(t, err, ErrSessionKeyTombstoned, "the generic resource mapping route also observes session tombstones")
	_, err = orchestrator.CreateSession(ctx, p, request)
	require.ErrorIs(t, err, ErrSessionKeyTombstoned)
	var tombstoneColumns int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_name='gis_session_external_key_tombstones' AND column_name IN ('application_id','environment_id','kind','external_key','tombstoned_at')`).Scan(&tombstoneColumns))
	require.Equal(t, 5, tombstoneColumns, "tombstone stores scope, key and time only")
}

func TestSessionRetentionPurgesOperationReceiptOnItsOwnClock(t *testing.T) {
	store, ctx := startT34RetentionStore(t)
	app, env := createBindingTestEnvironment(t, ctx, store)
	p := SessionPrincipal{ApplicationID: app, EnvironmentID: env, Scopes: []string{"game.sessions.manage"}}
	orchestrator := NewSessionOrchestrator(store, newSessionOwnerScript().adapters())
	created, err := orchestrator.CreateSession(ctx, p, CreateSessionInput{OperationID: uuid.New(), Kind: "match", ExternalKey: "live-match", DisplayName: "Live", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{uuid.New()}})
	require.NoError(t, err)
	require.NoError(t, advanceSessionToActive(t, ctx, orchestrator, created.OperationID))
	_, err = store.Pool.Exec(ctx, `UPDATE gis_session_operations SET terminalized_at=clock_timestamp()-interval '30 days' WHERE operation_id=$1`, created.OperationID)
	require.NoError(t, err)
	n, err := store.PurgeExpiredSessionData(ctx, 10)
	require.NoError(t, err)
	require.Zero(t, n, "active session content is outside session retention cleanup")
	var sessionCount, operationCount int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_sessions WHERE id=$1 AND session_status='active'`, created.SessionID).Scan(&sessionCount))
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_operations WHERE operation_id=$1`, created.OperationID).Scan(&operationCount))
	require.Equal(t, 1, sessionCount)
	require.Zero(t, operationCount, "the create receipt expires independently of the still-active match")
}

func TestSessionRetentionDoesNotSweepSharedParty(t *testing.T) {
	store, ctx := startT34RetentionStore(t)
	app, env := createBindingTestEnvironment(t, ctx, store)
	owners := newSessionOwnerScript()
	orchestrator := NewSessionOrchestrator(store, owners.adapters())
	p := SessionPrincipal{ApplicationID: app, EnvironmentID: env, Scopes: []string{"game.sessions.manage"}}
	party, err := orchestrator.CreateSession(ctx, p, CreateSessionInput{OperationID: uuid.New(), Kind: "party", ExternalKey: "shared-party", DisplayName: "Party", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{uuid.New()}})
	require.NoError(t, err)
	require.NoError(t, advanceSessionToActive(t, ctx, orchestrator, party.OperationID))
	match, err := orchestrator.CreateSession(ctx, p, CreateSessionInput{OperationID: uuid.New(), Kind: "match", ExternalKey: "child-match", ParentPartyKey: "shared-party", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{}})
	require.NoError(t, err)
	require.NoError(t, advanceSessionToActive(t, ctx, orchestrator, match.OperationID))
	closed, err := orchestrator.CloseSession(ctx, p, match.SessionID, uuid.New())
	require.NoError(t, err)
	require.NoError(t, advanceSessionToTerminal(t, ctx, orchestrator, closed.OperationID, "closed"))
	_, err = store.Pool.Exec(ctx, `UPDATE gis_sessions SET terminalized_at=clock_timestamp()-interval '31 days',history_expires_at=clock_timestamp()-interval '31 days' WHERE id=$1`, match.SessionID)
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, `UPDATE gis_session_operations SET terminalized_at=clock_timestamp()-interval '31 days' WHERE session_id=$1`, match.SessionID)
	require.NoError(t, err)
	n, err := store.PurgeExpiredSessionData(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	var partyStatus string
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT session_status FROM gis_sessions WHERE id=$1`, party.SessionID).Scan(&partyStatus))
	require.Equal(t, "active", partyStatus)
	var partyMappingCount int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM game_resource_mappings WHERE external_key='shared-party' AND status='active'`).Scan(&partyMappingCount))
	require.Equal(t, 1, partyMappingCount)
}

func TestSessionTerminalCloseFailureRaceCoalescesOnOneWinner(t *testing.T) {
	store, ctx := startT34RetentionStore(t)
	app, env := createBindingTestEnvironment(t, ctx, store)
	p := SessionPrincipal{ApplicationID: app, EnvironmentID: env, Scopes: []string{"game.sessions.manage"}}
	owners := newSessionOwnerScript()
	orchestrator := NewSessionOrchestrator(store, owners.adapters())
	created, err := orchestrator.CreateSession(ctx, p, CreateSessionInput{OperationID: uuid.New(), Kind: "match", ExternalKey: "race-match", DisplayName: "Race", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{uuid.New()}})
	require.NoError(t, err)
	require.NoError(t, advanceSessionToActive(t, ctx, orchestrator, created.OperationID))
	closeID, failID := uuid.New(), uuid.New()
	closeResult := make(chan error, 1)
	failResult := make(chan error, 1)
	go func() { _, e := orchestrator.CloseSession(ctx, p, created.SessionID, closeID); closeResult <- e }()
	go func() {
		_, e := orchestrator.FailSession(ctx, p, created.SessionID, failID, "owner_rejected")
		failResult <- e
	}()
	require.NoError(t, <-closeResult)
	require.NoError(t, <-failResult)
	var winner uuid.UUID
	var kind string
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT terminalization_operation_id,terminalization_kind FROM gis_sessions WHERE id=$1`, created.SessionID).Scan(&winner, &kind))
	require.Contains(t, []uuid.UUID{closeID, failID}, winner)
	result, err := advanceSessionToTerminalResult(t, ctx, orchestrator, winner)
	require.NoError(t, err)
	wantStatus := "closed"
	if kind == "failure" {
		wantStatus = "failed"
	}
	require.Equal(t, wantStatus, result.SessionStatus)
	loser := closeID
	if winner == closeID {
		loser = failID
	}
	_, err = orchestrator.AdvanceScoped(ctx, SessionOperationKey{ApplicationID: app, EnvironmentID: env, OperationID: loser})
	require.NoError(t, err)
	var terminalOperations int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_operations WHERE session_id=$1 AND status IN ('succeeded','failed') AND terminalized_at IS NOT NULL`, created.SessionID).Scan(&terminalOperations))
	require.Equal(t, 3, terminalOperations, "create and both explicit terminal receipts have independent terminal timestamps")
}

func TestSessionOperationTerminalTimestampIsImmutableAfterFirstTransition(t *testing.T) {
	store, ctx := startT34RetentionStore(t)
	app, env := createBindingTestEnvironment(t, ctx, store)
	p := SessionPrincipal{ApplicationID: app, EnvironmentID: env, Scopes: []string{"game.sessions.manage"}}
	orchestrator := NewSessionOrchestrator(store, newSessionOwnerScript().adapters())
	created, err := orchestrator.CreateSession(ctx, p, CreateSessionInput{OperationID: uuid.New(), Kind: "match", ExternalKey: "terminal-clock", DisplayName: "Clock", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{uuid.New()}})
	require.NoError(t, err)
	require.NoError(t, advanceSessionToActive(t, ctx, orchestrator, created.OperationID))
	var original time.Time
	var status string
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT status,terminalized_at FROM gis_session_operations WHERE operation_id=$1`, created.OperationID).Scan(&status, &original))
	require.Equal(t, "succeeded", status)
	require.False(t, original.IsZero())
	_, err = store.Pool.Exec(ctx, `UPDATE gis_session_operations SET terminalized_at=clock_timestamp()-interval '1 day' WHERE operation_id=$1`, created.OperationID)
	require.NoError(t, err)
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT terminalized_at FROM gis_session_operations WHERE operation_id=$1`, created.OperationID).Scan(&original))
	_, err = store.Pool.Exec(ctx, `UPDATE gis_session_operations SET status='failed' WHERE operation_id=$1`, created.OperationID)
	require.NoError(t, err)
	var after time.Time
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT terminalized_at FROM gis_session_operations WHERE operation_id=$1`, created.OperationID).Scan(&after))
	require.Equal(t, original, after, "terminal-to-terminal transitions must preserve the first terminal timestamp")
}

func advanceSessionToActive(t *testing.T, ctx context.Context, orchestrator *SessionOrchestrator, operationID uuid.UUID) error {
	t.Helper()
	for i := 0; i < 10; i++ {
		result, err := orchestrator.AdvanceOne(ctx, operationID)
		if err != nil {
			return err
		}
		if result.SessionStatus == "active" {
			return nil
		}
	}
	return context.DeadlineExceeded
}

func advanceSessionToTerminal(t *testing.T, ctx context.Context, orchestrator *SessionOrchestrator, operationID uuid.UUID, terminal string) error {
	t.Helper()
	for i := 0; i < 20; i++ {
		result, err := orchestrator.AdvanceOne(ctx, operationID)
		if err != nil {
			return err
		}
		if result.SessionStatus == terminal {
			return nil
		}
	}
	return context.DeadlineExceeded
}

func advanceSessionToTerminalResult(t *testing.T, ctx context.Context, orchestrator *SessionOrchestrator, operationID uuid.UUID) (SessionOperation, error) {
	t.Helper()
	for i := 0; i < 20; i++ {
		result, err := orchestrator.AdvanceOne(ctx, operationID)
		if err != nil {
			return SessionOperation{}, err
		}
		if result.SessionStatus == "closed" || result.SessionStatus == "failed" {
			return result, nil
		}
	}
	return SessionOperation{}, context.DeadlineExceeded
}
