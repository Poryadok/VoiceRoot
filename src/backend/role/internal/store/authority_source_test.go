package store

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/authoritysource"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/role/permissions"
)

func roleSourceStore(t *testing.T) (context.Context, *RoleStore) {
	t.Helper()
	ctx, st := authorityRevisionRole(t)
	// These tests apply complete owner DDL directly, then record the matching
	// clean source marker. Pinned migrator acceptance is a separate test.
	_, err := st.Pool.Exec(ctx, `UPDATE schema_migrations SET version=15,dirty=false`)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, role14SQL(t, "000016_sdk_authority_revision.up.sql"))
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE schema_migrations SET version=16,dirty=false`)
	require.NoError(t, err)
	return ctx, st
}

func TestRoleSourceRevisionChangesWithSdkGrantReplacementAndRevocation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL complete Role owner revision")
	}
	ctx, st := roleSourceStore(t)
	space, room, profile := uuid.New(), uuid.New(), uuid.New()
	_, err := st.Pool.Exec(ctx, `INSERT INTO roles(space_id,name) VALUES($1,'source first')`, space)
	require.NoError(t, err)
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.String(), VoiceRoomIds: []string{room.String()}}
	before, err := st.ReadAuthorityRevision(ctx, scope)
	require.NoError(t, err)
	request := roleGrantRequest(uuid.New(), uuid.New(), uuid.New(), room, uuid.New(), 1, []uuid.UUID{profile}, sha256.Sum256([]byte("source grant")))
	_, err = st.ApplyGameSessionGrants(ctx, request)
	require.NoError(t, err)
	after, err := st.ReadAuthorityRevision(ctx, scope)
	require.NoError(t, err)
	require.Greater(t, after, before, "SDK grant changes must invalidate a complete Role source read")
	request.OperationID = uuid.New()
	request.RosterRevision = 2
	request.ProfileIDs = nil
	request.RequestSHA256 = sha256.Sum256([]byte("remove profile"))
	_, err = st.ApplyGameSessionGrants(ctx, request)
	require.NoError(t, err)
	removed, err := st.ReadAuthorityRevision(ctx, scope)
	require.NoError(t, err)
	require.Greater(t, removed, after)
	_, err = st.RevokeGameSessionGrants(ctx, request.ApplicationID, request.EnvironmentID, request.SessionID, uuid.New(), sha256.Sum256([]byte("revoke")))
	require.NoError(t, err)
	revoked, err := st.ReadAuthorityRevision(ctx, scope)
	require.NoError(t, err)
	require.Greater(t, revoked, removed)
}

func TestRoleSourceCompleteSnapshotMatchesCanonicalPermissionFoldAndScope(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL complete owning snapshot")
	}
	ctx, st := roleSourceStore(t)
	space, control, room, otherRoom, chat := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	member, custom, owner := uuid.New(), uuid.New(), uuid.New()
	actor, ownerProfile, outsider := uuid.New(), uuid.New(), uuid.New()
	join, err := permissions.MaskFor(permissions.VoiceJoin)
	require.NoError(t, err)
	speak, err := permissions.MaskFor(permissions.VoiceSpeak)
	require.NoError(t, err)
	history, err := permissions.MaskFor(permissions.TextChatReadHistory)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO roles(id,space_id,name,permissions,is_default_join) VALUES
	 ($1,$4,'Member',$5,true),($2,$4,'custom',$6,false),($3,$4,'Owner',0,false)`, member, custom, owner, space, join, speak)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO roles(space_id,name) VALUES($1,'private control role')`, control)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) VALUES($1,$2,$3,$2),($1,$4,$5,$4)`, space, actor, custom, ownerProfile, owner)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO voice_room_overrides(voice_room_id,role_id,deny) VALUES($1,$2,$3)`, room, custom, speak)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO chat_overrides(chat_id,role_id,allow) VALUES($1,$2,$3)`, chat, custom, history)
	require.NoError(t, err)
	app, env, session := uuid.New(), uuid.New(), uuid.New()
	_, err = st.ApplyGameSessionGrants(ctx, roleGrantRequest(app, env, session, room, uuid.New(), 1, []uuid.UUID{actor}, sha256.Sum256([]byte("scoped source grant"))))
	require.NoError(t, err)
	_, err = st.ApplyGameSessionGrants(ctx, roleGrantRequest(app, env, uuid.New(), otherRoom, uuid.New(), 1, []uuid.UUID{outsider}, sha256.Sum256([]byte("control grant"))))
	require.NoError(t, err)
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.String(), VoiceRoomIds: []string{room.String()}}
	read, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.True(t, read.Complete)
	state, err := authoritysource.DecodeRoleState(read.CanonicalState)
	require.NoError(t, err)
	require.Len(t, state.Roles, 3)
	require.Len(t, state.Assignments, 2)
	require.Len(t, state.SDKSessionGrants, 1)
	require.Equal(t, app.String(), state.SDKSessionGrants[0].ApplicationID)
	require.Equal(t, state.RoleRevision+state.SDKRevision, read.Revision)
	for _, profile := range []uuid.UUID{actor, ownerProfile, outsider} {
		for _, kind := range []bool{false, true} {
			var canonical uint64
			if kind {
				canonical, err = st.GetEffectiveMask(ctx, space, profile, nil, &room)
			} else {
				canonical, err = st.GetEffectiveMask(ctx, space, profile, &chat, nil)
			}
			require.NoError(t, err)
			resource := chat.String()
			if kind {
				resource = room.String()
			}
			require.Equal(t, canonical, state.EffectiveMask(profile.String(), resource, kind))
		}
	}
	revision, err := st.ReadAuthorityRevision(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, read.Revision, revision)
	restarted := &RoleStore{Pool: st.Pool}
	replay, err := restarted.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, read, replay)
	_, err = st.Pool.Exec(ctx, `DELETE FROM member_roles WHERE space_id=$1 AND profile_id=$2`, space, actor)
	require.NoError(t, err)
	newRead, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.Greater(t, newRead.Revision, read.Revision)
	newState, err := authoritysource.DecodeRoleState(newRead.CanonicalState)
	require.NoError(t, err)
	require.Zero(t, newState.EffectiveMask(actor.String(), room.String(), true))
}

func TestRoleSourceRejectsDirtyFilteredDisabledAndAlteredCatalogs(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL fail-closed source catalogs")
	}
	for name, mutation := range map[string]string{
		"dirty":               `UPDATE schema_migrations SET dirty=true`,
		"unknown version":     `UPDATE schema_migrations SET version=17`,
		"filtered SDK source": `ALTER TABLE game_session_grants ENABLE ROW LEVEL SECURITY`,
		"inactive policy":     `CREATE POLICY hidden_members ON member_roles USING(false)`,
		"disabled counter":    `ALTER TABLE game_session_grants DISABLE TRIGGER role_sdk_authority_grants`,
		"altered function":    `CREATE OR REPLACE FUNCTION role_sdk_authority_changed() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$`,
		"unlogged source":     `ALTER TABLE member_roles SET UNLOGGED`,
	} {
		t.Run(name, func(t *testing.T) {
			ctx, st := roleSourceStore(t)
			space := uuid.New()
			_, err := st.Pool.Exec(ctx, `INSERT INTO roles(space_id,name) VALUES($1,'source guard')`, space)
			require.NoError(t, err)
			scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.String()}
			_, err = st.ReadAuthoritySnapshot(ctx, scope)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, mutation)
			require.NoError(t, err)
			read, err := st.ReadAuthoritySnapshot(ctx, scope)
			require.Error(t, err)
			require.False(t, read.Complete)
			revision, err := st.ReadAuthorityRevision(ctx, scope)
			require.Error(t, err)
			require.Zero(t, revision)
		})
	}
}

func TestRoleSourceRejectsCrossSpaceAssignmentsAndUnknownPermissionBits(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL malformed owner state")
	}
	for _, name := range []string{"cross-space assignment", "unknown bit"} {
		t.Run(name, func(t *testing.T) {
			ctx, st := roleSourceStore(t)
			space, other, role := uuid.New(), uuid.New(), uuid.New()
			_, err := st.Pool.Exec(ctx, `INSERT INTO roles(id,space_id,name) VALUES($1,$2,'source malformed')`, role, space)
			require.NoError(t, err)
			if name == "cross-space assignment" {
				_, err = st.Pool.Exec(ctx, `INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) VALUES($1,$2,$3,$2)`, other, uuid.New(), role)
			} else {
				_, err = st.Pool.Exec(ctx, `UPDATE roles SET permissions=(1::bigint<<62) WHERE id=$1`, role)
			}
			require.NoError(t, err)
			read, err := st.ReadAuthoritySnapshot(ctx, &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.String()})
			require.Error(t, err)
			require.False(t, read.Complete)
		})
	}
}

func TestRoleSourceReadCapturesStateAndRevisionInOneRepeatableCut(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL concurrent owner snapshot")
	}
	ctx, st := roleSourceStore(t)
	space, role, room, profile := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	join, err := permissions.MaskFor(permissions.VoiceJoin)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO roles(id,space_id,name,permissions) VALUES($1,$2,'concurrent source',$3)`, role, space, join)
	require.NoError(t, err)
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.String(), VoiceRoomIds: []string{room.String()}}
	tx, err := st.beginAuthorityRead(ctx, scope)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	roleBefore, sdkBefore, err := roleSourceRevisions(ctx, tx, space.String())
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE roles SET permissions=0 WHERE id=$1`, role)
	require.NoError(t, err)
	_, err = st.ApplyGameSessionGrants(ctx, roleGrantRequest(uuid.New(), uuid.New(), uuid.New(), room, uuid.New(), 1, []uuid.UUID{profile}, sha256.Sum256([]byte("concurrent grant"))))
	require.NoError(t, err)
	var observed uint64
	require.NoError(t, tx.QueryRow(ctx, `SELECT permissions FROM roles WHERE id=$1`, role).Scan(&observed))
	require.Equal(t, join, observed)
	var rows int
	require.NoError(t, tx.QueryRow(ctx, `SELECT count(*) FROM game_session_grants WHERE voice_room_id=$1`, room).Scan(&rows))
	require.Zero(t, rows)
	roleAfter, sdkAfter, err := roleSourceRevisions(ctx, tx, space.String())
	require.NoError(t, err)
	require.Equal(t, roleBefore, roleAfter)
	require.Equal(t, sdkBefore, sdkAfter)
	require.NoError(t, tx.Commit(ctx))
	fresh, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	state, err := authoritysource.DecodeRoleState(fresh.CanonicalState)
	require.NoError(t, err)
	require.Greater(t, state.RoleRevision, roleBefore)
	require.Greater(t, state.SDKRevision, sdkBefore)
	require.Zero(t, state.Roles[0].Permissions)
	require.Len(t, state.SDKSessionGrants, 1)
}

func TestRoleSourceSdkClockCannotRewindDisappearOrBypassMutationInvalidation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL persistent SDK source clock")
	}
	ctx, st := roleSourceStore(t)
	request := roleGrantRequest(uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), 1, []uuid.UUID{uuid.New()}, sha256.Sum256([]byte("SDK floor guard")))
	_, err := st.ApplyGameSessionGrants(ctx, request)
	require.NoError(t, err)
	var before int64
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT revision FROM role_sdk_authority_revision`).Scan(&before))
	require.Greater(t, before, int64(1))
	for _, sql := range []string{
		`UPDATE role_sdk_authority_revision SET revision=1`,
		`UPDATE role_sdk_authority_revision SET revision=revision+2`,
		`DELETE FROM role_sdk_authority_revision`,
		`TRUNCATE role_sdk_authority_revision`,
		`TRUNCATE game_session_grants`,
		`TRUNCATE game_session_grants,game_session_grant_sessions`,
		role14SQL(t, "000016_sdk_authority_revision.down.sql"),
	} {
		_, err = st.Pool.Exec(ctx, sql)
		require.Error(t, err)
		var after int64
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT revision FROM role_sdk_authority_revision`).Scan(&after))
		require.Equal(t, before, after)
		allowed, err := st.CheckGameSessionGrant(ctx, request.ApplicationID, request.EnvironmentID, request.SessionID, request.VoiceRoomID, request.ProfileIDs[0])
		require.NoError(t, err)
		require.True(t, allowed)
	}
}

func TestRoleSourceSdkMigrationFailureRetainsOldCatalogAndSavedEvidence(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL atomic SDK authority activation")
	}
	for name, mutation := range map[string]string{
		"filtered source":         `ALTER TABLE game_session_grants ENABLE ROW LEVEL SECURITY`,
		"invalid marker":          `UPDATE schema_migrations SET version=14`,
		"late activation failure": `CREATE FUNCTION role_sdk_authority_changed() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$`,
	} {
		t.Run(name, func(t *testing.T) {
			ctx, st := authorityRevisionRole(t)
			_, err := st.Pool.Exec(ctx, `UPDATE schema_migrations SET version=15,dirty=false`)
			require.NoError(t, err)
			request := roleGrantRequest(uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), 1, []uuid.UUID{uuid.New()}, sha256.Sum256([]byte("saved SDK migration state")))
			_, err = st.ApplyGameSessionGrants(ctx, request)
			require.NoError(t, err)
			before := map[string]string{}
			for _, table := range []string{"game_session_grant_sessions", "game_session_grants", "game_session_grant_operations"} {
				before[table] = role14Rows(t, ctx, st.Pool, table)
			}
			_, err = st.Pool.Exec(ctx, mutation)
			require.NoError(t, err)
			conn, err := st.Pool.Acquire(ctx)
			require.NoError(t, err)
			defer conn.Release()
			_, err = conn.Exec(ctx, role14SQL(t, "000016_sdk_authority_revision.up.sql"))
			require.Error(t, err)
			_, err = conn.Exec(ctx, `ROLLBACK`)
			require.NoError(t, err)
			conn.Release()
			var exists bool
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT to_regclass('public.role_sdk_authority_revision') IS NOT NULL`).Scan(&exists))
			require.False(t, exists)
			for table, rows := range before {
				require.True(t, role14Rows(t, ctx, st.Pool, table) == rows, "failed activation preserves exact saved table rows")
			}
		})
	}
}
