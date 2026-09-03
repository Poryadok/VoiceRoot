package grpcsvc

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"

	"voice/backend/role/internal/roleevents"
	"voice/backend/role/internal/store"
	"voice/backend/role/permissions"
)

type recordedRoleCreatedEvent struct {
	spaceID string
	roleID  string
	name    string
}

type recordingRoleEvents struct {
	roleevents.NoopPublisher
	mu      sync.Mutex
	created []recordedRoleCreatedEvent
}

func (p *recordingRoleEvents) PublishRoleCreated(_ context.Context, spaceID, roleID, name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.created = append(p.created, recordedRoleCreatedEvent{spaceID: spaceID, roleID: roleID, name: name})
	return nil
}

func (p *recordingRoleEvents) createdEvents() []recordedRoleCreatedEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]recordedRoleCreatedEvent(nil), p.created...)
}

func bootstrapRoleManagerAtPositionTwo(t *testing.T, s *store.RoleStore) (spaceID, ownerID, actorID uuid.UUID) {
	t.Helper()
	spaceID = uuid.New()
	ownerID = uuid.New()
	actorID = uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, ownerID))
	manageRolesMask, err := permissions.MaskFor(permissions.SpaceManageRoles)
	require.NoError(t, err)
	manager, err := s.CreateCustomRole(context.Background(), spaceID, "Role manager", manageRolesMask, 2, &ownerID)
	require.NoError(t, err)
	require.NoError(t, s.AssignMemberRole(context.Background(), spaceID, actorID, manager.ID, ownerID))
	return spaceID, ownerID, actorID
}

func TestGetVoiceRoomOverrides_SetAndList(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	voiceRoomID := uuid.New().String()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, ownerID))

	roles, err := s.ListRoles(context.Background(), spaceID)
	require.NoError(t, err)
	var memberRoleID string
	for _, r := range roles {
		if r.Name == permissions.RoleMember {
			memberRoleID = r.ID.String()
		}
	}

	speakMask, err := permissions.MaskFor(permissions.VoiceSpeak)
	require.NoError(t, err)
	_, err = client.SetVoiceRoomOverride(ctxWithProfile(ownerID), &rolev1.SetVoiceRoomOverrideRequest{
		SpaceId:     spaceID.String(),
		VoiceRoomId: voiceRoomID,
		RoleId:      memberRoleID,
		DenyMask:    speakMask,
	})
	require.NoError(t, err)

	list, err := client.GetVoiceRoomOverrides(context.Background(), &rolev1.GetVoiceRoomOverridesRequest{
		SpaceId:     spaceID.String(),
		VoiceRoomId: &voiceRoomID,
	})
	require.NoError(t, err)
	require.Len(t, list.GetOverrideList().GetOverrides(), 1)
}

func TestRemoveChatOverride_ClearsRow(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	chatID := uuid.New().String()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, ownerID))

	roles, err := s.ListRoles(context.Background(), spaceID)
	require.NoError(t, err)
	var memberRoleID string
	for _, r := range roles {
		if r.Name == permissions.RoleMember {
			memberRoleID = r.ID.String()
		}
	}

	sendMask, err := permissions.MaskFor(permissions.TextChatSendMessages)
	require.NoError(t, err)
	_, err = client.SetChatOverride(ctxWithProfile(ownerID), &rolev1.SetChatOverrideRequest{
		SpaceId:  spaceID.String(),
		Chat:     &chatv1.ChatRef{Id: chatID},
		RoleId:   memberRoleID,
		DenyMask: sendMask,
	})
	require.NoError(t, err)

	_, err = client.RemoveChatOverride(ctxWithProfile(ownerID), &rolev1.RemoveChatOverrideRequest{
		SpaceId: spaceID.String(),
		Chat:    &chatv1.ChatRef{Id: chatID},
		RoleId:  memberRoleID,
	})
	require.NoError(t, err)

	list, err := client.GetChatOverrides(context.Background(), &rolev1.GetChatOverridesRequest{
		SpaceId:    spaceID.String(),
		FilterChat: &chatv1.ChatRef{Id: chatID},
	})
	require.NoError(t, err)
	require.Empty(t, list.GetOverrideList().GetOverrides())
}

func TestCreateRole_RequiresManageRolesPermission(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	memberID := uuid.New()
	require.NoError(t, s.BootstrapSystemRoles(context.Background(), spaceID))
	roles, err := s.ListRoles(context.Background(), spaceID)
	require.NoError(t, err)
	for _, r := range roles {
		if r.Name == permissions.RoleMember {
			require.NoError(t, s.AssignMemberRole(context.Background(), spaceID, memberID, r.ID, memberID))
		}
	}

	_, err = client.CreateRole(ctxWithProfile(memberID), &rolev1.CreateRoleRequest{
		SpaceId: spaceID.String(),
		Name:    "Nope",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestCreateRole_PositionAtOrAboveActorDenied verifies role managers may only create below their top role.
func TestCreateRole_PositionAtOrAboveActorDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	events := &recordingRoleEvents{}
	client, stop := startRoleGRPCTestServer(t, s.Pool, func(svc *RoleGRPC) { svc.Events = events })
	defer stop()

	spaceID, _, actorID := bootstrapRoleManagerAtPositionTwo(t, s)
	for _, tc := range []struct {
		name     string
		position int32
	}{
		{name: "equal-position", position: 2},
		{name: "higher-position", position: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			roleName := "Denied " + tc.name
			_, err := client.CreateRole(ctxWithProfile(actorID), &rolev1.CreateRoleRequest{
				SpaceId:  spaceID.String(),
				Name:     roleName,
				Position: tc.position,
			})
			require.Equal(t, codes.PermissionDenied, status.Code(err))

			roles, listErr := client.ListRoles(context.Background(), &rolev1.ListRolesRequest{SpaceId: spaceID.String()})
			require.NoError(t, listErr)
			for _, role := range roles.GetRoleList().GetRoles() {
				require.NotEqual(t, roleName, role.GetName())
			}
		})
	}
	require.Empty(t, events.createdEvents(), "denied creates must not emit role.created")
}

// TestCreateRole_PositionBelowActorSucceeds verifies a non-owner role manager may create below their top role.
func TestCreateRole_PositionBelowActorSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	events := &recordingRoleEvents{}
	client, stop := startRoleGRPCTestServer(t, s.Pool, func(svc *RoleGRPC) { svc.Events = events })
	defer stop()

	spaceID, _, actorID := bootstrapRoleManagerAtPositionTwo(t, s)
	created, err := client.CreateRole(ctxWithProfile(actorID), &rolev1.CreateRoleRequest{
		SpaceId:  spaceID.String(),
		Name:     "Below role manager",
		Position: 1,
	})
	require.NoError(t, err)
	require.Equal(t, "Below role manager", created.GetRole().GetName())
	require.EqualValues(t, 1, created.GetRole().GetPosition())

	roles, err := client.ListRoles(context.Background(), &rolev1.ListRolesRequest{SpaceId: spaceID.String()})
	require.NoError(t, err)
	found := false
	for _, role := range roles.GetRoleList().GetRoles() {
		if role.GetId() == created.GetRole().GetId() {
			found = true
			break
		}
	}
	require.True(t, found, "created role must persist")
	eventsCreated := events.createdEvents()
	require.Len(t, eventsCreated, 1)
	require.Equal(t, spaceID.String(), eventsCreated[0].spaceID)
	require.Equal(t, created.GetRole().GetId(), eventsCreated[0].roleID)
	require.Equal(t, created.GetRole().GetName(), eventsCreated[0].name)
}

// TestCreateRole_PositionOwnerBypassPreserved verifies hierarchy validation does not constrain the Owner.
func TestCreateRole_PositionOwnerBypassPreserved(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, ownerID))

	created, err := client.CreateRole(ctxWithProfile(ownerID), &rolev1.CreateRoleRequest{
		SpaceId:  spaceID.String(),
		Name:     "Owner position bypass",
		Position: 4,
	})
	require.NoError(t, err)
	require.EqualValues(t, 4, created.GetRole().GetPosition())
}

func TestGetEffectivePermissions_ReturnsMask(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, ownerID))

	resp, err := client.GetEffectivePermissions(context.Background(), &rolev1.GetEffectivePermissionsRequest{
		SpaceId:   spaceID.String(),
		ProfileId: ownerID.String(),
	})
	require.NoError(t, err)
	require.NotZero(t, resp.GetPermissionSet().GetEffectiveMask())
	require.NotEmpty(t, resp.GetPermissionSet().GetPermissionNames())
}
