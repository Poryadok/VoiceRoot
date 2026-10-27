package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/user/internal/identityassociation"
)

func TestPrimaryProfileBirthCaptureAtomic(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account, id := uuid.New(), uuid.New()
	created, err := profiles.EnsurePrimaryProfile(ctx, account, &id, "Atomic primary", true)
	require.NoError(t, err, "guarded primary INSERT must share canonical BIRTH preparation")
	require.Equal(t, id, created.ID)
	require.True(t, created.IsPrimary)
	var births, onboarding, journal, outbox int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM identity_journal WHERE kind='BIRTH'),
 (SELECT count(*) FROM onboarding_state WHERE profile_id=$1),
 (SELECT count(*) FROM user_profile_search_journal WHERE profile_id=$1),
 (SELECT count(*) FROM user_profile_search_outbox)`, id).Scan(&births, &onboarding, &journal, &outbox))
	require.Equal(t, 1, births)
	require.Equal(t, 1, onboarding)
	require.Equal(t, 1, journal)
	require.Equal(t, 1, outbox)
}

func TestPrimaryProfileBirthCaptureIdempotent(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account, id, other := uuid.New(), uuid.New(), uuid.New()
	first, err := profiles.EnsurePrimaryProfile(ctx, account, &id, "Original primary", true)
	require.NoError(t, err)
	before := birthCaptureState(t, db.Pool)
	again, err := profiles.EnsurePrimaryProfile(ctx, account, &other, "Do not replace", false)
	require.NoError(t, err)
	require.Equal(t, first.ID, again.ID)
	require.Equal(t, first.DisplayName, again.DisplayName)
	require.Equal(t, before, birthCaptureState(t, db.Pool), "repeat bootstrap does not append identity or search facts")
	var onboarding int
	var guest bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM onboarding_state WHERE profile_id=$1),is_guest_account FROM profiles WHERE id=$1`, id).Scan(&onboarding, &guest))
	require.Equal(t, 1, onboarding)
	require.True(t, guest)
}

func TestPrimaryProfileBirthCaptureOutboxFailureRollsBack(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	before := birthCaptureState(t, db.Pool)
	_, err := db.Pool.Exec(ctx, `CREATE FUNCTION fail_primary_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture primary outbox failed'; END; $$;
	 CREATE TRIGGER fail_primary_outbox BEFORE INSERT ON user_profile_search_outbox FOR EACH ROW EXECUTE FUNCTION fail_primary_outbox()`)
	require.NoError(t, err)
	account, id := uuid.New(), uuid.New()
	created, err := profiles.EnsurePrimaryProfile(ctx, account, &id, "Must roll back", false)
	require.ErrorContains(t, err, "fixture primary outbox failed")
	require.Nil(t, created)
	require.Equal(t, before, birthCaptureState(t, db.Pool))
	var onboarding int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM onboarding_state WHERE profile_id=$1`, id).Scan(&onboarding))
	require.Zero(t, onboarding, "onboarding shares the birth/profile/search rollback")
}

type concurrentPrimaryParent struct {
	parent  birthParentAuthority
	ready   chan struct{}
	release chan struct{}
}

func (a concurrentPrimaryParent) ObserveParent(ctx context.Context, account uuid.UUID) (identityassociation.ParentObservation, error) {
	proof, err := a.parent.ObserveParent(ctx, account)
	if err != nil {
		return proof, err
	}
	a.ready <- struct{}{}
	select {
	case <-a.release:
		return proof, nil
	case <-ctx.Done():
		return identityassociation.ParentObservation{}, ctx.Err()
	}
}

func TestPrimaryProfileBirthCaptureConcurrentEnsure(t *testing.T) {
	db, profiles, parent := capturedBirthFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	barrier := concurrentPrimaryParent{parent, make(chan struct{}, 2), make(chan struct{})}
	verifier, err := identityassociation.NewParentVerifier(barrier)
	require.NoError(t, err)
	profiles, err = NewProfileStoreWithBirthCapture(db.Pool, profiles.birthCapture.identities, verifier)
	require.NoError(t, err)
	type result struct {
		profile *ProfileRow
		err     error
	}
	results := make(chan result, 2)
	account := uuid.New()
	for i := 0; i < 2; i++ {
		go func() {
			p, e := profiles.EnsurePrimaryProfile(ctx, account, nil, "Concurrent primary", false)
			results <- result{p, e}
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-barrier.ready:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	close(barrier.release)
	first, second := <-results, <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.Equal(t, first.profile.ID, second.profile.ID)
	var profilesCount, births, onboarding, journal, outbox int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM profiles WHERE account_id=$1),
	 (SELECT count(*) FROM identity_journal WHERE kind='BIRTH'),
	 (SELECT count(*) FROM onboarding_state WHERE profile_id=$2),
	 (SELECT count(*) FROM user_profile_search_journal WHERE profile_id=$2),
	 (SELECT count(*) FROM user_profile_search_outbox)`, account, first.profile.ID).Scan(&profilesCount, &births, &onboarding, &journal, &outbox))
	for _, count := range []int{profilesCount, births, onboarding, journal, outbox} {
		require.Equal(t, 1, count)
	}
}
