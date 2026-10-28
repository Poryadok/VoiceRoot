package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func startT32SessionStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	return startT31SessionStore(t)
}

func TestT32AcceptedInitialRosterPersistsDigestAndLease(t *testing.T) {
	store, ctx := startT32SessionStore(t)
	principal := newT31TestSessionPrincipal(t, ctx, store)
	owners := newSessionOwnerScript()
	orchestrator := NewSessionOrchestrator(store, owners.adapters())
	members := []uuid.UUID{uuid.New(), uuid.New()}
	if members[0].String() > members[1].String() {
		members[0], members[1] = members[1], members[0]
	}
	host := members[0]
	input := CreateSessionInput{
		OperationID: uuid.New(), Kind: "party", ExternalKey: "t32-authority-party",
		DisplayName: "T32 authority", RosterRevision: 1, RosterComplete: true,
		Members: members, HostProfileID: &host,
	}
	created, err := orchestrator.CreateSession(ctx, principal, input)
	require.NoError(t, err)
	var leaseAtAcceptance time.Time
	err = store.Pool.QueryRow(ctx, `SELECT roster_hash,lease_expires_at FROM gis_sessions WHERE id=$1`, created.SessionID).
		Scan(new([]byte), &leaseAtAcceptance)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(60*time.Second), leaseAtAcceptance, 3*time.Second)
	advanceSessionUntil(t, ctx, orchestrator, principal, created.OperationID, "active")

	var revision int64
	var hash []byte
	var expiresAt time.Time
	err = store.Pool.QueryRow(ctx, `SELECT roster_revision,roster_hash,lease_expires_at FROM gis_sessions WHERE id=$1`, created.SessionID).
		Scan(&revision, &hash, &expiresAt)
	require.NoError(t, err)
	require.EqualValues(t, input.RosterRevision, revision)
	hostString := host.String()
	canonical, err := json.Marshal(struct {
		Revision int64    `json:"revision"`
		Members  []string `json:"members"`
		Host     *string  `json:"host_profile_id"`
	}{Revision: input.RosterRevision, Members: []string{members[0].String(), members[1].String()}, Host: &hostString})
	require.NoError(t, err)
	canonical, err = jsoncanonicalizer.Transform(canonical)
	require.NoError(t, err)
	wantHash := sha256.Sum256(canonical)
	require.Equal(t, wantHash[:], hash)
	require.Equal(t, leaseAtAcceptance, expiresAt, "owner receipt and later stages must not renew the accepted roster lease")
	replay, err := orchestrator.CreateSession(ctx, principal, input)
	require.NoError(t, err)
	require.Equal(t, created.OperationID, replay.OperationID)
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT lease_expires_at FROM gis_sessions WHERE id=$1`, created.SessionID).Scan(&expiresAt))
	require.Equal(t, leaseAtAcceptance, expiresAt, "exact create retry must not renew the roster lease")
}

func TestT32CreateSessionHostMustBelongToRoster(t *testing.T) {
	member := uuid.New()
	host := uuid.New()
	err := validateCreate(CreateSessionInput{
		OperationID: uuid.New(), Kind: "party", ExternalKey: "t32-host-outside-roster",
		DisplayName: "T32 host", RosterRevision: 1, RosterComplete: true,
		Members: []uuid.UUID{member}, HostProfileID: &host,
	})
	require.ErrorContains(t, err, "host must be present")
}

func TestT32ExpiredInitialRosterNeverPublishesActiveAndClosesResources(t *testing.T) {
	store, ctx := startT32SessionStore(t)
	principal := newT31TestSessionPrincipal(t, ctx, store)
	owners := newSessionOwnerScript()
	orchestrator := NewSessionOrchestrator(store, owners.adapters())
	input := CreateSessionInput{
		OperationID: uuid.New(), Kind: "match", ExternalKey: "t32-expired-before-active",
		DisplayName: "Expired roster", RosterRevision: 1, RosterComplete: true,
		Members: []uuid.UUID{uuid.New()},
	}
	created, err := orchestrator.CreateSession(ctx, principal, input)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		_, err = orchestrator.AdvanceOne(ctx, created.OperationID)
		require.NoError(t, err)
	}
	var stage string
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT stage FROM gis_session_operations WHERE operation_id=$1`, created.OperationID).Scan(&stage))
	require.Equal(t, "grants_ready", stage)
	_, err = store.Pool.Exec(ctx, `UPDATE gis_sessions SET lease_expires_at=clock_timestamp()-interval '1 millisecond' WHERE id=$1`, created.SessionID)
	require.NoError(t, err)
	_, err = orchestrator.AdvanceOne(ctx, created.OperationID)
	require.NoError(t, err)

	var status string
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT session_status FROM gis_sessions WHERE id=$1`, created.SessionID).Scan(&status))
	require.Equal(t, "closing", status, "expiry starts the durable close/revoke fence")
	var outboxRows int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_outbox WHERE session_id=$1 AND event_type='voice.game.sessions.active.v1'`, created.SessionID).Scan(&outboxRows))
	require.Zero(t, outboxRows, "expired roster must never publish active")

	failureID := deterministicOwnerID(principal.ApplicationID, principal.EnvironmentID, created.SessionID, "roster_lease_expiry", "1")
	advanceSessionUntil(t, ctx, orchestrator, principal, failureID, "failed")
	operation, err := orchestrator.GetOperation(ctx, principal, created.OperationID)
	require.NoError(t, err)
	require.Equal(t, "failed", operation.Status)
	require.Equal(t, 1, len(owners.calls["voice_close"]))
	require.Equal(t, 1, len(owners.calls["role_revoke"]))
}

func TestT32ExpiredActiveRosterIsSelectedForDurableClose(t *testing.T) {
	store, ctx := startT32SessionStore(t)
	principal := newT31TestSessionPrincipal(t, ctx, store)
	owners := newSessionOwnerScript()
	orchestrator := NewSessionOrchestrator(store, owners.adapters())
	created, err := orchestrator.CreateSession(ctx, principal, CreateSessionInput{
		OperationID: uuid.New(), Kind: "match", ExternalKey: "t32-expired-active",
		DisplayName: "Expired active roster", RosterRevision: 1, RosterComplete: true,
		Members: []uuid.UUID{uuid.New()},
	})
	require.NoError(t, err)
	advanceSessionUntil(t, ctx, orchestrator, principal, created.OperationID, "active")
	_, err = store.Pool.Exec(ctx, `UPDATE gis_sessions SET lease_expires_at=clock_timestamp()-interval '1 millisecond' WHERE id=$1`, created.SessionID)
	require.NoError(t, err)

	expired, err := store.expiredActiveSessions(ctx)
	require.NoError(t, err)
	require.Len(t, expired, 1)
	require.Equal(t, created.SessionID, expired[0].SessionID)
	closeID := deterministicOwnerID(principal.ApplicationID, principal.EnvironmentID, created.SessionID,
		"roster_lease_close", "1")
	closeOperation, err := orchestrator.CloseSession(ctx, principal, created.SessionID, closeID)
	require.NoError(t, err)
	require.Equal(t, "pending", closeOperation.Status)
	advanceSessionUntil(t, ctx, orchestrator, principal, closeID, "closed")
	closed, err := orchestrator.GetOperation(ctx, principal, created.OperationID)
	require.NoError(t, err)
	require.Equal(t, "closed", closed.SessionStatus)
	require.Len(t, owners.calls["voice_close"], 1)
	require.Len(t, owners.calls["role_revoke"], 1)
}
