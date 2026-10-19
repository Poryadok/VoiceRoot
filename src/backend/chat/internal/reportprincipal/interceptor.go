package reportprincipal

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

const Method = chatv1.ChatService_CheckMessageReadEntitlement_FullMethodName

type Verifier struct {
	Keys   principal.KeyResolver
	Replay principal.ReplayGuard
	Clock  func() time.Time
}
type proofKey struct{}
type verifiedProof struct{ hash string }

func canonicalID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id != uuid.Nil && id.String() == s
}
func ValidRequest(r *chatv1.CheckMessageReadEntitlementRequest) bool {
	return r != nil && len(r.ProtoReflect().GetUnknown()) == 0 && canonicalID(r.ChatId) && canonicalID(r.ProfileId) && r.MessageCreatedAt != nil && len(r.MessageCreatedAt.ProtoReflect().GetUnknown()) == 0 && r.MessageCreatedAt.CheckValid() == nil
}
func (v Verifier) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, value any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if info.FullMethod != Method {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on report policy listener")
		}
		r, ok := value.(*chatv1.CheckMessageReadEntitlementRequest)
		if !ok || !ValidRequest(r) {
			return nil, status.Error(codes.InvalidArgument, "invalid report policy request")
		}
		deny := status.Error(codes.Unauthenticated, "invalid report policy principal")
		if v.Keys == nil || v.Replay == nil {
			return nil, deny
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || !canonicalID(transport.RequestID) {
			return nil, deny
		}
		hash, err := principal.RequestHash(r)
		if err != nil {
			return nil, deny
		}
		verified, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "messaging", ExpectedAudience: "chat", ExpectedRPC: Method, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.Replay, Clock: v.Clock})
		if err != nil || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
			return nil, deny
		}
		return next(context.WithValue(ctx, proofKey{}, verifiedProof{hash: hash}), r)
	}
}
func Producer(ctx context.Context, r *chatv1.CheckMessageReadEntitlementRequest) bool {
	if ctx == nil || !ValidRequest(r) {
		return false
	}
	p, ok := ctx.Value(proofKey{}).(verifiedProof)
	if !ok {
		return false
	}
	hash, err := principal.RequestHash(r)
	return err == nil && hash == p.hash
}
