package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
)

// A rejected answer has its own purpose. It can never fabricate a successful
// decision basis or be consumed by native Prepare/FIRST_PUBLISH.
type JoinRejectedSubmissionProposal struct {
	request                        *spacev1.SubmitJoinAttemptRequest
	call                           *lifecycleprincipal.VerifiedAdmissionCall
	operation                      JoinOperationInput
	attempt                        JoinAttemptInput
	attemptID                      uuid.UUID
	policyVersion                  int64
	binding, policySHA, submission []byte
	answers                        []entrypolicy.Answer
}
type VerifiedJoinRejectedSubmission struct {
	proposal *JoinRejectedSubmissionProposal
	subject  *admissionauthority.VerifiedSubject
}

func (s *SpaceStore) PrepareJoinRejectedSubmission(ctx context.Context, request *spacev1.SubmitJoinAttemptRequest) (*JoinRejectedSubmissionProposal, error) {
	call, err := lifecycleprincipal.RequireAdmissionSubmit(ctx, request)
	if err != nil {
		return nil, err
	}
	request = proto.Clone(request).(*spacev1.SubmitJoinAttemptRequest)
	actor, err := call.ActorFor(request)
	if err != nil {
		return nil, err
	}
	space, err := canonicalJoinID(request.SpaceId)
	if err != nil {
		return nil, err
	}
	attempt, err := canonicalJoinID(request.AttemptId)
	if err != nil {
		return nil, err
	}
	operation, err := canonicalJoinID(request.OperationId)
	if err != nil {
		return nil, err
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, entrypolicy.ErrInvalid
	}
	sha := sha256.Sum256(raw)
	op := JoinOperationInput{SpaceID: space, ActorAccountID: uuid.MustParse(actor.AccountID), ActorProfileID: uuid.MustParse(actor.ProfileID), OperationID: operation, Method: JoinOperationSubmit, RequestSHA256: sha[:]}
	reserved, err := s.ReserveJoinOperation(ctx, op, &attempt)
	if err != nil {
		return nil, err
	}
	if reserved.State != "RESERVED" {
		return nil, ErrJoinAttemptUnavailable
	}
	return withOwnershipScopeValue(s, ctx, []uuid.UUID{space}, func(local *SpaceStore) (*JoinRejectedSubmissionProposal, error) {
		a, err := scanJoinAttempt(local.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1 FOR UPDATE`, attempt))
		if err != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		if a.Input.SpaceID != space || a.Input.AccountID != op.ActorAccountID || a.Input.ProfileID != op.ActorProfileID {
			return nil, ErrJoinAttemptDenied
		}
		if a.State != "COLLECTING" || a.submission != nil {
			return nil, ErrJoinAttemptDenied
		}
		if err = local.localJoinAttemptGates(ctx, a.Input); err != nil {
			return nil, err
		}
		policy, err := local.GetEntryPolicy(ctx, space)
		if err != nil {
			return nil, err
		}
		if policy.Version != a.PolicyVersion || !bytes.Equal(joinDigest("policy", policy.Policy), a.policyDigest) || !bytes.Equal(joinBinding(a.Input, a.PolicyVersion, a.policyDigest), a.binding) {
			return nil, ErrJoinAttemptDenied
		}
		if policy.Policy.RequirePhone && a.Input.Guest {
			return nil, ErrJoinAttemptDenied
		}
		if policy.Policy.RequirePhone || policy.Policy.RequireCaptcha {
			return nil, ErrJoinAttemptUnavailable
		}
		if request.GetCaptchaToken() != "" {
			return nil, entrypolicy.ErrInvalid
		}
		answers := make([]entrypolicy.Answer, 0, len(request.Answers))
		for _, answer := range request.Answers {
			if answer == nil || len(answer.ProtoReflect().GetUnknown()) != 0 {
				return nil, entrypolicy.ErrInvalid
			}
			id, err := canonicalJoinID(answer.QuestionId)
			if err != nil {
				return nil, err
			}
			answers = append(answers, entrypolicy.Answer{QuestionID: id, Value: answer.Value})
		}
		canonical, answerErr := canonicalJoinAnswers(policy.Policy, answers)
		if !errors.Is(answerErr, entrypolicy.ErrDenied) {
			if answerErr != nil {
				return nil, answerErr
			}
			return nil, ErrJoinAttemptConflict
		}
		var now time.Time
		if err = local.db().QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return nil, err
		}
		if !now.Before(a.CollectionDeadline) {
			return nil, ErrJoinAttemptDenied
		}
		if _, err = call.ActorFor(request); err != nil {
			return nil, err
		}
		return &JoinRejectedSubmissionProposal{request: request, call: call, operation: op, attempt: a.Input, attemptID: a.ID, policyVersion: a.PolicyVersion, binding: bytes.Clone(a.binding), policySHA: bytes.Clone(a.policyDigest), submission: joinDigest("submission", json.RawMessage(canonical)), answers: answers}, nil
	})
}

func (p *JoinRejectedSubmissionProposal) OwnerRequest() (*authv1.ObserveSpaceAdmissionSubjectRequest, error) {
	if p == nil || p.call == nil || p.request == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	actor, err := p.call.ActorFor(p.request)
	if err != nil {
		return nil, err
	}
	op := p.operation
	if actor.AccountID != op.ActorAccountID.String() || actor.ProfileID != op.ActorProfileID.String() || actor.RPC != JoinOperationSubmit || !validJoinDecisionDigest(p.binding) || !validJoinDecisionDigest(p.policySHA) || !validJoinDecisionDigest(p.submission) {
		return nil, ErrJoinAttemptUnavailable
	}
	frame := []byte("voice.space.join.rejected-submission-context.v1\x00")
	frame = binary.BigEndian.AppendUint32(frame, 1)
	frame = binary.BigEndian.AppendUint32(frame, uint32(len(op.Method)))
	frame = append(frame, op.Method...)
	frame = append(frame, op.SpaceID[:]...)
	frame = append(frame, op.ActorAccountID[:]...)
	frame = append(frame, op.ActorProfileID[:]...)
	frame = append(frame, op.OperationID[:]...)
	frame = append(frame, op.RequestSHA256...)
	frame = append(frame, p.attemptID[:]...)
	frame = append(frame, p.binding...)
	frame = binary.BigEndian.AppendUint64(frame, uint64(p.policyVersion))
	frame = append(frame, p.policySHA...)
	frame = append(frame, p.submission...)
	sha := sha256.Sum256(frame)
	epoch := uint64(actor.SessionEpoch)
	return &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SUBMIT, SubjectRole: authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT, AccountId: actor.AccountID, ProfileId: actor.ProfileID, SpaceId: op.SpaceID.String(), AdmissionBindingSha256: sha[:], InteractiveSessionEpoch: &epoch}, nil
}
func (p *JoinRejectedSubmissionProposal) Seal(subject *admissionauthority.VerifiedSubject) (*VerifiedJoinRejectedSubmission, error) {
	expected, err := p.OwnerRequest()
	if err != nil {
		return nil, err
	}
	if _, err = subject.Check(expected); err != nil {
		return nil, err
	}
	return &VerifiedJoinRejectedSubmission{proposal: p, subject: subject}, nil
}
func (d *VerifiedJoinRejectedSubmission) check() (admissionauthority.SubjectIdentity, error) {
	if d == nil || d.proposal == nil || d.subject == nil {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	expected, err := d.proposal.OwnerRequest()
	if err != nil {
		return admissionauthority.SubjectIdentity{}, err
	}
	return d.subject.Check(expected)
}

func validRejectedSubmissionOutcome(replay *JoinOperationReplay, attempt uuid.UUID) bool {
	if replay == nil || replay.Operation.State != "COMPLETED" || replay.Operation.AttemptID == nil || *replay.Operation.AttemptID != attempt || replay.Code == nil || *replay.Code != int32(codes.PermissionDenied) {
		return false
	}
	var response spacev1.SubmitJoinAttemptResponse
	return proto.Unmarshal(replay.Proto, &response) == nil && response.Result != nil && response.Result.State == spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_FAILED && response.Result.SpaceMembership == nil && response.Result.RequestId == ""
}

// CommitJoinRejectedSubmission returns nil only after FAILED plus the generic
// denied outcome are durable. ErrDenied from the inner collector is handled as
// data here; propagating it through this transaction would undo FAILED.
func (s *SpaceStore) CommitJoinRejectedSubmission(ctx context.Context, d *VerifiedJoinRejectedSubmission) error {
	if s == nil || s.Pool == nil || s.tx != nil || d == nil || d.proposal == nil {
		return ErrJoinAttemptUnavailable
	}
	p := d.proposal
	op := p.operation
	if _, err := p.call.ActorFor(p.request); err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	if err = lockJoinPublicOperation(ctx, tx, op); err != nil {
		return err
	}
	local := *s
	local.tx = tx
	replay, exists, err := local.LookupJoinOperationReplay(ctx, op)
	if err != nil {
		return err
	}
	if !exists {
		return ErrJoinAttemptUnavailable
	}
	if replay.Operation.State == "COMPLETED" {
		if !validRejectedSubmissionOutcome(replay, p.attemptID) {
			return ErrJoinAttemptUnavailable
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		_, err = p.call.ActorFor(p.request)
		return err
	}
	if replay.Operation.State != "RESERVED" || replay.Operation.AttemptID == nil || *replay.Operation.AttemptID != p.attemptID {
		return ErrJoinAttemptUnavailable
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{op.SpaceID}); err != nil {
		return err
	}
	identity, err := d.check()
	if err != nil {
		return err
	}
	a, err := scanJoinAttempt(local.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1 FOR UPDATE`, p.attemptID))
	if err != nil {
		return ErrJoinAttemptUnavailable
	}
	if a.State != "COLLECTING" || a.submission != nil || a.PolicyVersion != p.policyVersion || !bytes.Equal(a.binding, p.binding) || !bytes.Equal(a.policyDigest, p.policySHA) || !bytes.Equal(joinDigest("input", a.Input), joinDigest("input", p.attempt)) {
		return ErrJoinAttemptConflict
	}
	if p.attempt.Guest || identity.AccountType == "guest" {
		effective := p.attempt
		effective.Guest = true
		if err = local.localJoinAttemptGates(ctx, effective); err != nil {
			return err
		}
	}
	guard := func() error { _, err := d.check(); return err }
	failed, err := local.submitJoinAttempt(ctx, p.attempt, p.attemptID, p.answers, nil, guard)
	if !errors.Is(err, entrypolicy.ErrDenied) || failed == nil || failed.State != "FAILED" {
		if err != nil {
			return err
		}
		return ErrJoinAttemptUnavailable
	}
	var submission []byte
	if err = tx.QueryRow(ctx, `SELECT submission_sha256 FROM space_join_attempts WHERE id=$1`, p.attemptID).Scan(&submission); err != nil {
		return err
	}
	if !bytes.Equal(submission, p.submission) {
		return ErrJoinAttemptUnavailable
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(&spacev1.SubmitJoinAttemptResponse{Result: &spacev1.JoinAdmissionResult{State: spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_FAILED}})
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE space_join_operations SET state='COMPLETED',outcome_code=$3,outcome_proto=$4 WHERE actor_profile_id=$1 AND operation_id=$2 AND state='RESERVED' AND selected_attempt_id=$5 AND decision_basis IS NULL AND observed_basis_sha256 IS NULL`, op.ActorProfileID, op.OperationID, int32(codes.PermissionDenied), raw, p.attemptID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrJoinAttemptConflict
	}
	var within bool
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()<collection_deadline FROM space_join_attempts WHERE id=$1`, p.attemptID).Scan(&within); err != nil {
		return err
	}
	if !within {
		return ErrJoinAttemptDenied
	}
	if err = guard(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
