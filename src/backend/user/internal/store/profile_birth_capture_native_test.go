package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/user/internal/identityassociation"
	"voice/backend/user/internal/testsupport/associationfixture"
)

type birthParentAuthority struct {
	handle string
	pool   *pgxpool.Pool
}

type unavailableBirthParent struct{}

func (unavailableBirthParent) ObserveParent(context.Context, uuid.UUID) (identityassociation.ParentObservation, error) {
	return identityassociation.ParentObservation{}, errors.New("fixture parent unavailable")
}

func (a birthParentAuthority) ObserveParent(ctx context.Context, account uuid.UUID) (identityassociation.ParentObservation, error) {
	// In this uncontended fixture, taking I NOWAIT through another connection
	// proves the production wrapper has not taken I before its authority call.
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return identityassociation.ParentObservation{}, err
	}
	_, err = tx.Exec(ctx, `SELECT singleton FROM identity_meta WHERE singleton FOR UPDATE NOWAIT`)
	_ = tx.Rollback(ctx)
	if err != nil {
		return identityassociation.ParentObservation{}, err
	}
	now := time.Now().UTC()
	return identityassociation.ParentObservation{
		AccountID: account, OwnerHandle: a.handle, LifecycleRevision: 1,
		State: identityassociation.ParentActive, Eligibility: identityassociation.ParentEligible,
		Disposition: identityassociation.ParentNotIrreversible,
		ObservedAt:  now.Add(-time.Millisecond), ValidUntil: now.Add(time.Minute),
		Cut: identityassociation.ParentCut{Owner: "auth", Stream: "fixture-auth-stream", HistoryGeneration: "fixture-auth-generation", ThroughRevision: 1, Digest: [32]byte{1}, KeysetRevision: 1, RequiredVersions: []uint32{1}, Complete: true},
	}, nil
}

func capturedBirthFixture(t *testing.T, guards bool) (associationfixture.Database, *ProfileStore, birthParentAuthority) {
	t.Helper()
	db := associationfixture.New(t, 1)
	receipt, err := associationfixture.ReadInstallerReceipt(db.InstallerReceiptPath)
	require.NoError(t, err)
	keys, err := identityassociation.NewKeyset(identityassociation.KeysetSnapshot{Revision: 1, ActiveVersion: receipt.KeyVersion, Versions: map[uint32][32]byte{receipt.KeyVersion: receipt.KeyFingerprint}}, map[uint32][]byte{receipt.KeyVersion: receipt.UserKey})
	require.NoError(t, err)
	ctx := context.Background()
	identities, err := identityassociation.NewFreshStore(ctx, db.Pool, keys, identityassociation.FreshSourcePin{HistoryGeneration: receipt.HistoryGeneration, UserSourceGeneration: receipt.UserSourceGeneration})
	require.NoError(t, err)
	if guards {
		require.NoError(t, identities.ActivateGuards(ctx))
	}
	parent := birthParentAuthority{handle: "fixture-auth-parent", pool: db.Pool}
	verifier, err := identityassociation.NewParentVerifier(parent)
	require.NoError(t, err)
	profiles, err := NewProfileStoreWithBirthCapture(db.Pool, identities, verifier)
	require.NoError(t, err)
	return db, profiles, parent
}

func TestSecondaryProfileBirthCaptureAtomic(t *testing.T) {
	db, profiles, parent := capturedBirthFixture(t, true)
	ctx := context.Background()
	account := db.Accounts[db.Profiles[0]]
	created, err := profiles.CreateSecondaryProfile(ctx, account, "Atomic secondary", nil, nil)
	require.NoError(t, err, "guarded production insert must share canonical BIRTH preparation")
	var origin, handle string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT a.capture_origin,a.parent_handle FROM profiles p JOIN identity_associations a ON a.association_id=p.identity_association_id WHERE p.id=$1 AND p.account_id=$2`, created.ID, account).Scan(&origin, &handle))
	require.Equal(t, "BIRTH", origin)
	require.Equal(t, parent.handle, handle)
	var reservations, journal, outbox int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM identity_profile_reservations),(SELECT count(*) FROM user_profile_search_journal WHERE profile_id=$1),(SELECT count(*) FROM user_profile_search_outbox)`, created.ID).Scan(&reservations, &journal, &outbox))
	require.Equal(t, 1, reservations)
	require.Equal(t, 1, journal)
	require.Equal(t, 1, outbox)
}

func birthCaptureState(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var state string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT jsonb_build_object(
	 'profiles',(SELECT count(*) FROM profiles),
	 'associations',(SELECT count(*) FROM identity_associations),
	 'reservations',(SELECT count(*) FROM identity_profile_reservations),
	 'births',(SELECT count(*) FROM identity_journal WHERE kind='BIRTH'),
	 'search',(SELECT count(*) FROM user_profile_search_journal),
	 'outbox',(SELECT count(*) FROM user_profile_search_outbox),
	 'fences',(SELECT count(*) FROM identity_parent_fences),
	 'aliases',(SELECT count(*) FROM identity_account_parent_aliases),
	 'head',history_head,'digest',encode(head_digest,'hex'))::text FROM identity_meta WHERE singleton`).Scan(&state))
	return state
}

func TestSecondaryProfileBirthCaptureOutboxFailureRollsBack(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	before := birthCaptureState(t, db.Pool)
	_, err := db.Pool.Exec(ctx, `CREATE FUNCTION fail_birth_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture outbox append failed'; END; $$;
	 CREATE TRIGGER fail_birth_outbox BEFORE INSERT ON user_profile_search_outbox FOR EACH ROW EXECUTE FUNCTION fail_birth_outbox()`)
	require.NoError(t, err)
	created, err := profiles.CreateSecondaryProfile(ctx, db.Accounts[db.Profiles[0]], "Must roll back", nil, nil)
	require.ErrorContains(t, err, "fixture outbox append failed")
	require.Nil(t, created)
	require.Equal(t, before, birthCaptureState(t, db.Pool), "BIRTH, parent fence, reservation, profile and both search effects share rollback")
}

func TestSecondaryProfileBirthCaptureDoesNotAllowMixedWriter(t *testing.T) {
	db, _, _ := capturedBirthFixture(t, true)
	before := birthCaptureState(t, db.Pool)
	_, err := NewProfileStore(db.Pool).CreateSecondaryProfile(context.Background(), db.Accounts[db.Profiles[0]], "Unqualified writer", nil, nil)
	require.ErrorContains(t, err, "canonical birth preparation")
	require.Equal(t, before, birthCaptureState(t, db.Pool))
}

func TestSecondaryProfileBirthCapturePreservesLegacyMode(t *testing.T) {
	db, _, _ := capturedBirthFixture(t, false)
	created, err := NewProfileStore(db.Pool).CreateSecondaryProfile(context.Background(), db.Accounts[db.Profiles[0]], "Legacy writer", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, created)
	var births int
	require.NoError(t, db.Pool.QueryRow(context.Background(), `SELECT count(*) FROM identity_journal WHERE kind='BIRTH'`).Scan(&births))
	require.Zero(t, births, "legacy mode is not labeled canonical capture")
}

func TestSecondaryProfileBirthCaptureRejectsDifferentPool(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	other, err := pgxpool.NewWithConfig(context.Background(), db.Pool.Config())
	require.NoError(t, err)
	t.Cleanup(other.Close)
	_, err = NewProfileStoreWithBirthCapture(other, profiles.birthCapture.identities, profiles.birthCapture.parents)
	require.ErrorIs(t, err, identityassociation.ErrIncomplete)
}

func TestSecondaryProfileBirthCaptureDoesNotFallbackWhenAuthorityFails(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, false)
	verifier, err := identityassociation.NewParentVerifier(unavailableBirthParent{})
	require.NoError(t, err)
	configured, err := NewProfileStoreWithBirthCapture(db.Pool, profiles.birthCapture.identities, verifier)
	require.NoError(t, err)
	before := birthCaptureState(t, db.Pool)
	_, err = configured.CreateSecondaryProfile(context.Background(), db.Accounts[db.Profiles[0]], "No fallback", nil, nil)
	require.ErrorIs(t, err, identityassociation.ErrIncomplete)
	require.Equal(t, before, birthCaptureState(t, db.Pool), "unavailable authority cannot select the legacy INSERT path")
}

func TestSecondaryProfileBirthCaptureRetriesWholeTransaction(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	// The sequence is a fault-injection attempt counter only. Production owner
	// revisions remain transactional; the first error must roll back its BIRTH.
	_, err := db.Pool.Exec(ctx, `CREATE SEQUENCE birth_retry_attempts;
	 CREATE FUNCTION retry_birth_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
	 IF nextval('birth_retry_attempts')=1 THEN RAISE EXCEPTION 'fixture unique conflict' USING ERRCODE='23505'; END IF;
	 RETURN NEW; END; $$;
	 CREATE TRIGGER retry_birth_outbox BEFORE INSERT ON user_profile_search_outbox FOR EACH ROW EXECUTE FUNCTION retry_birth_outbox()`)
	require.NoError(t, err)
	created, err := profiles.CreateSecondaryProfile(ctx, db.Accounts[db.Profiles[0]], "Retried secondary", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, created)
	var attempts, profilesCount, associations, reservations, births, journal, outbox int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT
	 (SELECT last_value FROM birth_retry_attempts),(SELECT count(*) FROM profiles),
	 (SELECT count(*) FROM identity_associations),(SELECT count(*) FROM identity_profile_reservations),
	 (SELECT count(*) FROM identity_journal WHERE kind='BIRTH'),
	 (SELECT count(*) FROM user_profile_search_journal),(SELECT count(*) FROM user_profile_search_outbox)`).Scan(&attempts, &profilesCount, &associations, &reservations, &births, &journal, &outbox))
	require.Equal(t, 2, attempts)
	require.Equal(t, 2, profilesCount) // one retained fixture profile, one birth
	for _, count := range []int{associations, reservations, births, journal, outbox} {
		require.Equal(t, 1, count)
	}
}
