package principalgrpc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/pkg/principal"
)

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

func IsProtectedMethod(method string) bool {
	return method == filev1.FileService_ValidateStoryMedia_FullMethodName
}

func OrdinaryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if IsProtectedMethod(info.FullMethod) {
			return nil, status.Error(codes.Unavailable, "protected method unavailable on ordinary listener")
		}
		return handler(ctx, request)
	}
}

func StrictUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !IsProtectedMethod(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on protected listener")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || verifier == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil || !message.ProtoReflect().IsValid() || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		p, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		if p.Kind != "service" || p.Subject != "service:story" || p.Issuer != "story" || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
			return nil, status.Error(codes.PermissionDenied, "story service required")
		}
		if p.Audience != "file" || p.RPC != info.FullMethod || p.RequestID != transport.RequestID || p.RequestHash != hash {
			return nil, status.Error(codes.Unauthenticated, "invalid principal binding")
		}
		return handler(principal.WithVerified(ctx, p), request)
	}
}

// Unavailable preserves dependency errors for the common invalid-credential deny path.
func Unavailable(err error) error { return err }
