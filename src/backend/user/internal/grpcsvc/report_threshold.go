package grpcsvc

import (
	"context"
	"math"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/user/internal/reportaudience"
)

// ReportThresholdGRPC is registered only on Moderation's protected listener.
// Ordinary UserGRPC continues to embed the unimplemented private method.
type ReportThresholdGRPC struct {
	userv1.UnimplementedUserServiceServer
	Audience reportaudience.Service
}

func (s *ReportThresholdGRPC) ResolveReportThresholdContext(ctx context.Context, req *userv1.ResolveReportThresholdContextRequest) (*userv1.ResolveReportThresholdContextResponse, error) {
	verified, ok := principal.FromContext(ctx)
	hash, err := principal.RequestHash(req)
	if !ok || err != nil || verified.Kind != "service" || verified.Issuer != "moderation" || verified.Subject != "service:moderation" || verified.Audience != "user" ||
		verified.RPC != userv1.UserService_ResolveReportThresholdContext_FullMethodName || verified.RequestHash != hash || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return nil, status.Error(codes.Unauthenticated, "verified Moderation report authority required")
	}
	evaluation, err := canonicalSDKUUID(req.GetEvaluationId())
	if err != nil {
		return nil, err
	}
	profile, err := canonicalSDKUUID(req.GetTargetProfileId())
	if err != nil {
		return nil, err
	}
	var expected *reportaudience.Binding
	if req.ExpectedTargetAccountId != nil || req.ExpectedProfileRevision != nil {
		if req.ExpectedTargetAccountId == nil || req.ExpectedProfileRevision == nil || req.GetExpectedProfileRevision() == 0 || req.GetExpectedProfileRevision() > math.MaxInt64 {
			return nil, status.Error(codes.InvalidArgument, "complete expected target binding required")
		}
		account, parseErr := canonicalSDKUUID(req.GetExpectedTargetAccountId())
		if parseErr != nil {
			return nil, parseErr
		}
		expected = &reportaudience.Binding{ProfileID: profile, AccountID: account, Revision: req.GetExpectedProfileRevision()}
	}
	if s == nil {
		return nil, status.Error(codes.Unavailable, "report audience authority unavailable")
	}
	result, err := s.Audience.Resolve(ctx, evaluation, profile, expected)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "report audience authority unavailable")
	}
	state := userv1.ReportThresholdContextState_REPORT_THRESHOLD_CONTEXT_STATE_UNSPECIFIED
	switch result.State {
	case reportaudience.Live:
		state = userv1.ReportThresholdContextState_REPORT_THRESHOLD_CONTEXT_STATE_LIVE
	case reportaudience.Deleted:
		state = userv1.ReportThresholdContextState_REPORT_THRESHOLD_CONTEXT_STATE_DELETED
	case reportaudience.Missing:
		state = userv1.ReportThresholdContextState_REPORT_THRESHOLD_CONTEXT_STATE_MISSING
	case reportaudience.EmptyAudience:
		state = userv1.ReportThresholdContextState_REPORT_THRESHOLD_CONTEXT_STATE_EMPTY_AUDIENCE
	}
	out := &userv1.ResolveReportThresholdContextResponse{State: state, TargetProfileId: profile.String(), ProfileRevision: result.Revision, MetricVersion: result.Metric, AudienceAccounts: result.Audience, EvaluationId: evaluation.String(), SnapshotId: result.SnapshotID.String(), ObservationStartedAt: timestamppb.New(result.StartedAt), ObservationCompletedAt: timestamppb.New(result.CompletedAt), ExpiresAt: timestamppb.New(result.ExpiresAt)}
	if result.State != reportaudience.Missing {
		out.TargetAccountId = result.AccountID.String()
	}
	return out, nil
}

func RegisterReportThresholdServer(server grpc.ServiceRegistrar, audience reportaudience.Service) {
	desc := userv1.UserService_ServiceDesc
	desc.Methods = nil
	desc.Streams = nil
	for _, method := range userv1.UserService_ServiceDesc.Methods {
		if method.MethodName == "ResolveReportThresholdContext" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	server.RegisterService(&desc, &ReportThresholdGRPC{Audience: audience})
}
