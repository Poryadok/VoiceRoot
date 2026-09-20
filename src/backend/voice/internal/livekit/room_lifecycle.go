package livekit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	protocol "github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/twitchtv/twirp"
)

var (
	ErrRoomServiceNotConfigured = errors.New("livekit room service not configured")
	ErrRoomNameRequired         = errors.New("livekit room name is required")
)

// RoomServiceSDK is the small LiveKit Server SDK surface needed for room
// lifecycle effects. Keeping it explicit makes retries deterministic in tests.
type RoomServiceSDK interface {
	CreateRoom(context.Context, *protocol.CreateRoomRequest) (*protocol.Room, error)
	DeleteRoom(context.Context, *protocol.DeleteRoomRequest) (*protocol.DeleteRoomResponse, error)
}

// RoomLifecycle manages only the LiveKit room resource. It intentionally does
// not publish events or modify Voice lifecycle/roster state.
type RoomLifecycle struct {
	sdk RoomServiceSDK
}

func NewRoomLifecycle(sdk RoomServiceSDK) *RoomLifecycle {
	return &RoomLifecycle{sdk: sdk}
}

func NewSDKRoomLifecycle(url, apiKey, apiSecret string) *RoomLifecycle {
	return NewRoomLifecycle(lksdk.NewRoomServiceClient(url, apiKey, apiSecret))
}

// EnsureRoom creates roomName. An already-existing room is a successful replay;
// every other SDK error is returned for the caller's retry policy.
func (a *RoomLifecycle) EnsureRoom(ctx context.Context, roomName string) error {
	if err := a.validate(roomName); err != nil {
		return err
	}
	_, err := a.sdk.CreateRoom(ctx, &protocol.CreateRoomRequest{Name: roomName})
	if isTwirpCode(err, twirp.AlreadyExists) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("livekit create room %q: %w", roomName, err)
	}
	return nil
}

// CloseRoom deletes roomName. A missing room is a successful replay; every
// other SDK error is returned for the caller's retry policy.
func (a *RoomLifecycle) CloseRoom(ctx context.Context, roomName string) error {
	if err := a.validate(roomName); err != nil {
		return err
	}
	_, err := a.sdk.DeleteRoom(ctx, &protocol.DeleteRoomRequest{Room: roomName})
	if isTwirpCode(err, twirp.NotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("livekit delete room %q: %w", roomName, err)
	}
	return nil
}

func (a *RoomLifecycle) validate(roomName string) error {
	if a == nil || a.sdk == nil {
		return ErrRoomServiceNotConfigured
	}
	if strings.TrimSpace(roomName) == "" {
		return ErrRoomNameRequired
	}
	return nil
}

func isTwirpCode(err error, want twirp.ErrorCode) bool {
	var twerr twirp.Error
	return errors.As(err, &twerr) && twerr.Code() == want
}
