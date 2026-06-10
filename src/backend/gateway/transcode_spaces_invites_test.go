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

	spacev1 "voice.app/voice/space/v1"
)

type recordingSpaceInvites struct {
	spacev1.UnimplementedSpaceServiceServer
	lastCreate *spacev1.CreateInviteRequest
	lastList   *spacev1.ListInvitesRequest
	lastRevoke *spacev1.RevokeInviteRequest
	lastGet    *spacev1.GetInviteRequest
	lastJoin   *spacev1.JoinByInviteRequest
}

func (s *recordingSpaceInvites) CreateInvite(_ context.Context, req *spacev1.CreateInviteRequest) (*spacev1.CreateInviteResponse, error) {
	s.lastCreate = req
	return &spacev1.CreateInviteResponse{
		Invite: &spacev1.Invite{Id: "inv-1", SpaceId: req.GetSpaceId(), Code: "abc123"},
	}, nil
}

func (s *recordingSpaceInvites) ListInvites(_ context.Context, req *spacev1.ListInvitesRequest) (*spacev1.ListInvitesResponse, error) {
	s.lastList = req
	return &spacev1.ListInvitesResponse{
		InviteList: &spacev1.InviteList{
			Invites: []*spacev1.Invite{{Id: "inv-1", SpaceId: req.GetSpaceId(), Code: "abc123"}},
		},
	}, nil
}

func (s *recordingSpaceInvites) RevokeInvite(_ context.Context, req *spacev1.RevokeInviteRequest) (*spacev1.RevokeInviteResponse, error) {
	s.lastRevoke = req
	return &spacev1.RevokeInviteResponse{}, nil
}

func (s *recordingSpaceInvites) GetInvite(_ context.Context, req *spacev1.GetInviteRequest) (*spacev1.GetInviteResponse, error) {
	s.lastGet = req
	return &spacev1.GetInviteResponse{
		Invite: &spacev1.Invite{Id: "inv-1", Code: req.GetCode(), SpaceId: "space-1"},
	}, nil
}

func (s *recordingSpaceInvites) JoinByInvite(_ context.Context, req *spacev1.JoinByInviteRequest) (*spacev1.JoinByInviteResponse, error) {
	s.lastJoin = req
	return &spacev1.JoinByInviteResponse{
		SpaceMembership: &spacev1.SpaceMembership{SpaceId: "space-1", ProfileId: "profile-1"},
	}, nil
}

func startBufconnSpaceInvitesClient(t *testing.T, impl spacev1.SpaceServiceServer) (spacev1.SpaceServiceClient, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	spacev1.RegisterSpaceServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return spacev1.NewSpaceServiceClient(conn), func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

func newInvitesContractGateway(t *testing.T, rec *recordingSpaceInvites) http.Handler {
	t.Helper()
	spaceClient, cleanup := startBufconnSpaceInvitesClient(t, rec)
	t.Cleanup(cleanup)
	return newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{space: spaceClient}},
		restUpstreams: map[string]http.Handler{
			"spaces": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		},
	})
}

func TestTranscodeSpacesCreateInvite(t *testing.T) {
	t.Parallel()
	rec := &recordingSpaceInvites{}
	h := newInvitesContractGateway(t, rec)

	resp := performRequest(h, http.MethodPost, "/api/v1/spaces/space-1/invites", `{"maxUses":5}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastCreate)
	require.Equal(t, "space-1", rec.lastCreate.GetSpaceId())
	require.Equal(t, int32(5), rec.lastCreate.GetMaxUses())
}

func TestTranscodeSpacesListInvites(t *testing.T) {
	t.Parallel()
	rec := &recordingSpaceInvites{}
	h := newInvitesContractGateway(t, rec)

	resp := performRequest(h, http.MethodGet, "/api/v1/spaces/space-9/invites", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastList)
	require.Equal(t, "space-9", rec.lastList.GetSpaceId())
}

func TestTranscodeSpacesRevokeInvite(t *testing.T) {
	t.Parallel()
	rec := &recordingSpaceInvites{}
	h := newInvitesContractGateway(t, rec)

	resp := performRequest(h, http.MethodDelete, "/api/v1/spaces/space-1/invites/inv-42", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusNoContent, resp.Code)
	require.NotNil(t, rec.lastRevoke)
	require.Equal(t, "inv-42", rec.lastRevoke.GetInviteId())
}

func TestTranscodeInvitesGetByCode(t *testing.T) {
	t.Parallel()
	rec := &recordingSpaceInvites{}
	h := newInvitesContractGateway(t, rec)

	resp := performRequest(h, http.MethodGet, "/api/v1/invites/secret-code", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastGet)
	require.Equal(t, "secret-code", rec.lastGet.GetCode())
}

func TestTranscodeInvitesJoin(t *testing.T) {
	t.Parallel()
	rec := &recordingSpaceInvites{}
	h := newInvitesContractGateway(t, rec)

	resp := performRequest(h, http.MethodPost, "/api/v1/invites/secret-code/join", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastJoin)
	require.Equal(t, "secret-code", rec.lastJoin.GetCode())
}
