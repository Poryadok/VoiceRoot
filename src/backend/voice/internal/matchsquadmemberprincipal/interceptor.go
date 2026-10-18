package matchsquadmemberprincipal

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
	JoinMethod  = callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName
	TokenMethod = callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName
	LeaveMethod = callsv1.MatchSquadMemberService_LeaveMatchSquadRoom_FullMethodName
)

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

func AllowsMethod(method string) bool {
	return method == JoinMethod || method == TokenMethod || method == LeaveMethod
}

func StrictUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !AllowsMethod(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on Voice MatchSquad member listener")
		}
		message, ok := req.(proto.Message)
		if !ok || message == nil || hasUnknownFields(message.ProtoReflect()) {
			return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad member request")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || !canonicalUUID(transport.RequestID) {
			return nil, status.Error(codes.Unauthenticated, "invalid delegated user principal")
		}
		if !mutationRequestIDMatches(message, info.FullMethod, transport.RequestID) {
			return nil, status.Error(codes.Unauthenticated, "invalid delegated user principal binding")
		}
		if verifier == nil {
			return nil, status.Error(codes.Unavailable, "Voice MatchSquad member verifier unavailable")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad member request")
		}
		verified, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			if status.Code(err) != codes.Unknown {
				return nil, err
			}
			return nil, status.Error(codes.Unauthenticated, "invalid delegated user principal")
		}
		if verified.Kind != "delegated_user" || verified.Issuer != "gateway" || verified.Audience != "voice" || verified.RPC != info.FullMethod || verified.RequestID != transport.RequestID || verified.RequestHash != hash || !canonicalUUID(verified.AccountID) || !canonicalUUID(verified.Subject) || verified.Subject != verified.AccountID || !canonicalUUID(verified.ProfileID) || verified.SessionEpoch <= 0 || verified.ExpiresAt.IsZero() {
			return nil, status.Error(codes.Unauthenticated, "invalid delegated user principal binding")
		}
		return handler(principal.WithVerified(ctx, verified), req)
	}
}

func mutationRequestIDMatches(message proto.Message, method, requestID string) bool {
	switch req := message.(type) {
	case *callsv1.JoinMatchSquadRoomRequest:
		return method == JoinMethod && strings.TrimSpace(req.GetOperationId()) == requestID && canonicalUUID(req.GetOperationId())
	case *callsv1.LeaveMatchSquadRoomRequest:
		return method == LeaveMethod && strings.TrimSpace(req.GetOperationId()) == requestID && canonicalUUID(req.GetOperationId())
	case *callsv1.GetMatchSquadJoinTokenRequest:
		return method == TokenMethod
	default:
		return false
	}
}

func canonicalUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
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
				if field.MapValue().Kind() == protoreflect.MessageKind || field.MapValue().Kind() == protoreflect.GroupKind {
					unknown = hasUnknownFields(nested.Message())
				}
				return !unknown
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
