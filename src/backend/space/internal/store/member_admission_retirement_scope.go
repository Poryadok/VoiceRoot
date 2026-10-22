package store

import (
	"context"

	"github.com/google/uuid"
	spacev1 "voice.app/voice/space/v1"
)

// Only RetireCandidate's owned RC transaction enters this scope, after its
// operation identity and actual Space fence. It is a negative maintenance
// boundary, never a restore, initialization or positive membership grant.
func (s *SpaceStore) memberAdmissionRetirementScope(ctx context.Context, space uuid.UUID) error {
	if s == nil || s.tx == nil {
		return ErrEffectiveEpisodeUnavailable
	}
	if err := checkMemberAuthorityCatalog(ctx, s.tx); err != nil {
		return err
	}
	version, err := s.memberDecisionSchema(ctx)
	if err != nil {
		return err
	}
	if version != 39 && version != 40 {
		if err = s.effectiveMutationStorageScope(ctx, space); err != nil {
			return err
		}
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

func (s *SpaceStore) captureMemberAdmissionRetirement(ctx context.Context, intent *MemberAdmissionIntent) (*effectiveMemberMutations, error) {
	if intent == nil || intent.State != "PREPARING" {
		return nil, ErrMemberAdmissionStale
	}
	if err := s.memberAdmissionRetirementScope(ctx, intent.Input.SpaceID); err != nil {
		return nil, err
	}
	version, err := s.memberDecisionSchema(ctx)
	if err != nil {
		return nil, err
	}
	if version == 39 || version == 40 {
		return nil, nil
	}
	capture, err := s.captureEffectiveMemberMutationFacts(ctx, intent.Input.SpaceID, []uuid.UUID{intent.Input.ProfileID})
	if err != nil {
		return nil, err
	}
	capture.purpose = effectiveMutationCandidateRetirement
	return capture, nil
}
