package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	moderationv1 "voice.app/voice/moderation/v1"
	"voice/backend/pkg/principal"
)

type recordingModerationReports struct {
	moderationv1.UnimplementedModerationServiceServer
	lastCreate *moderationv1.CreateReportRequest
	lastAppeal *moderationv1.SubmitAppealRequest
}

func (s *recordingModerationReports) SubmitAppeal(_ context.Context, req *moderationv1.SubmitAppealRequest) (*moderationv1.SubmitAppealResponse, error) {
	s.lastAppeal = req
	return &moderationv1.SubmitAppealResponse{
		Appeal: &moderationv1.Appeal{
			Id:                 "appeal-created",
			SanctionId:         req.GetSanctionId(),
			AppellantAccountId: "account-1",
			Reason:             req.GetReason(),
			Status:             "pending",
		},
	}, nil
}

func (s *recordingModerationReports) CreateReport(_ context.Context, req *moderationv1.CreateReportRequest) (*moderationv1.CreateReportResponse, error) {
	s.lastCreate = req
	return &moderationv1.CreateReportResponse{
		Report: &moderationv1.Report{
			Id: "report-accepted",
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
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	return newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 1, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)},
		},
		transcoder: &transcoder{lifecycleIssuer: issuer, clients: grpcClients{moderation: modClient, reportIntake: modClient}},
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

// TestTranscodeModeration_SubmitAppeal documents POST /api/v1/moderation/appeals → 201 Created.
func TestTranscodeModeration_SubmitAppeal(t *testing.T) {
	t.Parallel()
	rec := &recordingModerationReports{}
	h := newReportsContractGateway(t, rec)

	body := `{"sanction_id":"sanction-1","reason":"I believe this was a mistake"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/moderation/appeals", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusCreated, resp.Code)
	require.NotNil(t, rec.lastAppeal)
	require.Equal(t, "sanction-1", rec.lastAppeal.GetSanctionId())
	require.Equal(t, "I believe this was a mistake", rec.lastAppeal.GetReason())
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
