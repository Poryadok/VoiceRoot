package gameprincipal

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
)

const (
	ProvisionMethod = callsv1.GameSessionProvisioningService_ProvisionGameSessionRoom_FullMethodName
	trustedIssuer   = "gameintegration"
	trustedAudience = "voice"
)

type Verifier struct {
	Resolve principal.KeyResolver
	Replay  principal.ReplayGuard
}

func AllowsMethod(method string) bool { return method == ProvisionMethod }

func UnaryServerInterceptor(verifier *Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !AllowsMethod(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on GIS listener")
		}
		message, ok := request.(proto.Message)
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		if hasUnknownFields(message.ProtoReflect()) {
			return nil, status.Error(codes.InvalidArgument, "unknown request fields are not supported")
		}
		provision, ok := request.(*callsv1.ProvisionGameSessionRoomRequest)
		if !ok || provision == nil || strings.TrimSpace(provision.GetOperationId()) == "" {
			return nil, status.Error(codes.InvalidArgument, "operation_id is required")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || transport.RequestID != provision.GetOperationId() {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		if verifier == nil || verifier.Resolve == nil || verifier.Replay == nil {
			return nil, status.Error(codes.Unavailable, "principal verifier unavailable")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		verified, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{
			ExpectedIssuer: trustedIssuer, ExpectedAudience: trustedAudience, ExpectedRPC: info.FullMethod,
			ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash,
			KeyResolver: verifier.Resolve, ReplayGuard: verifier.Replay,
		})
		if err != nil || verified.Issuer != trustedIssuer || verified.Subject != "service:"+trustedIssuer ||
			verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}

func hasUnknownFields(message protoreflect.Message) bool {
	if len(message.GetUnknown()) != 0 {
		return true
	}
	found := false
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.Kind() == protoreflect.MessageKind && hasUnknownFields(value.Message()) {
			found = true
			return false
		}
		return true
	})
	return found
}

// RequireProvisioning repeats method, operation and request binding at the
// handler boundary so direct server calls cannot bypass listener verification.
func RequireProvisioning(ctx context.Context, request *callsv1.ProvisionGameSessionRoomRequest) error {
	if request == nil || hasUnknownFields(request.ProtoReflect()) || !AllowsMethod(ProvisionMethod) || strings.TrimSpace(request.GetOperationId()) == "" {
		return status.Error(codes.InvalidArgument, "invalid request")
	}
	verified, ok := principal.FromContext(ctx)
	if !ok || verified.Kind != "service" || verified.Issuer != trustedIssuer || verified.Subject != "service:"+trustedIssuer {
		return status.Error(codes.Unauthenticated, "verified GIS principal required")
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid request")
	}
	if verified.Audience != trustedAudience || verified.RPC != ProvisionMethod || verified.RequestID != request.GetOperationId() ||
		verified.RequestHash != hash || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return status.Error(codes.Unauthenticated, "invalid principal binding")
	}
	return nil
}
