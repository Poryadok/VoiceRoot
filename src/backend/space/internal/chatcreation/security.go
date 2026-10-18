package chatcreation

import (
	"context"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type ServiceVerifier struct {
	Keys   principal.KeyResolver
	Replay principal.ReplayGuard
	Clock  func() time.Time
}

func ServiceMethod(method string) bool {
	return method == spacev1.SpaceService_CommitSpaceChatCreation_FullMethodName || method == spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName
}

func (v ServiceVerifier) Verify(ctx context.Context, method string, request proto.Message) (principal.Principal, error) {
	deny := status.Error(codes.Unauthenticated, "invalid Chat composite principal")
	if !ServiceMethod(method) {
		return principal.Principal{}, status.Error(codes.PermissionDenied, "method is not a Chat composite RPC")
	}
	if v.Keys == nil || v.Replay == nil {
		return principal.Principal{}, status.Error(codes.Unavailable, "Chat composite verifier unavailable")
	}
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil || !CanonicalID(transport.RequestID) {
		return principal.Principal{}, deny
	}
	if request == nil || !request.ProtoReflect().IsValid() || HasUnknown(request) {
		return principal.Principal{}, status.Error(codes.InvalidArgument, "invalid composite request")
	}
	operation := ""
	switch req := request.(type) {
	case *spacev1.CommitSpaceChatCreationRequest:
		if method != spacev1.SpaceService_CommitSpaceChatCreation_FullMethodName || ValidateBinding(req.GetBinding()) != nil {
			return principal.Principal{}, status.Error(codes.InvalidArgument, "invalid composite binding")
		}
		operation = req.GetBinding().GetOperationId()
	case *spacev1.GetSpaceChatCreationOutcomeRequest:
		if method != spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName || !CanonicalID(req.GetSpaceId()) {
			return principal.Principal{}, status.Error(codes.InvalidArgument, "invalid composite lookup")
		}
		operation = req.GetOperationId()
	default:
		return principal.Principal{}, status.Error(codes.InvalidArgument, "invalid composite request type")
	}
	if !CanonicalID(operation) || operation != transport.RequestID {
		return principal.Principal{}, deny
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return principal.Principal{}, deny
	}
	verified, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "chat", ExpectedAudience: "space", ExpectedRPC: method, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.Replay, Clock: v.Clock})
	if errors.Is(err, ErrDependencyUnavailable) {
		return principal.Principal{}, status.Error(codes.Unavailable, "Chat composite authentication dependency unavailable")
	}
	if err != nil || verified.Kind != "service" || verified.Subject != "service:chat" || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return principal.Principal{}, deny
	}
	return verified, nil
}

func OrdinaryUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if ServiceMethod(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "protected composite listener required")
		}
		return next(ctx, request)
	}
}

func (v ServiceVerifier) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		message, ok := request.(proto.Message)
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "invalid composite request")
		}
		verified, err := v.Verify(ctx, info.FullMethod, message)
		if err != nil {
			return nil, err
		}
		return next(principal.WithVerified(ctx, verified), request)
	}
}
