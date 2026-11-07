package sourcecustody

import (
	"context"
	"crypto/sha256"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
	"voice/backend/user/internal/identityassociation"
	"voice/backend/user/internal/testsupport/associationfixture"
)

// Synthetic OPEN exercises held-stream mechanics, not production source custody.
func TestAuthProfileQualifiedReadHoldsPermitThroughCommitAndDeniesClosedSource(t *testing.T) {
	ctx := context.Background()
	root := custodyDatabase(t, ctx)
	t.Setenv("USER_ASSOCIATION_TEST_DSN", root.Config().ConnString())
	db := associationfixture.New(t, 1)
	receipt, err := associationfixture.ReadInstallerReceipt(db.InstallerReceiptPath)
	require.NoError(t, err)
	fingerprint := sha256.Sum256(receipt.UserKey)
	keys, err := identityassociation.NewKeyset(identityassociation.KeysetSnapshot{Revision: 1, ActiveVersion: receipt.KeyVersion, Versions: map[uint32][32]byte{receipt.KeyVersion: fingerprint}}, map[uint32][]byte{receipt.KeyVersion: receipt.UserKey})
	require.NoError(t, err)
	trace := &capturePreReadTrace{started: make(chan uint32, 4), commitStarted: make(chan struct{}), commitRelease: make(chan struct{})}
	config := db.Pool.Config().Copy()
	config.ConnConfig.Tracer = trace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	store, err := identityassociation.NewStore(pool, keys, receipt.HistoryGeneration)
	require.NoError(t, err)
	require.NoError(t, store.Bootstrap(ctx))
	require.NoError(t, store.ActivateGuards(ctx))
	binding := custodyBinding()
	binding.HistoryGeneration = receipt.HistoryGeneration
	require.NoError(t, pool.QueryRow(ctx, `SELECT user_source_generation FROM identity_meta WHERE singleton`).Scan(&binding.UserSourceGeneration))
	binding.KeysetDigest, err = keys.CustodyDigest()
	require.NoError(t, err)
	f := newStreamFixtureBound(t, false, time.Now, 100*time.Millisecond, &binding, nil)
	installStreamPhysical(t, f, db.Pool)
	client, err := NewClient(f.client, f.issuer, 500*time.Millisecond)
	require.NoError(t, err)
	reader, err := NewCaptureClient(client, binding, store)
	require.NoError(t, err)
	// Setup captures the one known profile; the tested Auth read must not capture.
	_, err = reader.CaptureProfiles(ctx, db.Profiles)
	require.NoError(t, err)
	var before uint64
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&before))
	out, err := reader.ReadAuthProfiles(ctx, db.Accounts[db.Profiles[0]], db.Profiles)
	require.NoError(t, err)
	require.Equal(t, "MATCH", out.Results[0].Match)
	require.Equal(t, before, out.Cut.ThroughRevision)
	out, err = reader.ReadAuthProfiles(ctx, uuid.New(), []uuid.UUID{uuid.New()})
	require.NoError(t, err)
	require.Equal(t, "INCOMPLETE", out.Results[0].Match)
	require.Equal(t, before, out.Cut.ThroughRevision)
	trace.commitArmed.Store(true)
	var once sync.Once
	release := func() { once.Do(func() { close(trace.commitRelease) }) }
	defer release()
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, e := reader.ReadAuthProfiles(call, db.Accounts[db.Profiles[0]], db.Profiles); done <- e }()
	select {
	case <-trace.commitStarted:
	case e := <-done:
		t.Fatalf("read returned before commit: %v", e)
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("read never reached commit")
	}
	var held int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND mode='ShareLock' AND granted AND objsubid=1 AND classid=(6219270163691208962::bigint >> 32)::oid AND objid=(6219270163691208962::bigint & 4294967295)::oid`).Scan(&held))
	require.Equal(t, 1, held)
	// A cancellation while commit is paused cannot publish even a read result.
	cancel()
	release()
	require.Error(t, <-done)
	require.NoError(t, f.control.CloseAdmission(ctx))
	require.NoError(t, f.control.WaitPermits(ctx))
	_, err = reader.ReadAuthProfiles(ctx, db.Accounts[db.Profiles[0]], db.Profiles)
	require.Error(t, err)
	var after uint64
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&after))
	require.Equal(t, before, after)
}
