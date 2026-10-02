package spacelifecycle

import (
	"context"

	commonv1 "voice.app/voice/common/v1"
)

type Service struct {
	Store   *PostgresStore
	Effects *Effects
}

func (s *Service) CheckAdmission(ctx context.Context, spaceID string) error {
	if s == nil || s.Store == nil {
		return ErrUnavailable
	}
	return s.Store.CheckAdmission(ctx, spaceID)
}

func (s *Service) ApplySpaceLifecycleFence(ctx context.Context, req *commonv1.SpaceLifecycleFenceRequest) (*commonv1.SpaceLifecycleFenceReceipt, error) {
	if s == nil || s.Store == nil || s.Effects == nil {
		return nil, ErrUnavailable
	}
	return s.Store.ApplyFence(ctx, req, func(ctx context.Context, frozen *commonv1.SpaceLifecycleFenceRequest) error {
		switch frozen.GetDesiredState() {
		case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN:
			return s.Effects.FreezeSpace(ctx, frozen.GetSpaceId())
		case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE,
			commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED:
			return nil
		default:
			return ErrInvalidRequest
		}
	})
}

func (s *Service) PurgeSpace(ctx context.Context, req *commonv1.SpacePurgeRequest) (*commonv1.SpacePurgeReceipt, error) {
	if s == nil || s.Store == nil || s.Effects == nil {
		return nil, ErrUnavailable
	}
	return s.Store.PurgeSpace(ctx, req, func(ctx context.Context, purge *commonv1.SpacePurgeRequest) error {
		return s.Effects.PurgeSpace(ctx, purge.GetSpaceId())
	})
}
