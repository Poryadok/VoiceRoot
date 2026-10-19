package store

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"slices"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/lifecycleprincipal"
)

func (p *JoinApprovalProposal) LegacyContext() (JoinLegacyDecisionContext, error) {
	if p == nil || p.legacyContext == nil || p.decisionContext.Kind != 0 {
		return JoinLegacyDecisionContext{}, ErrJoinAttemptUnavailable
	}
	copy := cloneJoinLegacyBasis(JoinLegacyDecisionBasis{Context: *p.legacyContext})
	return copy.Context, nil
}

// An A has one original submission. Do not select a preferred family or most
// recent journal when retained data contains more than one possible parent.
func (s *SpaceStore) joinApprovalOriginalParent(ctx context.Context, attempt uuid.UUID) ([]byte, bool, error) {
	rows, err := s.db().Query(ctx, `SELECT decision_basis_sha256,decision_basis->'context'->>'kind' FROM space_join_operations WHERE selected_attempt_id=$1 AND state<>'RESERVED' AND decision_basis_sha256 IS NOT NULL AND decision_basis->'context'->>'kind' IN ('1','4') ORDER BY actor_profile_id,operation_id LIMIT 2`, attempt)
	if err != nil {
		return nil, false, err
	}
	var digest []byte
	var kind string
	count := 0
	for rows.Next() {
		if err = rows.Scan(&digest, &kind); err != nil {
			rows.Close()
			return nil, false, err
		}
		count++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, false, err
	}
	if count != 1 || !validJoinDecisionDigest(digest) {
		return nil, false, ErrJoinAttemptUnavailable
	}
	return digest, kind == "4", nil
}

func (s *SpaceStore) prepareLegacyJoinApprovalProposal(ctx context.Context, op JoinOperationInput, request *spacev1.ApproveJoinRequestRequest, call *lifecycleprincipal.VerifiedAdmissionCall, a *joinAttemptPrivate, parentSHA []byte) (*JoinApprovalProposal, error) {
	parent, err := s.loadLegacyJoinDecision(ctx, parentSHA, false)
	if err != nil {
		return nil, err
	}
	pc := parent.basis.Context
	if parent.publication != nil || pc.Kind != JoinLegacyPending || pc.SpaceID != op.SpaceID || pc.AttemptID != a.ID || pc.ActorAccountID != a.Input.AccountID || pc.ActorProfileID != a.Input.ProfileID || !bytes.Equal(pc.AttemptBindingSHA256, a.binding) || !bytes.Equal(pc.SubmissionSHA256, a.submission) {
		return nil, ErrJoinAttemptUnavailable
	}
	a.approvedAccount = &op.ActorAccountID
	a.approvedProfile = &op.ActorProfileID
	a.approvalOperation = &op.OperationID
	c := JoinLegacyDecisionContext{Kind: JoinApproveLegacyReady, OperationOrigin: JoinOperationClientKey, SpaceID: op.SpaceID, AttemptID: a.ID, AttemptBindingSHA256: slices.Clone(a.binding), SubmissionSHA256: slices.Clone(a.submission), ReadyEvidenceSHA256: a.evidence(), ActorAccountID: op.ActorAccountID, ActorProfileID: op.ActorProfileID, OperationID: op.OperationID, RequestMethod: op.Method, RequestSHA256: slices.Clone(op.RequestSHA256), PendingBasisSHA256: slices.Clone(parentSHA)}
	if _, err = c.SHA256(); err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if call != nil {
		actor, err := call.ActorFor(request)
		if err != nil || actor.AccountID != op.ActorAccountID.String() || actor.ProfileID != op.ActorProfileID.String() || actor.RPC != JoinOperationApprove {
			return nil, ErrJoinAttemptUnavailable
		}
	}
	return &JoinApprovalProposal{legacyContext: &c, request: request, call: call, input: a.Input, policyVersion: a.PolicyVersion}, nil
}

func (p *JoinApprovalProposal) legacyOwnerRequests() (*authv1.ObserveSpaceAdmissionSubjectRequest, *authv1.ObserveSpaceAdmissionSubjectRequest, error) {
	c, err := p.LegacyContext()
	if err != nil || p.call == nil || p.request == nil {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	actor, err := p.call.ActorFor(p.request)
	digest, e := c.SHA256()
	if err != nil || e != nil || actor.AccountID != c.ActorAccountID.String() || actor.ProfileID != c.ActorProfileID.String() || actor.RPC != JoinOperationApprove {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	applicant := &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_APPROVE, SubjectRole: authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT, AccountId: p.input.AccountID.String(), ProfileId: p.input.ProfileID.String(), SpaceId: p.input.SpaceID.String(), AdmissionBindingSha256: slices.Clone(digest[:])}
	epoch := uint64(actor.SessionEpoch)
	reviewer := &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_APPROVE, SubjectRole: authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER, AccountId: c.ActorAccountID.String(), ProfileId: c.ActorProfileID.String(), SpaceId: c.SpaceID.String(), AdmissionBindingSha256: slices.Clone(digest[:]), InteractiveSessionEpoch: &epoch}
	return applicant, reviewer, nil
}
