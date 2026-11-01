package sourcecustody

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQualifiedStreamReturnsAndHoldsPhysicalBinding(t *testing.T) {
	f := newStreamFixture(t)
	physical := PhysicalSource{SystemIdentifier: "123456789", Database: "source_fixture", DatabaseOID: 7654}
	require.NotEqual(t, f.control.controlSystem, physical.SystemIdentifier)
	_, err := f.pool.Exec(f.ctx, `CREATE TABLE user_custody_physical_source(source_id text NOT NULL,epoch bigint NOT NULL,system_identifier text NOT NULL,database_name text NOT NULL,database_oid oid NOT NULL,PRIMARY KEY(source_id,epoch))`)
	require.NoError(t, err)
	_, err = f.pool.Exec(f.ctx, `INSERT INTO user_custody_physical_source VALUES($1,$2,$3,$4,$5)`, f.control.binding.SourceID, f.control.binding.Epoch, physical.SystemIdentifier, physical.Database, physical.DatabaseOID)
	require.NoError(t, err)
	client, err := NewClient(f.client, f.issuer, time.Second)
	require.NoError(t, err)
	grant, err := client.AcquireQualified(f.ctx, f.control.binding)
	require.NoError(t, err)
	require.Equal(t, physical, grant.Physical())
	bounded, cancel := context.WithTimeout(f.ctx, 200*time.Millisecond)
	_, err = f.pool.Exec(bounded, `UPDATE user_custody_physical_source SET database_name='changed' WHERE source_id=$1 AND epoch=$2`, f.control.binding.SourceID, f.control.binding.Epoch)
	cancel()
	require.Error(t, err)
	require.NoError(t, grant.Context().Err())
	grant.Release()
	require.Eventually(t, func() bool {
		bounded, cancel := context.WithTimeout(f.ctx, 200*time.Millisecond)
		defer cancel()
		_, updateErr := f.pool.Exec(bounded, `UPDATE user_custody_physical_source SET database_name='changed' WHERE source_id=$1 AND epoch=$2`, f.control.binding.SourceID, f.control.binding.Epoch)
		return updateErr == nil
	}, 5*time.Second, 50*time.Millisecond)
}
