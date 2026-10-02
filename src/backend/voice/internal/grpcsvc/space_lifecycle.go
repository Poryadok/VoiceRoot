package grpcsvc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/spacelifecycle"
)

type SpaceLifecycleController interface {
	CheckAdmission(context.Context, string) error
	ApplySpaceLifecycleFence(context.Context, *commonv1.SpaceLifecycleFenceRequest) (*commonv1.SpaceLifecycleFenceReceipt, error)
	PurgeSpace(context.Context, *commonv1.SpacePurgeRequest) (*commonv1.SpacePurgeReceipt, error)
}

func (s *VoiceGRPC) ApplySpaceLifecycleFence(ctx context.Context, req *callsv1.ApplySpaceLifecycleFenceRequest) (*callsv1.ApplySpaceLifecycleFenceResponse, error) {
	if err := requireSpaceLifecyclePrincipal(ctx, callsv1.VoiceService_ApplySpaceLifecycleFence_FullMethodName); err != nil {
		return nil, err
	}
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.GetFence() == nil {
		return nil, status.Error(codes.InvalidArgument, "lifecycle fence required")
	}
	if s == nil || s.SpaceLifecycle == nil {
		return nil, status.Error(codes.Unavailable, "Voice Space lifecycle unavailable")
	}
	receipt, err := s.SpaceLifecycle.ApplySpaceLifecycleFence(ctx, req.GetFence())
	if err != nil {
		return nil, spaceLifecycleStatus(err)
	}
	return &callsv1.ApplySpaceLifecycleFenceResponse{Receipt: receipt}, nil
}

func (s *VoiceGRPC) PurgeSpace(ctx context.Context, req *callsv1.PurgeSpaceRequest) (*callsv1.PurgeSpaceResponse, error) {
	if err := requireSpaceLifecyclePrincipal(ctx, callsv1.VoiceService_PurgeSpace_FullMethodName); err != nil {
		return nil, err
	}
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.GetPurge() == nil {
		return nil, status.Error(codes.InvalidArgument, "Space purge required")
	}
	if s == nil || s.SpaceLifecycle == nil {
		return nil, status.Error(codes.Unavailable, "Voice Space lifecycle unavailable")
	}
	receipt, err := s.SpaceLifecycle.PurgeSpace(ctx, req.GetPurge())
	if err != nil {
		return nil, spaceLifecycleStatus(err)
	}
	return &callsv1.PurgeSpaceResponse{Receipt: receipt}, nil
}

func requireSpaceLifecyclePrincipal(ctx context.Context, method string) error {
	verified, ok := principal.FromContext(ctx)
	if !ok || verified.Kind != "service" || verified.Issuer != "space" || verified.Audience != "voice" || verified.RPC != method || verified.RequestID == "" || verified.RequestHash == "" {
		return status.Error(codes.Unauthenticated, "invalid Space lifecycle principal")
	}
	return nil
}

func spaceLifecycleStatus(err error) error {
	switch {
	case errors.Is(err, spacelifecycle.ErrInvalidRequest):
		return status.Error(codes.InvalidArgument, "invalid Space lifecycle request")
	case errors.Is(err, spacelifecycle.ErrConflict):
		return status.Error(codes.FailedPrecondition, "Space lifecycle state conflict")
	case errors.Is(err, spacelifecycle.ErrSpaceFrozen):
		return status.Error(codes.FailedPrecondition, "Space voice admission is fenced")
	default:
		return status.Error(codes.Unavailable, "Voice Space lifecycle unavailable")
	}
}
