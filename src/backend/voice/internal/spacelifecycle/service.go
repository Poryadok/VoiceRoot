package spacelifecycle

import (
	"context"
	"errors"
	"google.golang.org/protobuf/proto"
	callsv1 "voice.app/voice/calls/v1"

	commonv1 "voice.app/voice/common/v1"
)

type Service struct {
	Store   *PostgresStore
	Effects *Effects
}

// RetirementCapability is consumed by future full deletion activation preflight.
// Ejection is useful local progress, never a completed protocol-v1 barrier.
func (s *Service) RetirementCapability() string {
	if s == nil || s.Store == nil || s.Effects == nil {
		return "UNKNOWN"
	}
	capability, ok := s.Effects.media.(interface{ RetirementCapability() string })
	if !ok {
		return "UNKNOWN"
	}
	return capability.RetirementCapability()
}

func (s *Service) ResumePending(ctx context.Context) error {
	if s == nil || s.Store == nil || s.Effects == nil {
		return ErrUnavailable
	}
	rows, err := s.Store.pool.Query(ctx, `SELECT request_bytes FROM voice_space_lifecycle_fence_receipts WHERE receipt_bytes IS NULL AND applied_state='FROZEN' ORDER BY space_id,generation`)
	if err != nil {
		return err
	}
	var requests [][]byte
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			rows.Close()
			return err
		}
		requests = append(requests, raw)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var failures []error
	for _, raw := range requests {
		var wrapper callsv1.ApplySpaceLifecycleFenceRequest
		if err := proto.Unmarshal(raw, &wrapper); err != nil {
			failures = append(failures, err)
			continue
		}
		if err := s.Effects.ResumeFrozen(ctx, wrapper.GetFence()); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
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
			return s.Effects.ResumeFrozen(ctx, frozen)
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
