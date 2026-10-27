package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/user/internal/identityassociation"
)

func TestProfileRetirementCaptureWaitsForIdentityOwner(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	account := db.Accounts[db.Profiles[0]]
	profile, err := profiles.CreateSecondaryProfile(ctx, account, "Archive after owner", nil, nil)
	require.NoError(t, err)
	owner, err := profiles.birthCapture.identities.Begin(ctx)
	require.NoError(t, err)
	defer owner.Rollback(context.Background())
	result := make(chan error, 1)
	go func() { result <- profiles.SoftDeleteProfile(ctx, account, profile.ID) }()
	select {
	case err := <-result:
		t.Fatalf("configured archive must wait for the identity owner, not finish early: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, owner.Commit(ctx))
	require.NoError(t, <-result)
	var deleted bool
	var lifecycle, searchEvents int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT a.deleted,
	 (SELECT count(*) FROM identity_journal WHERE association_id=a.association_id AND kind='LIFECYCLE'),
	 (SELECT count(*) FROM user_profile_search_journal WHERE profile_id=p.id)
	 FROM profiles p JOIN identity_associations a ON a.association_id=p.identity_association_id WHERE p.id=$1 AND p.deleted_at IS NOT NULL`, profile.ID).Scan(&deleted, &lifecycle, &searchEvents))
	require.True(t, deleted)
	require.Equal(t, 1, lifecycle)
	require.Equal(t, 2, searchEvents) // original birth upsert, then archive delete
}

func TestProfileRetirementCaptureOutboxFailureRollsBack(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account := db.Accounts[db.Profiles[0]]
	profile, err := profiles.CreateSecondaryProfile(ctx, account, "Rollback archive", nil, nil)
	require.NoError(t, err)
	before := birthCaptureState(t, db.Pool)
	_, err = db.Pool.Exec(ctx, `CREATE FUNCTION fail_archive_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture archive outbox failed'; END; $$;
	 CREATE TRIGGER fail_archive_outbox BEFORE INSERT ON user_profile_search_outbox FOR EACH ROW EXECUTE FUNCTION fail_archive_outbox()`)
	require.NoError(t, err)
	require.ErrorContains(t, profiles.SoftDeleteProfile(ctx, account, profile.ID), "fixture archive outbox failed")
	require.Equal(t, before, birthCaptureState(t, db.Pool))
	var deleted, capturedDeleted bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT p.deleted_at IS NOT NULL,a.deleted FROM profiles p JOIN identity_associations a ON a.association_id=p.identity_association_id WHERE p.id=$1`, profile.ID).Scan(&deleted, &capturedDeleted))
	require.False(t, deleted)
	require.False(t, capturedDeleted)
}

func TestProfileRetirementCaptureRefusesForeignPrimaryAndRepeatedArchive(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account := db.Accounts[db.Profiles[0]]
	secondary, err := profiles.CreateSecondaryProfile(ctx, account, "Owned secondary", nil, nil)
	require.NoError(t, err)
	primary, err := profiles.EnsurePrimaryProfile(ctx, uuid.New(), nil, "Primary remains", false)
	require.NoError(t, err)
	before := birthCaptureState(t, db.Pool)
	require.ErrorIs(t, profiles.SoftDeleteProfile(ctx, uuid.New(), secondary.ID), pgx.ErrNoRows)
	require.ErrorIs(t, profiles.SoftDeleteProfile(ctx, primary.AccountID, primary.ID), pgx.ErrNoRows)
	require.Equal(t, before, birthCaptureState(t, db.Pool))
	require.NoError(t, profiles.SoftDeleteProfile(ctx, account, secondary.ID))
	after := birthCaptureState(t, db.Pool)
	require.ErrorIs(t, profiles.SoftDeleteProfile(ctx, account, secondary.ID), pgx.ErrNoRows)
	require.Equal(t, after, birthCaptureState(t, db.Pool))
}

func TestProfileRetirementCaptureRefusesUncapturedLegacyRow(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	// Retained raw fixture data is deliberately not certified as whole history.
	_, err := db.Pool.Exec(ctx, `UPDATE profiles SET is_primary=false WHERE id=$1`, db.Profiles[0])
	require.NoError(t, err)
	before := birthCaptureState(t, db.Pool)
	require.ErrorIs(t, profiles.SoftDeleteProfile(ctx, db.Accounts[db.Profiles[0]], db.Profiles[0]), identityassociation.ErrIncomplete)
	require.Equal(t, before, birthCaptureState(t, db.Pool))
}

func TestProfileRetirementCapturePreservesLegacyMode(t *testing.T) {
	db, _, _ := capturedBirthFixture(t, false)
	ctx := context.Background()
	profiles := NewProfileStore(db.Pool)
	account := db.Accounts[db.Profiles[0]]
	profile, err := profiles.CreateSecondaryProfile(ctx, account, "Legacy archive", nil, nil)
	require.NoError(t, err)
	require.NoError(t, profiles.SoftDeleteProfile(ctx, account, profile.ID))
	var deleted bool
	var lifecycle int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL,(SELECT count(*) FROM identity_journal WHERE kind='LIFECYCLE') FROM profiles WHERE id=$1`, profile.ID).Scan(&deleted, &lifecycle))
	require.True(t, deleted)
	require.Zero(t, lifecycle, "legacy operation is not relabeled canonical capture")
}

func TestProfileRetirementCaptureConcurrentArchiveIsSingleEffect(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	account := db.Accounts[db.Profiles[0]]
	profile, err := profiles.CreateSecondaryProfile(ctx, account, "Concurrent archive", nil, nil)
	require.NoError(t, err)
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- profiles.SoftDeleteProfile(ctx, account, profile.ID) }()
	}
	first, second := <-results, <-results
	if first != nil {
		first, second = second, first
	}
	require.NoError(t, first)
	require.ErrorIs(t, second, pgx.ErrNoRows)
	var lifecycle, searchEvents, outbox int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM identity_journal WHERE kind='LIFECYCLE'),
	 (SELECT count(*) FROM user_profile_search_journal WHERE profile_id=$1),
	 (SELECT count(*) FROM user_profile_search_outbox)`, profile.ID).Scan(&lifecycle, &searchEvents, &outbox))
	require.Equal(t, 1, lifecycle)
	require.Equal(t, 2, searchEvents)
	require.Equal(t, 2, outbox)
}
