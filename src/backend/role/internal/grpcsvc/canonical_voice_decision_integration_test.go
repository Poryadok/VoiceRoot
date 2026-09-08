package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	rolev1 "voice.app/voice/role/v1"

	"voice/backend/role/permissions"
)

// TestCheckPermission_CanonicalVoiceDecisionDeniesStaleMember makes the
// fail-closed requirement executable through the existing direct gRPC
// surface.  A profile which has no current Role membership must not inherit a
// default role and receive voice access while Space/Role membership state is
// stale or missing.
func TestCheckPermission_CanonicalVoiceDecisionDeniesStaleMember(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	staleMemberID := uuid.New() // No member_roles row exists for this profile.
	voiceRoomID := uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(ctx, spaceID, ownerID))

	for _, action := range []string{
		permissions.VoiceJoin,
		permissions.VoiceSpeak,
		permissions.VoiceMuteOthers,
		permissions.VoiceMoveOthers,
	} {
		resp, err := client.CheckPermission(ctxWithProfile(staleMemberID), &rolev1.CheckPermissionRequest{
			SpaceId:        spaceID.String(),
			ProfileId:      staleMemberID.String(),
			PermissionName: action,
			VoiceRoomId:    ptr(voiceRoomID.String()),
		})
		require.NoError(t, err, "action %s", action)
		require.False(t, resp.GetAllowed(), "action %s must deny a missing/stale Role membership", action)
	}
}

// TestCheckPermission_CanonicalVoiceDecisionAppliesVoiceRoomOverride
// characterizes the existing Role-owned half of the room decision.  The
// caller supplies the explicit subject profile, and a prevalidated canonical
// voice_room_id scopes the resulting permission evaluation.  Room ownership
// itself remains the canonical Space resolver's responsibility.
func TestCheckPermission_CanonicalVoiceDecisionAppliesVoiceRoomOverride(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	profileID := uuid.New()
	voiceRoomID := uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(ctx, spaceID, ownerID))

	memberRoleID, err := s.RoleIDByName(ctx, spaceID, permissions.RoleMember)
	require.NoError(t, err)
	require.NoError(t, s.AssignMemberRole(ctx, spaceID, profileID, memberRoleID, ownerID))

	denyMask := uint64(0)
	for _, action := range []string{
		permissions.VoiceJoin,
		permissions.VoiceSpeak,
		permissions.VoiceMuteOthers,
		permissions.VoiceMoveOthers,
	} {
		bit, err := permissions.MaskFor(action)
		require.NoError(t, err)
		denyMask |= bit
	}
	require.NoError(t, s.SetVoiceRoomOverride(ctx, voiceRoomID, memberRoleID, 0, denyMask))

	for _, action := range []string{
		permissions.VoiceJoin,
		permissions.VoiceSpeak,
		permissions.VoiceMuteOthers,
		permissions.VoiceMoveOthers,
	} {
		resp, err := client.CheckPermission(ctxWithProfile(profileID), &rolev1.CheckPermissionRequest{
			SpaceId:        spaceID.String(),
			ProfileId:      profileID.String(),
			PermissionName: action,
			VoiceRoomId:    ptr(voiceRoomID.String()),
		})
		require.NoError(t, err, "action %s", action)
		require.False(t, resp.GetAllowed(), "voice-room deny override must apply to %s", action)
	}
}

// TestCheckPermission_CanonicalVoiceDecisionDeniesUnknownSpace keeps the
// existing direct gRPC surface explicit about the no-implicit-allow rule.  A
// caller cannot obtain room permission from a syntactically valid but unknown
// Space ID.
func TestCheckPermission_CanonicalVoiceDecisionDeniesUnknownSpace(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	profileID := uuid.New()
	voiceRoomID := uuid.New()
	resp, err := client.CheckPermission(ctxWithProfile(profileID), &rolev1.CheckPermissionRequest{
		SpaceId:        uuid.NewString(),
		ProfileId:      profileID.String(),
		PermissionName: permissions.VoiceJoin,
		VoiceRoomId:    ptr(voiceRoomID.String()),
	})
	require.NoError(t, err)
	require.False(t, resp.GetAllowed(), "unknown space must fail closed")
}

// TestCheckPermission_CanonicalVoiceDecisionDeniesRevokedRole makes stale
// permission state visible to the consumer contract: once a role grant is
// revoked, a later room-scoped decision cannot retain its allow.
func TestCheckPermission_CanonicalVoiceDecisionDeniesRevokedRole(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	profileID := uuid.New()
	voiceRoomID := uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(ctx, spaceID, ownerID))

	moveMask, err := permissions.MaskFor(permissions.VoiceMoveOthers)
	require.NoError(t, err)
	role, err := s.CreateCustomRole(ctx, spaceID, "temporary mover", moveMask, 2, &ownerID)
	require.NoError(t, err)
	require.NoError(t, s.AssignMemberRole(ctx, spaceID, profileID, role.ID, ownerID))
	require.NoError(t, s.RevokeMemberRole(ctx, spaceID, profileID, role.ID))

	resp, err := client.CheckPermission(ctxWithProfile(profileID), &rolev1.CheckPermissionRequest{
		SpaceId:        spaceID.String(),
		ProfileId:      profileID.String(),
		PermissionName: permissions.VoiceMoveOthers,
		VoiceRoomId:    ptr(voiceRoomID.String()),
	})
	require.NoError(t, err)
	require.False(t, resp.GetAllowed(), "revoked VOICE_MOVE_OTHERS role must not remain effective")
}

// TestCheckPermission_CanonicalVoiceDecisionFailsClosedWhenRoleUnavailable
// defines the direct-client half of the consumer contract.  A persistence
// outage must return an error and never a synthetic allow which a Voice or
// Gateway caller could treat as permission to continue.
func TestCheckPermission_CanonicalVoiceDecisionFailsClosedWhenRoleUnavailable(t *testing.T) {
	client, stop := startRoleGRPCTestServer(t, nil, func(svc *RoleGRPC) {
		svc.Store = nil
	})
	defer stop()

	profileID := uuid.New()
	_, err := client.CheckPermission(ctxWithProfile(profileID), &rolev1.CheckPermissionRequest{
		SpaceId:        uuid.NewString(),
		ProfileId:      profileID.String(),
		PermissionName: permissions.VoiceJoin,
		VoiceRoomId:    ptr(uuid.NewString()),
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func ptr(value string) *string { return &value }
