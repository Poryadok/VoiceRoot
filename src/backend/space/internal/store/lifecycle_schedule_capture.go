package store

import (
	"context"

	"github.com/google/uuid"
	spacev1 "voice.app/voice/space/v1"
)

// ReserveLifecycleSchedule has already checked the current owner and LIVE
// lifecycle under its own transaction/fence. Capture precedes its first guarded
// operation INSERT. A reversible freeze does not retire membership sources.
func (s *SpaceStore) captureLifecycleScheduleMutation(ctx context.Context, space, owner, operation uuid.UUID) (*effectiveMemberMutations, error) {
	if operation == uuid.Nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	capture, err := s.captureMemberMutationIfPresent(ctx, space, []uuid.UUID{owner})
	if err != nil || capture == nil {
		return capture, err
	}
	capture.purpose = effectiveMutationLifecycleSchedule
	capture.scheduleOperation = operation
	return capture, nil
}

// Only the proof writer's first receipt UPDATE enters here, after verifying
// the immutable DELETE/actor/Auth binding and taking its own Space fence. The
// reservation is already pending, so positive LIVE capture is not its domain.
// Exact saved-receipt replay never enters this mutation path.
func (s *SpaceStore) captureLifecycleProofReceiptMutation(ctx context.Context, operation lifecycleOperation) (*effectiveMemberMutations, error) {
	if s == nil || s.Pool == nil || s.tx == nil || ctx == nil || ctx.Err() != nil || operation.method != "DELETE" || operation.operationID == uuid.Nil || operation.spaceID == uuid.Nil || operation.actorProfileID == uuid.Nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	var marker, native bool
	if err := s.tx.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL,
 to_regclass('public.space_ordinary_member_heads') IS NOT NULL OR to_regclass('public.space_effective_member_heads') IS NOT NULL OR to_regclass('native_membership_guard.epoch') IS NOT NULL`).Scan(&marker, &native); err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if !marker {
		if native {
			return nil, ErrEffectiveEpisodeUnavailable
		}
		return nil, nil // preserved pre-native storage protocol
	}
	var single, dirty bool
	var version int64
	if err := s.tx.QueryRow(ctx, `SELECT count(*)=1,COALESCE(max(version),0),COALESCE(bool_or(dirty),true) FROM public.schema_migrations`).Scan(&single, &version, &dirty); err != nil || !single || dirty {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if version <= 34 && !native {
		return nil, nil
	}
	if version == 39 || version == 40 {
		if err := checkMemberAuthorityCatalog(ctx, s.tx); err != nil {
			return nil, err
		}
		return nil, nil // no effective-history storage on these qualified profiles
	}
	if version != 41 && version != 42 && version != 44 && version != 45 && version != 50 {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	bound := &effectiveMemberMutations{space: operation.spaceID, scheduleOperation: operation.operationID}
	if err := s.lifecycleScheduleMutationScope(ctx, bound); err != nil {
		return nil, err
	}
	capture, err := s.captureEffectiveMemberMutationFacts(ctx, operation.spaceID, []uuid.UUID{operation.actorProfileID})
	if err != nil {
		return nil, err
	}
	capture.purpose = effectiveMutationLifecycleSchedule
	capture.scheduleOperation = operation.operationID
	return capture, nil
}

// This final scope permits only the exact schedule this transaction reserved.
// It does not grant a generic frozen-state mutation or relax positive writers'
// LIVE requirement. The complete native cohort/clock checks still run normally.
func (s *SpaceStore) lifecycleScheduleMutationScope(ctx context.Context, capture *effectiveMemberMutations) error {
	if capture == nil || capture.scheduleOperation == uuid.Nil {
		return ErrEffectiveEpisodeUnavailable
	}
	if err := s.effectiveMutationStorageScope(ctx, capture.space); err != nil {
		return err
	}
	lifecycle, err := loadLifecycle(ctx, s.tx, capture.space, true)
	if err != nil || lifecycle.Phase() != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING || lifecycle.Snapshot().DeletionOperationID != capture.scheduleOperation.String() {
		return ErrEffectiveEpisodeUnavailable
	}
	ownership, err := readMemberOwnership(ctx, s.tx, capture.space)
	if err != nil || ownership.GetState() != spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE {
		return ErrEffectiveEpisodeUnavailable
	}
	return nil
}
