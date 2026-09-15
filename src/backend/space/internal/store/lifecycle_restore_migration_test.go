package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func restoreMigrationSQL(t *testing.T, direction string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000017_lifecycle_restore_outcome."+direction+".sql"))
	require.NoError(t, err)
	return string(raw)
}

func TestLifecycleRestoreMigration_ExpandAndSafeRollback(t *testing.T) {
	up, down := restoreMigrationSQL(t, "up"), restoreMigrationSQL(t, "down")
	if testing.Short() {
		return
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationsThrough12ForStoreTest(t, ctx, pool)
	applyLifecycleMigration(t, ctx, pool, "up")
	_, err := pool.Exec(ctx, up)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)
	st := &SpaceStore{Pool: pool}
	_, _, _, _, _, request, _, _ := reserveLifecycleOperationFixture(t, st)
	var before string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_jsonb(o)::text FROM space_lifecycle_operations o WHERE operation_id=$1`, request.OperationId).Scan(&before))
	// Existing DELETE reservations survive expand/contract without losing factors.
	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)
	var method string
	require.NoError(t, pool.QueryRow(ctx, `SELECT method FROM space_lifecycle_operations WHERE operation_id=$1`, request.OperationId).Scan(&method))
	require.Equal(t, "DELETE", method)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)
	var after string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_jsonb(o)::text FROM space_lifecycle_operations o WHERE operation_id=$1`, request.OperationId).Scan(&after))
	require.Equal(t, before, after)
}

func TestLifecycleRestoreMigration_ConstrainsMethodsAndRefusesEvidenceLoss(t *testing.T) {
	up, down := restoreMigrationSQL(t, "up"), restoreMigrationSQL(t, "down")
	if testing.Short() {
		return
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationsThrough12ForStoreTest(t, ctx, pool)
	applyLifecycleMigration(t, ctx, pool, "up")
	_, err := pool.Exec(ctx, up)
	require.NoError(t, err)
	operationID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO space_lifecycle_operations(operation_id,account_id,actor_profile_id,space_id,session_epoch,method,request_sha256,binding_sha256,state,deletion_operation_id,generation)
	VALUES($1,$2,$3,$4,1,'RESTORE',decode(repeat('01',32),'hex'),decode(repeat('02',32),'hex'),'RESTORE_PENDING',$5,2)`, operationID, uuid.New(), uuid.New(), uuid.New(), uuid.New())
	require.NoError(t, err)
	for _, mutation := range []string{
		`confirmation_name_sha256=decode(repeat('01',32),'hex')`,
		`proof_digest_sha256=decode(repeat('01',32),'hex')`,
		`auth_receipt_bytes='x'::bytea,auth_receipt_sha256=decode(repeat('01',32),'hex')`,
		`deletion_operation_id=NULL`, `deletion_operation_id=operation_id`, `generation=NULL`, `generation=0`,
		`method='DELETE',state='SCHEDULE_PENDING',deletion_operation_id=NULL,generation=NULL`,
		`state='SCHEDULE_PENDING'`, `state='COMPLETED'`,
	} {
		_, err := pool.Exec(ctx, `UPDATE space_lifecycle_operations SET `+mutation+` WHERE operation_id=$1`, operationID)
		require.Error(t, err, mutation)
	}
	var before, after string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_jsonb(o)::text FROM space_lifecycle_operations o WHERE operation_id=$1`, operationID).Scan(&before))
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, down)
	require.Error(t, err, "downgrade must preserve admitted restore evidence")
	_, err = conn.Exec(ctx, `ROLLBACK`)
	require.NoError(t, err)
	require.NoError(t, conn.QueryRow(ctx, `SELECT to_jsonb(o)::text FROM space_lifecycle_operations o WHERE operation_id=$1`, operationID).Scan(&after))
	require.Equal(t, before, after)
}
