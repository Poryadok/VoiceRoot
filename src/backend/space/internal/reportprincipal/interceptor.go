package reportprincipal

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type Verifier struct {
	Keys   principal.KeyResolver
	Replay principal.ReplayGuard
	Clock  func() time.Time
}
type proofKey struct{}
type verifiedProof struct{ hash string }

func canonicalID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}
func ValidRequest(req *spacev1.ResolveSpaceReportReadContextRequest) bool {
	return req != nil && len(req.ProtoReflect().GetUnknown()) == 0 && req.ProtocolVersion == 1 && canonicalID(req.OperationId) && canonicalID(req.SpaceId) && canonicalID(req.ViewerProfileId) && req.Chat != nil && len(req.Chat.ProtoReflect().GetUnknown()) == 0 && canonicalID(req.Chat.Id) && req.Chat.Type != nil && (req.Chat.GetType() == chatv1.ChatType_CHAT_TYPE_GROUP || req.Chat.GetType() == chatv1.ChatType_CHAT_TYPE_CHANNEL)
}
func (v Verifier) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, value any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if info.FullMethod != spacev1.SpaceService_ResolveSpaceReportReadContext_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on report read listener")
		}
		req, ok := value.(*spacev1.ResolveSpaceReportReadContextRequest)
		if !ok || !ValidRequest(req) {
			return nil, status.Error(codes.InvalidArgument, "invalid report read request")
		}
		deny := status.Error(codes.Unauthenticated, "invalid report read principal")
		if v.Keys == nil || v.Replay == nil {
			return nil, deny
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || transport.RequestID != req.OperationId {
			return nil, deny
		}
		hash, err := principal.RequestHash(req)
		if err != nil {
			return nil, deny
		}
		verified, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "messaging", ExpectedAudience: "space", ExpectedRPC: info.FullMethod, ExpectedRequestID: req.OperationId, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.Replay, Clock: v.Clock})
		if err != nil || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
			return nil, deny
		}
		return next(context.WithValue(ctx, proofKey{}, verifiedProof{hash: hash}), req)
	}
}
func Producer(ctx context.Context, req *spacev1.ResolveSpaceReportReadContextRequest) bool {
	if ctx == nil || !ValidRequest(req) {
		return false
	}
	proof, ok := ctx.Value(proofKey{}).(verifiedProof)
	if !ok {
		return false
	}
	hash, err := principal.RequestHash(req)
	return err == nil && hash == proof.hash
}
func OrdinaryUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, value any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if info.FullMethod == spacev1.SpaceService_ResolveSpaceReportReadContext_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "protected report read listener required")
		}
		return next(ctx, value)
	}
}
