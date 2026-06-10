package main

import (
	"context"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	rolev1 "voice.app/voice/role/v1"
)

type recordingRoleService struct {
	rolev1.UnimplementedRoleServiceServer
	lastList   *rolev1.ListRolesRequest
	lastCreate *rolev1.CreateRoleRequest
	lastAssign *rolev1.AssignRoleRequest
	lastCheck  *rolev1.CheckPermissionRequest
	lastChat   *rolev1.SetChatOverrideRequest
}

func (s *recordingRoleService) ListRoles(_ context.Context, req *rolev1.ListRolesRequest) (*rolev1.ListRolesResponse, error) {
	s.lastList = req
	return &rolev1.ListRolesResponse{
		RoleList: &rolev1.RoleList{
			Roles: []*rolev1.Role{{Id: "role-1", SpaceId: req.GetSpaceId(), Name: "Owner", Position: 4}},
		},
	}, nil
}

func (s *recordingRoleService) CreateRole(_ context.Context, req *rolev1.CreateRoleRequest) (*rolev1.CreateRoleResponse, error) {
	s.lastCreate = req
	return &rolev1.CreateRoleResponse{
		Role: &rolev1.Role{Id: "role-new", SpaceId: req.GetSpaceId(), Name: req.GetName()},
	}, nil
}

func (s *recordingRoleService) AssignRole(_ context.Context, req *rolev1.AssignRoleRequest) (*rolev1.AssignRoleResponse, error) {
	s.lastAssign = req
	return &rolev1.AssignRoleResponse{}, nil
}

func (s *recordingRoleService) CheckPermission(_ context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
	s.lastCheck = req
	return &rolev1.CheckPermissionResponse{Allowed: true}, nil
}

func (s *recordingRoleService) SetChatOverride(_ context.Context, req *rolev1.SetChatOverrideRequest) (*rolev1.SetChatOverrideResponse, error) {
	s.lastChat = req
	return &rolev1.SetChatOverrideResponse{}, nil
}

func startBufconnRoleConn(t *testing.T, impl rolev1.RoleServiceServer) (rolev1.RoleServiceClient, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	rolev1.RegisterRoleServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return rolev1.NewRoleServiceClient(conn), func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

func newRolesContractGateway(t *testing.T, rec *recordingRoleService) http.Handler {
	t.Helper()
	roleClient, cleanup := startBufconnRoleConn(t, rec)
	t.Cleanup(cleanup)
	return newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{role: roleClient}},
		restUpstreams: map[string]http.Handler{
			"roles": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) }),
		},
	})
}

// TestTranscodeRolesList documents PLAN Phase 5 REST: GET /api/v1/roles?space_id=.
func TestTranscodeRolesList(t *testing.T) {
	t.Parallel()
	rec := &recordingRoleService{}
	h := newRolesContractGateway(t, rec)

	resp := performRequest(h, http.MethodGet, "/api/v1/roles?space_id=space-1", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastList)
	require.Equal(t, "space-1", rec.lastList.GetSpaceId())
}

// TestTranscodeRolesCreate documents POST /api/v1/roles with name and permissions_mask.
func TestTranscodeRolesCreate(t *testing.T) {
	t.Parallel()
	rec := &recordingRoleService{}
	h := newRolesContractGateway(t, rec)

	body := `{"space_id":"space-1","name":"Raid Leader","permissions_mask":1024}`
	resp := performRequest(h, http.MethodPost, "/api/v1/roles", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastCreate)
	require.Equal(t, "Raid Leader", rec.lastCreate.GetName())
}

// TestTranscodeRolesAssign documents POST /api/v1/roles/assign.
func TestTranscodeRolesAssign(t *testing.T) {
	t.Parallel()
	rec := &recordingRoleService{}
	h := newRolesContractGateway(t, rec)

	body := `{"space_id":"space-1","profile_id":"profile-2","role_id":"role-member"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/roles/assign", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastAssign)
	require.Equal(t, "profile-2", rec.lastAssign.GetProfileId())
}

// TestTranscodeRolesCheckPermission documents GET /api/v1/roles/check permission probe.
func TestTranscodeRolesCheckPermission(t *testing.T) {
	t.Parallel()
	rec := &recordingRoleService{}
	h := newRolesContractGateway(t, rec)

	resp := performRequest(h, http.MethodGet, "/api/v1/roles/check?space_id=space-1&profile_id=profile-1&permission_name=SPACE_MANAGE_INVITES", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastCheck)
	require.Equal(t, "SPACE_MANAGE_INVITES", rec.lastCheck.GetPermissionName())
}

// TestTranscodeRolesChatOverride documents POST /api/v1/roles/chat-overrides.
func TestTranscodeRolesChatOverride(t *testing.T) {
	t.Parallel()
	rec := &recordingRoleService{}
	h := newRolesContractGateway(t, rec)

	body := `{"space_id":"space-1","chat":{"id":"chat-1"},"deny_mask":8}`
	resp := performRequest(h, http.MethodPost, "/api/v1/roles/chat-overrides", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastChat)
	require.Equal(t, "chat-1", rec.lastChat.GetChat().GetId())
}
