package gameprovision

import (
	"context"
	"errors"
	"strings"

	"voice/backend/voice/internal/livekit"
)

type ManagedRoomCloser interface {
	CloseRoom(context.Context, string) error
}

type ManagedRoomParticipantRemover interface {
	RemoveParticipant(context.Context, string, string) error
}

// LiveKitRoomFencer resolves the persisted room locator after CLOSING commits
// and relies on LiveKit's idempotent delete/NotFound behavior across retries.
type LiveKitRoomFencer struct {
	Store  *PostgresStore
	Closer ManagedRoomCloser
}

func NewLiveKitRoomFencer(store *PostgresStore, closer ManagedRoomCloser) *LiveKitRoomFencer {
	return &LiveKitRoomFencer{Store: store, Closer: closer}
}

func (f *LiveKitRoomFencer) FenceManagedGameSession(ctx context.Context, roomID, operationID string) error {
	if f == nil || f.Store == nil || f.Closer == nil || strings.TrimSpace(operationID) == "" {
		return errors.New("managed game session media fencer unavailable")
	}
	room, err := f.Store.GetRoomForClose(ctx, roomID)
	if err != nil {
		return err
	}
	return f.Closer.CloseRoom(ctx, room.LiveKitRoomName)
}

func (f *LiveKitRoomFencer) FenceManagedGameSessionMembers(ctx context.Context, roomID, operationID string, profileIDs []string) error {
	if f == nil || f.Store == nil || f.Closer == nil || strings.TrimSpace(operationID) == "" {
		return errors.New("managed game session media fencer unavailable")
	}
	remover, ok := f.Closer.(ManagedRoomParticipantRemover)
	if !ok {
		return errors.New("managed game session participant remover unavailable")
	}
	room, err := f.Store.GetRoomForClose(ctx, roomID)
	if err != nil {
		return err
	}
	for _, profileID := range profileIDs {
		if err := remover.RemoveParticipant(ctx, room.LiveKitRoomName, profileID); err != nil {
			return err
		}
	}
	return nil
}

var _ ManagedGameSessionMediaFencer = (*LiveKitRoomFencer)(nil)
var _ ManagedRoomCloser = (*livekit.RoomLifecycle)(nil)
var _ ManagedRoomParticipantRemover = (*livekit.RoomLifecycle)(nil)
