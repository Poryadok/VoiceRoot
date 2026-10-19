package identityassociation

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func dueParentForTest(account uuid.UUID) VerifiedParentProof {
	parent := validParentForTest(account)
	parent.State = ParentTerminal
	parent.Eligibility = ParentIneligible
	parent.Disposition = ParentIrreversible
	parent.CycleID = uuid.New()
	parent.EraseAt = parent.ObservedAt.Add(-time.Second)
	return VerifiedParentProof{verified: true, observation: parent}
}

func TestNativeDueRawEraseWithoutSelectorsIssuesIdentityFreeGap(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	store, keys := nativeStore(t, pool)
	ctx := context.Background()
	other, otherAccount := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'unaffected','4321','Unaffected')`, other, otherAccount)
	require.NoError(t, err)
	require.NoError(t, store.ActivateGuards(ctx))
	fence := nativeFence(t, pool)
	var before uint64
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&before))
	// Preparation and cleanup need source metadata and sealed owner authority,
	// but take no Keyset or Store dependency.
	tx, err := BeginIdentityTx(ctx, pool, fence)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.NoError(t, PrepareDueRawEraseTx(ctx, tx, profile, dueParentForTest(account)))
	require.NoError(t, tx.Commit(ctx), "preparation alone is not an erasure")
	var after uint64
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&after))
	require.Equal(t, before, after)
	tx, err = BeginIdentityTx(ctx, pool, fence)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.NoError(t, PrepareDueRawEraseTx(ctx, tx, profile, dueParentForTest(account)))
	_, err = tx.SQL().Exec(ctx, `DELETE FROM profiles WHERE id=$1`, profile)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	var issued, covered uint64
	var gaps, associations, reservations int
	var minimized bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head,unresolved_erase_issued,unresolved_erase_covered,
	 (SELECT count(*) FROM identity_journal WHERE kind='UNRESOLVED_RAW_ERASURE'),
	 (SELECT count(*) FROM identity_associations),(SELECT count(*) FROM identity_profile_reservations),
	 (SELECT bool_and(association_id IS NULL AND association_revision IS NULL AND deleted IS NULL AND frozen IS NULL AND account_inactive IS NULL
	 AND fact_digest=sha256(convert_to('voice.user.identity-association.unresolved-raw-erasure.v1','UTF8') || uuid_send(m.history_generation) || int8send(j.revision) || convert_to('MISSING_REQUIRED_SELECTOR_COVERAGE','UTF8') || int8send(1))) FROM identity_journal j WHERE kind='UNRESOLVED_RAW_ERASURE')
	 FROM identity_meta m WHERE singleton`).Scan(&after, &issued, &covered, &gaps, &associations, &reservations, &minimized))
	require.Equal(t, before+1, after)
	require.Equal(t, after, issued)
	require.Zero(t, covered)
	require.Equal(t, 1, gaps)
	require.Zero(t, associations)
	require.Zero(t, reservations)
	require.True(t, minimized)
	newProfile, newAccount := uuid.New(), uuid.New()
	prepared, err := keys.PrepareSelectors(newProfile, newAccount)
	require.NoError(t, err)
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = store.CreateBirthTx(ctx, tx, prepared, VerifiedParentProof{verified: true, observation: validParentForTest(newAccount)})
	require.ErrorIs(t, err, ErrIncomplete, "random new UUID does not bypass lost namespace coverage")
	require.NoError(t, tx.Rollback(ctx))
	_, err = pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'bypass','9999','Bypass')`, newProfile, newAccount)
	require.Error(t, err)
	observed, err := store.CaptureProfiles(ctx, []uuid.UUID{profile, other})
	require.NoError(t, err)
	require.Equal(t, CaptureIncomplete, observed.Observations[profile].Status)
	require.Equal(t, CaptureAssociated, observed.Observations[other].Status, "unrelated guarded retained NOW remains truthful")
	tx, err = BeginIdentityTx(ctx, pool, fence)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.ErrorIs(t, PrepareDueRawEraseTx(ctx, tx, profile, dueParentForTest(account)), ErrUnknownProfile)
	require.NoError(t, tx.Rollback(ctx))
}

func TestNativeDueRawEraseRejectsUnprovedAuthorityAndExactMarkerMismatch(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	store, keys := nativeStore(t, pool)
	ctx := context.Background()
	other, otherAccount := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'markerother','5432','Other')`, other, otherAccount)
	require.NoError(t, err)
	require.NoError(t, store.ActivateGuards(ctx))
	fence := nativeFence(t, pool)
	for name, mutate := range map[string]func(*VerifiedParentProof){
		"unsealed":       func(p *VerifiedParentProof) { p.verified = false },
		"wrong-account":  func(p *VerifiedParentProof) { p.observation.AccountID = uuid.New() },
		"expired":        func(p *VerifiedParentProof) { p.observation.ValidUntil = time.Now().Add(-time.Second) },
		"incomplete-cut": func(p *VerifiedParentProof) { p.observation.Cut.Complete = false },
		"active":         func(p *VerifiedParentProof) { p.observation = validParentForTest(account) },
		"recoverable-past-deadline": func(p *VerifiedParentProof) {
			p.observation.State = ParentRecoverable
			p.observation.Disposition = ParentNotIrreversible
		},
	} {
		t.Run(name, func(t *testing.T) {
			proof := dueParentForTest(account)
			mutate(&proof)
			tx, err := BeginIdentityTx(ctx, pool, fence)
			require.NoError(t, err)
			defer tx.Rollback(ctx)
			require.Error(t, PrepareDueRawEraseTx(ctx, tx, profile, proof))
			require.NoError(t, tx.Rollback(ctx))
		})
	}
	t.Run("other-profile", func(t *testing.T) {
		tx, err := BeginIdentityTx(ctx, pool, fence)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		require.NoError(t, PrepareDueRawEraseTx(ctx, tx, profile, dueParentForTest(account)))
		_, err = tx.SQL().Exec(ctx, `DELETE FROM profiles WHERE id=$1`, other)
		require.Error(t, err)
		require.NoError(t, tx.Rollback(ctx))
	})
	t.Run("nullable-association-is-exact", func(t *testing.T) {
		tx, err := BeginIdentityTx(ctx, pool, fence)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		require.NoError(t, PrepareDueRawEraseTx(ctx, tx, profile, dueParentForTest(account)))
		prepared, err := keys.PrepareSelectors(profile, account)
		require.NoError(t, err)
		_, err = store.CaptureRetainedTx(ctx, tx, prepared)
		require.NoError(t, err)
		_, err = tx.SQL().Exec(ctx, `DELETE FROM profiles WHERE id=$1`, profile)
		require.Error(t, err, "NULL preparation is not a wildcard for a later association")
		require.NoError(t, tx.Rollback(ctx))
	})
	var count, gaps int
	require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM profiles),(SELECT count(*) FROM identity_journal WHERE kind='UNRESOLVED_RAW_ERASURE')`).Scan(&count, &gaps))
	require.Equal(t, 2, count)
	require.Zero(t, gaps)
}

func TestNativeDueRawEraseRechecksDeleteExpiryAndRollsBackParticipantFailure(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	store, _ := nativeStore(t, pool)
	ctx := context.Background()
	require.NoError(t, store.ActivateGuards(ctx))
	fence := nativeFence(t, pool)
	var before uint64
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&before))
	t.Run("expires-after-prepare", func(t *testing.T) {
		tx, err := BeginIdentityTx(ctx, pool, fence)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		proof := dueParentForTest(account)
		var now time.Time
		require.NoError(t, tx.SQL().QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now))
		proof.observation.ValidUntil = now.Add(250 * time.Millisecond)
		require.NoError(t, PrepareDueRawEraseTx(ctx, tx, profile, proof))
		_, err = tx.SQL().Exec(ctx, `SELECT pg_sleep(0.35)`)
		require.NoError(t, err)
		_, err = tx.SQL().Exec(ctx, `DELETE FROM profiles WHERE id=$1`, profile)
		require.Error(t, err)
		require.NoError(t, tx.Rollback(ctx))
	})
	t.Run("participant-failure", func(t *testing.T) {
		_, err := pool.Exec(ctx, `CREATE TABLE due_erase_participant(profile_id uuid REFERENCES profiles(id))`)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO due_erase_participant VALUES($1)`, profile)
		require.NoError(t, err)
		tx, err := BeginIdentityTx(ctx, pool, fence)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		require.NoError(t, PrepareDueRawEraseTx(ctx, tx, profile, dueParentForTest(account)))
		_, err = tx.SQL().Exec(ctx, `DELETE FROM profiles WHERE id=$1`, profile)
		require.Error(t, err, "FK failure after gap allocation rolls back the whole operation")
		require.NoError(t, tx.Rollback(ctx))
	})
	var after, issued, covered uint64
	var raw, gaps int
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head,unresolved_erase_issued,unresolved_erase_covered,(SELECT count(*) FROM profiles),(SELECT count(*) FROM identity_journal WHERE kind='UNRESOLVED_RAW_ERASURE') FROM identity_meta WHERE singleton`).Scan(&after, &issued, &covered, &raw, &gaps))
	require.Equal(t, before, after)
	require.Zero(t, issued)
	require.Zero(t, covered)
	require.Equal(t, 1, raw)
	require.Zero(t, gaps)
}

func TestNativeDueRawEraseNeedsNoRuntimeKeysetAndGapCannotBeReset(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	ctx := context.Background()
	// Deliberately never construct a Keyset or Store, bootstrap keys or activate
	// capture/birth guards. The cleanup-only capability still uses the real
	// pinned schema/generation and I-first transaction.
	tx, err := BeginIdentityTx(ctx, pool, nativeFence(t, pool))
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.NoError(t, PrepareDueRawEraseTx(ctx, tx, profile, dueParentForTest(account)))
	_, err = tx.SQL().Exec(ctx, `DELETE FROM profiles WHERE id=$1`, profile)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	for name, sql := range map[string]string{
		"rewind-issued":      `UPDATE identity_meta SET unresolved_erase_issued=0`,
		"unverified-closure": `UPDATE identity_meta SET unresolved_erase_covered=unresolved_erase_issued`,
		"generation-reset":   `UPDATE identity_meta SET history_generation=gen_random_uuid()`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := pool.Exec(ctx, sql)
			require.Error(t, err)
		})
	}
	_, err = pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'precutbypass','9999','Bypass')`, uuid.New(), uuid.New())
	require.Error(t, err, "legacy inactive guard mode cannot bypass an issued namespace gap")
	var issued, covered uint64
	var keys, remaining int
	require.NoError(t, pool.QueryRow(ctx, `SELECT unresolved_erase_issued,unresolved_erase_covered,(SELECT count(*) FROM identity_key_versions),(SELECT count(*) FROM profiles) FROM identity_meta WHERE singleton`).Scan(&issued, &covered, &keys, &remaining))
	require.EqualValues(t, 1, issued)
	require.Zero(t, covered)
	require.Zero(t, keys)
	require.Zero(t, remaining)
}

func TestNativeDueRawEraseRejectsProofExpiredDuringIdentityWait(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	fence := nativeFence(t, pool)
	holder, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer holder.Rollback(context.Background())
	var observed time.Time
	require.NoError(t, holder.QueryRow(ctx, `SELECT clock_timestamp() FROM identity_meta WHERE singleton FOR UPDATE`).Scan(&observed))
	proof := dueParentForTest(account)
	proof.observation.ObservedAt = observed.Add(-time.Second)
	proof.observation.ValidUntil = observed.Add(250 * time.Millisecond)
	require.True(t, proof.validAt(observed))
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
		returned <- PrepareDueRawEraseTx(ctx, cap, profile, proof)
	}()
	nativeWaitEdge(t, pool, waiter.Conn().PgConn().PID(), holder.Conn().PgConn().PID(), returned)
	_, err = holder.Exec(ctx, `SELECT pg_sleep(0.35)`)
	require.NoError(t, err)
	require.NoError(t, holder.Commit(ctx))
	require.ErrorIs(t, <-returned, ErrIncomplete)
	require.NoError(t, waiter.Rollback(ctx))
	var raw, gaps int
	require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM profiles),(SELECT count(*) FROM identity_journal WHERE kind='UNRESOLVED_RAW_ERASURE')`).Scan(&raw, &gaps))
	require.Equal(t, 1, raw)
	require.Zero(t, gaps)
}

func TestNativeUnresolvedGapRejectsSuppliedIdentityCommitments(t *testing.T) {
	pool, _, _ := nativeDatabase(t)
	ctx := context.Background()
	for name, query := range map[string]string{
		"caller-fact":      `SELECT identity_append_event(NULL,NULL,'UNRESOLVED_RAW_ERASURE',NULL,NULL,NULL,decode(repeat('ab',32),'hex'))`,
		"association-link": `SELECT identity_append_event(gen_random_uuid(),1,'UNRESOLVED_RAW_ERASURE',NULL,NULL,NULL,decode(repeat('ab',32),'hex'))`,
		"lifecycle-fact":   `SELECT identity_append_event(NULL,NULL,'UNRESOLVED_RAW_ERASURE',false,NULL,NULL)`,
	} {
		t.Run(name, func(t *testing.T) {
			tx, err := BeginIdentityTx(ctx, pool, nativeFence(t, pool))
			require.NoError(t, err)
			defer tx.Rollback(ctx)
			_, err = tx.SQL().Exec(ctx, query)
			require.Error(t, err, "a minimized gap cannot accept an identity commitment or lifecycle tuple")
			require.NoError(t, tx.Rollback(ctx))
		})
	}
}
