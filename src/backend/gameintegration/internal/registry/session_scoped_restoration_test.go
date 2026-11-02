package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func advanceScopedSessionTo(t *testing.T, ctx context.Context, orchestrator *SessionOrchestrator, p SessionPrincipal, operationID uuid.UUID, status string) SessionOperation {
	t.Helper()
	for range 12 {
		operation, err := orchestrator.AdvanceScoped(ctx, SessionOperationKey{ApplicationID: p.ApplicationID, EnvironmentID: p.EnvironmentID, OperationID: operationID})
		if errors.Is(err, ErrSessionLeaseLost) {
			continue
		}
		require.NoError(t, err)
		if operation.SessionStatus == status {
			return operation
		}
	}
	t.Fatalf("scoped operation did not reach %s", status)
	return SessionOperation{}
}

func TestAlreadyTerminalSessionOperationsPersistRetentionClocks(t *testing.T) {
	store, ctx := startT34RetentionStore(t)
	p := newT31TestSessionPrincipal(t, ctx, store)
	orchestrator := NewSessionOrchestrator(store, newSessionOwnerScript().adapters())
	created, err := orchestrator.CreateSession(ctx, p, CreateSessionInput{OperationID: uuid.New(), Kind: "party", ExternalKey: "terminal-roster-clocks", DisplayName: "Clocks", RosterRevision: 2, RosterComplete: true, Members: []uuid.UUID{}})
	require.NoError(t, err)
	advanceScopedSessionTo(t, ctx, orchestrator, p, created.OperationID, "active")
	for _, revision := range []int64{1, 2} {
		t.Run(string(rune('0'+revision)), func(t *testing.T) {
			operation, err := orchestrator.UpdateSessionRoster(ctx, p, created.SessionID, SessionRosterUpdateInput{OperationID: uuid.New(), RosterRevision: revision, RosterComplete: true, Members: []uuid.UUID{}})
			require.NoError(t, err)
			require.Equal(t, "succeeded", operation.Status)
			var timed bool
			require.NoError(t, store.Pool.QueryRow(ctx, `SELECT terminalized_at IS NOT NULL FROM gis_session_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, p.ApplicationID, p.EnvironmentID, operation.OperationID).Scan(&timed))
			require.True(t, timed, "a terminal-on-insert roster receipt has a bounded retry lifetime")
		})
	}
	for _, failure := range []bool{false, true} {
		name := "close-before-voice"
		if failure {
			name = "fail-before-voice"
		}
		t.Run(name, func(t *testing.T) {
			pending, err := orchestrator.CreateSession(ctx, p, CreateSessionInput{OperationID: uuid.New(), Kind: "match", ExternalKey: name, DisplayName: "Clocks", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{}})
			require.NoError(t, err)
			operationID := uuid.New()
			if failure {
				_, err = orchestrator.FailSession(ctx, p, pending.SessionID, operationID, "test-rejection")
			} else {
				_, err = orchestrator.CloseSession(ctx, p, pending.SessionID, operationID)
			}
			require.NoError(t, err)
			var operationTimed, sessionTimed bool
			require.NoError(t, store.Pool.QueryRow(ctx, `SELECT terminalized_at IS NOT NULL FROM gis_session_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, p.ApplicationID, p.EnvironmentID, operationID).Scan(&operationTimed))
			require.True(t, operationTimed)
			require.NoError(t, store.Pool.QueryRow(ctx, `SELECT terminalized_at IS NOT NULL AND history_expires_at IS NOT NULL FROM gis_sessions WHERE id=$1`, pending.SessionID).Scan(&sessionTimed))
			require.True(t, sessionTimed)
		})
	}
}

func TestSessionRetentionPreservesLiveForeignOperationWithSameUUID(t *testing.T) {
	store, ctx := startT34RetentionStore(t)
	orchestrator := NewSessionOrchestrator(store, newSessionOwnerScript().adapters())
	first := newT31TestSessionPrincipal(t, ctx, store)
	second := newT31TestSessionPrincipal(t, ctx, store)
	operationID := uuid.New()
	var created []SessionOperation
	for _, p := range []SessionPrincipal{first, second} {
		operation, err := orchestrator.CreateSession(ctx, p, CreateSessionInput{OperationID: operationID, Kind: "match", ExternalKey: "scoped-retention", DisplayName: "Retention", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{}})
		require.NoError(t, err)
		created = append(created, operation)
		advanceScopedSessionTo(t, ctx, orchestrator, p, operationID, "active")
	}
	closeID := uuid.New()
	_, err := orchestrator.CloseSession(ctx, first, created[0].SessionID, closeID)
	require.NoError(t, err)
	advanceScopedSessionTo(t, ctx, orchestrator, first, closeID, "closed")
	_, err = store.Pool.Exec(ctx, `UPDATE gis_sessions SET terminalized_at=clock_timestamp()-interval '31 days',history_expires_at=clock_timestamp()-interval '1 day' WHERE id=$1`, created[0].SessionID)
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, `UPDATE gis_session_operations SET terminalized_at=clock_timestamp()-interval '31 days' WHERE application_id=$1 AND environment_id=$2`, first.ApplicationID, first.EnvironmentID)
	require.NoError(t, err)
	var before int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_owner_receipts WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, second.ApplicationID, second.EnvironmentID, operationID).Scan(&before))
	require.Positive(t, before)
	_, err = store.PurgeExpiredSessionData(ctx, 50)
	require.NoError(t, err)
	live, err := orchestrator.GetOperation(ctx, second, operationID)
	require.NoError(t, err, "a foreign operation UUID never identifies the selected expired receipt")
	require.Equal(t, "active", live.SessionStatus)
	require.Equal(t, created[1].SessionID, live.SessionID)
	var after int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_owner_receipts WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, second.ApplicationID, second.EnvironmentID, operationID).Scan(&after))
	require.Equal(t, before, after)
}

func TestSessionRetentionPurgesExpiredConsentAndKeepGroupReceiptsOnlyForChild(t *testing.T) {
	store, ctx, p, sessionID, accountID, profileID, parentChatID := setupT34KeepGroupSession(t)
	_, err := store.RecordSessionConsent(ctx, accountID, profileID, sessionID, uuid.New(), 7)
	require.NoError(t, err)
	owners := &keepGroupOwnerRecorder{chatID: uuid.New()}
	orchestrator := NewSessionOrchestrator(store, SessionOwnerAdapters{CreateChat: owners.CreateChat, SyncChatRoster: owners.SyncChatRoster})
	_, err = orchestrator.KeepSessionGroup(ctx, SessionKeepGroupInput{OperationID: uuid.New(), SessionID: sessionID, AccountID: accountID, ProfileID: profileID, RosterRevision: 7})
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, `UPDATE gis_sessions SET session_status='closed',stage='closed',terminalized_at=clock_timestamp()-interval '31 days',history_expires_at=clock_timestamp()-interval '1 day' WHERE id=$1`, sessionID)
	require.NoError(t, err)
	purged, err := store.PurgeExpiredSessionData(ctx, 50)
	require.NoError(t, err)
	require.Equal(t, 1, purged)
	for _, table := range []string{"gis_session_consent_receipts", "gis_session_keep_group_operations"} {
		var count int
		require.NoError(t, store.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE session_id=$1", sessionID).Scan(&count))
		require.Zero(t, count)
	}
	var childCount, parentCount int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_sessions WHERE id=$1`, sessionID).Scan(&childCount))
	require.Zero(t, childCount)
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_sessions WHERE application_id=$1 AND environment_id=$2 AND kind='party' AND chat_id=$3 AND session_status='active'`, p.ApplicationID, p.EnvironmentID, parentChatID).Scan(&parentCount))
	require.Equal(t, 1, parentCount)
	require.Equal(t, 1, owners.addCalls, "GIS receipt cleanup does not replay a persistent party membership effect")
}
