package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/user/internal/identityassociation"
)

func TestFreeTierIdentityCaptureRollsBackEveryFreeze(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account := uuid.New()
	_, err := profiles.EnsurePrimaryProfile(ctx, account, nil, "Primary", false)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err = profiles.CreateSecondaryProfile(ctx, account, "Secondary", nil, nil)
		require.NoError(t, err)
	}
	before := birthCaptureState(t, db.Pool)
	_, err = db.Pool.Exec(ctx, `CREATE SEQUENCE free_tier_freezes;
	 CREATE FUNCTION fail_second_free_tier_freeze() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
	 IF NEW.frozen_at IS NOT NULL AND OLD.frozen_at IS NULL AND nextval('free_tier_freezes')=2 THEN RAISE EXCEPTION 'fixture second free-tier freeze failed'; END IF;
	 RETURN NEW; END; $$;
	 CREATE TRIGGER fail_second_free_tier_freeze BEFORE UPDATE ON profiles FOR EACH ROW EXECUTE FUNCTION fail_second_free_tier_freeze()`)
	require.NoError(t, err)
	require.ErrorContains(t, profiles.ApplyFreeTierFreeze(ctx, account), "fixture second free-tier freeze failed")
	var frozen int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM profiles WHERE account_id=$1 AND frozen_at IS NOT NULL`, account).Scan(&frozen))
	require.Zero(t, frozen, "second freeze failure must roll back every earlier freeze")
	require.Equal(t, before, birthCaptureState(t, db.Pool))
}

func TestFreeTierIdentityCaptureWaitsForFreshRetainedSlots(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	account := uuid.New()
	_, err := profiles.EnsurePrimaryProfile(ctx, account, nil, "Primary", false)
	require.NoError(t, err)
	ids := []uuid.UUID{}
	for i := 0; i < 3; i++ {
		p, e := profiles.CreateSecondaryProfile(ctx, account, "Secondary", nil, nil)
		require.NoError(t, e)
		ids = append(ids, p.ID)
	}
	owner, err := profiles.birthCapture.identities.Begin(ctx)
	require.NoError(t, err)
	defer owner.Rollback(context.Background())
	result := make(chan error, 1)
	go func() { result <- profiles.ApplyFreeTierFreeze(ctx, account) }()
	select {
	case err := <-result:
		t.Fatalf("free-tier selection must wait before reading slots: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, profiles.softDeleteProfileTx(ctx, owner.SQL(), account, ids[0]))
	require.NoError(t, owner.Commit(ctx))
	require.NoError(t, <-result)
	var nextFrozen, lastFrozen bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT (SELECT frozen_at IS NOT NULL FROM profiles WHERE id=$1),(SELECT frozen_at IS NOT NULL FROM profiles WHERE id=$2)`, ids[1], ids[2]).Scan(&nextFrozen, &lastFrozen))
	require.False(t, nextFrozen, "next earliest secondary takes the fresh retained slot")
	require.True(t, lastFrozen)
}

func TestFreeTierIdentityCapturePreservesFrozenStateAndRepeat(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account := uuid.New()
	primary, err := profiles.EnsurePrimaryProfile(ctx, account, nil, "Primary", false)
	require.NoError(t, err)
	ids := []uuid.UUID{}
	for i := 0; i < 3; i++ {
		p, e := profiles.CreateSecondaryProfile(ctx, account, "Secondary", nil, nil)
		require.NoError(t, e)
		ids = append(ids, p.ID)
	}
	owner, err := profiles.birthCapture.identities.Begin(ctx)
	require.NoError(t, err)
	defer owner.Rollback(context.Background())
	_, err = owner.SQL().Exec(ctx, `UPDATE profiles SET frozen_at=clock_timestamp() WHERE id=ANY($1)`, []uuid.UUID{primary.ID, ids[2]})
	require.NoError(t, err)
	require.NoError(t, owner.Commit(ctx))
	var original time.Time
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT frozen_at FROM profiles WHERE id=$1`, ids[2]).Scan(&original))
	require.NoError(t, profiles.ApplyFreeTierFreeze(ctx, account))
	var keptFrozen, earliestFrozen bool
	var excessTime time.Time
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT (SELECT frozen_at IS NOT NULL FROM profiles WHERE id=$1),(SELECT frozen_at IS NOT NULL FROM profiles WHERE id=$2),(SELECT frozen_at FROM profiles WHERE id=$3)`, primary.ID, ids[0], ids[2]).Scan(&keptFrozen, &earliestFrozen, &excessTime))
	require.True(t, keptFrozen, "legacy policy does not unfreeze a retained slot")
	require.False(t, earliestFrozen)
	require.Equal(t, original, excessTime, "already-frozen excess is skipped")
	before := birthCaptureState(t, db.Pool)
	require.NoError(t, profiles.ApplyFreeTierFreeze(ctx, account))
	require.Equal(t, before, birthCaptureState(t, db.Pool), "repeat appends no lifecycle or search facts")
}

func TestFreeTierIdentityCaptureWithinCapIsNoOp(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account := uuid.New()
	_, err := profiles.EnsurePrimaryProfile(ctx, account, nil, "Primary", false)
	require.NoError(t, err)
	_, err = profiles.CreateSecondaryProfile(ctx, account, "Retained", nil, nil)
	require.NoError(t, err)
	before := birthCaptureState(t, db.Pool)
	require.NoError(t, profiles.ApplyFreeTierFreeze(ctx, account))
	require.Equal(t, before, birthCaptureState(t, db.Pool))
}

func TestFreeTierIdentityCaptureRefusesUncapturedLegacyRow(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	before := birthCaptureState(t, db.Pool)
	err := profiles.ApplyFreeTierFreeze(context.Background(), db.Accounts[db.Profiles[0]])
	require.ErrorIs(t, err, identityassociation.ErrIncomplete)
	require.Equal(t, before, birthCaptureState(t, db.Pool))
}

func TestFreeTierIdentityCapturePreservesLegacyMode(t *testing.T) {
	db, _, _ := capturedBirthFixture(t, false)
	ctx := context.Background()
	profiles := NewProfileStore(db.Pool)
	account := uuid.New()
	_, err := profiles.EnsurePrimaryProfile(ctx, account, nil, "Legacy primary", false)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err = profiles.CreateSecondaryProfile(ctx, account, "Legacy secondary", nil, nil)
		require.NoError(t, err)
	}
	require.NoError(t, profiles.ApplyFreeTierFreeze(ctx, account))
	var frozen, lifecycle int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM profiles WHERE account_id=$1 AND frozen_at IS NOT NULL),(SELECT count(*) FROM identity_journal WHERE kind='LIFECYCLE')`, account).Scan(&frozen, &lifecycle))
	require.Equal(t, 2, frozen)
	require.Zero(t, lifecycle)
}
