package store

import (
	"bytes"
	"context"
	"crypto/sha256"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
)

// This returns historical identity, not a current caller capability. The
// original four-field protobuf VALUE is reconstructed only to verify its hash.
func (s *SpaceStore) joinApprovalOriginalOperation(ctx context.Context, native *MemberAdmissionIntent) (JoinOperationInput, uuid.UUID, *uuid.UUID, error) {
	fail := func() (JoinOperationInput, uuid.UUID, *uuid.UUID, error) {
		return JoinOperationInput{}, uuid.Nil, nil, ErrJoinAttemptUnavailable
	}
	if s == nil || s.tx == nil || native == nil || native.Input.JoinAttempt == nil {
		return fail()
	}
	basis, err := s.LoadVersionedJoinDecisionBasis(ctx, native.Input.DecisionBasisSHA256)
	if err != nil {
		return fail()
	}
	var op JoinOperationInput
	var attempt uuid.UUID
	var binding, submission, evidence []byte
	switch basis.Version() {
	case 1:
		v, e := basis.V1()
		if e != nil || v.Context.Kind != JoinDecisionApproveReady {
			return fail()
		}
		c := v.Context
		op = JoinOperationInput{SpaceID: c.SpaceID, ActorAccountID: c.ActorAccountID, ActorProfileID: c.ActorProfileID, OperationID: c.OperationID, Method: c.RequestMethod, RequestSHA256: c.RequestSHA256}
		attempt, binding, submission, evidence = c.AttemptID, c.AttemptBindingSHA256, c.SubmissionSHA256, c.ReadyEvidenceSHA256
	case 2:
		v, e := basis.LegacyV2()
		if e != nil || v.Context.Kind != JoinApproveLegacyReady {
			return fail()
		}
		c := v.Context
		op = JoinOperationInput{SpaceID: c.SpaceID, ActorAccountID: c.ActorAccountID, ActorProfileID: c.ActorProfileID, OperationID: c.OperationID, Method: c.RequestMethod, RequestSHA256: c.RequestSHA256}
		attempt, binding, submission, evidence = c.AttemptID, c.AttemptBindingSHA256, c.SubmissionSHA256, c.ReadyEvidenceSHA256
	default:
		return fail()
	}
	if op.Method != JoinOperationApprove || op.SpaceID != native.Input.SpaceID || attempt != native.Input.JoinAttempt.AttemptID || !bytes.Equal(evidence, native.Input.JoinAttempt.EvidenceSHA256) {
		return fail()
	}
	saved, err := scanJoinOperation(s.db().QueryRow(ctx, `SELECT `+joinOperationColumns+` FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID))
	if err != nil || !sameJoinRequestOperation(saved.Input, op) || saved.AttemptID == nil || *saved.AttemptID != attempt || (saved.State != "DECIDED" && saved.State != "COMPLETED") {
		return fail()
	}
	selection, err := s.loadJoinRequestSelection(ctx, op)
	if err != nil || !selection.matches(op, selection.request, attempt) {
		return fail()
	}
	if err = s.VerifyJoinRequestApprovalSelection(ctx, op, selection.request, attempt); err != nil {
		return fail()
	}
	request := &spacev1.ApproveJoinRequestRequest{SpaceId: op.SpaceID.String(), RequestId: selection.request.String(), OperationId: op.OperationID.String()}
	if selection.explicitA != nil {
		value := selection.explicitA.String()
		request.AttemptId = &value
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return fail()
	}
	hash := sha256.Sum256(raw)
	if !bytes.Equal(hash[:], op.RequestSHA256) {
		return fail()
	}
	a, err := scanJoinAttempt(s.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1`, attempt))
	if err != nil || a.Input.SpaceID != op.SpaceID || a.Input.AccountID != native.Input.AccountID || a.Input.ProfileID != native.Input.ProfileID || !bytes.Equal(a.binding, binding) || !bytes.Equal(a.submission, submission) || !bytes.Equal(a.EvidenceSHA256, evidence) || a.approvedAccount == nil || a.approvedProfile == nil || a.approvalOperation == nil || *a.approvedAccount != op.ActorAccountID || *a.approvedProfile != op.ActorProfileID || *a.approvalOperation != op.OperationID {
		return fail()
	}
	var savedBasis, observed []byte
	var publication *uuid.UUID
	var publicationState *string
	if err = s.db().QueryRow(ctx, `SELECT decision_basis_sha256,observed_basis_sha256,publication_operation_id,publication_state FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID).Scan(&savedBasis, &observed, &publication, &publicationState); err != nil || !bytes.Equal(savedBasis, native.Input.DecisionBasisSHA256) || observed != nil {
		return fail()
	}
	if (publication == nil) != (a.publication == nil) || (publication == nil) != (publicationState == nil) {
		return fail()
	}
	if publication != nil {
		if *publication != *a.publication || (*publicationState != "PREPARING" && *publicationState != "PUBLISHED") || ((*publicationState == "PUBLISHED") != (a.State == "PUBLISHED")) || (native.Input.OperationID != uuid.Nil && native.Input.OperationID != *publication) {
			return fail()
		}
	} else if a.State != "READY" {
		return fail()
	}
	return op, selection.request, selection.explicitA, nil
}
