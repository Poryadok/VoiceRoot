package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestAuditIngestionTerminalMinimizationKeepsNonpayloadFence(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	event := importFixture(t, st)
	applyAuditIngestionMigration(t, st)
	ctx := context.Background()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000029_audit_receipt_minimization.up.sql"))
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, string(body))
	require.NoError(t, err)
	require.NoError(t, st.ImportAuditEvent(ctx, "role", event))
	run := func(commit bool) {
		tx, e := st.Pool.Begin(ctx)
		require.NoError(t, e)
		defer rollbackLifecycleTx(ctx, tx)
		require.NoError(t, cleanupAuditIngressEvidence(ctx, tx))
		if commit {
			require.NoError(t, tx.Commit(ctx))
		}
	}
	assertPayload := func(minimized bool) {
		var flag bool
		var hash []byte
		var version *int
		var origin string
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT fingerprint_minimized,fingerprint_version,fingerprint,origin FROM audit_event_receipts WHERE audit_event_id=$1`, event.AuditEventID).Scan(&flag, &version, &hash, &origin))
		require.Equal(t, minimized, flag)
		require.Equal(t, "role", origin)
		if minimized {
			require.Nil(t, version)
			require.Empty(t, hash)
		} else {
			require.NotNil(t, version)
			require.Len(t, hash, 32)
		}
	}
	run(true)
	assertPayload(false) // no terminal fence, regardless of old occurrence time
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_permanent_purge_fences(space_id,deletion_operation_id,purge_generation,purged_at) VALUES($1,$2,1,clock_timestamp()-interval '29 days')`, event.SpaceID, uuid.New())
	require.NoError(t, err)
	run(true)
	assertPayload(false)
	_, err = st.Pool.Exec(ctx, `UPDATE audit_event_receipts SET fingerprint_version=NULL,fingerprint=NULL,fingerprint_minimized=true WHERE audit_event_id=$1`, event.AuditEventID)
	require.Error(t, err, "early minimization denied by database predicates")
	_, err = st.Pool.Exec(ctx, `UPDATE space_permanent_purge_fences SET purged_at=clock_timestamp()-interval '30 days' WHERE space_id=$1`, event.SpaceID)
	require.NoError(t, err)
	run(false)
	assertPayload(false) // failed enclosing cleanup keeps original evidence
	run(true)
	assertPayload(true)
	run(true)
	assertPayload(true)
	require.ErrorIs(t, st.ImportAuditEvent(ctx, "role", event), ErrLifecycleFrozen, "erased payload never means a new or acknowledged late delivery")
	assertImportCounts(t, st, event.AuditEventID, 1, 1, 1)
	for _, query := range []string{`DELETE FROM audit_event_receipts WHERE audit_event_id=$1`, `UPDATE audit_event_receipts SET origin='local' WHERE audit_event_id=$1`, `UPDATE audit_event_receipts SET audit_event_id=gen_random_uuid() WHERE audit_event_id=$1`} {
		_, err = st.Pool.Exec(ctx, query, event.AuditEventID)
		require.Error(t, err)
	}
	_, err = st.Pool.Exec(ctx, `INSERT INTO audit_event_receipts(audit_event_id,space_id,origin,fingerprint_minimized) VALUES($1,$2,'role',true)`, uuid.New(), event.SpaceID)
	require.Error(t, err, "compact markers can only result from due cleanup")
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000029_audit_receipt_minimization.down.sql"))
	require.NoError(t, err)
	connection, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer connection.Release()
	_, err = connection.Exec(ctx, string(down))
	require.Error(t, err, "erased hashes cannot be fabricated by rollback")
	_, err = connection.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	assertPayload(true)
}
