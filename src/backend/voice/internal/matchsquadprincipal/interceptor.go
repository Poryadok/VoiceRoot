package matchsquadprincipal

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
)

const (
	CreateMethod    = callsv1.MatchSquadVoiceService_CreateMatchSquadRoom_FullMethodName
	TeardownMethod  = callsv1.MatchSquadVoiceService_TeardownMatchSquadRoom_FullMethodName
	CompactMethod   = callsv1.MatchSquadVoiceService_CompactMatchSquadRoom_FullMethodName
	trustedIssuer   = "matchmaking"
	trustedSubject  = "service:matchmaking"
	trustedAudience = "voice"
)

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

func AllowsMethod(method string) bool {
	return method == CreateMethod || method == TeardownMethod || method == CompactMethod
}

func StrictUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !AllowsMethod(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on Voice MatchSquad listener")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil || hasUnknownFields(message.ProtoReflect()) {
			return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad request")
		}
		operationID := operationID(message)
		if !canonicalUUID(operationID) {
			return nil, status.Error(codes.InvalidArgument, "operation_id must be a canonical UUID")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || transport.RequestID != operationID {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		if verifier == nil {
			return nil, status.Error(codes.Unavailable, "Voice MatchSquad principal verifier unavailable")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad request")
		}
		verified, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			if status.Code(err) != codes.Unknown {
				return nil, err
			}
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		if !validServicePrincipal(verified, info.FullMethod, transport.RequestID, hash) {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}

func RequireCreate(ctx context.Context, req *callsv1.CreateMatchSquadRoomRequest) error {
	return requireVerified(ctx, req, CreateMethod, req.GetOperationId())
}

func RequireTeardown(ctx context.Context, req *callsv1.TeardownMatchSquadRoomRequest) error {
	return requireVerified(ctx, req, TeardownMethod, req.GetTeardownOperationId())
}

func RequireCompact(ctx context.Context, req *callsv1.CompactMatchSquadRoomRequest) error {
	return requireVerified(ctx, req, CompactMethod, req.GetOperationId())
}

func requireVerified(ctx context.Context, message proto.Message, method, opID string) error {
	if message == nil || hasUnknownFields(message.ProtoReflect()) || !AllowsMethod(method) || !canonicalUUID(opID) {
		return status.Error(codes.InvalidArgument, "invalid MatchSquad request")
	}
	verified, ok := principal.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "verified Matchmaking service principal required")
	}
	hash, err := principal.RequestHash(message)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid MatchSquad request")
	}
	if !validServicePrincipal(verified, method, opID, hash) {
		return status.Error(codes.Unauthenticated, "invalid service principal binding")
	}
	return nil
}

func validServicePrincipal(value principal.Principal, method, requestID, requestHash string) bool {
	return value.Kind == "service" && value.Issuer == trustedIssuer && value.Subject == trustedSubject &&
		value.Audience == trustedAudience && value.RPC == method && value.RequestID == requestID &&
		value.RequestHash == requestHash && value.AccountID == "" && value.ProfileID == "" && value.SessionEpoch == 0
}

func operationID(message proto.Message) string {
	switch req := message.(type) {
	case *callsv1.CreateMatchSquadRoomRequest:
		return strings.TrimSpace(req.GetOperationId())
	case *callsv1.TeardownMatchSquadRoomRequest:
		return strings.TrimSpace(req.GetTeardownOperationId())
	case *callsv1.CompactMatchSquadRoomRequest:
		return strings.TrimSpace(req.GetOperationId())
	default:
		return ""
	}
}

func canonicalUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil && parsed.String() == value
}

func hasUnknownFields(message protoreflect.Message) bool {
	if message == nil || len(message.GetUnknown()) > 0 {
		return true
	}
	unknown := false
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.Kind() != protoreflect.MessageKind && field.Kind() != protoreflect.GroupKind {
			return true
		}
		if field.IsMap() {
			value.Map().Range(func(_ protoreflect.MapKey, nested protoreflect.Value) bool {
				if field.MapValue().Kind() != protoreflect.MessageKind && field.MapValue().Kind() != protoreflect.GroupKind {
					return true
				}
				if hasUnknownFields(nested.Message()) {
					unknown = true
					return false
				}
				return true
			})
		} else if field.IsList() {
			list := value.List()
			for i := 0; i < list.Len(); i++ {
				if hasUnknownFields(list.Get(i).Message()) {
					unknown = true
					break
				}
			}
		} else {
			unknown = hasUnknownFields(value.Message())
		}
		return !unknown
	})
	return unknown
}
