package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/lifecycleprincipal"
)

// JoinStartProposal has private ingress/request binding and immutable saved
// preselection. Only a fresh exact Auth observation can seal its new decision.
type JoinStartProposal struct {
	request   *spacev1.StartJoinAttemptRequest
	call      *lifecycleprincipal.VerifiedAdmissionCall
	selection JoinStartSelectionContext
}
type VerifiedJoinStartDecision struct {
	proposal *JoinStartProposal
	subject  *admissionauthority.VerifiedSubject
}

func (s *SpaceStore) PrepareJoinStartDecision(ctx context.Context, request *spacev1.StartJoinAttemptRequest, selection *JoinStartSelection) (*JoinStartProposal, error) {
	call, err := lifecycleprincipal.RequireAdmissionStart(ctx, request)
	if err != nil {
		return nil, err
	}
	request = proto.Clone(request).(*spacev1.StartJoinAttemptRequest)
	actor, err := call.ActorFor(request)
	if err != nil {
		return nil, err
	}
	if selection == nil || selection.Operation.State != "RESERVED" || selection.Operation.AttemptID != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	c := selection.Context
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, err
	}
	sha := sha256.Sum256(raw)
	if actor.AccountID != c.Operation.ActorAccountID.String() || actor.ProfileID != c.Operation.ActorProfileID.String() || actor.RPC != JoinOperationStart || request.OperationId != c.Operation.OperationID.String() || !bytes.Equal(sha[:], c.Operation.RequestSHA256) || c.SelectedAttemptID != nil {
		return nil, ErrJoinAttemptConflict
	}
	// Reload owned storage rather than accepting a caller-supplied policy pin.
	saved, exists, err := s.LookupJoinStartSelection(ctx, c.Operation)
	if err != nil || !exists {
		return nil, ErrJoinAttemptUnavailable
	}
	actual, err := saved.Context.SHA256()
	if err != nil {
		return nil, err
	}
	expected, err := c.SHA256()
	if err != nil || actual != expected {
		return nil, ErrJoinAttemptConflict
	}
	return &JoinStartProposal{request: request, call: call, selection: saved.Context}, nil
}

func (p *JoinStartProposal) OwnerRequest() (*authv1.ObserveSpaceAdmissionSubjectRequest, error) {
	if p == nil || p.call == nil || p.request == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	actor, err := p.call.ActorFor(p.request)
	if err != nil {
		return nil, err
	}
	c := p.selection
	sha, err := c.SHA256()
	if err != nil {
		return nil, err
	}
	if actor.AccountID != c.Operation.ActorAccountID.String() || actor.ProfileID != c.Operation.ActorProfileID.String() || actor.RPC != JoinOperationStart {
		return nil, ErrJoinAttemptUnavailable
	}
	epoch := uint64(actor.SessionEpoch)
	return &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_START, SubjectRole: authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT, AccountId: actor.AccountID, ProfileId: actor.ProfileID, SpaceId: c.Operation.SpaceID.String(), AdmissionBindingSha256: sha[:], InteractiveSessionEpoch: &epoch}, nil
}
func (p *JoinStartProposal) Seal(subject *admissionauthority.VerifiedSubject) (*VerifiedJoinStartDecision, error) {
	expected, err := p.OwnerRequest()
	if err != nil {
		return nil, err
	}
	if _, err = subject.Check(expected); err != nil {
		return nil, err
	}
	return &VerifiedJoinStartDecision{proposal: p, subject: subject}, nil
}
func (d *VerifiedJoinStartDecision) check() (admissionauthority.SubjectIdentity, error) {
	if d == nil || d.proposal == nil || d.subject == nil {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	expected, err := d.proposal.OwnerRequest()
	if err != nil {
		return admissionauthority.SubjectIdentity{}, err
	}
	return d.subject.Check(expected)
}

func savedJoinStartResponse(saved *JoinStartSelection) (*spacev1.StartJoinAttemptResponse, error) {
	if saved == nil || saved.Operation.State != "COMPLETED" || saved.Operation.AttemptID == nil || saved.OutcomeCode == nil || *saved.OutcomeCode != 0 {
		return nil, ErrJoinAttemptUnavailable
	}
	var response spacev1.StartJoinAttemptResponse
	if proto.Unmarshal(saved.OutcomeProto, &response) != nil || response.Attempt == nil || response.Attempt.AttemptId != saved.Operation.AttemptID.String() || response.Attempt.SpaceId != saved.Operation.Input.SpaceID.String() || response.Attempt.Result == nil || response.Attempt.ExpiresAt == nil || !response.Attempt.ExpiresAt.IsValid() {
		return nil, ErrJoinAttemptUnavailable
	}
	return &response, nil
}

// ReplayJoinStartResponse returns saved safe bytes only. Caller must authenticate
// the exact actor/body through the private ingress before looking up the row.
func ReplayJoinStartResponse(saved *JoinStartSelection) (*spacev1.StartJoinAttemptResponse, error) {
	return savedJoinStartResponse(saved)
}

func joinStartResponse(view *JoinAttemptRead) (*spacev1.StartJoinAttemptResponse, error) {
	state := spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_UNSPECIFIED
	switch view.State {
	case "COLLECTING":
		state = spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_COLLECTING
	case "PENDING":
		state = spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PENDING
	case "READY":
		state = spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PROCESSING
	default:
		return nil, ErrJoinAttemptUnavailable
	}
	result := &spacev1.JoinAdmissionResult{State: state}
	if view.RequestID != nil {
		result.RequestId = view.RequestID.String()
	} else if view.State == "PENDING" {
		return nil, ErrJoinAttemptUnavailable
	}
	requirements := &spacev1.JoinRequirements{RequirePhone: view.Requirements.Phone, RequireCaptcha: view.Requirements.Captcha, RequireManualApproval: view.Requirements.Manual}
	for _, q := range view.Requirements.Questions {
		requirements.Questions = append(requirements.Questions, &spacev1.JoinQuestion{QuestionId: q.ID, Prompt: q.Prompt})
	}
	out := &spacev1.JoinAttemptView{AttemptId: view.ID.String(), SpaceId: view.SpaceID.String(), PolicyVersion: view.PolicyVersion, ExpiresAt: timestamppb.New(view.CollectionDeadline), Requirements: requirements, Result: result}
	for _, a := range view.Answers {
		out.SubmittedAnswers = append(out.SubmittedAnswers, &spacev1.JoinAnswer{QuestionId: a.QuestionID.String(), Value: a.Value})
	}
	return &spacev1.StartJoinAttemptResponse{Attempt: out}, nil
}

// CommitJoinStartDecision pins A and its safe completed outcome in one actor/op
// then Space transaction. No network callback executes under these locks.
func (s *SpaceStore) CommitJoinStartDecision(ctx context.Context, d *VerifiedJoinStartDecision) (*spacev1.StartJoinAttemptResponse, error) {
	if s == nil || s.Pool == nil || s.tx != nil || d == nil || d.proposal == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	p := d.proposal
	in := p.selection.Operation
	if _, err := p.call.ActorFor(p.request); err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	if err = lockJoinPublicOperation(ctx, tx, in); err != nil {
		return nil, err
	}
	local := *s
	local.tx = tx
	saved, err := local.loadJoinStartSelection(ctx, in)
	if err != nil {
		return nil, err
	}
	if saved.Operation.State == "COMPLETED" {
		response, err := savedJoinStartResponse(saved)
		if err != nil {
			return nil, err
		}
		return response, tx.Commit(ctx)
	}
	actual, err := saved.Context.SHA256()
	if err != nil {
		return nil, err
	}
	expected, err := p.selection.SHA256()
	if err != nil || actual != expected {
		return nil, ErrJoinAttemptConflict
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{in.SpaceID}); err != nil {
		return nil, err
	}
	identity, err := d.check()
	if err != nil {
		return nil, err
	}
	policy, err := local.GetEntryPolicy(ctx, in.SpaceID)
	if err != nil {
		return nil, err
	}
	if policy.Version != p.selection.PolicyVersion || !bytes.Equal(joinDigest("policy", policy.Policy), p.selection.PolicySHA256) {
		return nil, ErrJoinAttemptDenied
	}
	applicant := JoinAttemptInput{SpaceID: in.SpaceID, AccountID: in.ActorAccountID, ProfileID: in.ActorProfileID, InviteID: p.selection.InviteID, Guest: identity.AccountType == "guest"}
	a, err := local.StartJoinAttempt(ctx, applicant)
	if err != nil {
		return nil, err
	}
	if a.PolicyVersion != policy.Version {
		return nil, ErrJoinAttemptUnavailable
	}
	if err = local.SelectJoinOperationAttempt(ctx, in, a.ID); err != nil {
		return nil, err
	}
	guard := func() error { _, err := d.check(); return err }
	view, err := local.ReadOwnJoinAttempt(ctx, in.SpaceID, a.ID, in.ActorAccountID, in.ActorProfileID, guard)
	if err != nil {
		return nil, err
	}
	response, err := joinStartResponse(view)
	if err != nil {
		return nil, err
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(response)
	if err != nil {
		return nil, err
	}
	tag, err := tx.Exec(ctx, `UPDATE space_join_operations SET state='COMPLETED',outcome_code=0,outcome_proto=$3 WHERE actor_profile_id=$1 AND operation_id=$2 AND state='RESERVED' AND selected_attempt_id=$4`, in.ActorProfileID, in.OperationID, raw, a.ID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, ErrJoinAttemptConflict
	}
	if a.State == "COLLECTING" {
		var now time.Time
		if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return nil, err
		}
		if !now.Before(a.CollectionDeadline) {
			return nil, ErrJoinAttemptDenied
		}
	}
	if err = guard(); err != nil {
		return nil, err
	}
	return response, tx.Commit(ctx)
}
