package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/integrationtest"
)

// TestProfileMutation_SearchProjectionOutboxFailureRollsBack proves a failed
// journal/outbox write cannot leave a committed profile update without its
// authoritative Search projection event.
func TestProfileMutation_SearchProjectionOutboxFailureRollsBack(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, userModuleRepoRoot(t))

	accountID, profileID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'projectionrollback', '0001', 'Before rollback', true)`,
		profileID, accountID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		CREATE FUNCTION fail_search_projection_outbox() RETURNS trigger AS $$
		BEGIN RAISE EXCEPTION 'forced outbox failure'; END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER fail_search_projection_outbox
		BEFORE INSERT ON user_profile_search_outbox
		FOR EACH ROW EXECUTE FUNCTION fail_search_projection_outbox();`)
	require.NoError(t, err)

	name := "Must not commit"
	updated, err := NewProfileStore(pool).UpdateOwnedProfile(
		ctx, accountID, profileID, UpdateProfileInput{DisplayName: &name})
	require.Error(t, err)
	require.Nil(t, updated)

	var displayName string
	var journalCount, outboxCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT display_name FROM profiles WHERE id = $1`, profileID).Scan(&displayName))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM user_profile_search_journal WHERE profile_id = $1`, profileID).Scan(&journalCount))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM user_profile_search_outbox`).Scan(&outboxCount))
	require.Equal(t, "Before rollback", displayName)
	require.Zero(t, journalCount)
	require.Zero(t, outboxCount)
}

// TestSearchProjectionOutbox_LeaseExpiryReclaimsUnmarkedPubAck verifies the
// crash boundary after a broker PubAck and before delivered_at is persisted.
// The replay uses the same event ID and immutable bytes, so JetStream de-dupe
// and Search's inbox fence make the redelivery safe.
func TestSearchProjectionOutbox_LeaseExpiryReclaimsUnmarkedPubAck(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, userModuleRepoRoot(t))
	accountID, profileID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary, search_projection_revision)
		VALUES ($1,$2,'lease-reclaim','0001','Lease reclaim',true,1)`, profileID, accountID)
	require.NoError(t, err)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, AppendSearchProjection(ctx, tx, &userv1.SearchProfileProjectionEvent{
		ProtocolVersion: 1, EventId: uuid.NewString(), ProfileId: profileID.String(), SourceRevision: 1, OccurredAt: timestamppb.Now(),
		Payload: &userv1.SearchProfileProjectionEvent_Upsert{Upsert: &userv1.SearchProfileUpsert{AccountId: accountID.String(), Username: "lease-reclaim", Discriminator: "0001", DisplayName: "Lease reclaim", UsernameSearchKey: "leasereclaim", DisplayNameSearchKey: "leasereclaim", NormalizationVersion: 1}},
	}))
	require.NoError(t, AppendSearchProjection(ctx, tx, &userv1.SearchProfileProjectionEvent{
		ProtocolVersion: 1, EventId: uuid.NewString(), ProfileId: profileID.String(), SourceRevision: 2, OccurredAt: timestamppb.Now(),
		Payload: &userv1.SearchProfileProjectionEvent_Upsert{Upsert: &userv1.SearchProfileUpsert{AccountId: accountID.String(), Username: "lease-reclaim", Discriminator: "0001", DisplayName: "Lease reclaim N+1", UsernameSearchKey: "leasereclaim", DisplayNameSearchKey: "leasereclaim", NormalizationVersion: 1}},
	}))
	require.NoError(t, tx.Commit(ctx))

	store := NewProfileStore(pool)
	first, err := store.ClaimSearchProjectionOutbox(ctx, "crashed-owner", time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.NotNil(t, first)
	// A second dispatcher cannot skip the live head lease and publish N+1.
	second, err := store.ClaimSearchProjectionOutbox(ctx, "would-skip-head", time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.Nil(t, second)
	// Simulate a successfully acknowledged broker publish followed by process
	// death before MarkSearchProjectionOutboxDelivered.
	_, err = pool.Exec(ctx, `UPDATE user_profile_search_outbox SET leased_until = now() - interval '1 second' WHERE event_id = $1`, first.EventID)
	require.NoError(t, err)
	reclaimed, err := store.ClaimSearchProjectionOutbox(ctx, "recovery-owner", time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.NotNil(t, reclaimed)
	require.Equal(t, first.EventID, reclaimed.EventID)
	require.Equal(t, first.JournalOffset, reclaimed.JournalOffset)
	require.Equal(t, first.Payload, reclaimed.Payload)
	require.NoError(t, store.MarkSearchProjectionOutboxDelivered(ctx, reclaimed.EventID, "recovery-owner"))
	next, err := store.ClaimSearchProjectionOutbox(ctx, "recovery-owner", time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Greater(t, next.JournalOffset, reclaimed.JournalOffset)
	var deliveredAt *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT delivered_at FROM user_profile_search_outbox WHERE event_id=$1`, reclaimed.EventID).Scan(&deliveredAt))
	require.NotNil(t, deliveredAt)
}

// TestVerificationMutation_SearchProjectionOutboxFailureRollsBack proves that
// verification entrypoints share the same all-or-nothing projection boundary as
// normal profile updates.
func TestVerificationMutation_SearchProjectionOutboxFailureRollsBack(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, userModuleRepoRoot(t))

	accountID, profileID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'verificationrollback', '0001', 'Before verification rollback', true)`,
		profileID, accountID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		CREATE FUNCTION fail_verification_projection_outbox() RETURNS trigger AS $$
		BEGIN RAISE EXCEPTION 'forced outbox failure'; END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER fail_verification_projection_outbox
		BEFORE INSERT ON user_profile_search_outbox
		FOR EACH ROW EXECUTE FUNCTION fail_verification_projection_outbox();`)
	require.NoError(t, err)

	updated, err := NewProfileStore(pool).SetProfileVerification(ctx, profileID, "personal", "verified")
	require.Error(t, err)
	require.Nil(t, updated)

	var verificationType string
	var revision int64
	var journalCount, outboxCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT verification_type, search_projection_revision FROM profiles WHERE id = $1`, profileID).
		Scan(&verificationType, &revision))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM user_profile_search_journal WHERE profile_id = $1`, profileID).Scan(&journalCount))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM user_profile_search_outbox`).Scan(&outboxCount))
	require.Equal(t, "none", verificationType)
	require.Zero(t, revision)
	require.Zero(t, journalCount)
	require.Zero(t, outboxCount)
}

func TestVerificationSourceMutation_SearchProjectionOutboxFailureRollsBack(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, userModuleRepoRoot(t))

	accountID, profileID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'sourcerollback', '0001', 'Before source rollback', true)`,
		profileID, accountID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		CREATE FUNCTION fail_source_projection_outbox() RETURNS trigger AS $$
		BEGIN RAISE EXCEPTION 'forced outbox failure'; END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER fail_source_projection_outbox
		BEFORE INSERT ON user_profile_search_outbox
		FOR EACH ROW EXECUTE FUNCTION fail_source_projection_outbox();`)
	require.NoError(t, err)

	updated, applied, err := NewProfileStore(pool).ApplyVerificationSourceState(
		ctx, profileID, "twitch", 1, true, "personal", "twitch")
	require.Error(t, err)
	require.Nil(t, updated)
	require.False(t, applied)

	var verificationType string
	var revision int64
	var sourceCount, journalCount, outboxCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT verification_type, search_projection_revision FROM profiles WHERE id = $1`, profileID).
		Scan(&verificationType, &revision))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM profile_verification_sources WHERE profile_id = $1`, profileID).Scan(&sourceCount))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM user_profile_search_journal WHERE profile_id = $1`, profileID).Scan(&journalCount))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM user_profile_search_outbox`).Scan(&outboxCount))
	require.Equal(t, "none", verificationType)
	require.Zero(t, revision)
	require.Zero(t, sourceCount)
	require.Zero(t, journalCount)
	require.Zero(t, outboxCount)
}
