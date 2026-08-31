package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	moderationv1 "voice.app/voice/moderation/v1"
)

type recordingModerationAudit struct {
	moderationv1.UnimplementedModerationServiceServer
	exportCalled bool
}

func (r *recordingModerationAudit) ExportAuditLog(_ context.Context, _ *moderationv1.ExportAuditLogRequest) (*moderationv1.ExportAuditLogResponse, error) {
	r.exportCalled = true
	now := timestamppb.New(time.Now().UTC())
	return &moderationv1.ExportAuditLogResponse{
		AuditLogExport: &moderationv1.AuditLogExport{
			Entries: []*moderationv1.AuditLogEntry{{
				Id:              "audit-1",
				ActorProfileId:  "staff-profile",
				Action:          "apply_sanction",
				TargetType:      "account",
				TargetId:        "acct-1",
				Details:         `{"type":"warning"}`,
				CreatedAt:       now,
			}},
		},
	}, nil
}

type revokeSanctionRecorder struct {
	moderationv1.UnimplementedModerationServiceServer
	sanctionID string
}

func (r *revokeSanctionRecorder) RevokeSanction(_ context.Context, req *moderationv1.RevokeSanctionRequest) (*moderationv1.RevokeSanctionResponse, error) {
	r.sanctionID = req.GetSanctionId()
	return &moderationv1.RevokeSanctionResponse{}, nil
}

type accountSanctionsRecorder struct {
	moderationv1.UnimplementedModerationServiceServer
	accountID string
}

func (r *accountSanctionsRecorder) GetAccountSanctions(_ context.Context, req *moderationv1.GetAccountSanctionsRequest) (*moderationv1.GetAccountSanctionsResponse, error) {
	r.accountID = req.GetAccountId()
	return &moderationv1.GetAccountSanctionsResponse{
		SanctionList: &moderationv1.SanctionList{
			Sanctions: []*moderationv1.Sanction{{
				Id:                "sanction-1",
				TargetAccountId:   req.GetAccountId(),
				Type:              "warning",
				Reason:            "test",
				IssuedByProfileId: "staff-profile",
				CreatedAt:         timestamppb.Now(),
			}},
		},
	}, nil
}

func TestTranscodeModerationAdmin_auditExport_returnsEntries(t *testing.T) {
	t.Parallel()

	rec := &recordingModerationAudit{}
	modClient, cleanup := startBufconnModerationClient(t, rec)
	t.Cleanup(cleanup)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"staff-token": {UserID: "staff-account", ProfileID: "staff-profile", Roles: []string{"staff"}},
		},
		transcoder: &transcoder{clients: grpcClients{moderation: modClient}},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/admin/moderation/audit/export", "", map[string]string{
		"Authorization": "Bearer staff-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.True(t, rec.exportCalled)

	var body map[string][]map[string]string
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Len(t, body["entries"], 1)
	require.Equal(t, "audit-1", body["entries"][0]["id"])
	require.Equal(t, "apply_sanction", body["entries"][0]["action"])
}

func TestTranscodeModerationAdmin_revokeSanction_staffOnly(t *testing.T) {
	t.Parallel()

	rec := &revokeSanctionRecorder{}
	modClient, cleanup := startBufconnModerationClient(t, rec)
	t.Cleanup(cleanup)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"staff-token":  {UserID: "staff-account", ProfileID: "staff-profile", Roles: []string{"staff"}},
			"member-token": {UserID: "account-1", ProfileID: "profile-1", Roles: []string{"member"}},
		},
		transcoder: &transcoder{clients: grpcClients{moderation: modClient}},
	})

	staff := performRequest(h, http.MethodPost, "/api/v1/admin/moderation/sanctions/sanction-1/revoke", "", map[string]string{
		"Authorization": "Bearer staff-token",
	})
	require.Equal(t, http.StatusNoContent, staff.Code)
	require.Equal(t, "sanction-1", rec.sanctionID)

	member := performRequest(h, http.MethodPost, "/api/v1/admin/moderation/sanctions/sanction-1/revoke", "", map[string]string{
		"Authorization": "Bearer member-token",
	})
	require.Equal(t, http.StatusForbidden, member.Code)
}

func TestTranscodeModerationAdmin_getAccountSanctions_staffOnly(t *testing.T) {
	t.Parallel()

	rec := &accountSanctionsRecorder{}
	modClient, cleanup := startBufconnModerationClient(t, rec)
	t.Cleanup(cleanup)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"staff-token":  {UserID: "staff-account", ProfileID: "staff-profile", Roles: []string{"staff"}},
			"member-token": {UserID: "account-1", ProfileID: "profile-1", Roles: []string{"member"}},
		},
		transcoder: &transcoder{clients: grpcClients{moderation: modClient}},
	})

	staff := performRequest(h, http.MethodGet, "/api/v1/admin/moderation/sanctions?account_id=acct-1", "", map[string]string{
		"Authorization": "Bearer staff-token",
	})
	require.Equal(t, http.StatusOK, staff.Code)
	require.Equal(t, "acct-1", rec.accountID)

	member := performRequest(h, http.MethodGet, "/api/v1/admin/moderation/sanctions?account_id=acct-1", "", map[string]string{
		"Authorization": "Bearer member-token",
	})
	require.Equal(t, http.StatusForbidden, member.Code)
}
