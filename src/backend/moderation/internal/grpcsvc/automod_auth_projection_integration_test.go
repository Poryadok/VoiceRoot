package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Execute the exact Auth-owned SQL on PostgreSQL. The Java service separately
// proves private principal and RPC registration; this test is the cross-store
// ordering/deletion contract, not an Auth application or Flyway-wide sign-off.
func TestAutoModAuthProjection_RevisionsLegacyAndDeletionPrecedence(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	root := repoRoot(t)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(root, "src/backend/auth/src/main/resources/db/migration/V1__auth_schema.sql"))
	legacy := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO accounts(id,password_hash,type,status) VALUES($1,'test','regular','suspended')`, legacy)
	require.NoError(t, err)
	flyway := filepath.Join(root, "src/backend/auth/src/main/resources/db/migration/V27__moderation_subject_restrictions.sql")
	canonical, err := os.ReadFile(filepath.Join(root, "src/backend/migrations/auth_db/000028_moderation_subject_restrictions.up.sql"))
	require.NoError(t, err)
	exact, err := os.ReadFile(flyway)
	require.NoError(t, err)
	require.Equal(t, exact, canonical, "both migration engines must install the same authority")
	integrationtestApplySQL(t, ctx, pool, flyway)
	var legacySubject uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT moderation_subject_id FROM accounts WHERE id=$1`, legacy).Scan(&legacySubject))
	var revision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT auth_reconcile_moderation($1,$2,1,false)`, legacy, legacySubject).Scan(&revision))
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM accounts WHERE id=$1`, legacy).Scan(&state))
	require.Equal(t, "suspended", state, "new projection cannot clear an unknown existing suspension")
	account := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO accounts(id,password_hash,type,status) VALUES($1,'test','regular','active')`, account)
	require.NoError(t, err)
	var subject uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT moderation_subject_id FROM accounts WHERE id=$1`, account).Scan(&subject))
	for _, step := range []struct {
		revision int
		banned   bool
		expected string
	}{{2, true, "suspended"}, {1, false, "suspended"}, {2, true, "suspended"}, {3, false, "active"}} {
		require.NoError(t, pool.QueryRow(ctx, `SELECT auth_reconcile_moderation($1,$2,$3,$4)`, account, subject, step.revision, step.banned).Scan(&revision))
		require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM accounts WHERE id=$1`, account).Scan(&state))
		require.Equal(t, step.expected, state)
	}
	err = pool.QueryRow(ctx, `SELECT auth_reconcile_moderation($1,$2,3,true)`, account, subject).Scan(&revision)
	require.ErrorContains(t, err, "moderation revision conflict")
	require.NoError(t, pool.QueryRow(ctx, `SELECT auth_reconcile_moderation($1,$2,4,true)`, account, subject).Scan(&revision))
	_, err = pool.Exec(ctx, `UPDATE accounts SET status='active' WHERE id=$1`, account)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM accounts WHERE id=$1`, account).Scan(&state))
	require.Equal(t, "suspended", state, "legacy writer cannot bypass the versioned restriction")
	_, err = pool.Exec(ctx, `UPDATE accounts SET deleted_at=clock_timestamp(),status='deleted' WHERE id=$1`, account)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT auth_reconcile_moderation($1,$2,5,false)`, account, subject).Scan(&revision))
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM accounts WHERE id=$1`, account).Scan(&state))
	require.Equal(t, "deleted", state, "clear must not restore a deleted account")
	_, err = pool.Exec(ctx, `UPDATE accounts SET deleted_at=NULL,status='active' WHERE id=$1`, account)
	require.NoError(t, err)
	var lifecycle int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT moderation_lifecycle_revision FROM accounts WHERE id=$1`, account).Scan(&lifecycle))
	require.EqualValues(t, 3, lifecycle)
	_, err = pool.Exec(ctx, `UPDATE accounts SET deleted_at=clock_timestamp()-interval '31 days',status='deleted' WHERE id=$1`, account)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE accounts SET deleted_at=NULL,status='active' WHERE id=$1`, account)
	require.ErrorContains(t, err, "terminal moderation subject cannot be restored")
	err = pool.QueryRow(ctx, `SELECT auth_reconcile_moderation($1,$2,6,false)`, account, subject).Scan(&revision)
	require.ErrorContains(t, err, "terminal or unknown moderation subject")
	_, err = pool.Exec(ctx, `DELETE FROM accounts WHERE id=$1`, account)
	require.NoError(t, err)
	var fence bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM moderation_terminal_subjects WHERE subject_id=$1)`, subject).Scan(&fence))
	require.True(t, fence)
	_, err = pool.Exec(ctx, `INSERT INTO accounts(id,password_hash,type,status,moderation_subject_id) VALUES($1,'test','regular','active',$2)`, account, subject)
	require.ErrorContains(t, err, "terminal moderation subject cannot become active")
	backupAccount := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO accounts(id,password_hash,type,status) VALUES($1,'test','regular','active')`, backupAccount)
	require.NoError(t, err)
	var backupSubject uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT moderation_subject_id FROM accounts WHERE id=$1`, backupAccount).Scan(&backupSubject))
	_, err = pool.Exec(ctx, `INSERT INTO moderation_terminal_subjects(subject_id) VALUES($1)`, backupSubject)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE accounts SET moderation_subject_id=NULL WHERE id=$1`, backupAccount)
	require.NoError(t, err)
	var removed *uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT status,moderation_subject_id FROM accounts WHERE id=$1`, backupAccount).Scan(&state, &removed))
	require.Equal(t, "deleted", state)
	require.Nil(t, removed, "imported terminal fence removes backup's obsolete raw subject mapping")
}
