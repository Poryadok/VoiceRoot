package memberauthorityprincipal

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type Verifier struct {
	Keys   principal.KeyResolver
	Replay principal.ReplayGuard
	Clock  func() time.Time
}

type proofKey struct{}
type verifiedProof struct{ requestHash string }

func canonicalID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func validKnownRequest(req *spacev1.GetMemberAdmissionAuthorityRequest) bool {
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.ProtocolVersion != 1 || !canonicalID(req.SpaceId) || !canonicalID(req.ProfileId) {
		return false
	}
	if (req.ExpectedGeneration == nil) != (req.ExpectedAdmissionOperationId == nil) || req.GetExpectedGeneration() > math.MaxInt64 {
		return false
	}
	return (req.ExpectedAdmissionOperationId == nil || canonicalID(*req.ExpectedAdmissionOperationId)) && (req.ChatId == nil || canonicalID(*req.ChatId))
}

// Unary admits only an exact service:role request on this private listener.
// It creates no generic principal or delegated user authorization surface.
func (v Verifier) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, value any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "method is not available on member authority listener")
		}
		req, ok := value.(*spacev1.GetMemberAdmissionAuthorityRequest)
		if !ok || !validKnownRequest(req) {
			return nil, status.Error(codes.InvalidArgument, "invalid member authority request")
		}
		deny := status.Error(codes.Unauthenticated, "invalid member authority principal")
		if v.Keys == nil || v.Replay == nil {
			return nil, deny
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, deny
		}
		hash, err := principal.RequestHash(req)
		if err != nil {
			return nil, deny
		}
		_, err = principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{
			ExpectedIssuer: "role", ExpectedAudience: "space", ExpectedRPC: info.FullMethod,
			ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash,
			KeyResolver: v.Keys, ReplayGuard: v.Replay, Clock: v.Clock,
		})
		if err != nil {
			return nil, deny
		}
		return handler(context.WithValue(ctx, proofKey{}, verifiedProof{requestHash: hash}), req)
	}
}

// Producer binds the handler's consumed body to this package's private proof.
func Producer(ctx context.Context, req *spacev1.GetMemberAdmissionAuthorityRequest) (string, bool) {
	proof, ok := ctx.Value(proofKey{}).(verifiedProof)
	if !ok || !validKnownRequest(req) {
		return "", false
	}
	hash, err := principal.RequestHash(req)
	if err != nil || hash != proof.requestHash {
		return "", false
	}
	return "role", true
}

func OrdinaryUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod == spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "protected member authority listener required")
		}
		return handler(ctx, req)
	}
}
