package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	socialv1 "voice.app/voice/social/v1"
	"voice/backend/pkg/principal"
)

// MessagingReportBlocksGRPC registers only the bounded report-policy decision.
// Profile-to-account resolution remains in Social's existing native evaluator.
type MessagingReportBlocksGRPC struct {
	socialv1.UnimplementedSocialServiceServer
	Social *SocialGRPC
}

func (s *MessagingReportBlocksGRPC) IsProfilePairsBlocked(ctx context.Context, req *socialv1.IsProfilePairsBlockedRequest) (*socialv1.IsProfilePairsBlockedResponse, error) {
	p, ok := principal.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "verified Messaging principal required")
	}
	if p.Kind != "service" || p.Issuer != "messaging" || p.Subject != "service:messaging" || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
		return nil, status.Error(codes.PermissionDenied, "Messaging report policy principal required")
	}
	canonical := func(raw string) bool {
		id, e := uuid.Parse(raw)
		return e == nil && id != uuid.Nil && id.String() == raw
	}
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.ProtocolVersion != 1 || !canonical(req.OperationId) || !canonical(req.ViewerProfileId) || len(req.OtherProfileIds) == 0 || len(req.OtherProfileIds) > 100 {
		return nil, status.Error(codes.InvalidArgument, "invalid report block policy request")
	}
	seen := make(map[string]bool, len(req.OtherProfileIds))
	for _, id := range req.OtherProfileIds {
		if !canonical(id) || seen[id] {
			return nil, status.Error(codes.InvalidArgument, "invalid report block policy candidates")
		}
		seen[id] = true
	}
	hash, err := principal.RequestHash(req)
	if err != nil || p.Audience != "social" || p.RPC != socialv1.SocialService_IsProfilePairsBlocked_FullMethodName || p.RequestID != req.OperationId || p.RequestHash != hash {
		return nil, status.Error(codes.Unauthenticated, "invalid report block policy binding")
	}
	if s == nil || s.Social == nil {
		return nil, status.Error(codes.Unavailable, "report block policy unavailable")
	}
	response, err := s.Social.IsProfilePairsBlocked(ctx, req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		return nil, status.Error(codes.Unavailable, "report block policy unavailable")
	}
	if response == nil || len(response.ProtoReflect().GetUnknown()) != 0 || len(response.Results) != len(req.OtherProfileIds) {
		return nil, status.Error(codes.Unavailable, "report block policy unavailable")
	}
	for i, result := range response.Results {
		if result == nil || len(result.ProtoReflect().GetUnknown()) != 0 || result.OtherProfileId != req.OtherProfileIds[i] {
			return nil, status.Error(codes.Unavailable, "report block policy unavailable")
		}
	}
	response.ProtocolVersion = 1
	response.OperationId = req.OperationId
	response.ViewerProfileId = req.ViewerProfileId
	return response, nil
}

// OrdinaryMessagingReportUnary keeps the versioned capability off legacy routes.
// Existing unversioned callers retain their separately migrated contract.
func OrdinaryMessagingReportUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if info.FullMethod == socialv1.SocialService_IsProfilePairsBlocked_FullMethodName {
			if r, ok := req.(*socialv1.IsProfilePairsBlockedRequest); ok && (r.GetProtocolVersion() != 0 || r.GetOperationId() != "") {
				return nil, status.Error(codes.PermissionDenied, "report block policy requires protected listener")
			}
		}
		return next(ctx, req)
	}
}
