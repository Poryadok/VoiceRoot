package grpcsvc

import (
	"context"
	"errors"
)

// ErrScreenShareDenied is returned when role permission check denies screen share.
var ErrScreenShareDenied = errors.New("screen share not permitted")

// RolePermissionChecker validates voice-room permissions via Role Service.
type RolePermissionChecker interface {
	EnsureScreenShare(ctx context.Context, spaceID, profileID, voiceRoomID string) error
}

type mapRolePermissions struct {
	allowed map[string]map[string]bool // spaceID -> profileID
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
