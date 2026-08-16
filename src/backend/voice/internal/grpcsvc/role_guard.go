package grpcsvc

import (
	"context"
	"errors"
)

// ErrScreenShareDenied is returned when role permission check denies screen share.
var ErrScreenShareDenied = errors.New("screen share not permitted")

// ErrVoiceJoinDenied is returned when role permission check denies voice room join.
var ErrVoiceJoinDenied = errors.New("voice join not permitted")

// ErrMuteOthersDenied is returned when role permission check denies mute-others / floor control.
var ErrMuteOthersDenied = errors.New("mute others not permitted")

// RolePermissionChecker validates voice-room permissions via Role Service.
type RolePermissionChecker interface {
	EnsureScreenShare(ctx context.Context, spaceID, profileID, voiceRoomID string) error
	EnsureVoiceJoin(ctx context.Context, spaceID, profileID, voiceRoomID string) error
	EnsureMuteOthers(ctx context.Context, spaceID, profileID, voiceRoomID string) error
}

type mapRolePermissions struct {
	allowed     map[string]map[string]bool // spaceID -> profileID
	deniedRooms map[string]map[string]bool // voiceRoomID -> profileID (room override deny)
	muteOthers  map[string]map[string]bool // spaceID -> profileID for VOICE_MUTE_OTHERS
}

func (m *mapRolePermissions) EnsureScreenShare(_ context.Context, spaceID, profileID, _ string) error {
	if m == nil {
		return nil
	}
	space, ok := m.allowed[spaceID]
	if !ok || !space[profileID] {
		return ErrScreenShareDenied
	}
	return nil
}

func (m *mapRolePermissions) EnsureVoiceJoin(_ context.Context, spaceID, profileID, voiceRoomID string) error {
	if m == nil {
		return nil
	}
	if voiceRoomID != "" {
		if room, ok := m.deniedRooms[voiceRoomID]; ok && room[profileID] {
			return ErrVoiceJoinDenied
		}
	}
	space, ok := m.allowed[spaceID]
	if !ok || !space[profileID] {
		return ErrVoiceJoinDenied
	}
	return nil
}

func (m *mapRolePermissions) EnsureMuteOthers(_ context.Context, spaceID, profileID, _ string) error {
	if m == nil {
		return nil
	}
	space, ok := m.muteOthers[spaceID]
	if !ok || !space[profileID] {
		return ErrMuteOthersDenied
	}
	return nil
}
