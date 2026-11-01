package registry

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/integrationtest"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
)

func gisSourceFixture(t *testing.T) (context.Context, *Store) {
	t.Helper()
	ctx := context.Background()
	f := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join("..", "..", "..", "migrations", "game_integration_db"))
	_, err := f.Pool.Exec(ctx, `CREATE DATABASE game_integration_db`)
	require.NoError(t, err)
	_, err = f.Pool.Exec(ctx, `CREATE ROLE gameintegration_runtime NOLOGIN`)
	require.NoError(t, err)
	f.Run(true, "up")
	return ctx, &Store{Pool: f.Pool}
}

func TestGISAuthoritySourceReadOnlyAndWholeReadBudget(t *testing.T) {
	ctx, s := gisSourceFixture(t)
	tx, err := s.beginGISSource(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE gis_authority_revision SET revision=revision+1`)
	require.Error(t, err, "owning source cut is read-only")
	require.NoError(t, tx.Rollback(ctx))
	config := s.Pool.Config().Copy()
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer pool.Close()
	held, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer held.Release()
	blocked := &Store{Pool: pool}
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString(), EnvironmentId: uuid.NewString()}
	start := time.Now()
	_, err = blocked.ReadAuthorityRevision(ctx, scope)
	require.Error(t, err)
	require.Less(t, time.Since(start), 2*time.Second, "pool checkout consumes the same one-second source budget")
	held.Release()
	lock, err := s.Pool.Begin(ctx)
	require.NoError(t, err)
	defer lock.Rollback(ctx)
	_, err = lock.Exec(ctx, `LOCK TABLE schema_migrations IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	start = time.Now()
	_, err = s.ReadAuthoritySnapshot(ctx, scope)
	require.Error(t, err)
	require.Less(t, time.Since(start), 2*time.Second, "schema lock waits cannot escape the whole read deadline")
}

func TestGISAuthoritySourceRefusesAmbiguousRetainedFloor(t *testing.T) {
	ctx, s := gisSourceFixture(t)
	_, err := s.Pool.Exec(ctx, `ALTER TABLE gis_authority_revision DROP CONSTRAINT gis_authority_revision_pkey; INSERT INTO gis_authority_revision VALUES(true,1)`)
	require.NoError(t, err)
	require.Error(t, s.CheckAuthoritySourceSchema(ctx), "a noncanonical retained floor cannot qualify")
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString(), EnvironmentId: uuid.NewString()}
	_, err = s.ReadAuthoritySnapshot(ctx, scope)
	require.Error(t, err)
	_, err = s.ReadAuthorityRevision(ctx, scope)
	require.Error(t, err)
}

func TestGISAuthoritySourceSnapshotMissingClosedAndMaintenance(t *testing.T) {
	ctx, s := gisSourceFixture(t)
	pool, trace := tracedTestPool(t, ctx, s.Pool)
	s = &Store{Pool: pool}
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString(), EnvironmentId: uuid.NewString(), ProfileIds: []string{uuid.NewString()}}
	require.NoError(t, s.CheckAuthoritySourceSchema(ctx))
	missing, err := s.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.True(t, missing.Complete)
	state, err := authoritysource.DecodeGISState(missing.CanonicalState)
	require.NoError(t, err)
	require.False(t, state.EnvironmentExists)
	require.Equal(t, scope.ProfileIds, state.Subjects)
	rev, err := s.ReadAuthorityRevision(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, missing.Revision, rev)
	app := uuid.NewString()
	_, err = s.Pool.Exec(ctx, `INSERT INTO applications(id,owner_account_id,name,status) VALUES($1,$2,'secret display','sandbox')`, app, uuid.New())
	require.NoError(t, err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO environments(id,application_id,kind,status,provider_policy,redirect_uris) VALUES($1,$2,'sandbox','active','{"providers":["google"],"player_scopes":["game.chat.read"]}','["https://game.example/cb"]')`, scope.EnvironmentId, app)
	require.NoError(t, err)
	binding, chat := uuid.NewString(), uuid.NewString()
	_, err = s.Pool.Exec(ctx, `INSERT INTO player_bindings(binding_id,application_id,environment_id,provider,provider_subject_digest,account_id,actor_id,profile_id,device_id,status,authority_revision) VALUES($1,$2,$3,'google','hmac-sha256-v1:current:'||repeat('a',64),$4,$5,$5,$6,'active',1)`, binding, app, scope.EnvironmentId, uuid.New(), scope.ProfileIds[0], uuid.New())
	require.NoError(t, err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO game_resource_binding_chats(application_id,environment_id,binding_id,chat_id,roster_revision,status,lease_expires_at) VALUES($1,$2,$3,$4,1,'active',clock_timestamp()+interval '2 seconds')`, app, scope.EnvironmentId, binding, chat)
	require.NoError(t, err)
	live, err := s.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.Greater(t, live.Revision, rev)
	require.Positive(t, live.ValidUntilUnixMillis)
	require.NotContains(t, string(live.CanonicalState), "secret display")
	require.NotContains(t, string(live.CanonicalState), "hmac-sha256")
	time.Sleep(time.Until(time.UnixMilli(live.ValidUntilUnixMillis)) + 10*time.Millisecond)
	readStart := time.Now()
	trace.event("authority.expired.enter", readStart, nil)
	expired, err := s.ReadAuthoritySnapshot(ctx, scope)
	trace.event("authority.expired.return", readStart, err)
	require.NoError(t, err)
	require.Equal(t, live.CanonicalState, expired.CanonicalState)
	require.Equal(t, live.Revision, expired.Revision)
	require.Zero(t, expired.ValidUntilUnixMillis)
	_, err = s.Pool.Exec(ctx, `UPDATE player_bindings SET status='revoking',authority_revision=authority_revision+1 WHERE binding_id=$1`, binding)
	require.NoError(t, err)
	revoking, err := s.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.Greater(t, revoking.Revision, live.Revision)
	_, err = s.Pool.Exec(ctx, `UPDATE player_bindings SET status='revoked' WHERE binding_id=$1`, binding)
	require.NoError(t, err)
	revoked, err := s.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.Greater(t, revoked.Revision, revoking.Revision, "status-only finalization must invalidate source")
	_, err = s.Pool.Exec(ctx, `UPDATE environments SET status='retired' WHERE id=$1`, scope.EnvironmentId)
	require.NoError(t, err)
	closed, err := s.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	state, err = authoritysource.DecodeGISState(closed.CanonicalState)
	require.NoError(t, err)
	require.True(t, state.EnvironmentExists)
	require.Equal(t, "retired", state.EnvironmentStatus)
	_, err = s.Pool.Exec(ctx, `UPDATE schema_migrations SET dirty=true`)
	require.NoError(t, err)
	require.Error(t, s.CheckAuthoritySourceSchema(ctx))
	_, err = s.ReadAuthoritySnapshot(ctx, scope)
	require.Error(t, err)
	_, err = s.ReadAuthorityRevision(ctx, scope)
	require.Error(t, err)
}

func TestGISAuthoritySourceRejectsUnrepresentedAuthority(t *testing.T) {
	ctx, s := gisSourceFixture(t)
	app, foreign, env, profile := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, id := range []string{app, foreign} {
		_, err := s.Pool.Exec(ctx, `INSERT INTO applications(id,owner_account_id,name,status) VALUES($1,$2,'source','sandbox')`, id, uuid.New())
		require.NoError(t, err)
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO environments(id,application_id,kind,status) VALUES($1,$2,'sandbox','active')`, env, app)
	require.NoError(t, err)
	binding := uuid.NewString()
	_, err = s.Pool.Exec(ctx, `INSERT INTO player_bindings(binding_id,application_id,environment_id,provider,provider_subject_digest,account_id,actor_id,profile_id,device_id,status,authority_revision) VALUES($1,$2,$3,'google','hmac-sha256-v1:current:'||repeat('a',64),$4,$5,$5,$6,'active',1)`, binding, foreign, env, uuid.New(), profile, uuid.New())
	require.NoError(t, err)
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString(), EnvironmentId: env, ProfileIds: []string{profile}}
	_, err = s.ReadAuthoritySnapshot(ctx, scope)
	require.Error(t, err, "inconsistent app/environment tuple cannot be silently omitted")
	_, err = s.Pool.Exec(ctx, `UPDATE player_bindings SET application_id=$2,permitted_character_context='["unqualified"]' WHERE binding_id=$1`, binding, app)
	require.NoError(t, err)
	_, err = s.ReadAuthoritySnapshot(ctx, scope)
	require.Error(t, err, "unknown character authority cannot be silently dropped")
}

func TestGISAuthoritySourceInstallationResourceSessionAndClockCoverage(t *testing.T) {
	ctx, s := gisSourceFixture(t)
	app, env, profile, other := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	_, err := s.Pool.Exec(ctx, `INSERT INTO applications(id,owner_account_id,name,status) VALUES($1,$2,'private name','sandbox')`, app, uuid.New())
	require.NoError(t, err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO environments(id,application_id,kind,status) VALUES($1,$2,'sandbox','active')`, env, app)
	require.NoError(t, err)
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString(), EnvironmentId: env, ProfileIds: []string{profile}}
	install, bot, mapping, space, session, chat, room := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	_, err = s.Pool.Exec(ctx, `INSERT INTO installations(id,application_id,environment_id,callback_url,bot_id) VALUES($1,$2,$3,'https://private.example/secret-callback',$4)`, install, app, env, bot)
	require.NoError(t, err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO game_resource_mappings(mapping_id,application_id,environment_id,external_key,resource_kind,resource_id,status,mapping_revision) VALUES($1,$2,$3,'private external key','space',$4,'active',2)`, mapping, app, env, space)
	require.NoError(t, err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO gis_sessions(id,application_id,environment_id,kind,external_key,session_status,stage,roster_revision,roster_complete,members,chat_id,voice_room_id) VALUES($1,$2,$3,'match','secret session key','active','active',3,true,ARRAY[$4::uuid,$5::uuid],$6,$7)`, session, app, env, profile, other, chat, room)
	require.NoError(t, err)
	raw, err := s.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	state, err := authoritysource.DecodeGISState(raw.CanonicalState)
	require.NoError(t, err)
	require.Equal(t, []authoritysource.GISInstallation{{InstallationID: install, BotID: bot, Status: "active"}}, state.Installations)
	require.Equal(t, []authoritysource.GISMapping{{MappingID: mapping, Kind: "space", ResourceID: space, Status: "active", Revision: 2}}, state.Mappings)
	require.Len(t, state.Sessions, 1)
	require.Equal(t, []string{profile}, state.Sessions[0].Members)
	require.NotContains(t, string(raw.CanonicalState), other)
	require.NotContains(t, string(raw.CanonicalState), "secret")
	require.NotContains(t, string(raw.CanonicalState), "private")
	for _, table := range gisAuthorityTables {
		before, err := s.ReadAuthorityRevision(ctx, scope)
		require.NoError(t, err)
		// Statement triggers cover maintenance writers even when there are no rows.
		_, err = s.Pool.Exec(ctx, `DELETE FROM public.`+table+` WHERE false`)
		require.NoError(t, err)
		after, err := s.ReadAuthorityRevision(ctx, scope)
		require.NoError(t, err)
		require.Greater(t, after, before, table)
	}
	_, err = s.Pool.Exec(ctx, `ALTER TABLE gis_sessions DISABLE TRIGGER gis_authority_changed`)
	require.NoError(t, err)
	_, err = s.ReadAuthorityRevision(ctx, scope)
	require.Error(t, err)
	_, err = s.Pool.Exec(ctx, `ALTER TABLE gis_sessions ENABLE TRIGGER gis_authority_changed`)
	require.NoError(t, err)
	_, err = s.Pool.Exec(ctx, `CREATE OR REPLACE FUNCTION gis_authority_changed() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RETURN NULL; END'`)
	require.NoError(t, err)
	_, err = s.ReadAuthoritySnapshot(ctx, scope)
	require.Error(t, err, "same-name tampered clock function cannot qualify")
}
