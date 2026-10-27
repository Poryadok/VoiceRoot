package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/user/internal/identityassociation"
)

func TestDowngradeIdentityCaptureRollsBackEntireSelection(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account := uuid.New()
	primary, err := profiles.EnsurePrimaryProfile(ctx, account, nil, "Primary", false)
	require.NoError(t, err)
	kept, err := profiles.CreateSecondaryProfile(ctx, account, "Kept", nil, nil)
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = profiles.CreateSecondaryProfile(ctx, account, "Excess", nil, nil)
		require.NoError(t, err)
	}
	before := birthCaptureState(t, db.Pool)
	_, err = db.Pool.Exec(ctx, `CREATE SEQUENCE downgrade_freezes;
	 CREATE FUNCTION fail_second_downgrade_freeze() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
	 IF NEW.frozen_at IS NOT NULL AND OLD.frozen_at IS NULL AND nextval('downgrade_freezes')=2 THEN RAISE EXCEPTION 'fixture second freeze failed'; END IF;
	 RETURN NEW; END; $$;
	 CREATE TRIGGER fail_second_downgrade_freeze BEFORE UPDATE ON profiles FOR EACH ROW EXECUTE FUNCTION fail_second_downgrade_freeze()`)
	require.NoError(t, err)
	err = profiles.ApplyDowngradeProfileSelection(ctx, account, []uuid.UUID{primary.ID, kept.ID})
	require.ErrorContains(t, err, "fixture second freeze failed")
	var frozen int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM profiles WHERE account_id=$1 AND frozen_at IS NOT NULL`, account).Scan(&frozen))
	require.Zero(t, frozen, "failure on second freeze must roll back the first profile too")
	require.Equal(t, before, birthCaptureState(t, db.Pool), "all profile/LIFECYCLE effects share the selection rollback")
}

func TestDowngradeIdentityCaptureWaitsBeforeFreshSelection(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	account := uuid.New()
	primary, err := profiles.EnsurePrimaryProfile(ctx, account, nil, "Primary", false)
	require.NoError(t, err)
	kept, err := profiles.CreateSecondaryProfile(ctx, account, "Kept", nil, nil)
	require.NoError(t, err)
	excess, err := profiles.CreateSecondaryProfile(ctx, account, "Archived while waiting", nil, nil)
	require.NoError(t, err)
	owner, err := profiles.birthCapture.identities.Begin(ctx)
	require.NoError(t, err)
	defer owner.Rollback(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- profiles.ApplyDowngradeProfileSelection(ctx, account, []uuid.UUID{primary.ID, kept.ID})
	}()
	select {
	case err := <-result:
		t.Fatalf("selection must wait before observing profiles: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	// The current owner commits a real archive/search mutation before releasing I.
	// A stale pre-I selection would wrongly freeze this now-archived profile.
	require.NoError(t, profiles.softDeleteProfileTx(ctx, owner.SQL(), account, excess.ID))
	require.NoError(t, owner.Commit(ctx))
	require.NoError(t, <-result)
	var deleted, frozen bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL,frozen_at IS NOT NULL FROM profiles WHERE id=$1`, excess.ID).Scan(&deleted, &frozen))
	require.True(t, deleted)
	require.False(t, frozen, "selection must freshly exclude the archived profile after I")
}

func TestDowngradeIdentityCaptureKeepsPolicyAndRepeatedSelection(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account := uuid.New()
	primary, err := profiles.EnsurePrimaryProfile(ctx, account, nil, "Primary", false)
	require.NoError(t, err)
	secondary := []uuid.UUID{}
	for i := 0; i < 3; i++ {
		p, e := profiles.CreateSecondaryProfile(ctx, account, "Secondary", nil, nil)
		require.NoError(t, e)
		secondary = append(secondary, p.ID)
	}
	require.NoError(t, profiles.ApplyDowngradeProfileSelection(ctx, account, []uuid.UUID{primary.ID, secondary[0]}))
	// Preserve the legacy keep/unkeep meaning, including unfreezing a new kept ID.
	kept := []uuid.UUID{primary.ID, secondary[1]}
	require.NoError(t, profiles.ApplyDowngradeProfileSelection(ctx, account, kept))
	var frozen, visibleKept, searchEvents, outbox int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM profiles WHERE account_id=$1 AND frozen_at IS NOT NULL),
	 (SELECT count(*) FROM profiles WHERE id=ANY($2) AND frozen_at IS NULL),
	 (SELECT count(*) FROM user_profile_search_journal),
	 (SELECT count(*) FROM user_profile_search_outbox)`, account, kept).Scan(&frozen, &visibleKept, &searchEvents, &outbox))
	require.Equal(t, 2, frozen)
	require.Equal(t, 2, visibleKept)
	require.Equal(t, 4, searchEvents)
	require.Equal(t, 4, outbox, "downgrade does not invent a search visibility event")
	before := birthCaptureState(t, db.Pool)
	require.NoError(t, profiles.ApplyDowngradeProfileSelection(ctx, account, kept))
	require.Equal(t, before, birthCaptureState(t, db.Pool), "repeat does not append another LIFECYCLE revision")
}

func TestDowngradeIdentityCaptureRefusesUncapturedLegacyRow(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	before := birthCaptureState(t, db.Pool)
	err := profiles.ApplyDowngradeProfileSelection(context.Background(), db.Accounts[db.Profiles[0]], nil)
	require.ErrorIs(t, err, identityassociation.ErrIncomplete)
	require.Equal(t, before, birthCaptureState(t, db.Pool))
}

func TestDowngradeIdentityCapturePreservesLegacyMode(t *testing.T) {
	db, _, _ := capturedBirthFixture(t, false)
	ctx := context.Background()
	profiles := NewProfileStore(db.Pool)
	account := uuid.New()
	primary, err := profiles.EnsurePrimaryProfile(ctx, account, nil, "Legacy primary", false)
	require.NoError(t, err)
	kept := []uuid.UUID{primary.ID}
	for i := 0; i < 3; i++ {
		p, e := profiles.CreateSecondaryProfile(ctx, account, "Legacy secondary", nil, nil)
		require.NoError(t, e)
		if i == 0 {
			kept = append(kept, p.ID)
		}
	}
	require.NoError(t, profiles.ApplyDowngradeProfileSelection(ctx, account, kept))
	var frozen, lifecycle int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM profiles WHERE account_id=$1 AND frozen_at IS NOT NULL),(SELECT count(*) FROM identity_journal WHERE kind='LIFECYCLE')`, account).Scan(&frozen, &lifecycle))
	require.Equal(t, 2, frozen)
	require.Zero(t, lifecycle, "legacy data is not represented as captured history")
}
