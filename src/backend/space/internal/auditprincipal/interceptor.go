package auditprincipal

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type Verifier struct {
	Keys   principal.KeyResolver
	Replay principal.ReplayGuard
	Clock  func() time.Time
}

type proofKey struct{}
type verifiedProof struct{ producer, requestHash string }

func validKnownRequest(req *spacev1.AppendAuditEventRequest) bool {
	return req != nil && len(req.ProtoReflect().GetUnknown()) == 0 &&
		(req.OccurredAt == nil || len(req.OccurredAt.ProtoReflect().GetUnknown()) == 0)
}

// Unary authenticates only the closed Role/Chat ingestion method. No metadata
// projection or generic prepopulated context principal creates this private proof.
func (v Verifier) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, value any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != spacev1.SpaceService_AppendAuditEvent_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "method is not available on audit listener")
		}
		req, ok := value.(*spacev1.AppendAuditEventRequest)
		if !ok || !validKnownRequest(req) {
			return nil, status.Error(codes.InvalidArgument, "invalid audit request")
		}
		deny := status.Error(codes.Unauthenticated, "invalid audit principal")
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
		// VerifyService rejects an issuer mismatch before key/replay access. Only
		// the correctly bound issuer can consume its JTI, exactly once.
		for _, producer := range []string{"role", "chat"} {
			claims, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{
				ExpectedIssuer: producer, ExpectedAudience: "space", ExpectedRPC: info.FullMethod,
				ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash,
				KeyResolver: v.Keys, ReplayGuard: v.Replay, Clock: v.Clock,
			})
			if err == nil {
				proof := verifiedProof{producer: claims.Issuer, requestHash: hash}
				return handler(context.WithValue(ctx, proofKey{}, proof), req)
			}
		}
		return nil, deny
	}
}

// Producer proves the handler is consuming the same known body authenticated by
// this package's interceptor; principal.WithVerified alone is insufficient.
func Producer(ctx context.Context, req *spacev1.AppendAuditEventRequest) (string, bool) {
	proof, ok := ctx.Value(proofKey{}).(verifiedProof)
	if !ok || !validKnownRequest(req) {
		return "", false
	}
	hash, err := principal.RequestHash(req)
	if err != nil || hash != proof.requestHash {
		return "", false
	}
	return proof.producer, true
}

func OrdinaryUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod == spacev1.SpaceService_AppendAuditEvent_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "protected audit listener required")
		}
		return handler(ctx, req)
	}
}
