package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
)

type JoinApprovalOperation struct {
	request   *spacev1.ApproveJoinRequestRequest
	call      *lifecycleprincipal.VerifiedAdmissionCall
	input     JoinOperationInput
	requestID uuid.UUID
	explicit  *uuid.UUID
}

func PrepareJoinApprovalOperation(ctx context.Context, original *spacev1.ApproveJoinRequestRequest) (*JoinApprovalOperation, error) {
	if original == nil {
		return nil, entrypolicy.ErrInvalid
	}
	request := proto.Clone(original).(*spacev1.ApproveJoinRequestRequest)
	call, err := lifecycleprincipal.RequireAdmissionApproval(ctx, request)
	if err != nil {
		return nil, err
	}
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
	account, e := canonicalJoinID(actor.AccountID)
	profile, e2 := canonicalJoinID(actor.ProfileID)
	if e != nil || e2 != nil {
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
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, entrypolicy.ErrInvalid
	}
	sha := sha256.Sum256(raw)
	return &JoinApprovalOperation{request: request, call: call, input: JoinOperationInput{SpaceID: space, ActorAccountID: account, ActorProfileID: profile, OperationID: operation, Method: JoinOperationApprove, RequestSHA256: sha[:]}, requestID: q, explicit: explicit}, nil
}
func (p *JoinApprovalOperation) ingress() error {
	if p == nil || p.call == nil || p.request == nil {
		return ErrJoinAttemptUnavailable
	}
	actor, err := p.call.ActorFor(p.request)
	if err != nil {
		return err
	}
	if actor.AccountID != p.input.ActorAccountID.String() || actor.ProfileID != p.input.ActorProfileID.String() || actor.RPC != JoinOperationApprove {
		return ErrJoinAttemptUnavailable
	}
	return nil
}

// The historical read has no owner/Role/policy/membership lookup and never
// creates P. One snapshot binds original Q/A/body/actor and the explicit family.
func (s *SpaceStore) ReadJoinApprovalOperation(ctx context.Context, p *JoinApprovalOperation) (*spacev1.ApproveJoinRequestResponse, *JoinSubmissionRecovery, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	if err := p.ingress(); err != nil {
		return nil, nil, err
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, nil, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	local := *s
	local.tx = tx
	replay, exists, err := local.LookupJoinOperationReplay(ctx, p.input)
	if err != nil {
		return nil, nil, err
	}
	if !exists {
		return nil, nil, p.ingress()
	}
	op := replay.Operation
	if op.AttemptID == nil {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	selection, err := local.loadJoinRequestSelection(ctx, p.input)
	if err != nil {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	if !selection.matches(p.input, p.requestID, *op.AttemptID) || !sameOptionalJoinAttempt(selection.explicitA, p.explicit) {
		return nil, nil, ErrJoinAttemptConflict
	}
	if op.State == "RESERVED" {
		return nil, nil, p.ingress()
	}
	a, err := scanJoinAttempt(tx.QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1`, *op.AttemptID))
	if err != nil || a.Input.SpaceID != p.input.SpaceID {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	if op.State == "COMPLETED" {
		var ack spacev1.ApproveJoinRequestResponse
		if replay.Code == nil || *replay.Code != 0 || proto.Unmarshal(replay.Proto, &ack) != nil || len(ack.ProtoReflect().GetUnknown()) != 0 || ack.Result == nil || len(ack.Result.ProtoReflect().GetUnknown()) != 0 || ack.Result.State != spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED || ack.Result.RequestId != p.requestID.String() || ack.Result.SpaceMembership == nil {
			return nil, nil, ErrJoinAttemptUnavailable
		}
		member := ack.Result.SpaceMembership
		if member.SpaceId != p.input.SpaceID.String() || member.ProfileId != a.Input.ProfileID.String() || member.JoinedAt == nil || member.JoinedAt.CheckValid() != nil || len(member.ProtoReflect().GetUnknown()) != 0 {
			return nil, nil, ErrJoinAttemptUnavailable
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, nil, err
		}
		if err = p.ingress(); err != nil {
			return nil, nil, err
		}
		return &ack, nil, nil
	}
	if op.State != "DECIDED" {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	var observed []byte
	if err = tx.QueryRow(ctx, `SELECT observed_basis_sha256 FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, p.input.ActorProfileID, p.input.OperationID).Scan(&observed); err != nil {
		return nil, nil, err
	}
	if observed != nil {
		// An observational alias has its own current ACL path. It never receives
		// the original winner's owner-free completed retry treatment.
		return nil, nil, p.ingress()
	}
	var sha []byte
	if err = tx.QueryRow(ctx, `SELECT decision_basis_sha256 FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, p.input.ActorProfileID, p.input.OperationID).Scan(&sha); err != nil {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	basis, err := local.LoadVersionedJoinDecisionBasis(ctx, sha)
	if err != nil {
		return nil, nil, err
	}
	var attempt uuid.UUID
	var kind uint8
	var binding, submission, evidence, requestSHA []byte
	var account, profile, operation, space uuid.UUID
	var method string
	var publication *uuid.UUID
	var publicationState *string
	switch basis.Version() {
	case 1:
		v, e := basis.V1()
		if e != nil || v.Context.Kind != JoinDecisionApproveReady {
			return nil, nil, ErrJoinAttemptUnavailable
		}
		c := v.Context
		attempt, kind, binding, submission, evidence, requestSHA = c.AttemptID, c.Kind, c.AttemptBindingSHA256, c.SubmissionSHA256, c.ReadyEvidenceSHA256, c.RequestSHA256
		account, profile, operation, space, method = c.ActorAccountID, c.ActorProfileID, c.OperationID, c.SpaceID, c.RequestMethod
		saved, e := local.loadJoinDecision(ctx, sha, false)
		if e != nil {
			return nil, nil, e
		}
		publication, publicationState = saved.publication, saved.publicationState
	case 2:
		v, e := basis.LegacyV2()
		if e != nil || v.Context.Kind != JoinApproveLegacyReady {
			return nil, nil, ErrJoinAttemptUnavailable
		}
		c := v.Context
		attempt, kind, binding, submission, evidence, requestSHA = c.AttemptID, c.Kind, c.AttemptBindingSHA256, c.SubmissionSHA256, c.ReadyEvidenceSHA256, c.RequestSHA256
		account, profile, operation, space, method = c.ActorAccountID, c.ActorProfileID, c.OperationID, c.SpaceID, c.RequestMethod
		saved, e := local.loadLegacyJoinDecision(ctx, sha, false)
		if e != nil {
			return nil, nil, e
		}
		publication, publicationState = saved.publication, saved.publicationState
	default:
		return nil, nil, ErrJoinAttemptUnavailable
	}
	if attempt != *op.AttemptID || account != p.input.ActorAccountID || profile != p.input.ActorProfileID || operation != p.input.OperationID || space != p.input.SpaceID || method != p.input.Method || !bytes.Equal(requestSHA, p.input.RequestSHA256) || !bytes.Equal(a.binding, binding) || !bytes.Equal(a.submission, submission) || !bytes.Equal(a.EvidenceSHA256, evidence) || a.approvedAccount == nil || a.approvedProfile == nil || a.approvalOperation == nil || *a.approvedAccount != account || *a.approvedProfile != profile || *a.approvalOperation != operation || (a.State != "READY" && a.State != "CLAIMED" && a.State != "PUBLISHED") {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	if (publication == nil) != (a.publication == nil) || (publication != nil && (*publication != *a.publication || publicationState == nil || (*publicationState != "PREPARING" && *publicationState != "PUBLISHED") || ((*publicationState == "PUBLISHED") != (a.State == "PUBLISHED")))) {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	recovered := &JoinSubmissionRecovery{AttemptID: attempt, DecisionKind: kind, DecisionBasisSHA256: slices.Clone(sha)}
	if publication != nil {
		id := *publication
		recovered.PublicationOperationID = &id
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	if err = p.ingress(); err != nil {
		return nil, nil, err
	}
	return nil, recovered, nil
}

// FIRST publication only. Original reviewer operation receives the applicant's
// exact saved native outcome, never the reviewer's ambient membership row.
func (s *SpaceStore) MarkJoinApprovalPublishedOutcome(ctx context.Context, p *JoinApprovalOperation, native *MemberAdmissionIntent) error {
	if err := p.ingress(); err != nil {
		return err
	}
	original, requestID, explicit, err := s.joinApprovalOriginalOperation(ctx, native)
	if err != nil {
		return err
	}
	if !sameJoinRequestOperation(original, p.input) || requestID != p.requestID || !sameOptionalJoinAttempt(explicit, p.explicit) {
		return ErrJoinAttemptUnavailable
	}
	if err = s.markJoinApprovalOriginalOutcome(ctx, original, requestID, native); err != nil {
		return err
	}
	return p.ingress()
}

// Local FIRST-publication bookkeeping. Its original request integrity is
// reconstructed from durable history; it never creates an authenticated call.
func (s *SpaceStore) markJoinApprovalOriginalOutcome(ctx context.Context, op JoinOperationInput, requestID uuid.UUID, native *MemberAdmissionIntent) error {
	if s == nil || s.tx == nil || native == nil || native.Input.JoinAttempt == nil {
		return ErrJoinAttemptUnavailable
	}
	attempt := native.Input.JoinAttempt.AttemptID
	if native.State != "PUBLISHED" || native.Input.SpaceID != op.SpaceID {
		return ErrJoinAttemptUnavailable
	}
	if err := s.VerifyJoinRequestApprovalSelection(ctx, op, requestID, attempt); err != nil {
		return err
	}
	basis, err := s.LoadVersionedJoinDecisionBasis(ctx, native.Input.DecisionBasisSHA256)
	if err != nil {
		return err
	}
	var account, profile, operation uuid.UUID
	var requestSHA []byte
	var publication *uuid.UUID
	var state *string
	switch basis.Version() {
	case 1:
		v, e := basis.V1()
		if e != nil || v.Context.Kind != JoinDecisionApproveReady || v.Context.AttemptID != attempt {
			return ErrJoinAttemptUnavailable
		}
		c := v.Context
		account, profile, operation, requestSHA = c.ActorAccountID, c.ActorProfileID, c.OperationID, c.RequestSHA256
		saved, e := s.loadJoinDecision(ctx, native.Input.DecisionBasisSHA256, true)
		if e != nil {
			return e
		}
		publication, state = saved.publication, saved.publicationState
	case 2:
		v, e := basis.LegacyV2()
		if e != nil || v.Context.Kind != JoinApproveLegacyReady || v.Context.AttemptID != attempt {
			return ErrJoinAttemptUnavailable
		}
		c := v.Context
		account, profile, operation, requestSHA = c.ActorAccountID, c.ActorProfileID, c.OperationID, c.RequestSHA256
		saved, e := s.loadLegacyJoinDecision(ctx, native.Input.DecisionBasisSHA256, true)
		if e != nil {
			return e
		}
		publication, state = saved.publication, saved.publicationState
	default:
		return ErrJoinAttemptUnavailable
	}
	if account != op.ActorAccountID || profile != op.ActorProfileID || operation != op.OperationID || !bytes.Equal(requestSHA, op.RequestSHA256) || publication == nil || *publication != native.Input.OperationID || state == nil || *state != "PUBLISHED" {
		return ErrJoinAttemptUnavailable
	}
	a, err := scanJoinAttempt(s.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1`, attempt))
	if err != nil {
		return ErrJoinAttemptUnavailable
	}
	if a.State != "PUBLISHED" || a.publication == nil || *a.publication != native.Input.OperationID || a.Input.SpaceID != op.SpaceID || a.Input.ProfileID != native.Input.ProfileID || a.Input.AccountID != native.Input.AccountID || !bytes.Equal(a.EvidenceSHA256, native.Input.JoinAttempt.EvidenceSHA256) {
		return ErrJoinAttemptUnavailable
	}
	var joined time.Time
	request := admissionRequestHash(native.Input)
	if err = s.db().QueryRow(ctx, `SELECT published_at FROM space_member_admission_intents WHERE operation_id=$1 AND state='PUBLISHED' AND request_sha256=$2 AND space_id=$3 AND profile_id=$4 AND account_id=$5`, native.Input.OperationID, request[:], op.SpaceID, a.Input.ProfileID, a.Input.AccountID).Scan(&joined); err != nil {
		return ErrJoinAttemptUnavailable
	}
	response := &spacev1.ApproveJoinRequestResponse{Result: &spacev1.JoinAdmissionResult{State: spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED, RequestId: requestID.String(), SpaceMembership: &spacev1.SpaceMembership{SpaceId: op.SpaceID.String(), ProfileId: a.Input.ProfileID.String(), JoinedAt: timestamppb.New(joined)}}}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(response)
	if err != nil {
		return err
	}
	tag, err := s.db().Exec(ctx, `UPDATE space_join_operations SET state='COMPLETED',outcome_code=0,outcome_proto=$3 WHERE actor_profile_id=$1 AND operation_id=$2 AND actor_account_id=$4 AND space_id=$5 AND request_method=$6 AND request_sha256=$7 AND state='DECIDED' AND selected_attempt_id=$8 AND decision_basis_sha256=$9 AND observed_basis_sha256 IS NULL AND publication_operation_id=$10 AND publication_state='PUBLISHED'`, op.ActorProfileID, op.OperationID, raw, op.ActorAccountID, op.SpaceID, op.Method, op.RequestSHA256, attempt, native.Input.DecisionBasisSHA256, native.Input.OperationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrJoinAttemptConflict
	}
	return nil
}
