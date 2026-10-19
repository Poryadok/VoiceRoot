package store

import (
	"bytes"
	"context"

	"github.com/google/uuid"
	authv1 "voice.app/voice/auth/v1"
)

func (s *SpaceStore) memberDecisionSchema(ctx context.Context) (int64, error) {
	var version int64
	if err := s.db().QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || (version != 39 && version != 40 && version != 41 && version != 42 && version != 44 && version != 45) {
		return 0, ErrJoinAttemptUnavailable
	}
	return version, nil
}

func (s *SpaceStore) loadContinuationDecision(ctx context.Context, sha []byte) (*VerifiedAdmissionContinuation, error) {
	version, err := s.memberDecisionSchema(ctx)
	if err != nil {
		return nil, err
	}
	v := &VerifiedAdmissionContinuation{}
	if version == 39 {
		v.basis, err = s.LoadJoinDecisionBasis(ctx, sha)
		return v, err
	}
	tagged, err := s.LoadVersionedJoinDecisionBasis(ctx, sha)
	if err != nil {
		return nil, err
	}
	switch tagged.Version() {
	case 1:
		basis, err := tagged.V1()
		if err != nil {
			return nil, err
		}
		v.basis = &basis
	case 2:
		basis, err := tagged.LegacyV2()
		if err != nil {
			return nil, err
		}
		v.legacyBasis = &basis
	default:
		return nil, ErrJoinAttemptUnavailable
	}
	return v, nil
}

func (v *VerifiedAdmissionContinuation) request(i *MemberAdmissionIntent, reviewer bool) (*authv1.ObserveSpaceAdmissionSubjectRequest, error) {
	if v == nil || (v.basis == nil) == (v.legacyBasis == nil) {
		return nil, ErrJoinAttemptUnavailable
	}
	if v.basis != nil {
		return continuationRequest(i, v.basis, reviewer)
	}
	basis := v.legacyBasis
	if i == nil || i.Input.JoinAttempt == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	digest, err := basis.SHA256()
	c := basis.Context
	if err != nil || !bytes.Equal(digest[:], i.Input.DecisionBasisSHA256) || c.SpaceID != i.Input.SpaceID || c.AttemptID != i.Input.JoinAttempt.AttemptID || !bytes.Equal(c.ReadyEvidenceSHA256, i.Input.JoinAttempt.EvidenceSHA256) || (c.Kind != JoinLegacyReady && c.Kind != JoinApproveLegacyReady) {
		return nil, ErrJoinAttemptUnavailable
	}
	contextSHA, err := i.FirstPublicationContextSHA256()
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	account, profile := i.Input.AccountID, i.Input.ProfileID
	role := authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT
	if reviewer {
		if c.Kind != JoinApproveLegacyReady {
			return nil, ErrJoinAttemptUnavailable
		}
		account, profile = c.ActorAccountID, c.ActorProfileID
		role = authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER
	}
	return &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_FIRST_PUBLISH, SubjectRole: role,
		AccountId: account.String(), ProfileId: profile.String(), SpaceId: i.Input.SpaceID.String(), AdmissionBindingSha256: contextSHA[:]}, nil
}

func (v *VerifiedAdmissionContinuation) originalReviewer() (space, profile uuid.UUID, required bool, err error) {
	if v == nil || (v.basis == nil) == (v.legacyBasis == nil) {
		return uuid.Nil, uuid.Nil, false, ErrJoinAttemptUnavailable
	}
	if v.basis != nil {
		c := v.basis.Context
		return c.SpaceID, c.ActorProfileID, c.Kind == JoinDecisionApproveReady, nil
	}
	c := v.legacyBasis.Context
	return c.SpaceID, c.ActorProfileID, c.Kind == JoinApproveLegacyReady, nil
}

func (s *SpaceStore) markMemberDecisionPublished(ctx context.Context, ref JoinAttemptReference, sha []byte, publication uuid.UUID) error {
	version, err := s.memberDecisionSchema(ctx)
	if err != nil {
		return err
	}
	if version == 40 || version == 41 || version == 42 || version == 44 || version == 45 {
		return s.MarkVersionedJoinDecisionBasisPublished(ctx, ref, sha, publication)
	}
	return s.MarkJoinDecisionBasisPublished(ctx, ref, sha, publication)
}
