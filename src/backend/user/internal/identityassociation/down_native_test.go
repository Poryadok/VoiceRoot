package identityassociation

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestNativeDownRechecksAfterConcurrentInitialization(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	down, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../migrations/user_db/000022_identity_association_core.down.sql"))
	require.NoError(t, err)
	writer, err := BeginIdentityTx(ctx, pool, nativeFence(t, pool))
	require.NoError(t, err)
	defer writer.Rollback(context.Background())
	downConn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer downConn.Release()
	returned := make(chan error, 1)
	go func() { _, downErr := downConn.Exec(ctx, string(down)); returned <- downErr }()
	// The initializer already owns its catalog shared locks and I. The DOWN
	// must wait before trusting emptiness, not merely when DROP later needs an
	// exclusive relation lock after an obsolete empty observation.
	nativeWaitEdge(t, pool, downConn.Conn().PgConn().PID(), writer.SQL().Conn().PgConn().PID(), returned)
	keys := keysetForTest(t, 1, map[uint32][]byte{1: bytes.Repeat([]byte{7}, 32)})
	prepared, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	fingerprint := keys.Snapshot().Versions[1]
	_, err = writer.SQL().Exec(ctx, `INSERT INTO identity_key_versions(version,fingerprint) VALUES(1,$1)`, fingerprint[:])
	require.NoError(t, err)
	_, err = writer.SQL().Exec(ctx, `INSERT INTO identity_profile_reservations(key_version,profile_selector) VALUES(1,$1)`, prepared.aliases[0].profile[:])
	require.NoError(t, err)
	_, err = writer.SQL().Exec(ctx, `UPDATE identity_meta SET keyset_revision=1,active_key_version=1 WHERE singleton`)
	require.NoError(t, err)
	var revision uint64
	require.NoError(t, writer.SQL().QueryRow(ctx, `SELECT identity_append_event(NULL,NULL,'KEYSET',NULL,NULL,NULL)`).Scan(&revision))
	require.EqualValues(t, 1, revision)
	require.NoError(t, writer.Commit(ctx))
	var pgErr *pgconn.PgError
	require.ErrorAs(t, <-returned, &pgErr, "DOWN must refuse newly committed authority rather than discard it")
	require.Equal(t, "55000", pgErr.Code)
	var head uint64
	var keysCount, reservations, journal int
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head,(SELECT count(*) FROM identity_key_versions),(SELECT count(*) FROM identity_profile_reservations),(SELECT count(*) FROM identity_journal) FROM identity_meta WHERE singleton`).Scan(&head, &keysCount, &reservations, &journal))
	require.EqualValues(t, 1, head)
	require.Equal(t, 1, keysCount)
	require.Equal(t, 1, reservations)
	require.Equal(t, 1, journal)
}
