package store

import (
	"context"

	"github.com/google/uuid"
	spacev1 "voice.app/voice/space/v1"
)

// This private negative-maintenance scope is called only by the transaction
// owner after its existing lifecycle operation/actor/evidence checks. It never
// substitutes for those checks or admits positive membership while frozen.
func (s *SpaceStore) lifecycleMaintenanceMutationScope(ctx context.Context, space uuid.UUID) error {
	if err := s.effectiveMutationStorageScope(ctx, space); err != nil {
		return err
	}
	lifecycle, err := readMemberLifecycle(ctx, s.tx, space)
	if err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	switch lifecycle.GetState() {
	case spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE,
		spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_FROZEN,
		spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_PURGE_DECIDED:
	default:
		return ErrEffectiveEpisodeUnavailable
	}
	ownership, err := readMemberOwnership(ctx, s.tx, space)
	if err != nil || (ownership.GetState() != spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE && ownership.GetState() != spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_PREPARED) {
		return ErrEffectiveEpisodeUnavailable
	}
	return nil
}

func (s *SpaceStore) captureLifecycleMaintenanceMutation(ctx context.Context, space uuid.UUID) (*effectiveMemberMutations, error) {
	if s == nil || s.tx == nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	var marker, native bool
	if err := s.tx.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL,to_regclass('public.space_ordinary_member_heads') IS NOT NULL`).Scan(&marker, &native); err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if !marker {
		if native {
			return nil, ErrEffectiveEpisodeUnavailable
		}
		return nil, nil
	}
	var version int64
	var dirty bool
	if err := s.tx.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty); err != nil || dirty {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	switch version {
	case 39, 40, 41, 42, 43:
		return nil, nil // Preserved protocol before the mandatory DB frame.
	case 44, 45, 46, 47:
	default:
		if version <= 34 && !native {
			return nil, nil
		}
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if err := s.lifecycleMaintenanceMutationScope(ctx, space); err != nil {
		return nil, err
	}
	capture, err := s.captureEffectiveMemberMutationFacts(ctx, space, nil)
	if err != nil {
		return nil, err
	}
	capture.purpose = effectiveMutationLifecycleMaintenance
	return capture, nil
}

func (s *SpaceStore) finishLifecycleMaintenanceMutation(ctx context.Context, capture *effectiveMemberMutations) error {
	if capture == nil {
		return nil
	}
	if err := s.finishEffectiveMemberMutations(ctx, capture); err != nil {
		return err
	}
	return s.flushMemberAdmissionDeferredChecks(ctx, capture)
}
