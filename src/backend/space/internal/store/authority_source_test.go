package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/authoritysource"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
)

func spaceSourceStore(t *testing.T) (context.Context, *SpaceStore) {
	t.Helper()
	ctx, st := authorityRevisionSpace(t)
	// Complete DDL fixture with private matching marker. Actual deployment-loader
	// proof is separate; this helper never rewrites a live runtime's history.
	_, err := st.Pool.Exec(ctx, `CREATE TABLE schema_migrations(version bigint NOT NULL,dirty boolean NOT NULL); INSERT INTO schema_migrations VALUES(24,false)`)
	require.NoError(t, err)
	return ctx, st
}

func TestSpaceSourceCompleteMembershipAndStableLeaseExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL complete owning snapshot")
	}
	ctx, st := spaceSourceStore(t)
	owner, manual, roster, outsider := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	space, err := st.CreateSpace(ctx, owner, "source lease", "", "public")
	require.NoError(t, err)
	control, err := st.CreateSpace(ctx, uuid.New(), "control source", "", "private")
	require.NoError(t, err)
	room := uuid.New()
	_, err = st.Pool.Exec(ctx, `INSERT INTO voice_rooms(id,space_id,name) VALUES($1,$2,'source room')`, room, space.ID)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, manual)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE spaces SET allow_guests=true WHERE id=$1`, space.ID)
	require.NoError(t, err)
	ownerLease, memberLease := time.Now().UTC().Add(time.Minute), time.Now().UTC().Add(30*time.Second)
	_, err = st.Pool.Exec(ctx, `INSERT INTO community_owner_authority(space_id,application_id,environment_id,corporation_key,owner_account_id,owner_profile_id,owner_generation,status,roster_source_revision,roster_sha256,roster_lease_expires_at) VALUES($1,$2,$3,'source',$4,$5,2,'active',3,decode(repeat('00',32),'hex'),$6)`, space.ID, uuid.New(), uuid.New(), uuid.New(), owner, ownerLease)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO community_roster_members(space_id,profile_id,source_rank,source_reasons,source_revision,owner_generation,lease_expires_at) VALUES($1,$2,'member',ARRAY['membership'],3,2,$3)`, space.ID, roster, memberLease)
	require.NoError(t, err)
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.ID.String()}
	snapshot, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.True(t, snapshot.Complete)
	state, err := authoritysource.DecodeSpaceState(snapshot.CanonicalState)
	require.NoError(t, err)
	for _, profile := range []uuid.UUID{owner, manual, roster} {
		access, err := st.ResolveVoiceRoomAccess(ctx, space.ID, room, profile)
		require.NoError(t, err)
		require.True(t, access.Member)
		require.True(t, state.MemberAt(profile.String(), time.Now().UnixMilli()))
	}
	_, err = st.ResolveVoiceRoomAccess(ctx, space.ID, room, outsider)
	require.ErrorIs(t, err, ErrVoiceRoomNotFound)
	require.False(t, state.MemberAt(outsider.String(), time.Now().UnixMilli()))
	require.Equal(t, memberLease.UnixMilli(), snapshot.ValidUntilUnixMillis)
	require.False(t, state.MemberAt(roster.String(), snapshot.ValidUntilUnixMillis))
	require.True(t, state.MemberAt(manual.String(), snapshot.ValidUntilUnixMillis))
	again, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, snapshot.Revision, again.Revision)
	require.Equal(t, snapshot.CanonicalState, again.CanonicalState, "elapsed clock must not rewrite canonical payload")
	controlState, err := st.ReadAuthoritySnapshot(ctx, &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: control.ID.String()})
	require.NoError(t, err)
	decodedControl, err := authoritysource.DecodeSpaceState(controlState.CanonicalState)
	require.NoError(t, err)
	require.Empty(t, decodedControl.CommunityMembers)
	require.Empty(t, decodedControl.VoiceRooms)
}

func TestSpaceSourceRevisionCutIncludesBansTimeoutsAndLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL consistent cut")
	}
	ctx, st := spaceSourceStore(t)
	owner, account := uuid.New(), uuid.New()
	space, err := st.CreateSpace(ctx, owner, "source cut", "", "private")
	require.NoError(t, err)
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.ID.String()}
	tx, err := st.beginAuthorityRead(ctx, scope)
	require.NoError(t, err)
	defer authoritysource.RollbackRead(ctx, tx)
	before, err := spaceSourceRevision(ctx, tx, scope.SpaceId)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_bans(space_id,account_id,banned_by_profile_id) VALUES($1,$2,$3)`, space.ID, account, owner)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_member_timeouts(space_id,profile_id,timed_out_until,timed_out_by_profile_id) VALUES($1,$2,now()+interval '1 minute',$2)`, space.ID, owner)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_lifecycle_aggregates(space_id,deletion_operation_id,phase,generation) VALUES($1,$2,'FREEZE_PENDING',1)`, space.ID, uuid.New())
	require.NoError(t, err)
	old, err := spaceSourceRevision(ctx, tx, scope.SpaceId)
	require.NoError(t, err)
	require.Equal(t, before, old)
	var oldBanCount int
	require.NoError(t, tx.QueryRow(ctx, `SELECT count(*) FROM space_bans WHERE space_id=$1`, space.ID).Scan(&oldBanCount))
	require.Zero(t, oldBanCount, "one cut cannot mix old counter with new ban")
	require.NoError(t, tx.Commit(ctx))
	current, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.Greater(t, current.Revision, before)
	state, err := authoritysource.DecodeSpaceState(current.CanonicalState)
	require.NoError(t, err)
	require.Equal(t, []string{account.String()}, state.BannedAccounts)
	require.Len(t, state.Timeouts, 1)
	require.Equal(t, "FREEZE_PENDING", state.DeletionPhase)
	require.False(t, state.MemberAt(owner.String(), time.Now().UnixMilli()))
}

func TestSpaceSourceFailsClosedForMissingFloorAndCrossScopeTree(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL source guards")
	}
	ctx, st := spaceSourceStore(t)
	_, err := st.ReadAuthoritySnapshot(ctx, &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString()})
	require.Error(t, err, "no fallback for unknown Space floor")
	space, err := st.CreateSpace(ctx, uuid.New(), "source tree", "", "private")
	require.NoError(t, err)
	control, err := st.CreateSpace(ctx, uuid.New(), "source foreign category", "", "private")
	require.NoError(t, err)
	category := uuid.New()
	_, err = st.Pool.Exec(ctx, `INSERT INTO categories(id,space_id,name) VALUES($1,$2,'control')`, category, control.ID)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_tree_nodes(space_id,category_id,kind,chat_id) VALUES($1,$2,'text_chat',$3)`, space.ID, category, uuid.New())
	require.NoError(t, err)
	_, err = st.ReadAuthoritySnapshot(ctx, &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.ID.String()})
	require.Error(t, err, "a same-database FK cannot establish exact Space scope")
	_, err = st.Pool.Exec(ctx, `DELETE FROM space_tree_nodes WHERE space_id=$1`, space.ID)
	require.NoError(t, err)
	// Model a known closed scope without bypassing the production P3 deletion
	// guard. This is a source-state fixture, not a lifecycle purge receipt proof.
	closedScope := uuid.NewString()
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_voice_access_epochs(space_id,access_epoch) VALUES($1,1)`, closedScope)
	require.NoError(t, err)
	closed, err := st.ReadAuthoritySnapshot(ctx, &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: closedScope})
	require.NoError(t, err, "retained known floor provides complete closed state")
	var state authoritysource.SpaceState
	require.NoError(t, json.Unmarshal(closed.CanonicalState, &state))
	require.False(t, state.Exists)
	_, err = st.ReadAuthorityRevision(ctx, &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: closedScope})
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE schema_migrations SET dirty=true`)
	require.NoError(t, err)
	require.Error(t, st.CheckAuthoritySourceSchema(ctx))
}

func TestSpaceSourceExpiredRosterPreservesRawStateAndRevision(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL time-derived membership")
	}
	ctx, st := spaceSourceStore(t)
	owner, roster := uuid.New(), uuid.New()
	space, err := st.CreateSpace(ctx, owner, "source expires", "", "private")
	require.NoError(t, err)
	var until time.Time
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT clock_timestamp()+interval '700 milliseconds'`).Scan(&until))
	_, err = st.Pool.Exec(ctx, `INSERT INTO community_owner_authority(space_id,application_id,environment_id,corporation_key,owner_account_id,owner_profile_id,owner_generation,status,roster_source_revision,roster_sha256,roster_lease_expires_at) VALUES($1,$2,$3,'expiry',$4,$5,1,'active',1,decode(repeat('00',32),'hex'),$6)`, space.ID, uuid.New(), uuid.New(), uuid.New(), owner, until)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO community_roster_members(space_id,profile_id,source_rank,source_reasons,source_revision,owner_generation,lease_expires_at) VALUES($1,$2,'member',ARRAY['membership'],1,1,$3)`, space.ID, roster, until)
	require.NoError(t, err)
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.ID.String()}
	before, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, until.UnixMilli(), before.ValidUntilUnixMillis)
	// Observe the database clock crossing the saved lease; perform no mutation.
	require.Eventually(t, func() bool {
		var expired bool
		if st.Pool.QueryRow(ctx, `SELECT clock_timestamp()>$1`, until).Scan(&expired) != nil {
			return false
		}
		return expired
	}, 2*time.Second, 20*time.Millisecond)
	after, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, before.Revision, after.Revision)
	require.Equal(t, before.CanonicalState, after.CanonicalState)
	require.Zero(t, after.ValidUntilUnixMillis)
	state, err := authoritysource.DecodeSpaceState(after.CanonicalState)
	require.NoError(t, err)
	require.False(t, state.MemberAt(roster.String(), time.Now().UnixMilli()))
	require.True(t, state.MemberAt(owner.String(), time.Now().UnixMilli()))
}
