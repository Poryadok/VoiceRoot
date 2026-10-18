package spacelifecycle

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/spacebarrier"

	commonv1 "voice.app/voice/common/v1"
)

type Service struct {
	Store   *PostgresStore
	Effects *Effects
}

// RecoverySnapshot publishes the fully configured effect bindings to background
// recovery. The startup owner must call this after all composition is complete.
func (s *Service) RecoverySnapshot() *Service {
	if s == nil || s.Effects == nil {
		return nil
	}
	service := *s
	effects := *s.Effects
	service.Effects = &effects
	return &service
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
		_, err := s.Store.ApplyFence(ctx, wrapper.GetFence(), func(ctx context.Context, req *commonv1.SpaceLifecycleFenceRequest) error {
			return s.Effects.ResumeFrozen(ctx, req)
		})
		if err != nil && !errors.Is(err, ErrRetirementUnsupported) {
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
		case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE:
			gates, ok := s.Effects.ProjectionFence.(interface {
				ReinitializeSpaceLifecycle(context.Context, string, func(context.Context, string) (int64, bool, error)) error
			})
			if !ok {
				return ErrUnavailable
			}
			return gates.ReinitializeSpaceLifecycle(ctx, frozen.SpaceId, s.Store.readGateState)
		case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED:
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

func (s *PostgresStore) readGateState(ctx context.Context, space string) (int64, bool, error) {
	if err := s.CheckSchema(ctx); err != nil {
		return 0, false, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := spacebarrier.Lock(ctx, tx, space); err != nil {
		return 0, false, err
	}
	var generation int64
	var state string
	if err := tx.QueryRow(ctx, `SELECT generation,state FROM voice_space_lifecycle_fence_heads WHERE space_id=$1`, space).Scan(&generation, &state); err != nil {
		return 0, false, err
	}
	if state == "LIVE" {
		if _, err := spacebarrier.Live(ctx, tx, space); err != nil {
			return 0, false, err
		}
	}
	return generation, state == "LIVE", tx.Commit(ctx)
}
