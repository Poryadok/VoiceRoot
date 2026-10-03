package principalgrpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	notificationv1 "voice.app/voice/notification/v1"
	"voice/backend/pkg/principal"
)

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

func IsLifecycleMethod(method string) bool {
	return method == notificationv1.NotificationService_ApplySpaceLifecycleFence_FullMethodName || method == notificationv1.NotificationService_PurgeSpace_FullMethodName || method == notificationv1.NotificationService_ImportSpacePurgeManifestPage_FullMethodName
}

func OrdinaryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if IsLifecycleMethod(info.FullMethod) {
			return nil, status.Error(codes.Unavailable, "protected lifecycle method unavailable on ordinary listener")
		}
		return handler(ctx, request)
	}
}

func StrictUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if verifier == nil {
			return nil, status.Error(codes.Unavailable, "Space principal verifier unavailable")
		}
		if !IsLifecycleMethod(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on lifecycle listener")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		verified, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			if status.Code(err) != codes.Unknown {
				return nil, err
			}
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}
