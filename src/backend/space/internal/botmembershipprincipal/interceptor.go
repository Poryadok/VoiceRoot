package botmembershipprincipal

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type Verifier struct {
	Keys   principal.KeyResolver
	Replay principal.ReplayGuard
	Clock  func() time.Time
}

type InstallationBinding struct {
	OperationID, BotID, InstallationID, SpaceID, ProfileID, InstallOperationID uuid.UUID
	Generation                                                                 uint64
	Remove                                                                     bool
}

type proofKey struct{}

// VerifiedInstallation is minted only by the dedicated, authenticated listener.
type VerifiedInstallation struct {
	actor   principal.Principal
	binding InstallationBinding
	clock   func() time.Time
}

func canonicalID(value string) (uuid.UUID, bool) {
	id, err := uuid.Parse(value)
	return id, err == nil && id != uuid.Nil && id.String() == value
}

func requestBinding(value proto.Message) (InstallationBinding, string, error) {
	deny := status.Error(codes.InvalidArgument, "invalid Bot installation binding")
	var intent *spacev1.BotInstallationIntent
	var space, profile, installOp, method string
	var remove bool
	switch req := value.(type) {
	case *spacev1.AddBotMemberRequest:
		if req == nil {
			return InstallationBinding{}, "", deny
		}
		intent, space, profile = req.InstallationIntent, req.SpaceId, req.ProfileId
		method = spacev1.SpaceService_AddBotMember_FullMethodName
	case *spacev1.RemoveBotMemberRequest:
		if req == nil {
			return InstallationBinding{}, "", deny
		}
		intent, space, profile, installOp = req.InstallationIntent, req.SpaceId, req.ProfileId, req.InstallOperationId
		method, remove = spacev1.SpaceService_RemoveBotMember_FullMethodName, true
	default:
		return InstallationBinding{}, "", deny
	}
	if intent == nil || len(value.ProtoReflect().GetUnknown()) != 0 || len(intent.ProtoReflect().GetUnknown()) != 0 || intent.Generation == 0 || intent.Generation > math.MaxInt64 || space != intent.SpaceId || profile != intent.ActorProfileId {
		return InstallationBinding{}, "", deny
	}
	ids := []string{intent.OperationId, intent.BotId, intent.InstallationId, intent.SpaceId, intent.ActorProfileId}
	parsed := make([]uuid.UUID, len(ids))
	for i, value := range ids {
		var ok bool
		parsed[i], ok = canonicalID(value)
		if !ok {
			return InstallationBinding{}, "", deny
		}
	}
	original := parsed[0]
	if remove {
		var ok bool
		original, ok = canonicalID(installOp)
		if !ok || original == parsed[0] {
			return InstallationBinding{}, "", deny
		}
	}
	return InstallationBinding{OperationID: parsed[0], BotID: parsed[1], InstallationID: parsed[2], SpaceID: parsed[3], ProfileID: parsed[4], InstallOperationID: original, Generation: intent.Generation, Remove: remove}, method, nil
}

func (v Verifier) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, value any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info != nil && info.FullMethod == spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName {
			return v.verifyGlobalRemoval(ctx, value, handler)
		}
		if info == nil || (info.FullMethod != spacev1.SpaceService_AddBotMember_FullMethodName && info.FullMethod != spacev1.SpaceService_RemoveBotMember_FullMethodName) {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on Bot membership listener")
		}
		req, ok := value.(proto.Message)
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "invalid Bot installation request")
		}
		binding, method, err := requestBinding(req)
		if err != nil {
			return nil, err
		}
		deny := status.Error(codes.Unauthenticated, "invalid Bot installation principal")
		if method != info.FullMethod || v.Keys == nil || v.Replay == nil {
			return nil, deny
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || transport.RequestID != binding.OperationID.String() {
			return nil, deny
		}
		hash, err := principal.RequestHash(req)
		if err != nil {
			return nil, deny
		}
		clock := v.Clock
		if clock == nil {
			clock = time.Now
		}
		actor, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "space", ExpectedRPC: method, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.Replay, Clock: clock})
		if err != nil {
			return nil, deny
		}
		proof := &VerifiedInstallation{actor: actor, binding: binding, clock: clock}
		if _, err = proof.Check(req); err != nil {
			return nil, err
		}
		return handler(context.WithValue(principal.WithVerified(ctx, actor), proofKey{}, proof), value)
	}
}

func (v *VerifiedInstallation) Check(request proto.Message) (InstallationBinding, error) {
	deny := status.Error(codes.Unauthenticated, "verified Bot installation required")
	if v == nil || v.clock == nil {
		return InstallationBinding{}, deny
	}
	binding, method, err := requestBinding(request)
	if err != nil {
		return InstallationBinding{}, deny
	}
	hash, err := principal.RequestHash(request)
	p := v.actor
	if err != nil || binding != v.binding || p.Issuer != "bot" || p.Subject != "service:bot" || p.Audience != "space" || p.RPC != method || p.RequestID != binding.OperationID.String() || p.RequestHash != hash || !v.clock().Before(p.ExpiresAt) {
		return InstallationBinding{}, deny
	}
	return binding, nil
}

func requireInstallation(ctx context.Context, request proto.Message, remove bool) (*VerifiedInstallation, error) {
	proof, ok := ctx.Value(proofKey{}).(*VerifiedInstallation)
	actor, authenticated := principal.FromContext(ctx)
	if !ok || proof == nil || !authenticated || actor != proof.actor || ctx.Err() != nil {
		return nil, status.Error(codes.Unauthenticated, "verified Bot installation required")
	}
	binding, err := proof.Check(request)
	if err != nil {
		return nil, err
	}
	if binding.Remove != remove {
		return nil, status.Error(codes.Unauthenticated, "wrong Bot installation operation")
	}
	return proof, nil
}

func RequireAdd(ctx context.Context, request *spacev1.AddBotMemberRequest) (*VerifiedInstallation, error) {
	return requireInstallation(ctx, request, false)
}

func RequireRemove(ctx context.Context, request *spacev1.RemoveBotMemberRequest) (*VerifiedInstallation, error) {
	return requireInstallation(ctx, request, true)
}

func OrdinaryUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info != nil && (info.FullMethod == spacev1.SpaceService_AddBotMember_FullMethodName || info.FullMethod == spacev1.SpaceService_RemoveBotMember_FullMethodName || info.FullMethod == spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName) {
			return nil, status.Error(codes.PermissionDenied, "protected Bot membership listener required")
		}
		return handler(ctx, req)
	}
}
