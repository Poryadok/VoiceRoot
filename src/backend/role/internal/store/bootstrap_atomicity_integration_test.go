package store

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type cancelOwnerAssignmentTracer struct {
	cancel context.CancelFunc
	once   sync.Once
}

func (t *cancelOwnerAssignmentTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "INSERT INTO member_roles") {
		t.once.Do(t.cancel)
	}
	return ctx
}

func (*cancelOwnerAssignmentTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {
}

// TestBootstrapSpaceRoles_RollsBackSeedWhenOwnerAssignmentFails ensures a retry
// can publish system-role creation events after a failed first bootstrap.
func TestBootstrapSpaceRoles_RollsBackSeedWhenOwnerAssignmentFails(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	baseCtx := context.Background()
	pool := StartRoleDBForStoreTest(t, baseCtx)
	defer pool.Close()
	ApplyRoleMigrationsForStoreTest(t, baseCtx, pool)

	ctx, cancel := context.WithCancel(baseCtx)
	config := pool.Config().Copy()
	config.ConnConfig.Tracer = &cancelOwnerAssignmentTracer{cancel: cancel}
	tracedPool, err := pgxpool.NewWithConfig(baseCtx, config)
	require.NoError(t, err)
	defer tracedPool.Close()

	spaceID := uuid.New()
	ownerID := uuid.New()
	traced := &RoleStore{Pool: tracedPool}
	_, err = traced.BootstrapSpaceRolesWithCreatedSystemRoles(ctx, spaceID, ownerID)
	require.Error(t, err)

	roles, err := (&RoleStore{Pool: pool}).ListRoles(baseCtx, spaceID)
	require.NoError(t, err)
	require.Empty(t, roles, "failed bootstrap must not leave roles without role.created events")

	created, err := (&RoleStore{Pool: pool}).BootstrapSpaceRolesWithCreatedSystemRoles(baseCtx, spaceID, ownerID)
	require.NoError(t, err)
	require.Len(t, created, 5, "retry must learn it created all system roles")
}
