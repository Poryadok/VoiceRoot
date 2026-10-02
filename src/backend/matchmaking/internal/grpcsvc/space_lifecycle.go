package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/spacelifecycle"
)

func (s *MatchmakingGRPC) ApplySpaceLifecycleFence(ctx context.Context, req *matchmakingv1.ApplySpaceLifecycleFenceRequest) (*matchmakingv1.ApplySpaceLifecycleFenceResponse, error) {
	if s.SpaceLifecycle == nil {
		return nil, status.Error(codes.Unavailable, "Space lifecycle participant unavailable")
	}
	if req == nil || req.GetFence() == nil {
		return nil, status.Error(codes.InvalidArgument, "lifecycle fence required")
	}
	receipt, err := s.SpaceLifecycle.ApplyFence(ctx, req.GetFence(), func(ctx context.Context, spaceID uuid.UUID) error {
		if s.Queue == nil {
			return queue.ErrQueueUnavailable
		}
		return s.Queue.ClearSpaceQueues(ctx, spaceID)
	})
	if err != nil {
		return nil, lifecycleStatus(err)
	}
	return &matchmakingv1.ApplySpaceLifecycleFenceResponse{Receipt: receipt}, nil
}

func (s *MatchmakingGRPC) PurgeSpace(ctx context.Context, req *matchmakingv1.PurgeSpaceRequest) (*matchmakingv1.PurgeSpaceResponse, error) {
	if s.SpaceLifecycle == nil {
		return nil, status.Error(codes.Unavailable, "Space lifecycle participant unavailable")
	}
	if req == nil || req.GetPurge() == nil {
		return nil, status.Error(codes.InvalidArgument, "purge request required")
	}
	receipt, err := s.SpaceLifecycle.PurgeSpace(ctx, req.GetPurge(), func(ctx context.Context, spaceID uuid.UUID) error {
		if s.Queue == nil {
			return queue.ErrQueueUnavailable
		}
		return s.Queue.ClearSpaceQueues(ctx, spaceID)
	})
	if err != nil {
		return nil, lifecycleStatus(err)
	}
	return &matchmakingv1.PurgeSpaceResponse{Receipt: receipt}, nil
}

func lifecycleStatus(err error) error {
	switch {
	case errors.Is(err, spacelifecycle.ErrInvalidRequest):
		return status.Error(codes.InvalidArgument, "invalid Space lifecycle request")
	case errors.Is(err, spacelifecycle.ErrConflict):
		return status.Error(codes.FailedPrecondition, "Space lifecycle state conflict")
	case errors.Is(err, spacelifecycle.ErrUnavailable), errors.Is(err, queue.ErrQueueUnavailable):
		return status.Error(codes.Unavailable, "Space lifecycle participant unavailable")
	default:
		return status.Error(codes.Unavailable, "Space lifecycle participant unavailable")
	}
}

var _ matchmakingv1.MatchmakingServiceServer = (*MatchmakingGRPC)(nil)
