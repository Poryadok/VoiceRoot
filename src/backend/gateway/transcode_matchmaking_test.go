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

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

type recordingMatchmakingGRPC struct {
	matchmakingv1.UnimplementedMatchmakingServiceServer
	lastList     *matchmakingv1.ListGamesRequest
	lastGet      *matchmakingv1.GetGameRequest
	lastSearch   *matchmakingv1.SearchGamesRequest
	lastCreate   *matchmakingv1.CreateGameRequest
	lastSubmit   *matchmakingv1.SubmitGameRequestRequest
	lastListReq  *matchmakingv1.ListGameRequestsRequest
	lastApprove  *matchmakingv1.ApproveGameRequestRequest
	lastReject   *matchmakingv1.RejectGameRequestRequest
}

func (s *recordingMatchmakingGRPC) ListGames(_ context.Context, req *matchmakingv1.ListGamesRequest) (*matchmakingv1.ListGamesResponse, error) {
	s.lastList = req
	return &matchmakingv1.ListGamesResponse{
		GameList: &matchmakingv1.GameList{
			Games: []*matchmakingv1.Game{{
				Id:         "game-1",
				Name:       "Dota 2",
				ConfigJson: `{"regions":["eu"],"modes":[{"name":"5v5","slots":10,"party_size_min":1,"party_size_max":5,"roles":[{"name":"Carry","required":true}],"ranks":[{"name":"Herald","value":0}]}]}`,
				Status:     "active",
			}},
		},
	}, nil
}

func (s *recordingMatchmakingGRPC) GetGame(_ context.Context, req *matchmakingv1.GetGameRequest) (*matchmakingv1.GetGameResponse, error) {
	s.lastGet = req
	return &matchmakingv1.GetGameResponse{
		Game: &matchmakingv1.Game{Id: req.GetGameId(), Name: "Dota 2", Status: "active"},
	}, nil
}

func (s *recordingMatchmakingGRPC) SearchGames(_ context.Context, req *matchmakingv1.SearchGamesRequest) (*matchmakingv1.SearchGamesResponse, error) {
	s.lastSearch = req
	return &matchmakingv1.SearchGamesResponse{
		GameList: &matchmakingv1.GameList{
			Games: []*matchmakingv1.Game{{Id: "game-1", Name: "Dota 2"}},
		},
	}, nil
}

func (s *recordingMatchmakingGRPC) CreateGame(_ context.Context, req *matchmakingv1.CreateGameRequest) (*matchmakingv1.CreateGameResponse, error) {
	s.lastCreate = req
	return &matchmakingv1.CreateGameResponse{
		Game: &matchmakingv1.Game{Id: "new-game", Name: req.GetName(), ConfigJson: req.GetConfigJson()},
	}, nil
}

func (s *recordingMatchmakingGRPC) SubmitGameRequest(_ context.Context, req *matchmakingv1.SubmitGameRequestRequest) (*matchmakingv1.SubmitGameRequestResponse, error) {
	s.lastSubmit = req
	return &matchmakingv1.SubmitGameRequestResponse{
		Game: &matchmakingv1.Game{
			Id:         "pending-1",
			Name:       req.GetName(),
			ConfigJson: req.GetConfigJson(),
			Status:     "pending_moderation",
		},
	}, nil
}

func (s *recordingMatchmakingGRPC) ListGameRequests(_ context.Context, req *matchmakingv1.ListGameRequestsRequest) (*matchmakingv1.ListGameRequestsResponse, error) {
	s.lastListReq = req
	return &matchmakingv1.ListGameRequestsResponse{
		GameList: &matchmakingv1.GameList{
			Games: []*matchmakingv1.Game{{Id: "pending-1", Name: "Pending Game", Status: "pending_moderation"}},
		},
	}, nil
}

func (s *recordingMatchmakingGRPC) ApproveGameRequest(_ context.Context, req *matchmakingv1.ApproveGameRequestRequest) (*matchmakingv1.ApproveGameRequestResponse, error) {
	s.lastApprove = req
	return &matchmakingv1.ApproveGameRequestResponse{
		Game: &matchmakingv1.Game{Id: req.GetGameId(), Name: "Approved", Status: "active"},
	}, nil
}

func (s *recordingMatchmakingGRPC) RejectGameRequest(_ context.Context, req *matchmakingv1.RejectGameRequestRequest) (*matchmakingv1.RejectGameRequestResponse, error) {
	s.lastReject = req
	return &matchmakingv1.RejectGameRequestResponse{
		Game: &matchmakingv1.Game{Id: req.GetGameId(), Name: "Rejected", Status: "rejected"},
	}, nil
}

type recordingMatchmakingSearchGRPC struct {
	recordingMatchmakingGRPC
	lastStart  *matchmakingv1.StartSearchRequest
	lastCancel *matchmakingv1.CancelSearchRequest
	lastStatus *matchmakingv1.GetSearchStatusRequest
}

func (s *recordingMatchmakingSearchGRPC) StartSearch(_ context.Context, req *matchmakingv1.StartSearchRequest) (*matchmakingv1.StartSearchResponse, error) {
	s.lastStart = req
	return &matchmakingv1.StartSearchResponse{
		SearchSession: &matchmakingv1.SearchSession{
			Id:           "session-1",
			ProfileId:    "profile-1",
			GameId:       req.GetGameId(),
			Mode:         req.GetMode(),
			CriteriaJson: req.GetCriteriaJson(),
			Status:       "searching",
		},
	}, nil
}

func (s *recordingMatchmakingSearchGRPC) CancelSearch(_ context.Context, req *matchmakingv1.CancelSearchRequest) (*matchmakingv1.CancelSearchResponse, error) {
	s.lastCancel = req
	return &matchmakingv1.CancelSearchResponse{}, nil
}

func (s *recordingMatchmakingSearchGRPC) GetSearchStatus(_ context.Context, req *matchmakingv1.GetSearchStatusRequest) (*matchmakingv1.GetSearchStatusResponse, error) {
	s.lastStatus = req
	return &matchmakingv1.GetSearchStatusResponse{
		SearchSession: &matchmakingv1.SearchSession{
			Id:     req.GetSessionId(),
			Status: "searching",
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

func TestTranscodeMatchmakingListGames(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	rec := performRequest(h, http.MethodGet, "/api/v1/matchmaking/games", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, grpcRec.lastList)
}

func TestTranscodeMatchmakingGetGame(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	rec := performRequest(h, http.MethodGet, "/api/v1/matchmaking/games/game-1", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "game-1", grpcRec.lastGet.GetGameId())
}

func TestTranscodeMatchmakingSearchGames(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	rec := performRequest(h, http.MethodGet, "/api/v1/matchmaking/games/search?query=dota", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "dota", grpcRec.lastSearch.GetQuery())
}

func TestTranscodeMatchmakingCreateGameRequiresAuth(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	body := `{"name":"New Game","configJson":"{\"regions\":[\"eu\"],\"modes\":[{\"name\":\"Solo\",\"slots\":1,\"party_size_min\":1,\"party_size_max\":1}]}"}`
	rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/games", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "New Game", grpcRec.lastCreate.GetName())
}

func TestTranscodeMatchmakingStartSearch(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingSearchGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	body := `{"gameId":"game-1","mode":"5v5 Ranked","criteriaJson":"{\"region\":\"eu\"}"}`
	rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/search", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "game-1", grpcRec.lastStart.GetGameId())
}

func TestTranscodeMatchmakingGetSearchStatus(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingSearchGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	rec := performRequest(h, http.MethodGet, "/api/v1/matchmaking/search/session-1", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "session-1", grpcRec.lastStatus.GetSessionId())
}

func TestTranscodeMatchmakingCancelSearch(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingSearchGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	rec := performRequest(h, http.MethodDelete, "/api/v1/matchmaking/search/session-1", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "session-1", grpcRec.lastCancel.GetSessionId())
}

func TestTranscodeMatchmakingUnavailableWhenClientNil(t *testing.T) {
	t.Parallel()
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{}},
	})
	rec := performRequest(h, http.MethodGet, "/api/v1/matchmaking/games", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTranscodeMatchmakingSubmitGameRequest(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	body := `{"name":"Apex","config_json":"{\"regions\":[\"eu\"],\"modes\":[{\"name\":\"Duos\",\"slots\":2,\"party_size_min\":1,\"party_size_max\":2}]}"}`
	rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/game-requests", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, grpcRec.lastSubmit)
	require.Equal(t, "Apex", grpcRec.lastSubmit.GetName())
}

func TestTranscodeAdminMatchmaking_ApproveStaffOnly(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"staff-token":  {UserID: "staff-account", ProfileID: "staff-profile", Roles: []string{"staff"}},
			"member-token": {UserID: "account-1", ProfileID: "profile-1", Roles: []string{"member"}},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})

	list := performRequest(h, http.MethodGet, "/api/v1/admin/matchmaking/game-requests", "", map[string]string{
		"Authorization": "Bearer staff-token",
	})
	require.Equal(t, http.StatusOK, list.Code)
	require.NotNil(t, grpcRec.lastListReq)

	approve := performRequest(h, http.MethodPost, "/api/v1/admin/matchmaking/game-requests/pending-1/approve", "", map[string]string{
		"Authorization": "Bearer staff-token",
	})
	require.Equal(t, http.StatusOK, approve.Code)
	require.Equal(t, "pending-1", grpcRec.lastApprove.GetGameId())

	denied := performRequest(h, http.MethodPost, "/api/v1/admin/matchmaking/game-requests/pending-1/approve", "", map[string]string{
		"Authorization": "Bearer member-token",
	})
	require.Equal(t, http.StatusForbidden, denied.Code)
}

