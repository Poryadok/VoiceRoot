package grpcsvc

import (
	"context"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/store"
)

func (s *RoleGRPC) ApplySpaceLifecycleFence(ctx context.Context, req *rolev1.ApplySpaceLifecycleFenceRequest) (*rolev1.ApplySpaceLifecycleFenceResponse, error) {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Kind != "service" || p.Subject != "service:space" || p.Issuer != "space" || p.Audience != "role" || p.RPC != rolev1.RoleService_ApplySpaceLifecycleFence_FullMethodName || p.RequestID == "" {
		return nil, status.Error(codes.Unauthenticated, "verified Space lifecycle principal required")
	}
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.Fence == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid lifecycle request")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid lifecycle request")
	}
	if hash != p.RequestHash {
		return nil, status.Error(codes.Unauthenticated, "invalid lifecycle request binding")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "Role lifecycle persistence unavailable")
	}
	receipt, err := s.Store.ApplySpaceDeletionFence(ctx, req.Fence)
	switch {
	case err == nil:
		return &rolev1.ApplySpaceLifecycleFenceResponse{Receipt: receipt}, nil
	case errors.Is(err, store.ErrDeletionFenceInvalid):
		return nil, status.Error(codes.InvalidArgument, "invalid lifecycle fence")
	case errors.Is(err, store.ErrDeletionFenceConflict), errors.Is(err, store.ErrSpaceRetired), errors.Is(err, store.ErrSpaceFrozen):
		return nil, status.Error(codes.FailedPrecondition, "Role lifecycle state conflict")
	default:
		return nil, status.Error(codes.Unavailable, "Role lifecycle persistence unavailable")
	}
}
