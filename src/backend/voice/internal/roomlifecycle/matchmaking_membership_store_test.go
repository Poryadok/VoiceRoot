package roomlifecycle

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMatchmakingMembershipStoreSchemaAndReads(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL membership read contract requires testcontainers")
	}
	ctx := context.Background()
	pool := r22StartVoicePostgres(t, ctx, "mmschemareads")
	store := NewPostgresLifecycleStore(pool)
	require.NoError(t, store.CheckSchema(ctx))
	require.ErrorIs(t, store.CheckMatchmakingMembershipSchema(ctx), ErrUnavailable)
	_, _, err := store.LoadMembershipIdentity(ctx, uuid.New())
	require.ErrorIs(t, err, ErrUnavailable)
	_, _, err = store.LoadMatchmakingRoom(ctx, uuid.New())
	require.ErrorIs(t, err, ErrUnavailable)

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	seed, err := r22TrySeedMigrationRows(ctx, tx, nil)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	_, err = pool.Exec(ctx, mmReadMigration(t, "up"))
	require.NoError(t, err)
	require.NoError(t, store.CheckSchema(ctx))
	require.NoError(t, store.CheckMatchmakingMembershipSchema(ctx))
	_, found, err := store.LoadMembershipIdentity(ctx, uuid.New())
	require.NoError(t, err)
	require.False(t, found)
	_, found, err = store.LoadMatchmakingRoom(ctx, uuid.New())
	require.NoError(t, err)
	require.False(t, found)
	_, found, err = store.LoadMembershipIdentity(ctx, seed.membershipID)
	require.ErrorIs(t, err, ErrInvariant, "unknown legacy state cannot become SOLO or a usable membership")
	require.False(t, found)

	legacy, found, err := store.LoadMembership(ctx, seed.membershipID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, seed.membershipEpoch, legacy.MediaEpoch)
	account := uuid.New()
	_, err = pool.Exec(ctx, `UPDATE voice_room_memberships SET account_id=$1,session_epoch=7,membership_state='JOINED' WHERE profile_id=$2`, account, seed.membershipID)
	require.NoError(t, err)
	identity, found, err := store.LoadMembershipIdentity(ctx, seed.membershipID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, account, identity.AccountID)
	require.Equal(t, int64(7), identity.SessionEpoch)
	require.Equal(t, seed.membershipID, identity.ProfileID)
	require.Equal(t, seed.destinationRoomID, identity.RoomID)
	require.Equal(t, seed.membershipEpoch, identity.MediaEpoch)
	require.Equal(t, MembershipJoined, identity.State)
	room, found, err := store.LoadMatchmakingRoom(ctx, seed.destinationRoomID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "voice_room", room.RoomType)
	require.Equal(t, "ORDINARY", room.Purpose)
	require.Equal(t, seed.spaceID, *room.SpaceID)
	require.Equal(t, seed.destinationLogicalRoomID, *room.VoiceRoomID)
	require.Nil(t, room.ChatID)
	require.True(t, room.Active)

	_, err = pool.Exec(ctx, `UPDATE voice_room_memberships SET membership_state='RECONNECTING',reconnect_started_at=updated_at,reconnect_deadline=updated_at+interval '30 seconds' WHERE profile_id=$1`, seed.membershipID)
	require.NoError(t, err)
	identity, found, err = store.LoadMembershipIdentity(ctx, seed.membershipID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, MembershipReconnecting, identity.State)
	require.NotNil(t, identity.ReconnectStartedAt)
	require.NotNil(t, identity.ReconnectDeadline)
	// Stored state is evidence, not a current eligibility verdict. This old
	// fixture's deadline is intentionally in the past; RPC policy must check it.
	for _, tc := range []struct {
		kind  string
		match bool
	}{{"call", false}, {"group_voice", false}, {"group_voice", true}} {
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		id, err := mmInsertRoom(ctx, tx, tc.kind, tc.match, nil)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
		room, found, err := store.LoadMatchmakingRoom(ctx, id)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, tc.kind, room.RoomType)
		require.NotNil(t, room.ChatID)
		require.Nil(t, room.SpaceID)
		require.Nil(t, room.VoiceRoomID)
		if tc.match {
			require.Equal(t, "MATCH_SQUAD", room.Purpose)
			require.NotNil(t, room.OwnerID)
			require.NotNil(t, room.CreationOperationID)
			require.NotNil(t, room.CreationReceiptID)
			require.NotNil(t, room.ChatCreationReceiptID)
			require.Len(t, room.CreationManifestHash, 32)
		} else {
			require.Equal(t, "ORDINARY", room.Purpose)
		}
		_, err = pool.Exec(ctx, `UPDATE voice_room_instances SET state='closed',closed_at=now() WHERE room_id=$1`, id)
		require.NoError(t, err)
		closed, found, err := store.LoadMatchmakingRoom(ctx, id)
		require.NoError(t, err)
		require.True(t, found)
		require.False(t, closed.Active)
		room.Active = false
		require.Equal(t, room, closed)
	}
}
