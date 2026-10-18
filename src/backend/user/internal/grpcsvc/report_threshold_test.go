package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/user/internal/reportaudience"
)

type reportAudienceSource struct {
	calls            int
	profile, account uuid.UUID
}

func (s *reportAudienceSource) Snapshot(context.Context, uuid.UUID) (reportaudience.Snapshot, error) {
	s.calls++
	return reportaudience.Snapshot{Binding: reportaudience.Binding{ProfileID: s.profile, AccountID: s.account, Revision: 1, State: reportaudience.Live}}, nil
}
func (*reportAudienceSource) Recheck(context.Context, reportaudience.Binding) error { return nil }

type emptyReportClassifier struct{}

func (emptyReportClassifier) Classify(context.Context, uuid.UUID, []uuid.UUID) (reportaudience.Classification, error) {
	panic("empty candidate list")
}

func TestReportThresholdRequiresExactModerationAuthority(t *testing.T) {
	req := &userv1.ResolveReportThresholdContextRequest{EvaluationId: uuid.NewString(), TargetProfileId: uuid.NewString()}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	base := principal.Principal{Kind: "service", Issuer: "moderation", Subject: "service:moderation", Audience: "user", RPC: userv1.UserService_ResolveReportThresholdContext_FullMethodName, RequestHash: hash}
	for _, mode := range []string{"none", "wrong issuer", "wrong subject", "wrong audience", "wrong rpc", "wrong body", "delegated", "account", "profile", "epoch", "valid"} {
		t.Run(mode, func(t *testing.T) {
			source := &reportAudienceSource{profile: uuid.MustParse(req.TargetProfileId), account: uuid.New()}
			handler := &ReportThresholdGRPC{Audience: reportaudience.Service{Source: source, Classifier: emptyReportClassifier{}, Now: time.Now}}
			verified := base
			switch mode {
			case "wrong issuer":
				verified.Issuer = "social"
			case "wrong subject":
				verified.Subject = "service:social"
			case "wrong audience":
				verified.Audience = "auth"
			case "wrong rpc":
				verified.RPC = "/voice.user.v1.UserService/GetProfile"
			case "wrong body":
				verified.RequestHash = "sha256:wrong"
			case "delegated":
				verified.Kind = "delegated_user"
			case "account":
				verified.AccountID = uuid.NewString()
			case "profile":
				verified.ProfileID = uuid.NewString()
			case "epoch":
				verified.SessionEpoch = 1
			}
			ctx := context.Background()
			if mode != "none" {
				ctx = principal.WithVerified(ctx, verified)
			}
			got, err := handler.ResolveReportThresholdContext(ctx, req)
			if mode != "valid" {
				require.Equal(t, codes.Unauthenticated, status.Code(err))
				require.Zero(t, source.calls)
				return
			}
			require.NoError(t, err)
			require.Equal(t, userv1.ReportThresholdContextState_REPORT_THRESHOLD_CONTEXT_STATE_EMPTY_AUDIENCE, got.State)
			require.Equal(t, source.account.String(), got.TargetAccountId)
			require.Equal(t, 1, source.calls)
		})
	}
}

func TestReportThresholdPrivateServiceDefinition(t *testing.T) {
	server := grpc.NewServer()
	defer server.Stop()
	RegisterReportThresholdServer(server, reportaudience.Service{})
	methods := server.GetServiceInfo()["voice.user.v1.UserService"].Methods
	require.Len(t, methods, 1)
	require.Equal(t, "ResolveReportThresholdContext", methods[0].Name)
	_, err := (&UserGRPC{}).ResolveReportThresholdContext(context.Background(), &userv1.ResolveReportThresholdContextRequest{})
	require.Equal(t, codes.Unimplemented, status.Code(err), "ordinary User handler cannot expose private existence authority")
}
