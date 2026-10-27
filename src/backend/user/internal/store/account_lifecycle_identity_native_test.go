package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAccountInactiveIdentityWaitsBeforeFirstWrite(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	account, event := uuid.New(), uuid.New()
	primary, err := profiles.EnsurePrimaryProfile(ctx, account, nil, "Primary", false)
	require.NoError(t, err)
	owner, err := profiles.birthCapture.identities.Begin(ctx)
	require.NoError(t, err)
	defer owner.Rollback(context.Background())
	result := make(chan error, 1)
	go func() { result <- profiles.ApplyAccountDeleted(ctx, event, account, time.Now().UTC()) }()
	select {
	case err := <-result:
		t.Fatalf("legacy overlay must wait for I before writes: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, owner.Commit(ctx))
	require.NoError(t, <-result)
	var state string
	var inbox, search int
	var inactive bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT l.state,
	 (SELECT count(*) FROM user_account_lifecycle_inbox WHERE event_id=$1),
	 (SELECT count(*) FROM user_profile_search_journal WHERE profile_id=$2),
	 (SELECT a.account_inactive FROM profiles p JOIN identity_associations a ON a.association_id=p.identity_association_id WHERE p.id=$2)
	 FROM user_account_lifecycle l WHERE account_id=$3`, event, primary.ID, account).Scan(&state, &inbox, &search, &inactive))
	require.Equal(t, "ACCOUNT_INACTIVE", state)
	require.Equal(t, 1, inbox)
	require.Equal(t, 2, search)
	require.True(t, inactive)
}

func TestAccountInactiveIdentityDuplicateEventIsNoOp(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account, event := uuid.New(), uuid.New()
	primary, err := profiles.EnsurePrimaryProfile(ctx, account, nil, "Primary", false)
	require.NoError(t, err)
	when := time.Now().UTC()
	require.NoError(t, profiles.ApplyAccountDeleted(ctx, event, account, when))
	before := birthCaptureState(t, db.Pool)
	require.NoError(t, profiles.ApplyAccountDeleted(ctx, event, account, when.Add(time.Hour)))
	require.Equal(t, before, birthCaptureState(t, db.Pool))
	var inbox, revision, terminal int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM user_account_lifecycle_inbox WHERE account_id=$1),
	 (SELECT search_projection_revision FROM profiles WHERE id=$2),
	 (SELECT count(*) FROM identity_associations WHERE terminal_fact IS NOT NULL)`, account, primary.ID).Scan(&inbox, &revision, &terminal))
	require.Equal(t, 1, inbox)
	require.Equal(t, 2, revision)
	require.Zero(t, terminal, "legacy inactive overlay is not an A4 terminal fact")
}

func TestAccountInactiveIdentityOutboxFailureRollsBackAndRetries(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account, event := uuid.New(), uuid.New()
	_, err := profiles.EnsurePrimaryProfile(ctx, account, nil, "Primary", false)
	require.NoError(t, err)
	_, err = profiles.CreateSecondaryProfile(ctx, account, "Secondary", nil, nil)
	require.NoError(t, err)
	before := birthCaptureState(t, db.Pool)
	_, err = db.Pool.Exec(ctx, `CREATE FUNCTION fail_inactive_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture inactive outbox failed'; END; $$;
	 CREATE TRIGGER fail_inactive_outbox BEFORE INSERT ON user_profile_search_outbox FOR EACH ROW EXECUTE FUNCTION fail_inactive_outbox()`)
	require.NoError(t, err)
	when := time.Now().UTC()
	require.ErrorContains(t, profiles.ApplyAccountDeleted(ctx, event, account, when), "fixture inactive outbox failed")
	require.Equal(t, before, birthCaptureState(t, db.Pool))
	var inbox, overlay, revision int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM user_account_lifecycle_inbox WHERE account_id=$1),
	 (SELECT count(*) FROM user_account_lifecycle WHERE account_id=$1),
	 (SELECT max(search_projection_revision) FROM profiles WHERE account_id=$1)`, account).Scan(&inbox, &overlay, &revision))
	require.Zero(t, inbox)
	require.Zero(t, overlay)
	require.Equal(t, 1, revision)
	_, err = db.Pool.Exec(ctx, `DROP TRIGGER fail_inactive_outbox ON user_profile_search_outbox`)
	require.NoError(t, err)
	require.NoError(t, profiles.ApplyAccountDeleted(ctx, event, account, when), "failed attempt must leave the same event retriable")
	var events int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM user_profile_search_journal`).Scan(&events))
	require.Equal(t, 4, events) // two birth upserts, then two account tombstones
}

func TestAccountInactiveIdentityIncludesAlreadyArchivedProfile(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx := context.Background()
	account := uuid.New()
	primary, err := profiles.EnsurePrimaryProfile(ctx, account, nil, "Primary", false)
	require.NoError(t, err)
	archived, err := profiles.CreateSecondaryProfile(ctx, account, "Archived", nil, nil)
	require.NoError(t, err)
	require.NoError(t, profiles.SoftDeleteProfile(ctx, account, archived.ID))
	require.NoError(t, profiles.ApplyAccountDeleted(ctx, uuid.New(), account, time.Now().UTC()))
	var primaryRevision, archivedRevision, inactive, events int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT
	 (SELECT search_projection_revision FROM profiles WHERE id=$1),
	 (SELECT search_projection_revision FROM profiles WHERE id=$2),
	 (SELECT count(*) FROM identity_associations WHERE account_inactive),
	 (SELECT count(*) FROM user_profile_search_journal)`, primary.ID, archived.ID).Scan(&primaryRevision, &archivedRevision, &inactive, &events))
	require.Equal(t, 2, primaryRevision)
	require.Equal(t, 3, archivedRevision)
	require.Equal(t, 2, inactive)
	require.Equal(t, 5, events)
}

func TestAccountInactiveIdentityPreservesLegacyAndUncapturedData(t *testing.T) {
	for _, configured := range []bool{false, true} {
		name := "legacy"
		if configured {
			name = "configured_uncaptured"
		}
		t.Run(name, func(t *testing.T) {
			db, profiles, _ := capturedBirthFixture(t, configured)
			if !configured {
				profiles = NewProfileStore(db.Pool)
			}
			ctx := context.Background()
			profile := db.Profiles[0]
			account := db.Accounts[profile]
			var before uint64
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT search_projection_revision FROM profiles WHERE id=$1`, profile).Scan(&before))
			require.NoError(t, profiles.ApplyAccountDeleted(ctx, uuid.New(), account, time.Now().UTC()))
			var after uint64
			var uncaptured, inactive bool
			var canonical int
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT search_projection_revision,identity_association_id IS NULL,
			 EXISTS(SELECT 1 FROM user_account_lifecycle WHERE account_id=$2 AND state='ACCOUNT_INACTIVE'),
			 (SELECT count(*) FROM identity_associations) FROM profiles WHERE id=$1`, profile, account).Scan(&after, &uncaptured, &inactive, &canonical))
			require.Equal(t, before+1, after)
			require.True(t, inactive)
			require.True(t, uncaptured)
			require.Zero(t, canonical, "legacy projection does not invent history or terminal authority")
		})
	}
}

func TestAccountInactiveIdentityConcurrentDuplicateIsSingleEffect(t *testing.T) {
	db, profiles, _ := capturedBirthFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	account, event := uuid.New(), uuid.New()
	primary, err := profiles.EnsurePrimaryProfile(ctx, account, nil, "Primary", false)
	require.NoError(t, err)
	when := time.Now().UTC()
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- profiles.ApplyAccountDeleted(ctx, event, account, when) }()
	}
	require.NoError(t, <-results)
	require.NoError(t, <-results)
	var inbox, events, revision int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM user_account_lifecycle_inbox WHERE event_id=$1),
	 (SELECT count(*) FROM user_profile_search_journal WHERE profile_id=$2),
	 (SELECT search_projection_revision FROM profiles WHERE id=$2)`, event, primary.ID).Scan(&inbox, &events, &revision))
	require.Equal(t, 1, inbox)
	require.Equal(t, 2, events)
	require.Equal(t, 2, revision)
}
