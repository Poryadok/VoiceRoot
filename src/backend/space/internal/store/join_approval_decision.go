package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"slices"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
)

// JoinApprovalProposal is prospective server data, never an authorization.
// Its constructor stays internal until the Q-to-exact-A resolver is assembled.
type JoinApprovalProposal struct {
	decisionContext JoinDecisionContext
	legacyContext   *JoinLegacyDecisionContext
	request         *spacev1.ApproveJoinRequestRequest
	call            *lifecycleprincipal.VerifiedAdmissionCall
	input           JoinAttemptInput
	policyVersion   int64
}

// VerifiedJoinApprovalDecision retains only checked constructor-private current
// capabilities. Its historical observation hashes cannot authorize another call.
type VerifiedJoinApprovalDecision struct {
	proposal    *JoinApprovalProposal
	applicant   *admissionauthority.VerifiedSubject
	reviewer    *admissionauthority.VerifiedReviewerPermission
	basis       JoinDecisionBasis
	legacyBasis *JoinLegacyDecisionBasis
}

func (p *JoinApprovalProposal) Context() JoinDecisionContext {
	if p == nil {
		return JoinDecisionContext{}
	}
	c := p.decisionContext
	c.AttemptBindingSHA256 = slices.Clone(c.AttemptBindingSHA256)
	c.SubmissionSHA256 = slices.Clone(c.SubmissionSHA256)
	c.ReadyEvidenceSHA256 = slices.Clone(c.ReadyEvidenceSHA256)
	c.RequestSHA256 = slices.Clone(c.RequestSHA256)
	c.SubmitBasisSHA256 = slices.Clone(c.SubmitBasisSHA256)
	return c
}

// OwnerRequests never replays the applicant's original bearer/session epoch.
// The reviewer is the current exact signed caller; pending has no age TTL.
func (p *JoinApprovalProposal) OwnerRequests() (*authv1.ObserveSpaceAdmissionSubjectRequest, *authv1.ObserveSpaceAdmissionSubjectRequest, error) {
	if p == nil || p.call == nil || p.request == nil {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	if p.legacyContext != nil {
		return p.legacyOwnerRequests()
	}
	actor, err := p.call.ActorFor(p.request)
	c := p.decisionContext
	digest, hashErr := c.SHA256()
	if err != nil || hashErr != nil || actor.AccountID != c.ActorAccountID.String() || actor.ProfileID != c.ActorProfileID.String() || actor.RPC != JoinOperationApprove {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	applicant := &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_APPROVE, SubjectRole: authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT, AccountId: p.input.AccountID.String(), ProfileId: p.input.ProfileID.String(), SpaceId: p.input.SpaceID.String(), AdmissionBindingSha256: slices.Clone(digest[:])}
	epoch := uint64(actor.SessionEpoch)
	reviewer := &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_APPROVE, SubjectRole: authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER, AccountId: c.ActorAccountID.String(), ProfileId: c.ActorProfileID.String(), SpaceId: c.SpaceID.String(), AdmissionBindingSha256: slices.Clone(digest[:]), InteractiveSessionEpoch: &epoch}
	return applicant, reviewer, nil
}

func (p *JoinApprovalProposal) Seal(applicant *admissionauthority.VerifiedSubject, reviewer *admissionauthority.VerifiedReviewerPermission) (*VerifiedJoinApprovalDecision, error) {
	a, r, err := p.OwnerRequests()
	if err != nil {
		return nil, err
	}
	app, err := applicant.Transcript(a)
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	review, err := reviewer.Transcript(r)
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if p.legacyContext != nil {
		context, err := p.LegacyContext()
		if err != nil {
			return nil, err
		}
		basis := JoinLegacyDecisionBasis{Context: context, ApplicantObservationSHA256: slices.Clone(app.ObservationSHA256[:]), ReviewerObservationSHA256: slices.Clone(review.ObservationSHA256[:])}
		if _, err = basis.SHA256(); err != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		return &VerifiedJoinApprovalDecision{proposal: p, applicant: applicant, reviewer: reviewer, legacyBasis: &basis}, nil
	}
	basis := JoinDecisionBasis{Context: p.Context(), ApplicantObservationSHA256: slices.Clone(app.ObservationSHA256[:]), ReviewerObservationSHA256: slices.Clone(review.ObservationSHA256[:])}
	if _, err = basis.SHA256(); err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	return &VerifiedJoinApprovalDecision{proposal: p, applicant: applicant, reviewer: reviewer, basis: basis}, nil
}

func (d *VerifiedJoinApprovalDecision) currentApplicant() (admissionauthority.SubjectIdentity, error) {
	if d == nil || d.proposal == nil || d.applicant == nil || d.reviewer == nil {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	a, r, err := d.proposal.OwnerRequests()
	if err != nil {
		return admissionauthority.SubjectIdentity{}, err
	}
	app, err := d.applicant.Transcript(a)
	if err != nil {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	review, err := d.reviewer.Transcript(r)
	if err != nil {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	if d.proposal.legacyContext != nil {
		if d.legacyBasis == nil {
			return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
		}
		actual, e := d.legacyBasis.Context.SHA256()
		expected, e2 := d.proposal.legacyContext.SHA256()
		if e != nil || e2 != nil || actual != expected || !bytes.Equal(d.legacyBasis.ApplicantObservationSHA256, app.ObservationSHA256[:]) || !bytes.Equal(d.legacyBasis.ReviewerObservationSHA256, review.ObservationSHA256[:]) {
			return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
		}
		return app.Identity, nil
	}
	if d.legacyBasis != nil {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	actual, err := d.basis.Context.SHA256()
	expected, expectedErr := d.proposal.decisionContext.SHA256()
	if err != nil || expectedErr != nil || actual != expected || !bytes.Equal(d.basis.ApplicantObservationSHA256, app.ObservationSHA256[:]) || !bytes.Equal(d.basis.ReviewerObservationSHA256, review.ObservationSHA256[:]) {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	return app.Identity, nil
}

func (s *SpaceStore) prepareJoinApprovalProposal(ctx context.Context, op JoinOperationInput, request *spacev1.ApproveJoinRequestRequest, call *lifecycleprincipal.VerifiedAdmissionCall) (*JoinApprovalProposal, error) {
	if request == nil || len(request.ProtoReflect().GetUnknown()) != 0 || op.Method != JoinOperationApprove || request.SpaceId != op.SpaceID.String() || request.OperationId != op.OperationID.String() {
		return nil, entrypolicy.ErrInvalid
	}
	if _, err := canonicalJoinID(request.RequestId); err != nil {
		return nil, err
	}
	request = proto.Clone(request).(*spacev1.ApproveJoinRequestRequest)
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, entrypolicy.ErrInvalid
	}
	digest := sha256.Sum256(raw)
	if !bytes.Equal(digest[:], op.RequestSHA256) {
		return nil, ErrJoinAttemptConflict
	}
	return withOwnershipScopeValue(s, ctx, []uuid.UUID{op.SpaceID}, func(scoped *SpaceStore) (*JoinApprovalProposal, error) {
		saved, err := scanJoinOperation(scoped.db().QueryRow(ctx, `SELECT `+joinOperationColumns+` FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2 FOR UPDATE`, op.ActorProfileID, op.OperationID))
		if err != nil || saved.AttemptID == nil || saved.State != "RESERVED" {
			return nil, ErrJoinAttemptUnavailable
		}
		if !bytes.Equal(joinDigest("public-operation", saved.Input), joinDigest("public-operation", op)) {
			return nil, ErrJoinAttemptConflict
		}
		if request.AttemptId != nil && request.GetAttemptId() != saved.AttemptID.String() {
			return nil, ErrJoinAttemptConflict
		}
		a, err := scanJoinAttempt(scoped.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1 FOR UPDATE`, *saved.AttemptID))
		if err != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		if a.Input.SpaceID != op.SpaceID {
			return nil, ErrJoinAttemptDenied
		}
		// READY belongs to its original winner. Aliases need their own read-only
		// recovery path; they must never produce another prospective basis.
		if a.State != "PENDING" {
			return nil, ErrJoinAttemptUnavailable
		}
		if err = scoped.localJoinAttemptGates(ctx, a.Input); err != nil {
			return nil, err
		}
		current, err := scoped.GetEntryPolicy(ctx, op.SpaceID)
		if err != nil {
			return nil, err
		}
		if current.Version != a.PolicyVersion || !bytes.Equal(joinDigest("policy", current.Policy), a.policyDigest) {
			return nil, ErrJoinAttemptDenied
		}
		if !current.Policy.RequireManualApproval || current.Policy.RequirePhone || current.Policy.RequireCaptcha {
			return nil, ErrJoinAttemptUnavailable
		}
		if err = a.validateAutomaticEvidence(current.Policy); err != nil {
			return nil, err
		}
		member, err := scoped.IsSpaceMember(ctx, op.SpaceID, op.ActorProfileID)
		if err != nil {
			return nil, err
		}
		if !member {
			return nil, ErrJoinAttemptDenied
		}
		parentSHA, legacy, err := scoped.joinApprovalOriginalParent(ctx, a.ID)
		if err != nil {
			return nil, err
		}
		if legacy {
			return scoped.prepareLegacyJoinApprovalProposal(ctx, op, request, call, a, parentSHA)
		}
		parent, err := scoped.loadJoinDecision(ctx, parentSHA, false)
		if err != nil {
			return nil, err
		}
		c := parent.basis.Context
		if parent.publication != nil || c.Kind != JoinDecisionSubmitPending || c.SpaceID != op.SpaceID || c.AttemptID != a.ID || c.ActorAccountID != a.Input.AccountID || c.ActorProfileID != a.Input.ProfileID || !bytes.Equal(c.AttemptBindingSHA256, a.binding) || !bytes.Equal(c.SubmissionSHA256, a.submission) {
			return nil, ErrJoinAttemptUnavailable
		}
		a.approvedAccount = &op.ActorAccountID
		a.approvedProfile = &op.ActorProfileID
		a.approvalOperation = &op.OperationID
		c.Kind = JoinDecisionApproveReady
		c.ReadyEvidenceSHA256 = a.evidence()
		c.ActorAccountID = op.ActorAccountID
		c.ActorProfileID = op.ActorProfileID
		c.OperationID = op.OperationID
		c.RequestMethod = op.Method
		c.RequestSHA256 = slices.Clone(op.RequestSHA256)
		c.SubmitBasisSHA256 = slices.Clone(parentSHA)
		if _, err = c.SHA256(); err != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		if call != nil {
			actor, err := call.ActorFor(request)
			if err != nil || actor.AccountID != op.ActorAccountID.String() || actor.ProfileID != op.ActorProfileID.String() || actor.RPC != JoinOperationApprove {
				return nil, ErrJoinAttemptUnavailable
			}
		}
		return &JoinApprovalProposal{decisionContext: c, request: request, call: call, input: a.Input, policyVersion: a.PolicyVersion}, nil
	})
}
