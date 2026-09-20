package livekit

import (
	"context"
	"errors"
	"testing"

	protocol "github.com/livekit/protocol/livekit"
	"github.com/stretchr/testify/require"
	"github.com/twitchtv/twirp"
)

func TestRoomLifecycleEnsureAndCloseAreIdempotent(t *testing.T) {
	t.Parallel()

	fake := &fakeRoomService{
		createErr: twirp.AlreadyExists.Error("room already exists"),
		deleteErr: twirp.NotFound.Error("room not found"),
	}
	adapter := NewRoomLifecycle(fake)

	require.NoError(t, adapter.EnsureRoom(context.Background(), "space-1-room-2"))
	require.NoError(t, adapter.CloseRoom(context.Background(), "space-1-room-2"))
	require.Equal(t, []string{"space-1-room-2"}, fake.created)
	require.Equal(t, []string{"space-1-room-2"}, fake.deleted)
}

func TestRoomLifecycleReturnsRetryableSDKFailureUnchanged(t *testing.T) {
	t.Parallel()

	want := errors.New("temporary transport failure")
	fake := &fakeRoomService{createErr: want}
	err := NewRoomLifecycle(fake).EnsureRoom(context.Background(), "room-a")
	require.ErrorIs(t, err, want)
	require.Equal(t, []string{"room-a"}, fake.created)
}

func TestRoomLifecycleRejectsBlankRoomWithoutCallingSDK(t *testing.T) {
	t.Parallel()

	fake := &fakeRoomService{}
	adapter := NewRoomLifecycle(fake)
	require.Error(t, adapter.EnsureRoom(context.Background(), " \t"))
	require.Error(t, adapter.CloseRoom(context.Background(), ""))
	require.Empty(t, fake.created)
	require.Empty(t, fake.deleted)
}

type fakeRoomService struct {
	created   []string
	deleted   []string
	createErr error
	deleteErr error
}

func (f *fakeRoomService) CreateRoom(_ context.Context, request *protocol.CreateRoomRequest) (*protocol.Room, error) {
	f.created = append(f.created, request.Name)
	return &protocol.Room{Name: request.Name}, f.createErr
}

func (f *fakeRoomService) DeleteRoom(_ context.Context, request *protocol.DeleteRoomRequest) (*protocol.DeleteRoomResponse, error) {
	f.deleted = append(f.deleted, request.Room)
	return &protocol.DeleteRoomResponse{}, f.deleteErr
}
