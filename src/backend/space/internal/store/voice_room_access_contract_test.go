package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type voiceRoomAccessQueryCall struct{ voiceRoomID, profileID uuid.UUID }

type recordingVoiceRoomAccessQuery struct {
	calls  []voiceRoomAccessQueryCall
	result *VoiceRoomAccessRow
	err    error
}

func (q *recordingVoiceRoomAccessQuery) ResolveVoiceRoomAccessQuery(_ context.Context, roomID, profileID uuid.UUID) (*VoiceRoomAccessRow, error) {
	q.calls = append(q.calls, voiceRoomAccessQueryCall{roomID, profileID})
	return q.result, q.err
}

// This seam makes the one exact room+membership query observable, so a future
// ListMembers pagination fallback cannot silently authorize access.
func TestResolveVoiceRoomAccess_UsesOneExactResolverQuery(t *testing.T) {
	roomID, profileID, spaceID := uuid.New(), uuid.New(), uuid.New()
	q := &recordingVoiceRoomAccessQuery{result: &VoiceRoomAccessRow{SpaceID: spaceID, Member: true, Active: true}}
	access, err := (&SpaceStore{voiceRoomAccessQuery: q}).ResolveVoiceRoomAccess(context.Background(), roomID, profileID)
	require.NoError(t, err)
	require.Equal(t, q.result, access)
	require.Equal(t, []voiceRoomAccessQueryCall{{roomID, profileID}}, q.calls)
}

func TestResolveVoiceRoomAccess_PropagatesExactQueryFailure(t *testing.T) {
	wantErr := errors.New("space database unavailable")
	roomID, profileID := uuid.New(), uuid.New()
	q := &recordingVoiceRoomAccessQuery{err: wantErr}
	access, err := (&SpaceStore{voiceRoomAccessQuery: q}).ResolveVoiceRoomAccess(context.Background(), roomID, profileID)
	require.Nil(t, access)
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, []voiceRoomAccessQueryCall{{roomID, profileID}}, q.calls)
}
