package store

import (
	"bytes"
	"context"

	"github.com/google/uuid"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/space/internal/admissionauthority"
)

// VerifiedAdmissionContinuation is local, fresh authority for one exact saved
// P. Historical journal/transcript data and caller booleans cannot construct it.
type VerifiedAdmissionContinuation struct {
	basis       *JoinDecisionBasis
	legacyBasis *JoinLegacyDecisionBasis
	applicant   *admissionauthority.VerifiedSubject
	reviewer    *admissionauthority.VerifiedReviewerPermission
}

func continuationRequest(i *MemberAdmissionIntent, basis *JoinDecisionBasis, reviewer bool) (*authv1.ObserveSpaceAdmissionSubjectRequest, error) {
	if i == nil || basis == nil || i.Input.JoinAttempt == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	digest, err := basis.SHA256()
	if err != nil || !bytes.Equal(digest[:], i.Input.DecisionBasisSHA256) || basis.Context.SpaceID != i.Input.SpaceID || basis.Context.AttemptID != i.Input.JoinAttempt.AttemptID || !bytes.Equal(basis.Context.ReadyEvidenceSHA256, i.Input.JoinAttempt.EvidenceSHA256) || (basis.Context.Kind != JoinDecisionSubmitReady && basis.Context.Kind != JoinDecisionApproveReady) {
		return nil, ErrJoinAttemptUnavailable
	}
	contextSHA, err := i.FirstPublicationContextSHA256()
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	account, profile := i.Input.AccountID, i.Input.ProfileID
	role := authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT
	if reviewer {
		if basis.Context.Kind != JoinDecisionApproveReady {
			return nil, ErrJoinAttemptUnavailable
		}
		account, profile = basis.Context.ActorAccountID, basis.Context.ActorProfileID
		role = authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER
	}
	return &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_FIRST_PUBLISH, SubjectRole: role, AccountId: account.String(), ProfileId: profile.String(), SpaceId: i.Input.SpaceID.String(), AdmissionBindingSha256: contextSHA[:]}, nil
}

// ObserveMemberAdmissionContinuation performs owner calls only outside Space
// locks. It always reads the original saved decision; a different reviewer is
// never a recovery substitute. No client JWT/epoch is retained across delay.
func (s *SpaceStore) ObserveMemberAdmissionContinuation(ctx context.Context, operation uuid.UUID, subjects *admissionauthority.Client, reviewers *admissionauthority.ReviewerClient) (*VerifiedAdmissionContinuation, error) {
	if s == nil || s.Pool == nil || s.tx != nil || operation == uuid.Nil {
		return nil, ErrJoinAttemptUnavailable
	}
	i, err := s.LookupMemberAdmission(ctx, operation)
	if err != nil || i.State != "PREPARING" || i.Input.JoinAttempt == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	verified, err := s.loadContinuationDecision(ctx, i.Input.DecisionBasisSHA256)
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	request, err := verified.request(i, false)
	if err != nil {
		return nil, err
	}
	applicant, err := subjects.Observe(ctx, request)
	if err != nil {
		return nil, ErrJoinAttemptUnavailable // Coarse owner denial cannot retire P.
	}
	verified.applicant = applicant
	_, _, reviewerRequired, err := verified.originalReviewer()
	if err != nil {
		return nil, err
	}
	if reviewerRequired {
		reviewerRequest, err := verified.request(i, true)
		if err != nil {
			return nil, err
		}
		reviewerSubject, err := subjects.Observe(ctx, reviewerRequest)
		if err != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		verified.reviewer, err = reviewers.Observe(ctx, reviewerSubject, reviewerRequest)
		if err != nil {
			return nil, ErrJoinAttemptUnavailable
		}
	}
	if _, err = verified.check(i); err != nil {
		return nil, err
	}
	return verified, nil
}

// check is pure and reconstitutes the exact expected request. Run after all
// potentially waiting SQL, including first-publication bookkeeping/hooks.
func (v *VerifiedAdmissionContinuation) check(i *MemberAdmissionIntent) (bool, error) {
	if v == nil || v.applicant == nil {
		return false, ErrJoinAttemptUnavailable
	}
	request, err := v.request(i, false)
	if err != nil {
		return false, err
	}
	identity, err := v.applicant.Check(request)
	if err != nil {
		return false, ErrJoinAttemptUnavailable
	}
	_, _, reviewerRequired, err := v.originalReviewer()
	if err != nil {
		return false, err
	}
	if reviewerRequired {
		reviewerRequest, err := v.request(i, true)
		if err != nil || v.reviewer.Check(reviewerRequest) != nil {
			return false, ErrJoinAttemptUnavailable
		}
	}
	return i.Input.Guest || identity.AccountType == "guest", nil
}

func (s *SpaceStore) checkContinuationReviewer(ctx context.Context, v *VerifiedAdmissionContinuation) error {
	space, profile, required, err := v.originalReviewer()
	if err != nil {
		return err
	}
	if !required {
		return nil
	}
	member, err := s.IsSpaceMember(ctx, space, profile)
	if err != nil || !member {
		return ErrJoinAttemptUnavailable
	}
	return nil
}

// PublishMemberAdmissionWithContinuation is the interactive first-publication
// entrypoint. The completion hook is local SQL only and runs once. Exact saved
// publication replay bypasses fresh proof/hooks and never recreates membership.
func (s *SpaceStore) PublishMemberAdmissionWithContinuation(ctx context.Context, operation, role uuid.UUID, receipt []byte, continuation *VerifiedAdmissionContinuation, completion MemberAdmissionFinalGuard) (*MembershipRow, error) {
	return s.publishMemberAdmission(ctx, operation, role, receipt, completion, continuation)
}
