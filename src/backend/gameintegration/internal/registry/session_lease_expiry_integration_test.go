package registry

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSessionOrchestratorResourceCommitRejectsExpiredLeaseWithoutTakeover(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	p := newT31TestSessionPrincipal(t, ctx, store)
	orchestrator := NewSessionOrchestrator(store, SessionOwnerAdapters{})
	accepted, err := orchestrator.CreateSession(ctx, p, CreateSessionInput{
		OperationID: uuid.New(), Kind: "match", ExternalKey: "expired-resource-lease", DisplayName: "Expired resource lease",
		RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{},
	})
	require.NoError(t, err)
	key := SessionOperationKey{ApplicationID: p.ApplicationID, EnvironmentID: p.EnvironmentID, OperationID: accepted.OperationID}
	var before string
	var calls int
	orchestrator.Owners.CreateChat = func(_ context.Context, in SessionOwnerRequest) (SessionOwnerReceipt, error) {
		calls++
		_, before = expireT31LeaseWithoutTakeover(t, ctx, store, key)
		return SessionOwnerReceipt{ResourceID: uuid.New(), ReceiptID: in.OperationID, RequestHash: in.RequestHash}, nil
	}

	_, err = orchestrator.AdvanceScoped(ctx, key)
	require.ErrorIs(t, err, ErrSessionLeaseLost)
	require.Equal(t, 1, calls)
	require.NotEmpty(t, before)
	require.Equal(t, before, snapshotT31LeaseWrites(t, ctx, store, key),
		"expired Chat completion must not persist resource IDs, receipts, mappings, retries, or outbox writes")
}

func TestSessionOrchestratorNoOwnerStageRejectsExpiredLeaseWithoutTakeover(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	p := newT31TestSessionPrincipal(t, ctx, store)
	owners := newSessionOwnerScript()
	orchestrator := NewSessionOrchestrator(store, owners.adapters())
	party, err := orchestrator.CreateSession(ctx, p, CreateSessionInput{
		OperationID: uuid.New(), Kind: "party", ExternalKey: "expired-lease-party", DisplayName: "Lease party",
		RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{},
	})
	require.NoError(t, err)
	advanceSessionUntil(t, ctx, orchestrator, p, party.OperationID, "active")
	chatCalls := len(owners.calls["chat_create"]) + len(owners.calls["chat_roster"])
	child, err := orchestrator.CreateSession(ctx, p, CreateSessionInput{
		OperationID: uuid.New(), Kind: "match", ExternalKey: "expired-lease-child", ParentPartyKey: "expired-lease-party",
		RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{},
	})
	require.NoError(t, err)
	key := SessionOperationKey{ApplicationID: p.ApplicationID, EnvironmentID: p.EnvironmentID, OperationID: child.OperationID}
	_, _, stage, owner, err := store.claimScoped(ctx, key)
	require.NoError(t, err)
	require.Equal(t, "accepted", stage)
	expiredOwner, before := expireT31LeaseWithoutTakeover(t, ctx, store, key)
	require.Equal(t, owner, expiredOwner)

	_, err = orchestrator.advanceNoOwner(ctx, key, child.SessionID, stage, "chat_ready", owner)
	require.ErrorIs(t, err, ErrSessionLeaseLost)
	require.Equal(t, before, snapshotT31LeaseWrites(t, ctx, store, key),
		"expired child worker must not advance either durable stage or mutate inherited Chat state")
	require.Equal(t, chatCalls, len(owners.calls["chat_create"])+len(owners.calls["chat_roster"]))
}

func TestSessionOrchestratorActiveCommitRejectsExpiredLeaseWithoutTakeover(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	p := newT31TestSessionPrincipal(t, ctx, store)
	owners := newSessionOwnerScript()
	orchestrator := NewSessionOrchestrator(store, owners.adapters())
	accepted, err := orchestrator.CreateSession(ctx, p, CreateSessionInput{
		OperationID: uuid.New(), Kind: "match", ExternalKey: "expired-active-lease-no-takeover", DisplayName: "Expired active lease",
		RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{},
	})
	require.NoError(t, err)
	var ready SessionOperation
	for attempt := 0; attempt < 5; attempt++ {
		ready, err = orchestrator.AdvanceOne(ctx, accepted.OperationID)
		require.NoError(t, err)
		if ready.Stage == "grants_ready" {
			break
		}
	}
	require.Equal(t, "grants_ready", ready.Stage)
	key := SessionOperationKey{ApplicationID: p.ApplicationID, EnvironmentID: p.EnvironmentID, OperationID: accepted.OperationID}
	_, _, stage, owner, err := store.claimScoped(ctx, key)
	require.NoError(t, err)
	require.Equal(t, "grants_ready", stage)
	expiredOwner, before := expireT31LeaseWithoutTakeover(t, ctx, store, key)
	require.Equal(t, owner, expiredOwner)

	_, err = orchestrator.finishActive(ctx, key, accepted.SessionID, stage, owner, p)
	require.ErrorIs(t, err, ErrSessionLeaseLost)
	require.Equal(t, before, snapshotT31LeaseWrites(t, ctx, store, key),
		"expired worker must not activate the session, complete its operation, or insert an active event")
	outbox, err := orchestrator.GetActiveOutboxEvent(ctx, accepted.SessionID)
	require.NoError(t, err)
	require.Nil(t, outbox)
}

func TestSessionOrchestratorTerminalCommitRejectsExpiredLeaseWithoutTakeover(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	p := newT31TestSessionPrincipal(t, ctx, store)
	owners := newSessionOwnerScript()
	orchestrator := NewSessionOrchestrator(store, owners.adapters())
	created, err := orchestrator.CreateSession(ctx, p, CreateSessionInput{
		OperationID: uuid.New(), Kind: "match", ExternalKey: "expired-terminal-lease", DisplayName: "Expired terminal lease",
		RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{},
	})
	require.NoError(t, err)
	advanceSessionUntil(t, ctx, orchestrator, p, created.OperationID, "active")
	active, err := orchestrator.GetOperation(ctx, p, created.OperationID)
	require.NoError(t, err)
	require.NotNil(t, active.VoiceRoomID)
	closing, err := orchestrator.CloseSession(ctx, p, active.SessionID, uuid.New())
	require.NoError(t, err)
	key := SessionOperationKey{ApplicationID: p.ApplicationID, EnvironmentID: p.EnvironmentID, OperationID: closing.OperationID}
	var before string
	var calls int
	orchestrator.Owners.CloseVoice = func(_ context.Context, in SessionOwnerRequest) (SessionOwnerReceipt, error) {
		calls++
		_, before = expireT31LeaseWithoutTakeover(t, ctx, store, key)
		return SessionOwnerReceipt{ResourceID: *active.VoiceRoomID, ReceiptID: uuid.New(), RequestHash: in.RequestHash}, nil
	}

	_, err = orchestrator.AdvanceScoped(ctx, key)
	require.ErrorIs(t, err, ErrSessionLeaseLost)
	require.Equal(t, 1, calls)
	require.NotEmpty(t, before)
	require.Equal(t, before, snapshotT31LeaseWrites(t, ctx, store, key),
		"expired terminal worker must not persist its receipt, advance terminalization, or change the existing event")
}

// Change only the deadline after a real claim. In particular, keep the owner,
// pending status, and current stage valid so only the expiry predicate can deny
// the subsequent commit. Snapshot after expiry excludes the fixture mutation.
func expireT31LeaseWithoutTakeover(t *testing.T, ctx context.Context, store *Store, key SessionOperationKey) (uuid.UUID, string) {
	t.Helper()
	var owner uuid.UUID
	var live bool
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT lease_owner,lease_until>clock_timestamp()
		FROM gis_session_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3 AND status='pending'`,
		key.ApplicationID, key.EnvironmentID, key.OperationID).Scan(&owner, &live))
	require.NotEqual(t, uuid.Nil, owner)
	require.True(t, live, "fixture must first hold a live claimed lease")
	var expiredOwner uuid.UUID
	var expired bool
	require.NoError(t, store.Pool.QueryRow(ctx, `UPDATE gis_session_operations SET lease_until=clock_timestamp()-interval '1 second'
		WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3 AND lease_owner=$4 AND status='pending'
		RETURNING lease_owner,lease_until<clock_timestamp()`,
		key.ApplicationID, key.EnvironmentID, key.OperationID, owner).Scan(&expiredOwner, &expired))
	require.Equal(t, owner, expiredOwner, "expiry must not replace the claimed owner")
	require.True(t, expired)
	return owner, snapshotT31LeaseWrites(t, ctx, store, key)
}

// Compare complete scoped rows, including all operation/session columns and
// existing event bytes. This catches partial commits and retry bookkeeping as
// well as extra receipts, mapping operations, bindings, and outbox events.
func snapshotT31LeaseWrites(t *testing.T, ctx context.Context, store *Store, key SessionOperationKey) string {
	t.Helper()
	var snapshot string
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT jsonb_build_object(
		'sessions', (SELECT jsonb_agg(to_jsonb(s) ORDER BY s.id) FROM gis_sessions s WHERE application_id=$1 AND environment_id=$2),
		'operations', (SELECT jsonb_agg(to_jsonb(o) ORDER BY o.operation_id) FROM gis_session_operations o WHERE application_id=$1 AND environment_id=$2),
		'receipts', (SELECT jsonb_agg(to_jsonb(r) ORDER BY r.operation_id,r.stage) FROM gis_session_owner_receipts r WHERE application_id=$1 AND environment_id=$2),
		'mappings', (SELECT jsonb_agg(to_jsonb(m) ORDER BY m.external_key) FROM game_resource_mappings m WHERE application_id=$1 AND environment_id=$2),
		'mapping_operations', (SELECT jsonb_agg(to_jsonb(m) ORDER BY m.operation_id) FROM game_resource_operations m WHERE application_id=$1 AND environment_id=$2),
		'binding_chats', (SELECT jsonb_agg(to_jsonb(b) ORDER BY b.binding_id,b.chat_id) FROM game_resource_binding_chats b WHERE application_id=$1 AND environment_id=$2),
		'outbox', (SELECT jsonb_agg(to_jsonb(e) ORDER BY e.event_id) FROM gis_session_outbox e WHERE application_id=$1 AND environment_id=$2)
	)::text`, key.ApplicationID, key.EnvironmentID).Scan(&snapshot))
	return snapshot
}
