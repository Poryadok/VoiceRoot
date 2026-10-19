package store

import (
	"bytes"
	"context"

	"github.com/google/uuid"
	spacev1 "voice.app/voice/space/v1"
)

// Only the actual recovery transaction owner selects these private phases,
// after validating its saved input and current authority. PREPARED cannot be
// borrowed by an ordinary positive writer or an unrelated ownership operation.
func (s *SpaceStore) captureCommunityRecoveryMutation(ctx context.Context, in CommunityOwnerRecoveryInput, phase effectiveMutationPurpose) (*effectiveMemberMutations, error) {
	if s == nil || s.tx == nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	var marker, native bool
	if err := s.tx.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL,to_regclass('public.space_ordinary_member_heads') IS NOT NULL`).Scan(&marker, &native); err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if !native {
		if !marker {
			return nil, nil
		}
		var version int64
		if err := s.tx.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err == nil && version <= 34 {
			return nil, nil
		}
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if err := checkMemberAuthorityCatalog(ctx, s.tx); err != nil {
		return nil, err
	}
	version, err := s.memberDecisionSchema(ctx)
	if err != nil {
		return nil, err
	}
	if version == 39 || version == 40 {
		return nil, nil
	}
	if err = s.effectiveMutationStorageScope(ctx, in.SpaceID); err != nil {
		return nil, err
	}
	lifecycle, err := readMemberLifecycle(ctx, s.tx, in.SpaceID)
	if err != nil || lifecycle.State != spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	ownership, err := readMemberOwnership(ctx, s.tx, in.SpaceID)
	if err != nil || ownership.GetCurrentOwnerProfileId() != in.PreviousOwnerProfileID.String() {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	switch phase {
	case effectiveMutationCommunityRecoveryReservation:
		if ownership.State != spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE {
			return nil, ErrEffectiveEpisodeUnavailable
		}
	case effectiveMutationCommunityRecoveryCompletion:
		if ownership.State != spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_PREPARED {
			return nil, ErrEffectiveEpisodeUnavailable
		}
	default:
		return nil, ErrEffectiveEpisodeUnavailable
	}
	// Both phases change the authority of the entire roster. Older supported
	// ledger schemas have no DB frame to expand a requested subset, so derive
	// every affected profile under the already-held Space fence in all versions.
	rows, err := s.tx.Query(ctx, `SELECT profile_id FROM public.community_roster_members WHERE space_id=$1
 UNION SELECT profile_id FROM public.space_community_member_heads WHERE space_id=$1
 UNION SELECT $2::uuid UNION SELECT $3::uuid ORDER BY profile_id`, in.SpaceID, in.PreviousOwnerProfileID, in.ReplacementProfileID)
	if err != nil {
		return nil, err
	}
	var profiles []uuid.UUID
	for rows.Next() {
		var profile uuid.UUID
		if err = rows.Scan(&profile); err != nil {
			rows.Close()
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	capture, err := s.captureEffectiveMemberMutationFacts(ctx, in.SpaceID, profiles)
	if err != nil {
		return nil, err
	}
	capture.purpose, capture.recoveryOperation = phase, in.OperationID
	capture.recoveryHash, err = communityOwnerRecoveryHash(in)
	if err != nil {
		return nil, err
	}
	return capture, nil
}

func (s *SpaceStore) communityRecoveryMutationScope(ctx context.Context, capture *effectiveMemberMutations) error {
	if capture == nil || capture.recoveryOperation == uuid.Nil {
		return ErrEffectiveEpisodeUnavailable
	}
	if err := s.effectiveMutationStorageScope(ctx, capture.space); err != nil {
		return err
	}
	lifecycle, err := readMemberLifecycle(ctx, s.tx, capture.space)
	if err != nil || lifecycle.State != spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE {
		return ErrEffectiveEpisodeUnavailable
	}
	op, err := loadCommunityOwnerRecovery(ctx, s.tx, capture.recoveryOperation, false)
	if err != nil || op.Input.SpaceID != capture.space || !bytes.Equal(rowHash(op.Input), capture.recoveryHash[:]) {
		return ErrEffectiveEpisodeUnavailable
	}
	var account, profile uuid.UUID
	var generation int64
	var status string
	if err = s.tx.QueryRow(ctx, `SELECT owner_account_id,owner_profile_id,owner_generation,status FROM public.community_owner_authority
 WHERE space_id=$1 AND application_id=$2 AND environment_id=$3 AND corporation_key=$4`, capture.space, op.Input.ApplicationID, op.Input.EnvironmentID, op.Input.CorporationKey).Scan(&account, &profile, &generation, &status); err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	ownership, err := readMemberOwnership(ctx, s.tx, capture.space)
	if err != nil || ownership.GetCurrentOwnerProfileId() != profile.String() {
		return ErrEffectiveEpisodeUnavailable
	}
	switch capture.purpose {
	case effectiveMutationCommunityRecoveryReservation:
		if op.Status != "pending" || status != "recovery_pending" || generation != op.Input.ExpectedGeneration || account != op.Input.PreviousOwnerAccountID || profile != op.Input.PreviousOwnerProfileID || ownership.State != spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_PREPARED {
			return ErrEffectiveEpisodeUnavailable
		}
	case effectiveMutationCommunityRecoveryCompletion:
		if op.Status != "succeeded" || status != "active" || generation != op.Input.ExpectedGeneration+1 || op.ResultGeneration != generation || account != op.Input.ReplacementAccountID || profile != op.Input.ReplacementProfileID || ownership.State != spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE || validateCommunityRecoveryReceipt(op.Input, op.RoleReceipt) != nil {
			return ErrEffectiveEpisodeUnavailable
		}
	default:
		return ErrEffectiveEpisodeUnavailable
	}
	return nil
}

func (s *SpaceStore) retireCommunityRecoverySources(ctx context.Context, in CommunityOwnerRecoveryInput) error {
	var native bool
	if err := s.tx.QueryRow(ctx, `SELECT to_regclass('public.space_community_member_heads') IS NOT NULL`).Scan(&native); err != nil {
		return err
	}
	if !native {
		return nil
	}
	_, err := s.tx.Exec(ctx, `UPDATE public.space_community_member_heads SET state='RETIRED',retired_through=generation
 WHERE space_id=$1 AND owner_generation=$2 AND state='ACTIVE'`, in.SpaceID, in.ExpectedGeneration)
	return err
}
