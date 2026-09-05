package grpcsvc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	voicestore "voice/backend/voice/internal/store"
)

// ErrScreenShareDenied is returned when role permission check denies screen share.
var ErrScreenShareDenied = errors.New("screen share not permitted")

// ErrVoiceJoinDenied is returned when role permission check denies voice room join.
var ErrVoiceJoinDenied = errors.New("voice join not permitted")

// ErrVoiceSpeakDenied is returned when role permission check denies own voice audio.
var ErrVoiceSpeakDenied = errors.New("voice speak not permitted")

// ErrMuteOthersDenied is returned when role permission check denies mute-others / floor control.
var ErrMuteOthersDenied = errors.New("mute others not permitted")

// RolePermissionChecker validates voice-room permissions via Role Service.
type RolePermissionChecker interface {
	EnsureScreenShare(ctx context.Context, spaceID, profileID, voiceRoomID string) error
	EnsureVoiceJoin(ctx context.Context, spaceID, profileID, voiceRoomID string) error
	EnsureVoiceSpeak(ctx context.Context, spaceID, profileID, voiceRoomID string) error
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

func (m *mapRolePermissions) EnsureVoiceSpeak(_ context.Context, spaceID, profileID, _ string) error {
	if m == nil {
		return nil
	}
	space, ok := m.allowed[spaceID]
	if !ok || !space[profileID] {
		return ErrVoiceSpeakDenied
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

func (s *VoiceGRPC) ensureVoiceSpeakPermission(ctx context.Context, call voicestore.Call, profileID string) error {
	if !call.IsVoiceRoom() || call.SpaceID == "" {
		return nil
	}
	if s.Roles == nil {
		return status.Error(codes.PermissionDenied, "voice speak permission check unavailable")
	}
	if err := s.Roles.EnsureVoiceSpeak(ctx, call.SpaceID, profileID, call.VoiceRoomID); err != nil {
		if errors.Is(err, ErrVoiceSpeakDenied) {
			return status.Error(codes.PermissionDenied, "voice speak not permitted")
		}
		return status.Error(codes.PermissionDenied, "voice speak permission check unavailable")
	}
	return nil
}

func (s *VoiceGRPC) ensureMuteOthersPermission(ctx context.Context, call voicestore.Call, profileID string) error {
	if !call.IsVoiceRoom() || call.SpaceID == "" {
		return nil
	}
	if s.Roles == nil {
		return status.Error(codes.PermissionDenied, "mute others permission check unavailable")
	}
	if err := s.Roles.EnsureMuteOthers(ctx, call.SpaceID, profileID, call.VoiceRoomID); err != nil {
		if errors.Is(err, ErrMuteOthersDenied) {
			return status.Error(codes.PermissionDenied, "mute others not permitted")
		}
		return status.Error(codes.PermissionDenied, "mute others permission check unavailable")
	}
	return nil
}
