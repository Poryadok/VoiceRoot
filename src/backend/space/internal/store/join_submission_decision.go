package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
)

// JoinSubmissionProposal contains server-owned prospective data, not an allow.
type JoinSubmissionProposal struct {
	decisionContext JoinDecisionContext
	request         *spacev1.SubmitJoinAttemptRequest
	call            *lifecycleprincipal.VerifiedAdmissionCall
	input           JoinAttemptInput
	policyVersion   int64
}

// VerifiedJoinSubmissionDecision has no serializable/public allow fields.
// Its capabilities must still be checked after every final SQL wait.
type VerifiedJoinSubmissionDecision struct {
	proposal *JoinSubmissionProposal
	subject  *admissionauthority.VerifiedSubject
	basis    JoinDecisionBasis
}

// JoinSubmissionRecord is an accepted decision, never successful membership.
type JoinSubmissionRecord struct {
	Attempt             JoinAttempt
	DecisionBasisSHA256 []byte
	RequestID           *uuid.UUID
}

func (d *VerifiedJoinSubmissionDecision) currentApplicant() (admissionauthority.SubjectIdentity, error) {
	if d == nil || d.proposal == nil || d.subject == nil {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	expected, err := d.proposal.OwnerRequest()
	if err != nil {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	transcript, err := d.subject.Transcript(expected)
	if err != nil || !bytes.Equal(transcript.ObservationSHA256[:], d.basis.ApplicantObservationSHA256) {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	actual, err := d.basis.Context.SHA256()
	if err != nil {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	wanted, err := d.proposal.decisionContext.SHA256()
	if err != nil || actual != wanted {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	return transcript.Identity, nil
}

// CommitJoinSubmissionDecision records successful automatic stages and the
// original immutable basis atomically. It cannot publish membership. Historical
// decisions/other-operation aliases require their separate recovery adapter.
func (s *SpaceStore) CommitJoinSubmissionDecision(ctx context.Context, decision *VerifiedJoinSubmissionDecision) (*JoinSubmissionRecord, error) {
	identity, err := decision.currentApplicant()
	if err != nil || s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	p := decision.proposal
	c := p.decisionContext
	basisSHA, err := decision.basis.SHA256()
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	raw, err := json.Marshal(decision.basis)
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	answers := make([]entrypolicy.Answer, 0, len(p.request.Answers))
	for _, answer := range p.request.Answers {
		id, err := canonicalJoinID(answer.GetQuestionId())
		if err != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		answers = append(answers, entrypolicy.Answer{QuestionID: id, Value: answer.GetValue()})
	}
	return withOwnershipScopeValue(s, ctx, []uuid.UUID{p.input.SpaceID}, func(scoped *SpaceStore) (*JoinSubmissionRecord, error) {
		var saved JoinOperation
		err := scoped.db().QueryRow(ctx, `SELECT `+joinOperationColumns+` FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2 FOR UPDATE`, c.ActorProfileID, c.OperationID).Scan(&saved.Input.SpaceID, &saved.Input.ActorAccountID, &saved.Input.ActorProfileID, &saved.Input.OperationID, &saved.Input.Method, &saved.Input.RequestSHA256, &saved.AttemptID, &saved.State)
		if err != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		expected := JoinOperationInput{SpaceID: c.SpaceID, ActorAccountID: c.ActorAccountID, ActorProfileID: c.ActorProfileID, OperationID: c.OperationID, Method: c.RequestMethod, RequestSHA256: c.RequestSHA256}
		if saved.AttemptID == nil || *saved.AttemptID != c.AttemptID || !bytes.Equal(joinDigest("public-operation", saved.Input), joinDigest("public-operation", expected)) {
			return nil, ErrJoinAttemptConflict
		}
		if saved.State != "RESERVED" {
			return nil, ErrJoinAttemptUnavailable
		}
		guard := func() error {
			_, err := decision.currentApplicant()
			return err
		}
		if err = guard(); err != nil {
			return nil, err
		}
		// Current Auth type is checked through the private capability. Saved A
		// and its evidence remain unchanged; the shared Role default is unrelated.
		if p.input.Guest || identity.AccountType == "guest" {
			effective := p.input
			effective.Guest = true
			if err = scoped.localJoinAttemptGates(ctx, effective); err != nil {
				return nil, err
			}
		}
		hookRan := false
		var requestID *uuid.UUID
		hook := func(ctx context.Context, local *SpaceStore, a *joinAttemptPrivate) error {
			if a.ID != c.AttemptID || a.PolicyVersion != p.policyVersion || !bytes.Equal(joinDigest("input", a.Input), joinDigest("input", p.input)) || !bytes.Equal(a.binding, c.AttemptBindingSHA256) || !bytes.Equal(a.submission, c.SubmissionSHA256) || !bytes.Equal(a.EvidenceSHA256, c.ReadyEvidenceSHA256) || a.approvedAccount != nil || a.approvedProfile != nil || a.approvalOperation != nil {
				return ErrJoinAttemptUnavailable
			}
			if (c.Kind != JoinDecisionSubmitPending || a.State != "PENDING") && (c.Kind != JoinDecisionSubmitReady || a.State != "READY") {
				return ErrJoinAttemptUnavailable
			}
			tag, err := local.db().Exec(ctx, `UPDATE space_join_operations SET state='DECIDED',decision_basis=$3,decision_basis_sha256=$4 WHERE actor_profile_id=$1 AND operation_id=$2 AND state='RESERVED' AND selected_attempt_id=$5 AND request_sha256=$6`, c.ActorProfileID, c.OperationID, raw, basisSHA[:], c.AttemptID, c.RequestSHA256)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return ErrJoinAttemptConflict
			}
			if a.State == "PENDING" {
				group, err := local.AttachJoinRequestAttempt(ctx, a.ID)
				if err != nil {
					return err
				}
				id := group.ID
				requestID = &id
			}
			hookRan = true
			return nil
		}
		attempt, err := scoped.submitJoinAttempt(ctx, p.input, c.AttemptID, answers, hook, guard)
		if err != nil {
			return nil, err
		}
		if !hookRan {
			// Another operation won while owner observations were in flight.
			// Never manufacture a second basis or silently move its proof.
			return nil, ErrJoinAttemptUnavailable
		}
		var withinCollection bool
		if err = scoped.db().QueryRow(ctx, `SELECT clock_timestamp()<collection_deadline FROM space_join_attempts WHERE id=$1`, c.AttemptID).Scan(&withinCollection); err != nil {
			return nil, err
		}
		if !withinCollection {
			return nil, ErrJoinAttemptDenied
		}
		if err = guard(); err != nil {
			return nil, err
		}
		return &JoinSubmissionRecord{Attempt: *attempt, DecisionBasisSHA256: slices.Clone(basisSHA[:]), RequestID: requestID}, nil
	})
}

func (s *SpaceStore) PrepareJoinSubmissionDecision(ctx context.Context, request *spacev1.SubmitJoinAttemptRequest) (*JoinSubmissionProposal, error) {
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
		// A decided operation belongs to historical recovery. Never replace its
		// original body/observations with a newly fetched capability.
		return nil, ErrJoinAttemptUnavailable
	}
	return s.prepareJoinSubmissionProposal(ctx, op, request, call)
}

func canonicalJoinID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return uuid.Nil, entrypolicy.ErrInvalid
	}
	return id, nil
}

// prepareJoinSubmissionProposal builds data only. A nil ingress capability is
// useful to storage fixtures but cannot yield an owner request or a sealed allow.
func (s *SpaceStore) prepareJoinSubmissionProposal(ctx context.Context, op JoinOperationInput, request *spacev1.SubmitJoinAttemptRequest, call *lifecycleprincipal.VerifiedAdmissionCall) (*JoinSubmissionProposal, error) {
	if request == nil || len(request.ProtoReflect().GetUnknown()) != 0 || op.Method != JoinOperationSubmit || request.SpaceId != op.SpaceID.String() || request.OperationId != op.OperationID.String() {
		return nil, entrypolicy.ErrInvalid
	}
	attempt, err := canonicalJoinID(request.AttemptId)
	if err != nil {
		return nil, err
	}
	request = proto.Clone(request).(*spacev1.SubmitJoinAttemptRequest)
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, entrypolicy.ErrInvalid
	}
	requestSHA := sha256.Sum256(raw)
	if !bytes.Equal(requestSHA[:], op.RequestSHA256) {
		return nil, ErrJoinAttemptConflict
	}
	return withOwnershipScopeValue(s, ctx, []uuid.UUID{op.SpaceID}, func(scoped *SpaceStore) (*JoinSubmissionProposal, error) {
		a, err := scanJoinAttempt(scoped.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1 FOR UPDATE`, attempt))
		if err != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		if a.Input.SpaceID != op.SpaceID || a.Input.AccountID != op.ActorAccountID || a.Input.ProfileID != op.ActorProfileID {
			return nil, ErrJoinAttemptDenied
		}
		if a.State != "COLLECTING" || a.submission != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		if err = scoped.localJoinAttemptGates(ctx, a.Input); err != nil {
			return nil, err
		}
		current, err := scoped.GetEntryPolicy(ctx, op.SpaceID)
		if err != nil {
			return nil, err
		}
		if current.Version != a.PolicyVersion || !bytes.Equal(joinDigest("policy", current.Policy), a.policyDigest) || !bytes.Equal(joinBinding(a.Input, a.PolicyVersion, a.policyDigest), a.binding) {
			return nil, ErrJoinAttemptDenied
		}
		policy, err := entrypolicy.Parse(a.policyRaw)
		if err != nil || !bytes.Equal(joinDigest("policy", policy), a.policyDigest) {
			return nil, ErrJoinAttemptUnavailable
		}
		if policy.RequirePhone && a.Input.Guest {
			return nil, ErrJoinAttemptDenied
		}
		if policy.RequirePhone || policy.RequireCaptcha {
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
		canonical, err := canonicalJoinAnswers(policy, answers)
		if err != nil {
			// This constructor covers prospective successful decisions only.
			// The public negative-answer adapter must still terminally record FAILED.
			return nil, err
		}
		a.submission = joinDigest("submission", json.RawMessage(canonical))
		c := JoinDecisionContext{Kind: JoinDecisionSubmitReady, SpaceID: op.SpaceID, AttemptID: a.ID, AttemptBindingSHA256: slices.Clone(a.binding), SubmissionSHA256: slices.Clone(a.submission), ActorAccountID: op.ActorAccountID, ActorProfileID: op.ActorProfileID, OperationID: op.OperationID, RequestMethod: op.Method, RequestSHA256: slices.Clone(op.RequestSHA256)}
		if policy.RequireManualApproval {
			c.Kind = JoinDecisionSubmitPending
		} else {
			c.ReadyEvidenceSHA256 = a.evidence()
		}
		var now time.Time
		if err = scoped.db().QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return nil, err
		}
		if !now.Before(a.CollectionDeadline) {
			return nil, ErrJoinAttemptDenied
		}
		if call != nil {
			if _, err = call.ActorFor(request); err != nil {
				return nil, err
			}
		}
		return &JoinSubmissionProposal{decisionContext: c, request: request, call: call, input: a.Input, policyVersion: a.PolicyVersion}, nil
	})
}

func (p *JoinSubmissionProposal) Context() JoinDecisionContext {
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

func (p *JoinSubmissionProposal) OwnerRequest() (*authv1.ObserveSpaceAdmissionSubjectRequest, error) {
	if p == nil || p.call == nil || p.request == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	actor, err := p.call.ActorFor(p.request)
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	c := p.decisionContext
	sha, err := c.SHA256()
	if err != nil || actor.AccountID != c.ActorAccountID.String() || actor.ProfileID != c.ActorProfileID.String() || actor.RPC != JoinOperationSubmit {
		return nil, ErrJoinAttemptUnavailable
	}
	epoch := uint64(actor.SessionEpoch)
	return &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SUBMIT, SubjectRole: authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT, AccountId: p.input.AccountID.String(), ProfileId: p.input.ProfileID.String(), SpaceId: p.input.SpaceID.String(), AdmissionBindingSha256: sha[:], InteractiveSessionEpoch: &epoch}, nil
}

func (p *JoinSubmissionProposal) Seal(subject *admissionauthority.VerifiedSubject) (*VerifiedJoinSubmissionDecision, error) {
	expected, err := p.OwnerRequest()
	if err != nil {
		return nil, err
	}
	transcript, err := subject.Transcript(expected)
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	basis := JoinDecisionBasis{Context: p.Context(), ApplicantObservationSHA256: slices.Clone(transcript.ObservationSHA256[:])}
	if _, err = basis.SHA256(); err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	return &VerifiedJoinSubmissionDecision{proposal: p, subject: subject, basis: basis}, nil
}
