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

	moderationv1 "voice.app/voice/moderation/v1"
)

type recordingModerationReports struct {
	moderationv1.UnimplementedModerationServiceServer
	lastCreate *moderationv1.CreateReportRequest
}

func (s *recordingModerationReports) CreateReport(_ context.Context, req *moderationv1.CreateReportRequest) (*moderationv1.CreateReportResponse, error) {
	s.lastCreate = req
	return &moderationv1.CreateReportResponse{
		Report: &moderationv1.Report{
			Id:                 "report-accepted",
			ReporterProfileId:  "profile-1",
			TargetType:         req.GetTargetType(),
			TargetId:           req.GetTargetId(),
			Category:           req.GetCategory(),
			Status:             "pending",
			EvidenceJson:       req.GetEvidenceJson(),
			ResolutionJson:     `{}`,
		},
	}, nil
}

func startBufconnModerationClient(t *testing.T, impl moderationv1.ModerationServiceServer) (moderationv1.ModerationServiceClient, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	moderationv1.RegisterModerationServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return moderationv1.NewModerationServiceClient(conn), func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

func newReportsContractGateway(t *testing.T, rec *recordingModerationReports) http.Handler {
	t.Helper()
	modClient, cleanup := startBufconnModerationClient(t, rec)
	t.Cleanup(cleanup)
	return newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{moderation: modClient}},
		restUpstreams: map[string]http.Handler{
			"moderation": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotImplemented)
			}),
		},
	})
}

// TestTranscodeModeration_CreateReport documents POST /api/v1/moderation/reports → 202 Accepted.
func TestTranscodeModeration_CreateReport(t *testing.T) {
	t.Parallel()
	rec := &recordingModerationReports{}
	h := newReportsContractGateway(t, rec)

	body := `{"target_type":"user","target_id":"profile-target","category":"harassment","evidence_json":"{}"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/moderation/reports", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusAccepted, resp.Code)
	require.NotNil(t, rec.lastCreate)
	require.Equal(t, "user", rec.lastCreate.GetTargetType())
	require.Equal(t, "profile-target", rec.lastCreate.GetTargetId())
	require.Equal(t, "harassment", rec.lastCreate.GetCategory())
}

// TestTranscodeModeration_MMToxicAliasMapsToCheating documents gateway maps mm_toxic → cheating for Moderation RPC.
func TestTranscodeModeration_MMToxicAliasMapsToCheating(t *testing.T) {
	t.Parallel()
	rec := &recordingModerationReports{}
	h := newReportsContractGateway(t, rec)

	body := `{"target_type":"user","target_id":"profile-mm","category":"mm_toxic","evidence_json":"{}"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/moderation/reports", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusAccepted, resp.Code)
	require.NotNil(t, rec.lastCreate)
	require.Equal(t, "cheating", rec.lastCreate.GetCategory())
}

// TestTranscodeModeration_ListReports_NotPublic documents moderator queue is not exposed on public REST.
func TestTranscodeModeration_ListReports_NotPublic(t *testing.T) {
	t.Parallel()
	rec := &recordingModerationReports{}
	h := newReportsContractGateway(t, rec)

	resp := performRequest(h, http.MethodGet, "/api/v1/moderation/reports", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.NotEqual(t, http.StatusOK, resp.Code)
	require.Nil(t, rec.lastCreate)
}
