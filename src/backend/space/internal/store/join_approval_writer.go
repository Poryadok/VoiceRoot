package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"slices"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
)

type JoinApprovalRecord struct {
	Attempt             JoinAttempt
	DecisionBasisSHA256 []byte
}

func (s *SpaceStore) PrepareJoinApprovalDecision(ctx context.Context, request *spacev1.ApproveJoinRequestRequest) (*JoinApprovalProposal, error) {
	call, err := lifecycleprincipal.RequireAdmissionApproval(ctx, request)
	if err != nil {
		return nil, err
	}
	request = proto.Clone(request).(*spacev1.ApproveJoinRequestRequest)
	actor, err := call.ActorFor(request)
	if err != nil {
		return nil, err
	}
	space, err := canonicalJoinID(request.SpaceId)
	if err != nil {
		return nil, err
	}
	q, err := canonicalJoinID(request.RequestId)
	if err != nil {
		return nil, err
	}
	operation, err := canonicalJoinID(request.OperationId)
	if err != nil {
		return nil, err
	}
	account, err := canonicalJoinID(actor.AccountID)
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	profile, err := canonicalJoinID(actor.ProfileID)
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	var explicit *uuid.UUID
	if request.AttemptId != nil {
		a, err := canonicalJoinID(request.GetAttemptId())
		if err != nil {
			return nil, err
		}
		explicit = &a
	}
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, entrypolicy.ErrInvalid
	}
	sha := sha256.Sum256(raw)
	op := JoinOperationInput{SpaceID: space, ActorAccountID: account, ActorProfileID: profile, OperationID: operation, Method: JoinOperationApprove, RequestSHA256: sha[:]}
	selected, err := s.ResolveJoinRequestApproval(ctx, op, q, explicit)
	if err != nil {
		return nil, err
	}
	if selected == nil || selected.AttemptID == nil || selected.State != "RESERVED" || !bytes.Equal(joinDigest("public-operation", selected.Input), joinDigest("public-operation", op)) {
		return nil, ErrJoinAttemptUnavailable
	}
	approval := JoinAttemptApprovalInput{SpaceID: space, AttemptID: *selected.AttemptID, ActorAccountID: account, ActorProfileID: profile, OperationID: operation}
	reserved, err := s.ReserveJoinAttemptApproval(ctx, approval)
	if err != nil {
		return nil, err
	}
	if reserved.State != "RESERVED" {
		return nil, ErrJoinAttemptUnavailable
	}
	return s.prepareJoinApprovalProposal(ctx, op, request, call)
}

// CommitJoinApprovalDecision records one manual decision and its complete basis
// in the existing35 transaction. It never publishes membership or consumes an
// invite; the native coordinator still owes exact Role completion/publication.
func (s *SpaceStore) CommitJoinApprovalDecision(ctx context.Context, d *VerifiedJoinApprovalDecision) (*JoinApprovalRecord, error) {
	identity, err := d.currentApplicant()
	if err != nil {
		return nil, err
	}
	if d.legacyBasis != nil {
		return s.commitLegacyJoinApprovalDecision(ctx, d, identity)
	}
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	p := d.proposal
	c := p.decisionContext
	q, err := canonicalJoinID(p.request.RequestId)
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	sha, err := d.basis.SHA256()
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	raw, err := json.Marshal(d.basis)
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	op := JoinOperationInput{SpaceID: c.SpaceID, ActorAccountID: c.ActorAccountID, ActorProfileID: c.ActorProfileID, OperationID: c.OperationID, Method: c.RequestMethod, RequestSHA256: c.RequestSHA256}
	in := JoinAttemptApprovalInput{SpaceID: c.SpaceID, AttemptID: c.AttemptID, ActorAccountID: c.ActorAccountID, ActorProfileID: c.ActorProfileID, OperationID: c.OperationID}
	guard := joinApprovalCurrentGuard(d, op)
	hookRan := false
	hook := func(ctx context.Context, local *SpaceStore, a *joinAttemptPrivate) error {
		if err := local.VerifyJoinRequestApprovalSelection(ctx, op, q, c.AttemptID); err != nil {
			return err
		}
		if a.ID != c.AttemptID || a.State != "READY" || a.PolicyVersion != p.policyVersion || !bytes.Equal(joinDigest("input", a.Input), joinDigest("input", p.input)) || !bytes.Equal(a.binding, c.AttemptBindingSHA256) || !bytes.Equal(a.submission, c.SubmissionSHA256) || !bytes.Equal(a.EvidenceSHA256, c.ReadyEvidenceSHA256) || a.approvedAccount == nil || a.approvedProfile == nil || a.approvalOperation == nil || *a.approvedAccount != c.ActorAccountID || *a.approvedProfile != c.ActorProfileID || *a.approvalOperation != c.OperationID {
			return ErrJoinAttemptUnavailable
		}
		parent, err := local.loadJoinDecision(ctx, c.SubmitBasisSHA256, false)
		if err != nil {
			return err
		}
		pc := parent.basis.Context
		if parent.publication != nil || pc.Kind != JoinDecisionSubmitPending || pc.SpaceID != c.SpaceID || pc.AttemptID != c.AttemptID || pc.ActorAccountID != p.input.AccountID || pc.ActorProfileID != p.input.ProfileID || !bytes.Equal(pc.AttemptBindingSHA256, c.AttemptBindingSHA256) || !bytes.Equal(pc.SubmissionSHA256, c.SubmissionSHA256) {
			return ErrJoinAttemptUnavailable
		}
		if p.input.Guest || identity.AccountType == "guest" {
			effective := p.input
			effective.Guest = true
			if err = local.localJoinAttemptGates(ctx, effective); err != nil {
				return err
			}
		}
		var saved JoinOperation
		err = local.db().QueryRow(ctx, `SELECT `+joinOperationColumns+` FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2 FOR UPDATE`, op.ActorProfileID, op.OperationID).Scan(&saved.Input.SpaceID, &saved.Input.ActorAccountID, &saved.Input.ActorProfileID, &saved.Input.OperationID, &saved.Input.Method, &saved.Input.RequestSHA256, &saved.AttemptID, &saved.State)
		if err != nil {
			return ErrJoinAttemptUnavailable
		}
		if saved.AttemptID == nil || *saved.AttemptID != c.AttemptID || !bytes.Equal(joinDigest("public-operation", saved.Input), joinDigest("public-operation", op)) {
			return ErrJoinAttemptConflict
		}
		if saved.State != "RESERVED" {
			return ErrJoinAttemptUnavailable
		}
		tag, err := local.db().Exec(ctx, `UPDATE space_join_operations SET state='DECIDED',decision_basis=$3,decision_basis_sha256=$4 WHERE actor_profile_id=$1 AND operation_id=$2 AND state='RESERVED' AND selected_attempt_id=$5 AND request_sha256=$6`, c.ActorProfileID, c.OperationID, raw, sha[:], c.AttemptID, c.RequestSHA256)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrJoinAttemptConflict
		}
		hookRan = true
		return nil
	}
	a, err := s.approveJoinAttempt(ctx, in, guard, hook)
	if err != nil {
		return nil, err
	}
	if !hookRan {
		return nil, ErrJoinAttemptUnavailable
	}
	return &JoinApprovalRecord{Attempt: *a, DecisionBasisSHA256: slices.Clone(sha[:])}, nil
}
