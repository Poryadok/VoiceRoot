package store

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"

	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/space/internal/admissionauthority"
)

// Current owner capabilities and the actual native source are independent AND
// guards. The second invocation is after decision/journal SQL; unknown rolls
// back the whole transaction without losing the selected A or reviewer.
func joinApprovalCurrentGuard(d *VerifiedJoinApprovalDecision, op JoinOperationInput) JoinAttemptApprovalGuard {
	var before []*rolev1.MembershipBinding
	initialized := false
	return func(ctx context.Context, local *SpaceStore, _ JoinAttemptApprovalInput) error {
		if _, err := d.currentApplicant(); err != nil {
			return err
		}
		bindings, err := local.settingsMembership(ctx, op)
		if err != nil {
			return err
		}
		if initialized {
			if len(before) != len(bindings) {
				return ErrJoinAttemptDenied
			}
			for i := range before {
				if !proto.Equal(before[i], bindings[i]) {
					return ErrJoinAttemptDenied
				}
			}
		} else {
			before = bindings
			initialized = true
		}
		_, err = d.currentApplicant()
		return err
	}
}

func (s *SpaceStore) commitLegacyJoinApprovalDecision(ctx context.Context, d *VerifiedJoinApprovalDecision, identity admissionauthority.SubjectIdentity) (*JoinApprovalRecord, error) {
	if s == nil || s.Pool == nil || s.tx != nil || d == nil || d.proposal == nil || d.legacyBasis == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	p := d.proposal
	c, err := p.LegacyContext()
	if err != nil {
		return nil, err
	}
	q, err := canonicalJoinID(p.request.RequestId)
	if err != nil {
		return nil, err
	}
	sha, err := d.legacyBasis.SHA256()
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	raw, err := json.Marshal(d.legacyBasis)
	if err != nil {
		return nil, err
	}
	op := JoinOperationInput{SpaceID: c.SpaceID, ActorAccountID: c.ActorAccountID, ActorProfileID: c.ActorProfileID, OperationID: c.OperationID, Method: c.RequestMethod, RequestSHA256: c.RequestSHA256}
	in := JoinAttemptApprovalInput{SpaceID: c.SpaceID, AttemptID: c.AttemptID, ActorAccountID: c.ActorAccountID, ActorProfileID: c.ActorProfileID, OperationID: c.OperationID}
	hookRan := false
	hook := func(ctx context.Context, local *SpaceStore, a *joinAttemptPrivate) error {
		if err := local.VerifyJoinRequestApprovalSelection(ctx, op, q, c.AttemptID); err != nil {
			return err
		}
		if a.ID != c.AttemptID || a.State != "READY" || a.PolicyVersion != p.policyVersion || !bytes.Equal(joinDigest("input", a.Input), joinDigest("input", p.input)) || !bytes.Equal(a.binding, c.AttemptBindingSHA256) || !bytes.Equal(a.submission, c.SubmissionSHA256) || !bytes.Equal(a.EvidenceSHA256, c.ReadyEvidenceSHA256) || a.approvedAccount == nil || a.approvedProfile == nil || a.approvalOperation == nil || *a.approvedAccount != c.ActorAccountID || *a.approvedProfile != c.ActorProfileID || *a.approvalOperation != c.OperationID {
			return ErrJoinAttemptUnavailable
		}
		parent, err := local.loadLegacyJoinDecision(ctx, c.PendingBasisSHA256, false)
		if err != nil || !legacyOriginalParentMatches(c, parent) {
			return ErrJoinAttemptUnavailable
		}
		pc := parent.basis.Context
		if pc.ActorAccountID != p.input.AccountID || pc.ActorProfileID != p.input.ProfileID {
			return ErrJoinAttemptUnavailable
		}
		if p.input.Guest || identity.AccountType == "guest" {
			effective := p.input
			effective.Guest = true
			if err = local.localJoinAttemptGates(ctx, effective); err != nil {
				return err
			}
		}
		// The shared actor/operation lock was taken before Space by approveJoinAttempt.
		tag, err := local.db().Exec(ctx, `UPDATE space_join_operations SET state='DECIDED',basis_version=2,operation_origin=1,decision_basis=$3,decision_basis_sha256=$4 WHERE actor_profile_id=$1 AND operation_id=$2 AND actor_account_id=$5 AND space_id=$6 AND request_method=$7 AND request_sha256=$8 AND state='RESERVED' AND selected_attempt_id=$9 AND decision_basis IS NULL AND observed_basis_sha256 IS NULL AND publication_operation_id IS NULL`, c.ActorProfileID, c.OperationID, raw, sha[:], c.ActorAccountID, c.SpaceID, c.RequestMethod, c.RequestSHA256, c.AttemptID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrJoinAttemptConflict
		}
		hookRan = true
		return nil
	}
	a, err := s.approveJoinAttempt(ctx, in, joinApprovalCurrentGuard(d, op), hook)
	if err != nil {
		return nil, err
	}
	if !hookRan {
		return nil, ErrJoinAttemptUnavailable
	}
	return &JoinApprovalRecord{Attempt: *a, DecisionBasisSHA256: slices.Clone(sha[:])}, nil
}
