package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func startRoleAuditOutbox(t *testing.T) (*RoleStore, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	sql, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "role_db", "000018_space_audit_outbox.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sql))
	require.NoError(t, err)
	return &RoleStore{Pool: pool}, ctx
}

func TestRoleAuditOutbox_RejectsUnscopedEnqueue(t *testing.T) {
	for _, s := range []*RoleStore{nil, {}, {Pool: &pgxpool.Pool{}}} {
		err := s.EnqueueAuditEffect(context.Background(), uuid.New(), uuid.New(), []byte("request"))
		require.Error(t, err)
	}
}

func TestRoleAuditOutbox_MutationAndFactRollbackTogether(t *testing.T) {
	s, ctx := startRoleAuditOutbox(t)
	space, actor, event := uuid.New(), uuid.New(), uuid.New()
	rollback := errors.New("injected after audit enqueue")
	err := s.WithinSpaces(ctx, []uuid.UUID{space}, func(tx *RoleStore) error {
		_, err := tx.CreateCustomRole(ctx, space, "RollbackAudit", 0, 1, &actor)
		if err != nil {
			return err
		}
		if err := tx.EnqueueAuditEffect(ctx, event, space, []byte("frozen-request")); err != nil {
			return err
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	var roles, facts int
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM roles WHERE space_id=$1", space).Scan(&roles))
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM role_space_audit_outbox WHERE space_id=$1", space).Scan(&facts))
	require.Zero(t, roles)
	require.Zero(t, facts)
}

func TestRoleAuditOutbox_ExactBytesSurviveLeaseRecoveryAndStaleAck(t *testing.T) {
	s, ctx := startRoleAuditOutbox(t)
	space, event := uuid.New(), uuid.New()
	payload := []byte{0, 1, 2, 255}
	require.NoError(t, s.WithinSpaces(ctx, []uuid.UUID{space}, func(tx *RoleStore) error {
		return tx.EnqueueAuditEffect(ctx, event, space, payload)
	}))
	first, err := s.ClaimAuditEffects(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, payload, first[0].RequestBytes)
	other, err := s.ClaimAuditEffects(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Empty(t, other)
	_, err = s.Pool.Exec(ctx, "UPDATE role_space_audit_outbox SET lease_expires_at=clock_timestamp()-interval '1 second' WHERE audit_event_id=$1", event)
	require.NoError(t, err)
	second, err := s.ClaimAuditEffects(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.NotEqual(t, first[0].LeaseToken, second[0].LeaseToken)
	require.Equal(t, payload, second[0].RequestBytes)
	marked, err := s.CompleteAuditEffect(ctx, event, first[0].LeaseToken)
	require.NoError(t, err)
	require.False(t, marked)
	marked, err = s.CompleteAuditEffect(ctx, event, second[0].LeaseToken)
	require.NoError(t, err)
	require.True(t, marked)
	other, err = s.ClaimAuditEffects(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Empty(t, other)
	_, err = s.Pool.Exec(ctx, "UPDATE role_space_audit_outbox SET request_bytes=$2 WHERE audit_event_id=$1", event, []byte("changed"))
	require.Error(t, err, "delivery must never replace the committed fact")
}
