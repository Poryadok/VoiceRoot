package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
	spacev1 "voice.app/voice/space/v1"
)

type recordingSpaceQueueMM struct {
	matchmakingv1.UnimplementedMatchmakingServiceServer
	last *matchmakingv1.StartSpaceQueueRequest
}

func (s *recordingSpaceQueueMM) StartSpaceQueue(_ context.Context, req *matchmakingv1.StartSpaceQueueRequest) (*matchmakingv1.StartSpaceQueueResponse, error) {
	s.last = req
	sid := req.GetSpaceId()
	return &matchmakingv1.StartSpaceQueueResponse{
		SearchSession: &matchmakingv1.SearchSession{
			Id:        "sess-space-1",
			ProfileId: "profile-1",
			GameId:    req.GetGameId(),
			Mode:      req.GetMode(),
			Status:    "searching",
			SpaceId:   &sid,
		},
	}, nil
}

type recordingSpaceMmConfig struct {
	spacev1.UnimplementedSpaceServiceServer
	last *spacev1.UpdateSpaceMmConfigRequest
}

func (s *recordingSpaceMmConfig) UpdateSpaceMmConfig(_ context.Context, req *spacev1.UpdateSpaceMmConfigRequest) (*spacev1.UpdateSpaceMmConfigResponse, error) {
	s.last = req
	now := timestamppb.Now()
	return &spacev1.UpdateSpaceMmConfigResponse{
		Space: &spacev1.Space{
			Id:             req.GetSpaceId(),
			Name:           "space-1",
			MmConfigJson:   req.GetMmConfigJson(),
			OwnerProfileId: "profile-1",
			MemberCount:    1,
			Visibility:     "private",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}, nil
}

func startBufconnMatchmakingConn(t *testing.T, impl matchmakingv1.MatchmakingServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	matchmakingv1.RegisterMatchmakingServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn, func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

func TestTranscodeSpacesMatchmakingQueue(t *testing.T) {
	t.Parallel()
	mmRec := &recordingSpaceQueueMM{}
	mmConn, mmCleanup := startBufconnMatchmakingConn(t, mmRec)
	t.Cleanup(mmCleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{
			matchmaking: matchmakingv1.NewMatchmakingServiceClient(mmConn),
		}},
	})

	body := `{"gameId":"game-1","mode":"Duo","criteriaJson":"{\"region\":\"eu\"}"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/spaces/space-42/matchmaking/queue", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	require.NotNil(t, mmRec.last)
	require.Equal(t, "space-42", mmRec.last.GetSpaceId())
	require.Equal(t, "game-1", mmRec.last.GetGameId())
	require.Equal(t, "Duo", mmRec.last.GetMode())

	var out map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &out))
}

func TestTranscodeSpacesMatchmakingConfig(t *testing.T) {
	t.Parallel()
	spaceRec := &recordingSpaceMmConfig{}
	spaceConn, spaceCleanup := startBufconnSpaceConn(t, spaceRec)
	t.Cleanup(spaceCleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{
			space: spacev1.NewSpaceServiceClient(spaceConn),
		}},
	})

	body := `{"mmConfigJson":"{\"enabled\":true}"}`
	resp := performRequest(h, http.MethodPatch, "/api/v1/spaces/space-9/matchmaking/config", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	require.NotNil(t, spaceRec.last)
	require.Equal(t, "space-9", spaceRec.last.GetSpaceId())
	require.Equal(t, `{"enabled":true}`, spaceRec.last.GetMmConfigJson())
}
