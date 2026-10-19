package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/lifecycleprincipal"
)

func joinSubmissionOutcomeInput(ctx context.Context, request *spacev1.SubmitJoinAttemptRequest) (*lifecycleprincipal.VerifiedAdmissionCall, JoinOperationInput, uuid.UUID, error) {
	call, err := lifecycleprincipal.RequireAdmissionSubmit(ctx, request)
	if err != nil {
		return nil, JoinOperationInput{}, uuid.Nil, err
	}
	actor, err := call.ActorFor(request)
	if err != nil {
		return nil, JoinOperationInput{}, uuid.Nil, err
	}
	space, err := canonicalJoinID(request.SpaceId)
	if err != nil {
		return nil, JoinOperationInput{}, uuid.Nil, err
	}
	attempt, err := canonicalJoinID(request.AttemptId)
	if err != nil {
		return nil, JoinOperationInput{}, uuid.Nil, err
	}
	operation, err := canonicalJoinID(request.OperationId)
	if err != nil {
		return nil, JoinOperationInput{}, uuid.Nil, err
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, JoinOperationInput{}, uuid.Nil, err
	}
	sha := sha256.Sum256(raw)
	return call, JoinOperationInput{SpaceID: space, ActorAccountID: uuid.MustParse(actor.AccountID), ActorProfileID: uuid.MustParse(actor.ProfileID), OperationID: operation, Method: JoinOperationSubmit, RequestSHA256: sha[:]}, attempt, nil
}

func (s *SpaceStore) completeJoinSubmissionOutcome(ctx context.Context, op JoinOperationInput, attempt uuid.UUID, response *spacev1.SubmitJoinAttemptResponse) error {
	if s == nil || s.tx == nil || response == nil || response.Result == nil {
		return ErrJoinAttemptUnavailable
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(response)
	if err != nil {
		return err
	}
	tag, err := s.db().Exec(ctx, `UPDATE space_join_operations SET state='COMPLETED',outcome_code=0,outcome_proto=$3 WHERE actor_profile_id=$1 AND operation_id=$2 AND state='DECIDED' AND selected_attempt_id=$4 AND request_sha256=$5 AND decision_basis IS NOT NULL AND observed_basis_sha256 IS NULL`, op.ActorProfileID, op.OperationID, raw, attempt, op.RequestSHA256)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrJoinAttemptConflict
	}
	return nil
}

// CompletePendingJoinSubmissionOutcome projects one already committed original
// pending decision. It never regrades answers, observes an owner or approves Q.
func (s *SpaceStore) CompletePendingJoinSubmissionOutcome(ctx context.Context, request *spacev1.SubmitJoinAttemptRequest) (*spacev1.SubmitJoinAttemptResponse, error) {
	call, op, attempt, err := joinSubmissionOutcomeInput(ctx, request)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	history, err := s.RecoverJoinSubmissionDecision(ctx, request)
	if err != nil {
		return nil, err
	}
	if history.DecisionKind != JoinDecisionSubmitPending || history.AttemptID != attempt {
		return nil, ErrJoinAttemptUnavailable
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	if err = lockJoinPublicOperation(ctx, tx, op); err != nil {
		return nil, err
	}
	local := *s
	local.tx = tx
	replay, exists, err := local.LookupJoinOperationReplay(ctx, op)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrJoinAttemptUnavailable
	}
	if replay.Operation.State == "COMPLETED" {
		var saved spacev1.SubmitJoinAttemptResponse
		if replay.Code == nil || *replay.Code != 0 || proto.Unmarshal(replay.Proto, &saved) != nil || saved.Result == nil || saved.Result.State != spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PENDING || saved.Result.SpaceMembership != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		if _, err = canonicalJoinID(saved.Result.RequestId); err != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		if _, err = call.ActorFor(request); err != nil {
			return nil, err
		}
		return &saved, nil
	}
	if replay.Operation.AttemptID == nil || *replay.Operation.AttemptID != attempt || replay.Operation.State != "DECIDED" {
		return nil, ErrJoinAttemptUnavailable
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{op.SpaceID}); err != nil {
		return nil, err
	}
	saved, err := local.loadJoinDecision(ctx, history.DecisionBasisSHA256, true)
	if err != nil {
		return nil, err
	}
	c := saved.basis.Context
	if c.Kind != JoinDecisionSubmitPending || c.AttemptID != attempt || c.ActorAccountID != op.ActorAccountID || c.ActorProfileID != op.ActorProfileID || c.OperationID != op.OperationID || !bytes.Equal(c.RequestSHA256, op.RequestSHA256) || saved.publication != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	group, err := local.LookupJoinRequestGroupForAttempt(ctx, op.SpaceID, attempt)
	if err != nil {
		return nil, err
	}
	if group.SpaceID != op.SpaceID || group.AccountID != op.ActorAccountID || group.ProfileID != op.ActorProfileID {
		return nil, ErrJoinAttemptUnavailable
	}
	response := &spacev1.SubmitJoinAttemptResponse{Result: &spacev1.JoinAdmissionResult{State: spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PENDING, RequestId: group.ID.String()}}
	if err = local.completeJoinSubmissionOutcome(ctx, op, attempt, response); err != nil {
		return nil, err
	}
	if _, err = call.ActorFor(request); err != nil {
		return nil, err
	}
	return response, tx.Commit(ctx)
}

// MarkJoinSubmissionPublishedOutcome is the native first-publication local hook.
// It accepts neither a member row nor an allow flag from the caller. The exact
// saved native publication and35/36 chain must already exist in this same TX.
func (s *SpaceStore) MarkJoinSubmissionPublishedOutcome(ctx context.Context, request *spacev1.SubmitJoinAttemptRequest, native *MemberAdmissionIntent) error {
	call, op, attempt, err := joinSubmissionOutcomeInput(ctx, request)
	if err != nil {
		return err
	}
	if s == nil || s.tx == nil || native == nil || native.Input.JoinAttempt == nil || native.Input.JoinAttempt.AttemptID != attempt || native.Input.SpaceID != op.SpaceID || native.Input.AccountID != op.ActorAccountID || native.Input.ProfileID != op.ActorProfileID {
		return ErrJoinAttemptUnavailable
	}
	saved, err := s.loadJoinDecision(ctx, native.Input.DecisionBasisSHA256, true)
	if err != nil {
		return err
	}
	c := saved.basis.Context
	if c.Kind != JoinDecisionSubmitReady || c.AttemptID != attempt || c.OperationID != op.OperationID || c.ActorAccountID != op.ActorAccountID || c.ActorProfileID != op.ActorProfileID || !bytes.Equal(c.RequestSHA256, op.RequestSHA256) || saved.publication == nil || *saved.publication != native.Input.OperationID || saved.publicationState == nil || *saved.publicationState != "PUBLISHED" {
		return ErrJoinAttemptUnavailable
	}
	var joined time.Time
	sha := admissionRequestHash(native.Input)
	if err = s.db().QueryRow(ctx, `SELECT published_at FROM space_member_admission_intents WHERE operation_id=$1 AND state='PUBLISHED' AND request_sha256=$2 AND space_id=$3 AND profile_id=$4 AND account_id=$5`, native.Input.OperationID, sha[:], op.SpaceID, op.ActorProfileID, op.ActorAccountID).Scan(&joined); err != nil {
		return ErrJoinAttemptUnavailable
	}
	response := &spacev1.SubmitJoinAttemptResponse{Result: &spacev1.JoinAdmissionResult{State: spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED, SpaceMembership: &spacev1.SpaceMembership{SpaceId: op.SpaceID.String(), ProfileId: op.ActorProfileID.String(), JoinedAt: timestamppb.New(joined)}}}
	if err = s.completeJoinSubmissionOutcome(ctx, op, attempt, response); err != nil {
		return err
	}
	_, err = call.ActorFor(request)
	return err
}
