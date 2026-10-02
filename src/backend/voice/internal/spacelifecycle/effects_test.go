package spacelifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type recordingMediaLifecycle struct {
	removed   []string
	closed    []string
	removeErr error
}

func (m *recordingMediaLifecycle) RemoveParticipant(_ context.Context, room, identity string) error {
	m.removed = append(m.removed, room+"/"+identity)
	return m.removeErr
}

func (m *recordingMediaLifecycle) CloseRoom(_ context.Context, room string) error {
	m.closed = append(m.closed, room)
	return nil
}

func TestPostgresSpaceLifecycleEffectsEjectThenCloseAndAreRetrySafe(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceLifecyclePostgres(t, ctx)
	spaceID, roomID, voiceRoomID, profileID, mediaEpoch := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	roomName := "voice-room-" + voiceRoomID.String()
	now := time.Now().UTC()
	_, err := pool.Exec(ctx, `INSERT INTO voice_room_instances(room_id,space_id,voice_room_id,livekit_room_name,state,roster_version,created_at,updated_at)
VALUES($1,$2,$3,$4,'active',1,$5,$5)`, roomID, spaceID, voiceRoomID, roomName, now)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO voice_room_memberships(profile_id,room_id,media_epoch,space_access_epoch,role_policy_epoch,authorization_digest,
can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,joined_at,updated_at)
VALUES($1,$2,$3,1,1,$4,true,true,false,false,true,false,false,false,false,false,$5,$5)`, profileID, roomID, mediaEpoch, make([]byte, 32), now)
	require.NoError(t, err)
	media := &recordingMediaLifecycle{}
	effects := NewEffects(pool, media)
	require.NoError(t, effects.FreezeSpace(ctx, spaceID.String()))
	require.Equal(t, []string{roomName + "/profile:" + profileID.String() + ":media:" + mediaEpoch.String()}, media.removed)
	var memberships int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_room_memberships WHERE room_id=$1`, roomID).Scan(&memberships))
	require.Zero(t, memberships, "grant rows are removed only after ejection succeeds")
	require.NoError(t, effects.FreezeSpace(ctx, spaceID.String()))
	require.Len(t, media.removed, 1, "completed ejection is not repeated after the membership grant is removed")

	require.NoError(t, effects.PurgeSpace(ctx, spaceID.String()))
	require.Equal(t, []string{roomName}, media.closed)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM voice_room_instances WHERE room_id=$1`, roomID).Scan(&state))
	require.Equal(t, "closed", state, "immutable GIS references remain terminally fenced")
	require.NoError(t, effects.PurgeSpace(ctx, spaceID.String()))
	require.Len(t, media.closed, 1, "terminal room state makes purge retry-safe")
}

func TestPostgresSpaceLifecycleFreezeRetainsMembershipWhenEjectionFails(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceLifecyclePostgres(t, ctx)
	spaceID, roomID, voiceRoomID, profileID, mediaEpoch := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	roomName := "voice-room-" + voiceRoomID.String()
	now := time.Now().UTC()
	_, err := pool.Exec(ctx, `INSERT INTO voice_room_instances(room_id,space_id,voice_room_id,livekit_room_name,state,roster_version,created_at,updated_at)
VALUES($1,$2,$3,$4,'active',1,$5,$5)`, roomID, spaceID, voiceRoomID, roomName, now)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO voice_room_memberships(profile_id,room_id,media_epoch,space_access_epoch,role_policy_epoch,authorization_digest,
can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,joined_at,updated_at)
VALUES($1,$2,$3,1,1,$4,true,true,false,false,true,false,false,false,false,false,$5,$5)`, profileID, roomID, mediaEpoch, make([]byte, 32), now)
	require.NoError(t, err)
	mediaErr := context.DeadlineExceeded
	media := &recordingMediaLifecycle{removeErr: mediaErr}
	err = NewEffects(pool, media).FreezeSpace(ctx, spaceID.String())
	require.ErrorIs(t, err, mediaErr)
	var memberships int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_room_memberships WHERE room_id=$1`, roomID).Scan(&memberships))
	require.Equal(t, 1, memberships, "a failed ejection keeps the durable participant grant for exact retry")
}
