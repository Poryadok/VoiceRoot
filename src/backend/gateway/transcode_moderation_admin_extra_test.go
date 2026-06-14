package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	moderationv1 "voice.app/voice/moderation/v1"
)

type recordingModerationPhase14 struct {
	moderationv1.UnimplementedModerationServiceServer
	listReq *moderationv1.ListReportsRequest
}

func (r *recordingModerationPhase14) ListReports(_ context.Context, req *moderationv1.ListReportsRequest) (*moderationv1.ListReportsResponse, error) {
	r.listReq = req
	return &moderationv1.ListReportsResponse{
		ReportList: &moderationv1.ReportList{Reports: []*moderationv1.Report{}},
	}, nil
}

func (r *recordingModerationPhase14) ApplySanction(context.Context, *moderationv1.ApplySanctionRequest) (*moderationv1.ApplySanctionResponse, error) {
	return &moderationv1.ApplySanctionResponse{
		Sanction: &moderationv1.Sanction{
			Id:                uuid.NewString(),
			TargetAccountId:   "acct",
			Type:              "warning",
			Reason:            "ok",
			IssuedByProfileId: "staff-profile",
			CreatedAt:         timestamppb.Now(),
		},
	}, nil
}

func (r *recordingModerationPhase14) ResolveReport(context.Context, *moderationv1.ResolveReportRequest) (*moderationv1.ResolveReportResponse, error) {
	return &moderationv1.ResolveReportResponse{
		Report: &moderationv1.Report{Id: "report-1", Status: "resolved"},
	}, nil
}

func TestTranscodeModerationAdmin_forwardsQueueFilter(t *testing.T) {
	rec := &recordingModerationPhase14{}
	modClient, cleanup := startBufconnModerationClient(t, rec)
	t.Cleanup(cleanup)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"staff-token": {UserID: "staff-account", ProfileID: "staff-profile", Roles: []string{"staff"}},
		},
		transcoder: &transcoder{clients: grpcClients{moderation: modClient}},
	})
	resp := performRequest(h, http.MethodGet, "/api/v1/admin/moderation/reports?status=pending&queue=spaces", "", map[string]string{
		"Authorization": "Bearer staff-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.listReq)
	require.Equal(t, "pending", rec.listReq.GetStatusFilter())
	require.Equal(t, "spaces", rec.listReq.GetQueueFilter())
}
