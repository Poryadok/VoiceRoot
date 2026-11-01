package sourcecustody

import (
	"context"
	"crypto/sha256"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/user/internal/identityassociation"
	"voice/backend/user/internal/testsupport/associationfixture"
)

func TestCaptureRejectsTransactionOnDifferentPhysicalDatabase(t *testing.T) {
	ctx := context.Background()
	root := custodyDatabase(t, ctx)
	t.Setenv("USER_ASSOCIATION_TEST_DSN", root.Config().ConnString())
	db := associationfixture.New(t, 1)
	receipt, err := associationfixture.ReadInstallerReceipt(db.InstallerReceiptPath)
	require.NoError(t, err)
	fingerprint := sha256.Sum256(receipt.UserKey)
	keys, err := identityassociation.NewKeyset(identityassociation.KeysetSnapshot{Revision: 1, ActiveVersion: receipt.KeyVersion, Versions: map[uint32][32]byte{receipt.KeyVersion: fingerprint}}, map[uint32][]byte{receipt.KeyVersion: receipt.UserKey})
	require.NoError(t, err)
	store, err := identityassociation.NewStore(db.Pool, keys, receipt.HistoryGeneration)
	require.NoError(t, err)
	require.NoError(t, store.Bootstrap(ctx))
	require.NoError(t, store.ActivateGuards(ctx))
	var pin identityassociation.PhysicalSourceIdentity
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&pin.SystemIdentifier, &pin.Database, &pin.DatabaseOID))
	var before uint64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&before))
	config := db.Pool.Config().Copy()
	db.Pool.Close()
	clone := "custody_capture_clone"
	_, err = root.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{clone}.Sanitize()+` TEMPLATE `+pgx.Identifier{pin.Database}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, cleanupErr := root.Exec(context.Background(), `DROP DATABASE IF EXISTS `+pgx.Identifier{clone}.Sanitize()+` WITH (FORCE)`)
		require.NoError(t, cleanupErr)
	})
	var connections atomic.Int32
	config.MaxConns = 1
	config.MinConns = 0
	config.BeforeConnect = func(_ context.Context, conn *pgx.ConnConfig) error {
		if connections.Add(1) > 1 {
			conn.Database = clone
		}
		return nil
	}
	config.AfterRelease = func(*pgx.Conn) bool { return false }
	mixed, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer mixed.Close()
	store, err = identityassociation.NewStore(mixed, keys, receipt.HistoryGeneration)
	require.NoError(t, err)
	digest, err := keys.CustodyDigest()
	require.NoError(t, err)
	_, err = store.CaptureProfilesForPhysicalSource(ctx, db.Profiles, receipt.HistoryGeneration, receipt.UserSourceGeneration, digest, pin)
	require.Error(t, err)
	require.GreaterOrEqual(t, connections.Load(), int32(2), "lookup and transaction must have used different sessions")
	cloneConfig := root.Config().Copy()
	cloneConfig.ConnConfig.Database = clone
	clonePool, err := pgxpool.NewWithConfig(ctx, cloneConfig)
	require.NoError(t, err)
	defer clonePool.Close()
	var after, associations uint64
	require.NoError(t, clonePool.QueryRow(ctx, `SELECT history_head,(SELECT count(*) FROM identity_associations) FROM identity_meta WHERE singleton`).Scan(&after, &associations))
	require.Equal(t, before, after)
	require.Zero(t, associations)
}
