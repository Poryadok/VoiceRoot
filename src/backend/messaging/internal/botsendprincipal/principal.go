// Package botsendprincipal admits only exact immediate Bot send requests.
package botsendprincipal

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/principal"
)

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}
type key struct{}
type Verified struct {
	hash  string
	proof principal.Principal
}

func Unknown(message proto.Message) bool {
	if message == nil || !message.ProtoReflect().IsValid() {
		return true
	}
	m := message.ProtoReflect()
	bad := len(m.GetUnknown()) != 0
	m.Range(func(f protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if f.Message() != nil {
			if f.IsList() {
				for i := 0; i < v.List().Len(); i++ {
					bad = bad || Unknown(v.List().Get(i).Message().Interface())
				}
			} else if f.IsMap() {
				v.Map().Range(func(_ protoreflect.MapKey, x protoreflect.Value) bool {
					if f.MapValue().Message() != nil {
						bad = bad || Unknown(x.Message().Interface())
					}
					return !bad
				})
			} else {
				bad = bad || Unknown(v.Message().Interface())
			}
		}
		return !bad
	})
	return bad
}
func (v Verified) Validate(request *messagingv1.SendBotMessageRequest) error {
	p := v.proof
	now := time.Now()
	hash, err := principal.RequestHash(request)
	if err != nil || Unknown(request) || hash != v.hash || hash != p.RequestHash || p.Kind != "service" || p.Issuer != "bot" || p.Subject != "service:bot" || p.Audience != "messaging" || p.RPC != messagingv1.MessagingService_SendBotMessage_FullMethodName || p.JWTID == "" || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.ActorProfileID != "" || p.BotID != "" || p.SpaceID != "" || p.InstallationID != "" || p.BotScope != "" || p.IssuedAt.IsZero() || p.IssuedAt.After(now.Add(5*time.Second)) || !p.ExpiresAt.After(now) || !p.ExpiresAt.After(p.IssuedAt) || p.ExpiresAt.Sub(p.IssuedAt) > 30*time.Second {
		return errors.New("invalid Bot send proof")
	}
	return nil
}
func Require(ctx context.Context, request *messagingv1.SendBotMessageRequest) (Verified, error) {
	v, ok := ctx.Value(key{}).(Verified)
	if !ok {
		return Verified{}, errors.New("missing Bot send proof")
	}
	return v, v.Validate(request)
}
func UnaryServerInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != messagingv1.MessagingService_SendBotMessage_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on Bot send listener")
		}
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		body, ok := request.(*messagingv1.SendBotMessageRequest)
		if !ok || Unknown(body) {
			return nil, status.Error(codes.InvalidArgument, "invalid Bot send request")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || verifier == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid Bot send transport")
		}
		id, err := uuid.Parse(transport.RequestID)
		if err != nil || id == uuid.Nil || id.String() != transport.RequestID {
			return nil, status.Error(codes.Unauthenticated, "invalid Bot send request id")
		}
		owned := proto.Clone(body).(*messagingv1.SendBotMessageRequest)
		hash, err := principal.RequestHash(owned)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid Bot send body")
		}
		p, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		v := Verified{hash: hash, proof: p}
		if err != nil || p.RequestID != transport.RequestID || v.Validate(owned) != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid Bot send binding")
		}
		ctx, expire := context.WithDeadline(ctx, p.ExpiresAt)
		defer expire()
		return handler(context.WithValue(ctx, key{}, v), owned)
	}
}
func DenyStreams(_ any, _ grpc.ServerStream, _ *grpc.StreamServerInfo, _ grpc.StreamHandler) error {
	return status.Error(codes.PermissionDenied, "streams unavailable on Bot send listener")
}
