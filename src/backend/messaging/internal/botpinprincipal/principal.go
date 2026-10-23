package botpinprincipal

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/principal"
)

var ErrInvalidProof = errors.New("invalid Bot pin-cleanup proof")

type ServiceVerifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

// VerifiedCleanup cannot be constructed from public identity context. Its body
// is private owned storage and every consumer rechecks it after external waits.
type VerifiedCleanup struct {
	body  []byte
	proof principal.Principal
}

func (v VerifiedCleanup) Validate(request *messagingv1.ApplyBotChatPinRemovalRequest) error {
	if request == nil || len(v.body) == 0 {
		return ErrInvalidProof
	}
	p := v.proof
	if p.Kind != "service" || p.Issuer != "bot" || p.Subject != "service:bot" || p.Audience != "messaging" || p.RPC != messagingv1.MessagingService_ApplyBotChatPinRemoval_FullMethodName || p.RequestID != request.OperationId || p.JWTID == "" || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.ActorProfileID != "" || p.BotID != "" || p.SpaceID != "" || p.InstallationID != "" || p.BotScope != "" {
		return ErrInvalidProof
	}
	now := time.Now().UTC()
	if p.IssuedAt.IsZero() || p.IssuedAt.After(now.Add(5*time.Second)) || !p.ExpiresAt.After(now) || !p.ExpiresAt.After(p.IssuedAt) || p.ExpiresAt.Sub(p.IssuedAt) > 30*time.Second {
		return ErrInvalidProof
	}
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil || !bytes.Equal(raw, v.body) {
		return ErrInvalidProof
	}
	hash, err := principal.RequestHash(request)
	if err != nil || p.RequestHash != hash {
		return ErrInvalidProof
	}
	return nil
}

func (v VerifiedCleanup) Request() (*messagingv1.ApplyBotChatPinRemovalRequest, error) {
	request := new(messagingv1.ApplyBotChatPinRemovalRequest)
	if proto.Unmarshal(v.body, request) != nil {
		return nil, ErrInvalidProof
	}
	if err := v.Validate(request); err != nil {
		return nil, err
	}
	return request, nil
}

type cleanupKey struct{}

func Require(ctx context.Context, request *messagingv1.ApplyBotChatPinRemovalRequest) (VerifiedCleanup, error) {
	v, ok := ctx.Value(cleanupKey{}).(VerifiedCleanup)
	if !ok {
		return VerifiedCleanup{}, ErrInvalidProof
	}
	return v, v.Validate(request)
}

func UnaryServerInterceptor(verifier ServiceVerifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != messagingv1.MessagingService_ApplyBotChatPinRemoval_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on Bot cleanup listener")
		}
		body, ok := request.(*messagingv1.ApplyBotChatPinRemovalRequest)
		if !ok || body == nil || len(body.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid Bot cleanup request")
		}
		operation, err := uuid.Parse(body.OperationId)
		if err != nil || operation == uuid.Nil || operation.String() != body.OperationId {
			return nil, status.Error(codes.InvalidArgument, "invalid Bot cleanup operation")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || verifier == nil || transport.RequestID != body.OperationId {
			return nil, status.Error(codes.Unauthenticated, "invalid Bot cleanup transport")
		}
		raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(body)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid Bot cleanup body")
		}
		owned := new(messagingv1.ApplyBotChatPinRemovalRequest)
		if proto.Unmarshal(raw, owned) != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid Bot cleanup body")
		}
		hash, err := principal.RequestHash(owned)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid Bot cleanup body")
		}
		verified, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid Bot cleanup principal")
		}
		capability := VerifiedCleanup{body: raw, proof: verified}
		if capability.Validate(owned) != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid Bot cleanup binding")
		}
		return handler(context.WithValue(ctx, cleanupKey{}, capability), owned)
	}
}

func DenyStreams(_ any, _ grpc.ServerStream, _ *grpc.StreamServerInfo, _ grpc.StreamHandler) error {
	return status.Error(codes.PermissionDenied, "streams unavailable on Bot cleanup listener")
}
