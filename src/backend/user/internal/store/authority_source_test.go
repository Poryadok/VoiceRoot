package store

import (
	"context"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/integrationtest"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
)

func userSourceStore(t *testing.T) (context.Context, *ProfileStore) {
	t.Helper()
	if testing.Short() {
		t.Skip("requires owning PostgreSQL authority source")
	}
	ctx := context.Background()
	fixture := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(userModuleRepoRoot(t), "src", "backend", "migrations", "user_db"))
	fixture.Run(true, "up")
	return ctx, NewProfileStore(fixture.Pool)
}

func TestUserSourceCompleteExactProfilesAndSDKActorFences(t *testing.T) {
	ctx, st := userSourceStore(t)
	profile, actor, control, account := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	_, err := st.pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'owning','0001','private display')`, profile, account)
	require.NoError(t, err)
	_, err = st.pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'control','0001','control')`, control, uuid.NewString())
	require.NoError(t, err)
	ids := []string{profile, actor}
	sort.Strings(ids)
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString(), ProfileIds: ids}
	snapshot, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.True(t, snapshot.Complete)
	state, err := authoritysource.DecodeUserState(snapshot.CanonicalState)
	require.NoError(t, err)
	require.Len(t, state.Profiles, 2)
	require.True(t, state.EligibleProfile(profile, account))
	require.False(t, state.EligibleProfile(actor, account))
	require.False(t, state.ActorRetired(actor, account))
	require.NotContains(t, string(snapshot.CanonicalState), "private display")
	require.NotContains(t, string(snapshot.CanonicalState), control)
	_, err = st.pool.Exec(ctx, `INSERT INTO sdk_author_tombstones(operation_id,receipt_id,source_account_id,source_actor_id,target_account_id,target_profile_id,profile_revision,frozen_binding_id,frozen_authority_epoch,freeze_receipt_id,request_hash) VALUES($1,$2,$3,$4,$5,$6,1,$7,1,$8,repeat('0',64))`, uuid.NewString(), uuid.NewString(), account, actor, uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString())
	require.NoError(t, err)
	fenced, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.Greater(t, fenced.Revision, snapshot.Revision)
	state, err = authoritysource.DecodeUserState(fenced.CanonicalState)
	require.NoError(t, err)
	require.True(t, state.ActorRetired(actor, account))
	require.Len(t, state.Tombstones, 1)
	require.NotContains(t, string(fenced.CanonicalState), "receipt")
	require.NotContains(t, string(fenced.CanonicalState), "request_hash")
	revision, err := st.ReadAuthorityRevision(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, fenced.Revision, revision)
}

func TestUserSourceCounterAndEligibilityShareOneCut(t *testing.T) {
	ctx, st := userSourceStore(t)
	profile, account := uuid.NewString(), uuid.NewString()
	_, err := st.pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'cut','0001','cut')`, profile, account)
	require.NoError(t, err)
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString(), ProfileIds: []string{profile}}
	tx, err := st.beginAuthorityRead(ctx, scope)
	require.NoError(t, err)
	defer authoritysource.RollbackRead(ctx, tx)
	oldRevision, err := userSourceRevision(ctx, tx)
	require.NoError(t, err)
	_, err = st.pool.Exec(ctx, `UPDATE profiles SET frozen_at=now() WHERE id=$1`, profile)
	require.NoError(t, err)
	_, err = st.pool.Exec(ctx, `INSERT INTO user_account_lifecycle(account_id,state,source_event_id,occurred_at) VALUES($1,'ACCOUNT_INACTIVE',$2,now())`, account, uuid.NewString())
	require.NoError(t, err)
	sameRevision, err := userSourceRevision(ctx, tx)
	require.NoError(t, err)
	require.Equal(t, oldRevision, sameRevision)
	var frozen, inactive bool
	require.NoError(t, tx.QueryRow(ctx, `SELECT frozen_at IS NOT NULL,EXISTS(SELECT 1 FROM user_account_lifecycle WHERE account_id=$2) FROM profiles WHERE id=$1`, profile, account).Scan(&frozen, &inactive))
	require.False(t, frozen)
	require.False(t, inactive)
	require.NoError(t, tx.Commit(ctx))
	fresh, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.Greater(t, fresh.Revision, oldRevision)
	state, err := authoritysource.DecodeUserState(fresh.CanonicalState)
	require.NoError(t, err)
	require.True(t, state.Profiles[0].Frozen)
	require.True(t, state.Profiles[0].AccountInactive)
	require.False(t, state.EligibleProfile(profile, account))
}

func TestUserSourceRejectsScopeAndCatalogDrift(t *testing.T) {
	ctx, st := userSourceStore(t)
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString(), ProfileIds: []string{uuid.NewString()}}
	require.NoError(t, st.CheckAuthoritySourceSchema(ctx))
	scope.AccountIds = []string{uuid.NewString()}
	_, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.Error(t, err)
	scope.AccountIds = nil
	_, err = st.pool.Exec(ctx, `ALTER TABLE profiles DISABLE TRIGGER user_authority_profiles`)
	require.NoError(t, err)
	_, err = st.ReadAuthorityRevision(ctx, scope)
	require.Error(t, err)
	_, err = st.pool.Exec(ctx, `ALTER TABLE profiles ENABLE TRIGGER user_authority_profiles; UPDATE schema_migrations SET dirty=true`)
	require.NoError(t, err)
	require.Error(t, st.CheckAuthoritySourceSchema(ctx))
}

func TestUserSourceAccountDeletedReplayRetainsAllProfileFences(t *testing.T) {
	ctx, st := userSourceStore(t)
	account, event, first, second := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := st.pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name,is_primary) VALUES($1,$3,'deleted-first','0001','first',true),($2,$3,'deleted-second','0001','second',false)`, first, second, account)
	require.NoError(t, err)
	ids := []string{first.String(), second.String()}
	sort.Strings(ids)
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString(), ProfileIds: ids}
	before, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.NoError(t, st.ApplyAccountDeleted(ctx, event, account, time.Now().UTC()))
	deleted, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.Greater(t, deleted.Revision, before.Revision)
	state, err := authoritysource.DecodeUserState(deleted.CanonicalState)
	require.NoError(t, err)
	for _, item := range state.Profiles {
		require.True(t, item.AccountInactive)
		require.False(t, state.EligibleProfile(item.ProfileID, account.String()))
	}
	var projected int
	require.NoError(t, st.pool.QueryRow(ctx, `SELECT count(*) FROM user_profile_search_journal WHERE profile_id=ANY($1::uuid[])`, ids).Scan(&projected))
	require.Equal(t, 2, projected, "every profile has a durable Search delete in the same commit")
	require.NoError(t, st.ApplyAccountDeleted(ctx, event, account, time.Now().UTC()))
	replayed, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, deleted.Revision, replayed.Revision)
	require.Equal(t, deleted.CanonicalState, replayed.CanonicalState)
	// A distinct duplicate account event also preserves the original immutable
	// overlay while the canonical writer produces fresh revisioned deletes.
	require.NoError(t, st.ApplyAccountDeleted(ctx, uuid.New(), account, time.Now().UTC()))
	var original string
	require.NoError(t, st.pool.QueryRow(ctx, `SELECT source_event_id::text FROM user_account_lifecycle WHERE account_id=$1`, account).Scan(&original))
	require.Equal(t, event.String(), original)
}

func TestUserSourceAccountDeletionOutboxFailureRollsBackWholeCut(t *testing.T) {
	ctx, st := userSourceStore(t)
	account, event, profile := uuid.New(), uuid.New(), uuid.New()
	_, err := st.pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'rollback-delete','0001','before')`, profile, account)
	require.NoError(t, err)
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString(), ProfileIds: []string{profile.String()}}
	before, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	_, err = st.pool.Exec(ctx, `CREATE FUNCTION fail_source_deletion_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced source deletion failure'; END $$;
 CREATE TRIGGER fail_source_deletion_outbox BEFORE INSERT ON user_profile_search_outbox FOR EACH ROW EXECUTE FUNCTION fail_source_deletion_outbox()`)
	require.NoError(t, err)
	require.Error(t, st.ApplyAccountDeleted(ctx, event, account, time.Now().UTC()))
	after, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, before.Revision, after.Revision)
	require.Equal(t, before.CanonicalState, after.CanonicalState)
	var overlays, inbox, journal int
	require.NoError(t, st.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM user_account_lifecycle),(SELECT count(*) FROM user_account_lifecycle_inbox),(SELECT count(*) FROM user_profile_search_journal)`).Scan(&overlays, &inbox, &journal))
	require.Zero(t, overlays)
	require.Zero(t, inbox)
	require.Zero(t, journal)
	_, err = st.pool.Exec(ctx, `DROP TRIGGER fail_source_deletion_outbox ON user_profile_search_outbox`)
	require.NoError(t, err)
	require.NoError(t, st.ApplyAccountDeleted(ctx, event, account, time.Now().UTC()), "the unchanged event is retryable after a failed commit")
}
