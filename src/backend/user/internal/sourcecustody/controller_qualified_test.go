package sourcecustody

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCustodyBindingRejectsEqualGenerations(t *testing.T) {
	binding := custodyBinding()
	binding.UserSourceGeneration = binding.HistoryGeneration
	require.False(t, binding.valid())
}

func TestQualifiedPermitPinsPhysicalRecordUntilRelease(t *testing.T) {
	ctx, control, source, binding, controller := custodyFixture(t)
	var physical PhysicalSource
	require.NoError(t, source.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&physical.SystemIdentifier, &physical.Database, &physical.DatabaseOID))
	_, err := control.Exec(ctx, `CREATE TABLE user_custody_physical_source(source_id text NOT NULL,epoch bigint NOT NULL,system_identifier text NOT NULL,database_name text NOT NULL,database_oid oid NOT NULL,PRIMARY KEY(source_id,epoch))`)
	require.NoError(t, err)
	_, err = control.Exec(ctx, `INSERT INTO user_custody_physical_source VALUES($1,$2,$3,$4,$5)`, binding.SourceID, binding.Epoch, physical.SystemIdentifier, physical.Database, physical.DatabaseOID)
	require.NoError(t, err)
	_, err = control.Exec(ctx, `UPDATE user_custody_state SET state='OPEN'`)
	require.NoError(t, err)
	permit, actual, err := controller.AcquireQualified(ctx, binding)
	require.NoError(t, err)
	require.Equal(t, physical, actual)
	bounded, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	_, err = control.Exec(bounded, `UPDATE user_custody_physical_source SET database_name=database_name||'_next' WHERE source_id=$1 AND epoch=$2`, binding.SourceID, binding.Epoch)
	require.Error(t, err, "physical record must remain locked while the grant is held")
	require.NoError(t, permit.Check(ctx))
	require.NoError(t, permit.Release(ctx))
	_, err = control.Exec(ctx, `UPDATE user_custody_physical_source SET database_name=database_name||'_next' WHERE source_id=$1 AND epoch=$2`, binding.SourceID, binding.Epoch)
	require.NoError(t, err)
}

func TestQualifiedPermitRejectsAbsentPhysicalRecord(t *testing.T) {
	ctx, control, _, binding, controller := custodyFixture(t)
	_, err := control.Exec(ctx, `UPDATE user_custody_state SET state='OPEN'`)
	require.NoError(t, err)
	permit, _, err := controller.AcquireQualified(ctx, binding)
	require.Error(t, err)
	require.Nil(t, permit)
}

func TestQualifiedPermitFailsWhenPinnedSessionDies(t *testing.T) {
	ctx, control, source, binding, controller := custodyFixtureWithFailure(t, true)
	var physical PhysicalSource
	require.NoError(t, source.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&physical.SystemIdentifier, &physical.Database, &physical.DatabaseOID))
	_, err := control.Exec(ctx, `CREATE TABLE user_custody_physical_source(source_id text NOT NULL,epoch bigint NOT NULL,system_identifier text NOT NULL,database_name text NOT NULL,database_oid oid NOT NULL,PRIMARY KEY(source_id,epoch))`)
	require.NoError(t, err)
	_, err = control.Exec(ctx, `INSERT INTO user_custody_physical_source VALUES($1,$2,$3,$4,$5)`, binding.SourceID, binding.Epoch, physical.SystemIdentifier, physical.Database, physical.DatabaseOID)
	require.NoError(t, err)
	_, err = control.Exec(ctx, `UPDATE user_custody_state SET state='OPEN'`)
	require.NoError(t, err)
	permit, _, err := controller.AcquireQualified(ctx, binding)
	require.NoError(t, err)
	var pid int
	require.NoError(t, permit.conn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	var terminated bool
	require.NoError(t, control.QueryRow(ctx, `SELECT pg_terminate_backend($1)`, pid).Scan(&terminated))
	require.True(t, terminated)
	require.Error(t, permit.Check(ctx))
	require.Error(t, permit.Release(ctx))
}
