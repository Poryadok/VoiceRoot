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

type recordingSpaceModeration struct {
	spacev1.UnimplementedSpaceServiceServer
	lastKick     *spacev1.KickMemberRequest
	lastBan      *spacev1.BanMemberRequest
	lastUnban    *spacev1.UnbanMemberRequest
	lastListBans *spacev1.ListBansRequest
	lastTimeout  *spacev1.TimeoutMemberRequest
	lastRmTO     *spacev1.RemoveMemberTimeoutRequest
}

func (s *recordingSpaceModeration) KickMember(_ context.Context, req *spacev1.KickMemberRequest) (*spacev1.KickMemberResponse, error) {
	s.lastKick = req
	return &spacev1.KickMemberResponse{}, nil
}

func (s *recordingSpaceModeration) BanMember(_ context.Context, req *spacev1.BanMemberRequest) (*spacev1.BanMemberResponse, error) {
	s.lastBan = req
	return &spacev1.BanMemberResponse{}, nil
}

func (s *recordingSpaceModeration) UnbanMember(_ context.Context, req *spacev1.UnbanMemberRequest) (*spacev1.UnbanMemberResponse, error) {
	s.lastUnban = req
	return &spacev1.UnbanMemberResponse{}, nil
}

func (s *recordingSpaceModeration) ListBans(_ context.Context, req *spacev1.ListBansRequest) (*spacev1.ListBansResponse, error) {
	s.lastListBans = req
	return &spacev1.ListBansResponse{
		BanList: &spacev1.BanList{
			Bans: []*spacev1.SpaceBan{{SpaceId: req.GetSpaceId(), AccountId: "account-b"}},
		},
	}, nil
}

func (s *recordingSpaceModeration) TimeoutMember(_ context.Context, req *spacev1.TimeoutMemberRequest) (*spacev1.TimeoutMemberResponse, error) {
	s.lastTimeout = req
	return &spacev1.TimeoutMemberResponse{}, nil
}

func (s *recordingSpaceModeration) RemoveMemberTimeout(_ context.Context, req *spacev1.RemoveMemberTimeoutRequest) (*spacev1.RemoveMemberTimeoutResponse, error) {
	s.lastRmTO = req
	return &spacev1.RemoveMemberTimeoutResponse{}, nil
}

func startBufconnSpaceModerationClient(t *testing.T, impl spacev1.SpaceServiceServer) (spacev1.SpaceServiceClient, func()) {
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

func newModerationContractGateway(t *testing.T, rec *recordingSpaceModeration) http.Handler {
	t.Helper()
	spaceClient, cleanup := startBufconnSpaceModerationClient(t, rec)
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

// TestTranscodeSpacesModeration_KickMember documents DELETE /api/v1/spaces/{spaceId}/members/{profileId}.
func TestTranscodeSpacesModeration_KickMember(t *testing.T) {
	t.Parallel()
	rec := &recordingSpaceModeration{}
	h := newModerationContractGateway(t, rec)

	resp := performRequest(h, http.MethodDelete, "/api/v1/spaces/space-1/members/profile-b", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastKick)
	require.Equal(t, "space-1", rec.lastKick.GetSpaceId())
	require.Equal(t, "profile-b", rec.lastKick.GetProfileId())
}

// TestTranscodeSpacesModeration_BanMember documents POST /api/v1/spaces/{spaceId}/bans.
func TestTranscodeSpacesModeration_BanMember(t *testing.T) {
	t.Parallel()
	rec := &recordingSpaceModeration{}
	h := newModerationContractGateway(t, rec)

	body := `{"account_id":"account-b","reason":"spam"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/spaces/space-1/bans", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusNoContent, resp.Code)
	require.NotNil(t, rec.lastBan)
	require.Equal(t, "space-1", rec.lastBan.GetSpaceId())
	require.Equal(t, "account-b", rec.lastBan.GetAccountId())
	require.Equal(t, "spam", rec.lastBan.GetReason())
}

// TestTranscodeSpacesModeration_UnbanMember documents DELETE /api/v1/spaces/{spaceId}/bans/{accountId}.
func TestTranscodeSpacesModeration_UnbanMember(t *testing.T) {
	t.Parallel()
	rec := &recordingSpaceModeration{}
	h := newModerationContractGateway(t, rec)

	resp := performRequest(h, http.MethodDelete, "/api/v1/spaces/space-1/bans/account-b", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusNoContent, resp.Code)
	require.NotNil(t, rec.lastUnban)
	require.Equal(t, "account-b", rec.lastUnban.GetAccountId())
}

// TestTranscodeSpacesModeration_ListBans documents GET /api/v1/spaces/{spaceId}/bans.
func TestTranscodeSpacesModeration_ListBans(t *testing.T) {
	t.Parallel()
	rec := &recordingSpaceModeration{}
	h := newModerationContractGateway(t, rec)

	resp := performRequest(h, http.MethodGet, "/api/v1/spaces/space-9/bans", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastListBans)
	require.Equal(t, "space-9", rec.lastListBans.GetSpaceId())
}

// TestTranscodeSpacesModeration_TimeoutMember documents POST /api/v1/spaces/{spaceId}/members/{profileId}/timeout.
func TestTranscodeSpacesModeration_TimeoutMember(t *testing.T) {
	t.Parallel()
	rec := &recordingSpaceModeration{}
	h := newModerationContractGateway(t, rec)

	body := `{"duration_seconds":600,"reason":"heated"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/spaces/space-1/members/profile-b/timeout", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusNoContent, resp.Code)
	require.NotNil(t, rec.lastTimeout)
	require.Equal(t, "profile-b", rec.lastTimeout.GetProfileId())
	require.Equal(t, int32(600), rec.lastTimeout.GetDurationSeconds())
}

// TestTranscodeSpacesModeration_RemoveMemberTimeout documents DELETE /api/v1/spaces/{spaceId}/members/{profileId}/timeout.
func TestTranscodeSpacesModeration_RemoveMemberTimeout(t *testing.T) {
	t.Parallel()
	rec := &recordingSpaceModeration{}
	h := newModerationContractGateway(t, rec)

	_ = performRequest(h, http.MethodDelete, "/api/v1/spaces/space-1/members/profile-b/timeout", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Nil(t, rec.lastKick, "timeout route must not fall through to kick handler")
	require.NotNil(t, rec.lastRmTO)
	require.Equal(t, "profile-b", rec.lastRmTO.GetProfileId())
}
