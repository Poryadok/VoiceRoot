package registry

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func sortedRoster(members ...uuid.UUID) []uuid.UUID {
	sort.Slice(members, func(i, j int) bool { return members[i].String() < members[j].String() })
	return members
}

func newActiveRosterUpdateSession(t *testing.T) (*Store, *SessionOrchestrator, context.Context, SessionPrincipal, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	store, ctx := startT31SessionStore(t)
	principal := newT31TestSessionPrincipal(t, ctx, store)
	owners := newSessionOwnerScript()
	orchestrator := NewSessionOrchestrator(store, owners.adapters())
	memberA, memberB := uuid.New(), uuid.New()
	if memberA.String() > memberB.String() {
		memberA, memberB = memberB, memberA
	}
	created, err := orchestrator.CreateSession(ctx, principal, CreateSessionInput{
		OperationID: uuid.New(), Kind: "party", ExternalKey: "roster-update-" + uuid.NewString(),
		DisplayName: "Roster update", RosterRevision: 2, RosterComplete: true,
		Members: []uuid.UUID{memberA, memberB}, HostProfileID: &memberA,
	})
	require.NoError(t, err)
	advanceSessionUntil(t, ctx, orchestrator, principal, created.OperationID, "active")
	return store, orchestrator, ctx, principal, created.SessionID, memberA, memberB
}

func TestUpdateSessionRosterStaleAndExactReplayDoNotRenewLease(t *testing.T) {
	store, orchestrator, ctx, principal, sessionID, host, secondMember := newActiveRosterUpdateSession(t)
	var original time.Time
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT lease_expires_at FROM gis_sessions WHERE id=$1`, sessionID).Scan(&original))

	stale := SessionRosterUpdateInput{
		OperationID: uuid.New(), RosterRevision: 1, RosterComplete: true,
		Members: []uuid.UUID{host, secondMember}, HostProfileID: &host,
	}
	_, err := orchestrator.UpdateSessionRoster(ctx, principal, sessionID, stale)
	require.NoError(t, err)
	var afterStale time.Time
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT lease_expires_at FROM gis_sessions WHERE id=$1`, sessionID).Scan(&afterStale))
	require.Equal(t, original, afterStale, "lower revisions are durable stale no-ops")

	current := SessionRosterUpdateInput{
		OperationID: uuid.New(), RosterRevision: 2, RosterComplete: true,
		Members: []uuid.UUID{host, secondMember}, HostProfileID: &host,
	}
	_, err = orchestrator.UpdateSessionRoster(ctx, principal, sessionID, current)
	require.NoError(t, err)
	var afterReplay time.Time
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT lease_expires_at FROM gis_sessions WHERE id=$1`, sessionID).Scan(&afterReplay))
	require.Equal(t, original, afterReplay, "exact same-revision replay must not renew the lease")
}

func TestUpdateSessionRosterHigherCompleteRevisionRenewsFromDatabaseClock(t *testing.T) {
	store, orchestrator, ctx, principal, sessionID, host, _ := newActiveRosterUpdateSession(t)
	input := SessionRosterUpdateInput{
		OperationID: uuid.New(), RosterRevision: 3, RosterComplete: true,
		Members: []uuid.UUID{host}, HostProfileID: &host,
	}
	_, err := orchestrator.UpdateSessionRoster(ctx, principal, sessionID, input)
	require.NoError(t, err)
	var revision int64
	var databaseNow, leaseExpiresAt time.Time
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT roster_revision,clock_timestamp(),lease_expires_at FROM gis_sessions WHERE id=$1`, sessionID).Scan(&revision, &databaseNow, &leaseExpiresAt))
	require.EqualValues(t, 3, revision)
	require.True(t, leaseExpiresAt.After(databaseNow.Add(59*time.Second)), "lease must start from the GIS database clock")
	require.True(t, !leaseExpiresAt.After(databaseNow.Add(60*time.Second)), "lease must be exactly 60 seconds, subject to query latency")
}

func TestUpdateSessionRosterReplayDuringConvergenceDoesNotClaimSuccessEarly(t *testing.T) {
	_, orchestrator, ctx, principal, sessionID, host, _ := newActiveRosterUpdateSession(t)
	accepted := SessionRosterUpdateInput{OperationID: uuid.New(), RosterRevision: 3, RosterComplete: true,
		Members: []uuid.UUID{host}, HostProfileID: &host}
	pending, err := orchestrator.UpdateSessionRoster(ctx, principal, sessionID, accepted)
	require.NoError(t, err)
	require.Equal(t, "pending", pending.Status)

	retry, err := orchestrator.UpdateSessionRoster(ctx, principal, sessionID, accepted)
	require.NoError(t, err, "same operation ID returns its durable in-flight operation")
	require.Equal(t, pending.OperationID, retry.OperationID)
	require.Equal(t, "pending", retry.Status)

	newOperation := accepted
	newOperation.OperationID = uuid.New()
	_, err = orchestrator.UpdateSessionRoster(ctx, principal, sessionID, newOperation)
	require.ErrorIs(t, err, ErrRosterUpdatePending,
		"a different operation ID must not claim owner convergence is complete while the accepted revision is pending")
}

func TestUpdateSessionRosterConcurrentRevisionCASHasSingleWinner(t *testing.T) {
	store, orchestrator, ctx, principal, sessionID, host, other := newActiveRosterUpdateSession(t)
	inputs := []SessionRosterUpdateInput{
		{OperationID: uuid.New(), RosterRevision: 3, RosterComplete: true,
			Members: []uuid.UUID{host}, HostProfileID: &host},
		{OperationID: uuid.New(), RosterRevision: 3, RosterComplete: true,
			Members: []uuid.UUID{host, other}, HostProfileID: &host},
	}
	start := make(chan struct{})
	type result struct{ err error }
	results := make(chan result, len(inputs))
	var workers sync.WaitGroup
	for _, input := range inputs {
		input := input
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, err := orchestrator.UpdateSessionRoster(ctx, principal, sessionID, input)
			results <- result{err: err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	var accepted, conflicted int
	for result := range results {
		if result.err == nil {
			accepted++
			continue
		}
		require.True(t, errors.Is(result.err, ErrIdempotencyConflict) || errors.Is(result.err, ErrRosterUpdatePending),
			"losing CAS returns a typed conflict or pending outcome: %v", result.err)
		conflicted++
	}
	require.Equal(t, 1, accepted)
	require.Equal(t, 1, conflicted)
	var revision int64
	var pending int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT roster_revision FROM gis_sessions WHERE id=$1`, sessionID).Scan(&revision))
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_operations WHERE session_id=$1 AND operation_kind='roster' AND status='pending'`, sessionID).Scan(&pending))
	require.EqualValues(t, 3, revision)
	require.Equal(t, 1, pending, "CAS creates exactly one owner-convergence operation")
}

func TestRosterOwnerReceiptTransactionCannotCommitAfterLeaseIsReplaced(t *testing.T) {
	store, orchestrator, ctx, principal, sessionID, host, _ := newActiveRosterUpdateSession(t)
	accepted, err := orchestrator.UpdateSessionRoster(ctx, principal, sessionID, SessionRosterUpdateInput{
		OperationID: uuid.New(), RosterRevision: 3, RosterComplete: true, Members: []uuid.UUID{host}, HostProfileID: &host,
	})
	require.NoError(t, err)
	_, kind, _, staleOwner, err := store.claim(ctx, accepted.OperationID)
	require.NoError(t, err)
	require.Equal(t, "roster", kind)
	currentOwner := uuid.New()
	_, err = store.Pool.Exec(ctx, `UPDATE gis_session_operations SET lease_owner=$2,lease_until=now()+interval '5 seconds' WHERE operation_id=$1`, accepted.OperationID, currentOwner)
	require.NoError(t, err)

	requestDigest := sha256.Sum256([]byte("stale-roster-owner-request"))
	require.NotNil(t, accepted.ChatID)
	receipt := SessionOwnerReceipt{ResourceID: *accepted.ChatID, ReceiptID: uuid.New(), RequestHash: "sha256:" + fmt.Sprintf("%x", requestDigest)}
	err = orchestrator.recordRosterOwnerReceipt(ctx, SessionOperationKey{ApplicationID: principal.ApplicationID, EnvironmentID: principal.EnvironmentID, OperationID: accepted.OperationID}, sessionID, staleOwner, uuid.New(),
		"chat_roster", requestDigest[:], receipt, "roster_voice_pending")
	require.ErrorContains(t, err, "lease lost")
	var receiptRows int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_owner_receipts WHERE operation_id=$1 AND stage='chat_roster'`, accepted.OperationID).Scan(&receiptRows))
	require.Zero(t, receiptRows, "receipt and binding-relation writes roll back with stale operation lease")
	var savedOwner uuid.UUID
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT lease_owner FROM gis_session_operations WHERE operation_id=$1`, accepted.OperationID).Scan(&savedOwner))
	require.Equal(t, currentOwner, savedOwner, "stale worker cannot clear the replacement worker lease")
}

func TestUpdateSessionRosterRevisionConflictsHostTransferAndValidation(t *testing.T) {
	_, orchestrator, ctx, principal, sessionID, currentHost, secondMember := newActiveRosterUpdateSession(t)
	changedAtSameRevision := SessionRosterUpdateInput{
		OperationID: uuid.New(), RosterRevision: 2, RosterComplete: true,
		Members: []uuid.UUID{currentHost},
	}
	_, err := orchestrator.UpdateSessionRoster(ctx, principal, sessionID, changedAtSameRevision)
	require.Error(t, err, "same revision with a changed body must conflict")

	successor := uuid.New()
	transfer := SessionRosterUpdateInput{
		OperationID: uuid.New(), RosterRevision: 3, RosterComplete: true,
		Members: sortedRoster(currentHost, secondMember, successor), HostProfileID: &successor,
	}
	_, err = orchestrator.UpdateSessionRoster(ctx, principal, sessionID, transfer)
	require.NoError(t, err, "a host transfer is part of the next complete revision")

	outsideHost := uuid.New()
	for _, invalid := range []SessionRosterUpdateInput{
		{OperationID: uuid.New(), RosterRevision: 5, RosterComplete: false, Members: []uuid.UUID{successor}},
		{OperationID: uuid.New(), RosterRevision: 5, RosterComplete: true, Members: nil},
		{OperationID: uuid.New(), RosterRevision: 5, RosterComplete: true, Members: []uuid.UUID{currentHost}, HostProfileID: &successor},
		{OperationID: uuid.New(), RosterRevision: 5, RosterComplete: true, Members: []uuid.UUID{currentHost}, HostProfileID: &outsideHost},
	} {
		_, err = orchestrator.UpdateSessionRoster(ctx, principal, sessionID, invalid)
		require.Error(t, err, "invalid/incomplete snapshot or skipped host transfer must be rejected")
	}
}

func TestUpdateSessionRosterOperationIDCannotBeReusedWithChangedBody(t *testing.T) {
	_, orchestrator, ctx, principal, sessionID, host, _ := newActiveRosterUpdateSession(t)
	operationID := uuid.New()
	first := SessionRosterUpdateInput{
		OperationID: operationID, RosterRevision: 3, RosterComplete: true,
		Members: []uuid.UUID{host}, HostProfileID: &host,
	}
	_, err := orchestrator.UpdateSessionRoster(ctx, principal, sessionID, first)
	require.NoError(t, err)
	changed := first
	changed.HostProfileID = nil
	_, err = orchestrator.UpdateSessionRoster(ctx, principal, sessionID, changed)
	require.ErrorIs(t, err, ErrIdempotencyConflict)
}
