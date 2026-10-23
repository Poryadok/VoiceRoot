package identityassociation

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func nativeDatabase(t *testing.T) (*pgxpool.Pool, uuid.UUID, uuid.UUID) {
	t.Helper()
	dsn := os.Getenv("USER_ASSOCIATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set USER_ASSOCIATION_TEST_DSN to an owned PostgreSQL16 instance")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	database := "capture_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = root.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize())
	require.NoError(t, err)
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.Database = database
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, err := root.Exec(ctx, "DROP DATABASE "+pgx.Identifier{database}.Sanitize())
		require.NoError(t, err)
		root.Close()
	})
	_, err = pool.Exec(ctx, `CREATE TABLE public.schema_migrations(version bigint NOT NULL,dirty boolean NOT NULL); INSERT INTO public.schema_migrations VALUES(0,false)`)
	require.NoError(t, err)
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "src", "backend", "migrations", "user_db")); err == nil {
			break
		}
		next := filepath.Dir(dir)
		require.NotEqual(t, dir, next)
		dir = next
	}
	migrations := filepath.Join(dir, "src", "backend", "migrations", "user_db")
	files, err := os.ReadDir(migrations)
	require.NoError(t, err)
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".up.sql") || file.Name() >= "000022" {
			continue
		}
		version, err := strconv.Atoi(file.Name()[:6])
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE public.schema_migrations SET version=$1,dirty=true`, version)
		require.NoError(t, err)
		sql, err := os.ReadFile(filepath.Join(migrations, file.Name()))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sql))
		require.NoError(t, err, file.Name())
		_, err = pool.Exec(ctx, `UPDATE public.schema_migrations SET dirty=false`)
		require.NoError(t, err)
	}
	profile, account := uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'capturefixture','1234','Capture fixture')`, profile, account)
	require.NoError(t, err)
	sql, err := os.ReadFile(filepath.Join(migrations, "000022_identity_association_core.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sql))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE public.schema_migrations SET version=22,dirty=false`)
	require.NoError(t, err)
	return pool, profile, account
}

func TestNativeIdentityGuardRejectsRawIDAndAccountRebind(t *testing.T) {
	for _, column := range []string{"account_id", "id"} {
		t.Run(column, func(t *testing.T) {
			pool, profile, _ := nativeDatabase(t)
			_, err := pool.Exec(context.Background(), "UPDATE profiles SET "+column+"=$1 WHERE id=$2", uuid.New(), profile)
			require.Error(t, err, "canonical profile identity must never rebind")
			var pgErr *pgconn.PgError
			require.ErrorAs(t, err, &pgErr)
			require.Equal(t, "23514", pgErr.Code)
			require.Equal(t, "user_profile_identity_immutable", pgErr.ConstraintName)
			_, err = pool.Exec(context.Background(), `UPDATE profiles SET display_name='Still mutable',account_id=account_id,id=id WHERE id=$1`, profile)
			require.NoError(t, err, "ordinary metadata mutation and identity-preserving update remain valid")
		})
	}
}

func nativeFence(t *testing.T, pool *pgxpool.Pool) *ExpectedFence {
	t.Helper()
	f := &ExpectedFence{}
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT history_generation,keyset_revision FROM identity_meta WHERE singleton`).Scan(&f.Generation, &f.KeysetRevision))
	return f
}

// A real wait edge, rather than elapsed sleep, is the synchronization barrier.
func nativeWaitEdge(t *testing.T, pool *pgxpool.Pool, waiter, holder uint32, returned <-chan error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-returned:
			t.Fatalf("operation returned before required mutex wait: %v", err)
		default:
		}
		var blocked bool
		err := pool.QueryRow(ctx, `SELECT $1::integer=ANY(pg_blocking_pids($2::integer))`, holder, waiter).Scan(&blocked)
		require.NoError(t, err)
		if blocked {
			return
		}
		select {
		case <-tick.C:
		case <-ctx.Done():
			t.Fatal("expected observed PostgreSQL lock edge was not established")
		}
	}
}

func TestNativeIdentityTxWaitsBeforeFreshProfileObservation(t *testing.T) {
	pool, profile, _ := nativeDatabase(t)
	fence := nativeFence(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	owner, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer owner.Rollback(context.Background())
	_, err = owner.Exec(ctx, `SELECT singleton FROM identity_meta WHERE singleton FOR UPDATE`)
	require.NoError(t, err)
	_, err = owner.Exec(ctx, `UPDATE profiles SET frozen_at=clock_timestamp() WHERE id=$1`, profile)
	require.NoError(t, err)
	waiter, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer waiter.Rollback(context.Background())
	returned := make(chan error, 1)
	var frozen bool
	go func() {
		cap, err := acquireIdentityTx(ctx, waiter, fence)
		if err == nil {
			err = cap.SQL().QueryRow(ctx, `SELECT frozen_at IS NOT NULL FROM profiles WHERE id=$1`, profile).Scan(&frozen)
		}
		returned <- err
	}()
	nativeWaitEdge(t, pool, waiter.Conn().PgConn().PID(), owner.Conn().PgConn().PID(), returned)
	require.NoError(t, owner.Commit(ctx))
	require.NoError(t, <-returned)
	require.True(t, frozen, "fresh command after I sees committed writer state")
}

func TestNativeIdentityTxRejectsOldRepeatableReadSnapshot(t *testing.T) {
	pool, _, _ := nativeDatabase(t)
	ctx := context.Background()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `SELECT count(*) FROM profiles`)
	require.NoError(t, err)
	_, err = acquireIdentityTx(ctx, tx, nativeFence(t, pool))
	require.Error(t, err, "old RR cannot become a CURRENT identity capture")
}

func TestNativeLegacyProfileLockCannotWaitBackwardsForIdentityMutex(t *testing.T) {
	pool, profile, _ := nativeDatabase(t)
	fence := nativeFence(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	legacy, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer legacy.Rollback(context.Background())
	_, err = legacy.Exec(ctx, `SELECT id FROM profiles WHERE id=$1 FOR UPDATE`, profile)
	require.NoError(t, err)
	identity, err := BeginIdentityTx(ctx, pool, fence)
	require.NoError(t, err)
	defer identity.Rollback(context.Background())
	returned := make(chan error, 1)
	go func() {
		_, err := identity.SQL().Exec(ctx, `SELECT id FROM profiles WHERE id=$1 FOR UPDATE`, profile)
		returned <- err
	}()
	nativeWaitEdge(t, pool, identity.SQL().Conn().PgConn().PID(), legacy.Conn().PgConn().PID(), returned)
	_, err = legacy.Exec(ctx, `UPDATE profiles SET display_name='Legacy writer' WHERE id=$1`, profile)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "55P03", pgErr.Code, "NOWAIT rejects the reverse wait edge, never a 40P01 deadlock victim")
	require.NoError(t, legacy.Rollback(ctx))
	require.NoError(t, <-returned)
	require.NoError(t, identity.Commit(ctx))
}

func nativeStore(t *testing.T, pool *pgxpool.Pool) (*Store, *Keyset) {
	t.Helper()
	keys := keysetForTest(t, 1, map[uint32][]byte{1: bytes.Repeat([]byte{7}, 32)})
	store, err := NewStore(pool, keys, nativeFence(t, pool).Generation)
	require.NoError(t, err)
	require.NoError(t, store.Bootstrap(context.Background()))
	return store, keys
}

func TestNativeBootstrapCommitsFingerprintAndIndependentIdentityHead(t *testing.T) {
	pool, _, _ := nativeDatabase(t)
	store, _ := nativeStore(t, pool)
	var versions int
	var head uint64
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM identity_key_versions`).Scan(&versions))
	require.Equal(t, 1, versions)
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&head))
	require.EqualValues(t, 1, head)
	require.NoError(t, store.Bootstrap(context.Background()))
	var replayHead uint64
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&replayHead))
	require.Equal(t, head, replayHead)
}

func TestNativeRetainedNowStableWithoutAuthParent(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	store, keys := nativeStore(t, pool)
	require.NoError(t, store.ActivateGuards(context.Background()))
	prepared, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	var original Fact
	for i := 0; i < 2; i++ {
		tx, err := store.Begin(context.Background())
		require.NoError(t, err)
		fact, err := store.CaptureRetainedTx(context.Background(), tx, prepared)
		require.NoError(t, err)
		require.Equal(t, OriginRetainedNow, fact.Origin)
		require.Empty(t, fact.ParentHandle)
		require.NotEqual(t, uuid.Nil, fact.AssociationID)
		require.NoError(t, tx.Commit(context.Background()))
		if i == 0 {
			original = fact
		} else {
			require.Equal(t, original.AssociationID, fact.AssociationID)
			require.Equal(t, original.ReceiptID, fact.ReceiptID)
			require.Equal(t, original.ReceiptDigest, fact.ReceiptDigest)
		}
	}
	_, err = pool.Exec(context.Background(), `UPDATE profiles SET deleted_at=clock_timestamp(),frozen_at=clock_timestamp() WHERE id=$1`, profile)
	require.NoError(t, err)
	tx, err := store.Begin(context.Background())
	require.NoError(t, err)
	defer tx.Rollback(context.Background())
	fact, err := store.CaptureRetainedTx(context.Background(), tx, prepared)
	require.NoError(t, err)
	require.True(t, fact.Deleted)
	require.True(t, fact.Frozen)
	require.Equal(t, original.AssociationID, fact.AssociationID)
	require.Greater(t, fact.Revision, original.Revision)
	require.NoError(t, tx.Commit(context.Background()))
}

func TestNativeActivatedGuardRejectsUnpreparedBirthAndGUC(t *testing.T) {
	pool, _, _ := nativeDatabase(t)
	store, _ := nativeStore(t, pool)
	require.NoError(t, store.ActivateGuards(context.Background()))
	tx, err := pool.Begin(context.Background())
	require.NoError(t, err)
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(context.Background(), `SET LOCAL voice.identity_prepared='true'`)
	require.NoError(t, err)
	_, err = tx.Exec(context.Background(), `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'unprepared','4321','Unprepared')`, uuid.New(), uuid.New())
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "23514", pgErr.Code)
}

func TestNativeRetainedCaptureRollbackLeavesNoReservationOrJournalHole(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	store, keys := nativeStore(t, pool)
	ctx := context.Background()
	require.NoError(t, store.ActivateGuards(ctx))
	prepared, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	var before uint64
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&before))
	tx, err := store.Begin(ctx)
	require.NoError(t, err)
	first, err := store.CaptureRetainedTx(ctx, tx, prepared)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	var head uint64
	var associations, reservations int
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head,(SELECT count(*) FROM identity_associations),(SELECT count(*) FROM identity_profile_reservations) FROM identity_meta WHERE singleton`).Scan(&head, &associations, &reservations))
	require.Equal(t, before, head)
	require.Zero(t, associations)
	require.Zero(t, reservations)
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	second, err := store.CaptureRetainedTx(ctx, tx, prepared)
	require.NoError(t, err)
	require.Equal(t, before+1, second.Revision)
	require.NotEqual(t, first.AssociationID, second.AssociationID)
	require.NoError(t, tx.Commit(ctx))
}

func TestNativeLoneReservationNeverBecomesAssociationOrTerminal(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	store, keys := nativeStore(t, pool)
	ctx := context.Background()
	require.NoError(t, store.ActivateGuards(ctx))
	prepared, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	a := prepared.aliases[0]
	_, err = pool.Exec(ctx, `INSERT INTO identity_profile_reservations VALUES($1,$2)`, a.version, a.profile[:])
	require.NoError(t, err)
	tx, err := store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = store.CaptureRetainedTx(ctx, tx, prepared)
	require.ErrorIs(t, err, ErrConflict)
}

func TestNativeCanonicalFactCannotBeRewrittenOrReset(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	store, keys := nativeStore(t, pool)
	ctx := context.Background()
	require.NoError(t, store.ActivateGuards(ctx))
	prepared, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	tx, err := store.Begin(ctx)
	require.NoError(t, err)
	fact, err := store.CaptureRetainedTx(ctx, tx, prepared)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	for _, sql := range []string{
		`UPDATE identity_associations SET receipt_id=gen_random_uuid()`,
		`UPDATE identity_associations SET capture_origin='VERIFIED_HISTORY'`,
		`UPDATE identity_key_versions SET fingerprint=decode(repeat('01',32),'hex')`,
		`DELETE FROM identity_profile_reservations`,
		`UPDATE association_selector_aliases SET selector=decode(repeat('01',32),'hex')`,
		`UPDATE identity_journal SET fact_digest=decode(repeat('01',32),'hex')`,
		`DELETE FROM identity_journal`,
	} {
		_, err = pool.Exec(ctx, sql)
		require.Error(t, err, sql)
	}
	var receipt uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT receipt_id FROM identity_associations WHERE association_id=$1`, fact.AssociationID).Scan(&receipt))
	require.Equal(t, fact.ReceiptID, receipt)
}

func TestNativeBirthNeedsSealedParentAndAtomicProfile(t *testing.T) {
	pool, _, _ := nativeDatabase(t)
	account := uuid.New()
	store, keys := nativeStore(t, pool)
	ctx := context.Background()
	require.NoError(t, store.ActivateGuards(ctx))
	profile := uuid.New()
	prepared, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	tx, err := store.Begin(ctx)
	require.NoError(t, err)
	_, err = store.CreateBirthTx(ctx, tx, prepared, VerifiedParentProof{})
	require.ErrorIs(t, err, ErrIncomplete)
	require.NoError(t, tx.Rollback(ctx))
	proof := VerifiedParentProof{verified: true, observation: validParentForTest(account)}
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	fact, err := store.CreateBirthTx(ctx, tx, prepared, proof)
	require.NoError(t, err)
	require.Equal(t, OriginBirth, fact.Origin)
	require.Equal(t, proof.observation.OwnerHandle, fact.ParentHandle)
	_, err = tx.SQL().Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'newbirth','4321','New birth')`, profile, account)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	var association uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT identity_association_id FROM profiles WHERE id=$1`, profile).Scan(&association))
	require.Equal(t, fact.AssociationID, association)
}

func validParentForTest(account uuid.UUID) ParentObservation {
	now := time.Now().UTC()
	return ParentObservation{AccountID: account, OwnerHandle: "opaque-parent", LifecycleRevision: 1, State: ParentActive, Eligibility: ParentEligible, Disposition: ParentNotIrreversible, ObservedAt: now.Add(-time.Second), ValidUntil: now.Add(time.Minute), Cut: ParentCut{Owner: "auth", Stream: "lifetime", HistoryGeneration: "auth-generation", ThroughRevision: 1, Digest: [32]byte{1}, KeysetRevision: 1, RequiredVersions: []uint32{1}, Complete: true}}
}

func TestNativeBirthRejectsProofThatExpiresWhileWaitingForIdentityMutex(t *testing.T) {
	pool, _, _ := nativeDatabase(t)
	store, keys := nativeStore(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, store.ActivateGuards(ctx))
	account, profile := uuid.New(), uuid.New()
	prepared, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	fence := nativeFence(t, pool)
	holder, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer holder.Rollback(context.Background())
	var observed time.Time
	require.NoError(t, holder.QueryRow(ctx, `SELECT clock_timestamp() FROM identity_meta WHERE singleton FOR UPDATE`).Scan(&observed))
	observation := validParentForTest(account)
	observation.ObservedAt = observed.Add(-time.Second)
	observation.ValidUntil = observed.Add(250 * time.Millisecond)
	proof := VerifiedParentProof{verified: true, observation: observation}
	require.True(t, proof.validForBirth(observed))
	var before uint64
	require.NoError(t, holder.QueryRow(ctx, `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&before))
	waiter, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer waiter.Rollback(context.Background())
	returned := make(chan error, 1)
	go func() {
		cap, acquireErr := acquireIdentityTx(ctx, waiter, fence)
		if acquireErr != nil {
			returned <- acquireErr
			return
		}
		_, birthErr := store.CreateBirthTx(ctx, cap, prepared, proof)
		returned <- birthErr
	}()
	nativeWaitEdge(t, pool, waiter.Conn().PgConn().PID(), holder.Conn().PgConn().PID(), returned)
	_, err = holder.Exec(ctx, `SELECT pg_sleep(0.35)`)
	require.NoError(t, err)
	var expired bool
	require.NoError(t, holder.QueryRow(ctx, `SELECT clock_timestamp()>=$1`, observation.ValidUntil).Scan(&expired))
	require.True(t, expired)
	require.NoError(t, holder.Commit(ctx))
	require.ErrorIs(t, <-returned, ErrIncomplete, "pre-wait proof cannot authorize birth after DB expiry")
	require.NoError(t, waiter.Rollback(ctx))
	var after uint64
	var reservations, associations int
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head,(SELECT count(*) FROM identity_profile_reservations),(SELECT count(*) FROM identity_associations) FROM identity_meta WHERE singleton`).Scan(&after, &reservations, &associations))
	require.Equal(t, before, after)
	require.Zero(t, reservations)
	require.Zero(t, associations)
}

func TestNativeAttachParentIsExactRevisionCASAndKeepsCaptureOrigin(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	store, keys := nativeStore(t, pool)
	ctx := context.Background()
	require.NoError(t, store.ActivateGuards(ctx))
	prepared, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	tx, err := store.Begin(ctx)
	require.NoError(t, err)
	original, err := store.CaptureRetainedTx(ctx, tx, prepared)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	proof := VerifiedParentProof{verified: true, observation: validParentForTest(account)}
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	_, err = store.AttachParentTx(ctx, tx, prepared, original.Revision+1, proof)
	require.NoError(t, tx.Rollback(ctx))
	require.ErrorIs(t, err, ErrConflict)
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	attached, err := store.AttachParentTx(ctx, tx, prepared, original.Revision, proof)
	require.NoError(t, err)
	require.Equal(t, OriginRetainedNow, attached.Origin)
	require.Equal(t, original.ReceiptID, attached.ReceiptID)
	require.Equal(t, original.ReceiptDigest, attached.ReceiptDigest)
	require.Equal(t, proof.observation.OwnerHandle, attached.ParentHandle)
	require.Greater(t, attached.Revision, original.Revision)
	require.NoError(t, tx.Commit(ctx))
	proof.observation.OwnerHandle = "another-lifetime"
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = store.AttachParentTx(ctx, tx, prepared, attached.Revision, proof)
	require.ErrorIs(t, err, ErrConflict)
}

func TestNativeTerminalParentDominatesStaleBirthAndRawCapture(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	store, keys := nativeStore(t, pool)
	ctx := context.Background()
	require.NoError(t, store.ActivateGuards(ctx))
	p, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	tx, err := store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	before, err := store.CaptureRetainedTx(ctx, tx, p)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	terminal := validParentForTest(account)
	terminal.State = ParentTerminal
	terminal.Eligibility = ParentIneligible
	terminal.Disposition = ParentIrreversible
	terminal.CycleID = uuid.New()
	terminal.EraseAt = terminal.ObservedAt.Add(-time.Second)
	terminal.LifecycleRevision = 2
	terminal.Cut.ThroughRevision = 2
	terminal.Cut.Digest = [32]byte{2}
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	attached, err := store.AttachParentTx(ctx, tx, p, before.Revision, VerifiedParentProof{verified: true, observation: terminal})
	require.NoError(t, err)
	require.Equal(t, ParentTerminal, attached.ParentState)
	require.NoError(t, tx.Commit(ctx))
	newProfile, err := keys.PrepareSelectors(uuid.New(), account)
	require.NoError(t, err)
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = store.CreateBirthTx(ctx, tx, newProfile, VerifiedParentProof{verified: true, observation: validParentForTest(account)})
	require.ErrorIs(t, err, ErrConflict)
	require.NoError(t, tx.Rollback(ctx))
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = store.CaptureRetainedTx(ctx, tx, p)
	require.ErrorIs(t, err, ErrIncomplete, "known positive terminal cannot produce a fresh raw ownership certificate")
}

func TestNativeInactiveOverlayAdvancesIdentityWithoutClaimingTerminal(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	store, keys := nativeStore(t, pool)
	ctx := context.Background()
	require.NoError(t, store.ActivateGuards(ctx))
	p, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	tx, err := store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	before, err := store.CaptureRetainedTx(ctx, tx, p)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	_, err = pool.Exec(ctx, `INSERT INTO user_account_lifecycle(account_id,state,source_event_id,occurred_at) VALUES($1,'ACCOUNT_INACTIVE',$2,clock_timestamp())`, account, uuid.New())
	require.NoError(t, err)
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	after, err := store.CaptureRetainedTx(ctx, tx, p)
	require.NoError(t, err)
	require.True(t, after.AccountInactive)
	require.Greater(t, after.Revision, before.Revision)
	require.Empty(t, after.ParentState)
}

func TestNativeDowngradeOnlyAllowsPristineUninitializedCore(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "src", "backend", "migrations", "user_db")); err == nil {
			break
		}
		next := filepath.Dir(dir)
		require.NotEqual(t, dir, next)
		dir = next
	}
	down, err := os.ReadFile(filepath.Join(dir, "src", "backend", "migrations", "user_db", "000022_identity_association_core.down.sql"))
	require.NoError(t, err)
	t.Run("pristine", func(t *testing.T) {
		pool, _, _ := nativeDatabase(t)
		_, err := pool.Exec(context.Background(), string(down))
		require.NoError(t, err)
		var missing bool
		require.NoError(t, pool.QueryRow(context.Background(), `SELECT to_regclass('public.identity_meta') IS NULL`).Scan(&missing))
		require.True(t, missing)
	})
	t.Run("initialized", func(t *testing.T) {
		pool, _, _ := nativeDatabase(t)
		nativeStore(t, pool)
		_, err := pool.Exec(context.Background(), string(down))
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr)
		require.Equal(t, "55000", pgErr.Code)
		var head uint64
		require.NoError(t, pool.QueryRow(context.Background(), `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&head))
		require.EqualValues(t, 1, head)
	})
}

func TestNativeCatalogRejectsChangedOwnerGuardsAndNonreuseConstraints(t *testing.T) {
	for name, sql := range map[string]string{
		"wrong-version":                `UPDATE schema_migrations SET version=19`,
		"disabled-identity-gate":       `ALTER TABLE profiles DISABLE TRIGGER identity_profiles_gate`,
		"disabled-user19-clock":        `ALTER TABLE profiles DISABLE TRIGGER user_authority_profiles`,
		"changed-user19-body":          `CREATE OR REPLACE FUNCTION user_authority_changed() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END; $$`,
		"lost-profile-alias-unique":    `DROP INDEX association_unique_profile_alias`,
		"lost-reservation-primary-key": `ALTER TABLE identity_profile_reservations DROP CONSTRAINT identity_profile_reservations_pkey`,
	} {
		t.Run(name, func(t *testing.T) {
			pool, _, _ := nativeDatabase(t)
			fence := nativeFence(t, pool)
			_, err := pool.Exec(context.Background(), sql)
			require.NoError(t, err)
			tx, err := BeginIdentityTx(context.Background(), pool, fence)
			if tx != nil {
				require.NoError(t, tx.Rollback(context.Background()))
			}
			require.Error(t, err, "changed authority catalog must fail before domain mutation")
		})
	}
}

func TestNativePositiveTerminalSurvivesRawCleanupAndPreventsUUIDReuse(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	store, keys := nativeStore(t, pool)
	ctx := context.Background()
	require.NoError(t, store.ActivateGuards(ctx))
	p, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	lookup, err := keys.PrepareProfileLookup(profile)
	require.NoError(t, err)
	tx, err := store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	fact, err := store.CaptureRetainedTx(ctx, tx, p)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	parent := validParentForTest(account)
	parent.State = ParentTerminal
	parent.Eligibility = ParentIneligible
	parent.Disposition = ParentIrreversible
	parent.CycleID = uuid.New()
	parent.EraseAt = parent.ObservedAt.Add(-time.Second)
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	terminal, err := store.AttachParentTx(ctx, tx, p, fact.Revision, VerifiedParentProof{verified: true, observation: parent})
	require.NoError(t, err)
	require.NotNil(t, terminal.Terminal, "actual User terminal event/receipt must exist before cleanup")
	require.NotEqual(t, uuid.Nil, terminal.Terminal.OperationID)
	require.NotEqual(t, uuid.Nil, terminal.Terminal.ReceiptID)
	require.NoError(t, tx.Commit(ctx))
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.NoError(t, store.PrepareTerminalDeleteTx(ctx, tx, p, terminal.Revision))
	_, err = tx.SQL().Exec(ctx, `DELETE FROM profiles WHERE id=$1`, profile)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	after, err := store.ReadByProfileSelectorsTx(ctx, tx, lookup)
	require.NoError(t, err)
	require.Equal(t, terminal.Terminal, after.Terminal)
	require.NoError(t, tx.Commit(ctx))
	otherAccount := uuid.New()
	reused, err := keys.PrepareSelectors(profile, otherAccount)
	require.NoError(t, err)
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = store.CreateBirthTx(ctx, tx, reused, VerifiedParentProof{verified: true, observation: validParentForTest(otherAccount)})
	require.ErrorIs(t, err, ErrConflict)
}

func TestNativeKnownParentTerminalAlsoFencesOtherUnresolvedRetainedProfile(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	other := uuid.New()
	_, err := pool.Exec(context.Background(), `INSERT INTO profiles(id,account_id,username,discriminator,display_name,is_primary) VALUES($1,$2,'otherfixture','4321','Other retained',false)`, other, account)
	require.NoError(t, err)
	store, keys := nativeStore(t, pool)
	ctx := context.Background()
	require.NoError(t, store.ActivateGuards(ctx))
	first, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	second, err := keys.PrepareSelectors(other, account)
	require.NoError(t, err)
	tx, err := store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = store.CaptureRetainedTx(ctx, tx, first)
	require.NoError(t, err)
	secondFact, err := store.CaptureRetainedTx(ctx, tx, second)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	parent := validParentForTest(account)
	parent.State = ParentTerminal
	parent.Eligibility = ParentIneligible
	parent.Disposition = ParentIrreversible
	parent.CycleID = uuid.New()
	parent.EraseAt = parent.ObservedAt.Add(-time.Second)
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = store.AttachParentTx(ctx, tx, second, secondFact.Revision, VerifiedParentProof{verified: true, observation: parent})
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = store.CaptureRetainedTx(ctx, tx, first)
	require.ErrorIs(t, err, ErrIncomplete, "positive account terminal fence cannot be bypassed through unresolved parent")
}

func TestNativeCaptureCompleteCohortBeyondHundred(t *testing.T) {
	pool, first, _ := nativeDatabase(t)
	profiles := []uuid.UUID{first}
	var added, accounts []uuid.UUID
	for i := 0; i < 120; i++ {
		id := uuid.New()
		profiles = append(profiles, id)
		added = append(added, id)
		accounts = append(accounts, uuid.New())
	}
	_, err := pool.Exec(context.Background(), `INSERT INTO profiles(id,account_id,username,discriminator,display_name) SELECT p,a,'bulk'||substr(replace(p::text,'-',''),1,16),'1234','Bulk capture' FROM unnest($1::uuid[],$2::uuid[]) candidate(p,a)`, added, accounts)
	require.NoError(t, err)
	store, _ := nativeStore(t, pool)
	require.NoError(t, store.ActivateGuards(context.Background()))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	firstResult, err := store.CaptureProfiles(ctx, profiles)
	require.NoError(t, err)
	require.Len(t, firstResult.Observations, 121)
	require.NotEqual(t, firstResult.Cut.UserAuthorityRevision, firstResult.Cut.ThroughRevision, "A and H are independent clocks")
	for _, id := range profiles {
		observation, ok := firstResult.Observations[id]
		require.True(t, ok)
		require.Equal(t, CaptureAssociated, observation.Status)
		require.NotEqual(t, uuid.Nil, observation.AccountID)
		require.NotEqual(t, uuid.Nil, observation.Fact.AssociationID)
		require.LessOrEqual(t, observation.Fact.Revision, firstResult.Cut.ThroughRevision)
	}
	second, err := store.CaptureProfiles(context.Background(), profiles)
	require.NoError(t, err)
	require.Equal(t, firstResult.Cut, second.Cut)
	for id, observation := range firstResult.Observations {
		require.Equal(t, observation.Fact.AssociationID, second.Observations[id].Fact.AssociationID)
		require.Equal(t, observation.Fact.ReceiptDigest, second.Observations[id].Fact.ReceiptDigest)
	}
}
