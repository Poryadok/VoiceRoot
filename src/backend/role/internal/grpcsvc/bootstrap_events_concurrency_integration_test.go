package grpcsvc

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	rolev1 "voice.app/voice/role/v1"
)

type bootstrapAdvisoryLockTracer struct {
	attempted chan struct{}
}

func (t *bootstrapAdvisoryLockTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "pg_advisory_xact_lock") {
		select {
		case t.attempted <- struct{}{}:
		default:
		}
	}
	return ctx
}

func (*bootstrapAdvisoryLockTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {
}

// TestBootstrapSpaceRoles_ConcurrentBootstrapPublishesCreatedEventsOnce holds the
// space-scoped bootstrap lock while two requests arrive at an empty space. After
// release, exactly one request must create roles and publish their events.
func TestBootstrapSpaceRoles_ConcurrentBootstrapPublishesCreatedEventsOnce(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()

	spaceID := uuid.New()
	roles, err := s.ListRoles(ctx, spaceID)
	require.NoError(t, err)
	require.Empty(t, roles, "first pre-list must see an empty space")
	roles, err = s.ListRoles(ctx, spaceID)
	require.NoError(t, err)
	require.Empty(t, roles, "second pre-list must see the same empty space")

	tracer := &bootstrapAdvisoryLockTracer{attempted: make(chan struct{}, 2)}
	config := s.Pool.Config().Copy()
	config.ConnConfig.Tracer = tracer
	tracedPool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer tracedPool.Close()

	events := &recordingRoleEvents{}
	client, stop := startRoleGRPCTestServer(t, tracedPool, func(svc *RoleGRPC) { svc.Events = events })
	defer stop()

	lockTx, err := s.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = lockTx.Rollback(ctx) }()
	_, err = lockTx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, spaceID.String())
	require.NoError(t, err)

	request := &rolev1.BootstrapSpaceRolesRequest{
		SpaceId:        spaceID.String(),
		OwnerProfileId: uuid.New().String(),
	}
	results := make(chan error, 2)
	var start sync.WaitGroup
	start.Add(1)
	for range 2 {
		go func() {
			start.Wait()
			_, callErr := client.BootstrapSpaceRoles(ctx, request)
			results <- callErr
		}()
	}
	start.Done()

	for range 2 {
		select {
		case <-tracer.attempted:
		case callErr := <-results:
			require.NoError(t, callErr, "bootstrap must wait on the atomic store boundary")
			require.FailNow(t, "bootstrap completed without the store creation lock")
		case <-time.After(5 * time.Second):
			require.FailNow(t, "both requests must reach the atomic store creation lock")
		}
	}
	require.NoError(t, lockTx.Commit(ctx))

	for range 2 {
		require.NoError(t, <-results)
	}
	created := events.createdEvents()
	require.Len(t, created, 5, "only the atomic creator may publish role.created events")
}
