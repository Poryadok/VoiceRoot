package grpcsvc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

// BotLifecycleStore is the durable Bot-owned participant boundary.
type BotLifecycleStore interface {
	ApplySpaceLifecycleFence(context.Context, *commonv1.SpaceLifecycleFenceRequest) (*commonv1.SpaceLifecycleFenceReceipt, error)
	PurgeSpace(context.Context, *commonv1.SpacePurgeRequest) (*commonv1.SpacePurgeReceipt, error)
}

func (s *BotGRPC) ApplySpaceLifecycleFence(ctx context.Context, req *botv1.ApplySpaceLifecycleFenceRequest) (*botv1.ApplySpaceLifecycleFenceResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "valid Bot lifecycle fence request required")
	}
	if err := requireBotLifecyclePrincipal(ctx, botv1.BotService_ApplySpaceLifecycleFence_FullMethodName, req); err != nil {
		return nil, err
	}
	if req.GetFence() == nil || len(req.ProtoReflect().GetUnknown()) != 0 || len(req.GetFence().ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "valid Bot lifecycle fence request required")
	}
	if req.GetFence().GetManifest() != nil && len(req.GetFence().GetManifest().ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "valid Bot lifecycle fence request required")
	}
	if s == nil || s.LifecycleStore == nil {
		return nil, status.Error(codes.Unavailable, "Bot lifecycle store unavailable")
	}
	receipt, err := s.LifecycleStore.ApplySpaceLifecycleFence(ctx, req.GetFence())
	if err != nil {
		return nil, mapBotLifecycleError(err)
	}
	if receipt == nil {
		return nil, status.Error(codes.DataLoss, "Bot lifecycle store omitted its fence receipt")
	}
	return &botv1.ApplySpaceLifecycleFenceResponse{Receipt: receipt}, nil
}

func (s *BotGRPC) PurgeSpace(ctx context.Context, req *botv1.PurgeSpaceRequest) (*botv1.PurgeSpaceResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "valid Bot lifecycle purge request required")
	}
	if err := requireBotLifecyclePrincipal(ctx, botv1.BotService_PurgeSpace_FullMethodName, req); err != nil {
		return nil, err
	}
	if req.GetPurge() == nil || len(req.ProtoReflect().GetUnknown()) != 0 || len(req.GetPurge().ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "valid Bot lifecycle purge request required")
	}
	if req.GetPurge().GetManifest() != nil && len(req.GetPurge().GetManifest().ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "valid Bot lifecycle purge request required")
	}
	if s == nil || s.LifecycleStore == nil {
		return nil, status.Error(codes.Unavailable, "Bot lifecycle store unavailable")
	}
	receipt, err := s.LifecycleStore.PurgeSpace(ctx, req.GetPurge())
	if err != nil {
		return nil, mapBotLifecycleError(err)
	}
	if receipt == nil {
		return nil, status.Error(codes.DataLoss, "Bot lifecycle store omitted its purge receipt")
	}
	return &botv1.PurgeSpaceResponse{Receipt: receipt}, nil
}

func requireBotLifecyclePrincipal(ctx context.Context, method string, request proto.Message) error {
	verified, ok := principal.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "Space lifecycle principal required")
	}
	hash, err := principal.RequestHash(request)
	if err != nil || verified.Kind != "service" || verified.Issuer != "space" || verified.Subject != "service:space" ||
		verified.Audience != "bot" || verified.RPC != method || verified.RequestID == "" || verified.RequestHash != hash ||
		verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return status.Error(codes.PermissionDenied, "Space lifecycle principal required")
	}
	return nil
}

func mapBotLifecycleError(err error) error {
	if errors.Is(err, store.ErrSpaceLifecycleConflict) {
		return status.Error(codes.Aborted, "Bot lifecycle request conflicts with saved state")
	}
	return status.Error(codes.Unavailable, "Bot lifecycle storage unavailable")
}
