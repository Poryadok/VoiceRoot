package inventorygrpc

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/referenceinventory"
)

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

func StrictUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !allowedMethod(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on inventory listener")
		}
		p, ok := peer.FromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "inventory mTLS required")
		}
		tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
		if !ok || len(tlsInfo.State.VerifiedChains) == 0 || len(tlsInfo.State.PeerCertificates) == 0 {
			return nil, status.Error(codes.Unauthenticated, "inventory mTLS required")
		}
		message, ok := req.(proto.Message)
		if !ok || referenceinventory.Strict(message) != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid inventory request")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || !referenceinventory.UUID(transport.RequestID) {
			return nil, status.Error(codes.Unauthenticated, "invalid inventory principal")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid inventory request")
		}
		if verifier == nil {
			return nil, status.Error(codes.Unavailable, "inventory verifier unavailable")
		}
		verified, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			if errors.Is(err, referenceinventory.ErrUnavailable) {
				return nil, status.Error(codes.Unavailable, "inventory verifier unavailable")
			}
			return nil, status.Error(codes.Unauthenticated, "invalid inventory principal")
		}
		ctx = principal.WithVerified(ctx, verified)
		if err := requireFile(ctx, info.FullMethod, message); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}
