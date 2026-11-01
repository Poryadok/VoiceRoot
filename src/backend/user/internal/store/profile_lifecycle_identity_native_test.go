package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestFreeTierFreezeUpdatesCapturedIdentity(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account := db.Accounts[db.Profiles[0]]
	_, err := profiles.CreateSecondaryProfileForTier(ctx, account, "First secondary", nil, nil, "premium")
	require.NoError(t, err)
	last, err := profiles.CreateSecondaryProfileForTier(ctx, account, "Second secondary", nil, nil, "premium")
	require.NoError(t, err)
	require.NoError(t, profiles.ApplyFreeTierFreeze(ctx, account))
	var frozen, factFrozen bool
	var revision uint64
	var event bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT p.frozen_at IS NOT NULL,a.frozen,a.association_revision,EXISTS(SELECT 1 FROM identity_journal j WHERE j.association_id=a.association_id AND j.association_revision=a.association_revision AND j.kind='LIFECYCLE' AND j.frozen) FROM profiles p JOIN identity_associations a ON a.association_id=p.identity_association_id WHERE p.id=$1`, last.ID).Scan(&frozen, &factFrozen, &revision, &event))
	require.True(t, frozen)
	require.True(t, factFrozen)
	require.Positive(t, revision)
	require.True(t, event)
}

func TestAccountDeletedUpdatesCapturedIdentityOnce(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account := db.Accounts[db.Profiles[0]]
	profile, err := profiles.CreateSecondaryProfile(ctx, account, "Captured secondary", nil, nil)
	require.NoError(t, err)
	eventID := uuid.New()
	now := time.Now().UTC()
	require.NoError(t, profiles.ApplyAccountDeleted(ctx, eventID, account, now))
	var inactive bool
	var revision uint64
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT a.account_inactive,a.association_revision,(SELECT count(*) FROM identity_journal j WHERE j.association_id=a.association_id AND j.kind='LIFECYCLE' AND j.account_inactive) FROM profiles p JOIN identity_associations a ON a.association_id=p.identity_association_id WHERE p.id=$1`, profile.ID).Scan(&inactive, &revision, &count))
	require.True(t, inactive)
	require.Positive(t, revision)
	require.Equal(t, 1, count)
	require.NoError(t, profiles.ApplyAccountDeleted(ctx, eventID, account, now))
	var again uint64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT a.association_revision FROM profiles p JOIN identity_associations a ON a.association_id=p.identity_association_id WHERE p.id=$1`, profile.ID).Scan(&again))
	require.Equal(t, revision, again, "duplicate deletion events must not advance identity history")
}

func TestAccountDeletedRollsBackCapturedIdentityOnOutboxFailure(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account := db.Accounts[db.Profiles[0]]
	profile, err := profiles.CreateSecondaryProfile(ctx, account, "Captured secondary", nil, nil)
	require.NoError(t, err)
	var before uint64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT a.association_revision FROM profiles p JOIN identity_associations a ON a.association_id=p.identity_association_id WHERE p.id=$1`, profile.ID).Scan(&before))
	_, err = db.Pool.Exec(ctx, `CREATE FUNCTION fail_account_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture account outbox failed'; END; $$;
	 CREATE TRIGGER fail_account_outbox BEFORE INSERT ON user_profile_search_outbox FOR EACH ROW EXECUTE FUNCTION fail_account_outbox()`)
	require.NoError(t, err)
	eventID := uuid.New()
	require.ErrorContains(t, profiles.ApplyAccountDeleted(ctx, eventID, account, time.Now().UTC()), "fixture account outbox failed")
	var inactive bool
	var after uint64
	var inbox int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT a.account_inactive,a.association_revision,(SELECT count(*) FROM user_account_lifecycle_inbox WHERE event_id=$2) FROM profiles p JOIN identity_associations a ON a.association_id=p.identity_association_id WHERE p.id=$1`, profile.ID, eventID).Scan(&inactive, &after, &inbox))
	require.False(t, inactive)
	require.Equal(t, before, after)
	require.Zero(t, inbox)
}

func TestSoftDeleteUpdatesCapturedIdentity(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account := db.Accounts[db.Profiles[0]]
	profile, err := profiles.CreateSecondaryProfile(ctx, account, "Captured secondary", nil, nil)
	require.NoError(t, err)
	require.NoError(t, profiles.SoftDeleteProfile(ctx, account, profile.ID))
	var deleted, factDeleted, journal bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT p.deleted_at IS NOT NULL,a.deleted,EXISTS(SELECT 1 FROM identity_journal j WHERE j.association_id=a.association_id AND j.association_revision=a.association_revision AND j.kind='LIFECYCLE' AND j.deleted) FROM profiles p JOIN identity_associations a ON a.association_id=p.identity_association_id WHERE p.id=$1`, profile.ID).Scan(&deleted, &factDeleted, &journal))
	require.True(t, deleted)
	require.True(t, factDeleted)
	require.True(t, journal)
}

func TestDowngradeSelectionUpdatesCapturedIdentityBothWays(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account := db.Accounts[db.Profiles[0]]
	profile, err := profiles.CreateSecondaryProfile(ctx, account, "Captured secondary", nil, nil)
	require.NoError(t, err)
	require.NoError(t, profiles.ApplyDowngradeProfileSelection(ctx, account, []uuid.UUID{db.Profiles[0]}))
	var frozen bool
	var firstRevision uint64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT a.frozen,a.association_revision FROM profiles p JOIN identity_associations a ON a.association_id=p.identity_association_id WHERE p.id=$1`, profile.ID).Scan(&frozen, &firstRevision))
	require.True(t, frozen)
	require.NoError(t, profiles.ApplyDowngradeProfileSelection(ctx, account, []uuid.UUID{db.Profiles[0], profile.ID}))
	var nextRevision uint64
	var journal bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT a.frozen,a.association_revision,EXISTS(SELECT 1 FROM identity_journal j WHERE j.association_id=a.association_id AND j.association_revision=a.association_revision AND j.kind='LIFECYCLE' AND NOT j.frozen) FROM profiles p JOIN identity_associations a ON a.association_id=p.identity_association_id WHERE p.id=$1`, profile.ID).Scan(&frozen, &nextRevision, &journal))
	require.False(t, frozen)
	require.Greater(t, nextRevision, firstRevision)
	require.True(t, journal)
}

func TestSoftDeleteFailsClosedWhileIdentityCutIsLocked(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account := db.Accounts[db.Profiles[0]]
	profile, err := profiles.CreateSecondaryProfile(ctx, account, "Captured secondary", nil, nil)
	require.NoError(t, err)
	cut, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = cut.Rollback(ctx) }()
	_, err = cut.Exec(ctx, `SELECT singleton FROM identity_meta WHERE singleton FOR UPDATE`)
	require.NoError(t, err)
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err = profiles.SoftDeleteProfile(bounded, account, profile.ID)
	var lockErr *pgconn.PgError
	require.ErrorAs(t, err, &lockErr)
	require.Equal(t, "55P03", lockErr.Code)
	var deleted bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM profiles WHERE id=$1`, profile.ID).Scan(&deleted))
	require.False(t, deleted)
	require.NoError(t, cut.Rollback(ctx))
	require.NoError(t, profiles.SoftDeleteProfile(ctx, account, profile.ID))
}
