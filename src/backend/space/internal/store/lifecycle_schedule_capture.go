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
