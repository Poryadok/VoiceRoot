package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
)

var ErrLegacyJoinInputRequired = errors.New("interactive admission input required")

type JoinLegacyDecisionProposal struct {
	start           *JoinLegacyStartProposal
	decisionContext JoinLegacyDecisionContext
	input           JoinAttemptInput
	policyVersion   int64
}
type VerifiedLegacyJoinDecision struct {
	proposal *JoinLegacyDecisionProposal
	subject  *admissionauthority.VerifiedSubject
	basis    JoinLegacyDecisionBasis
}

func (s *SpaceStore) PrepareLegacyJoinDecision(ctx context.Context, p *JoinLegacyStartProposal) (*JoinLegacyDecisionProposal, error) {
	if s == nil || s.Pool == nil || s.tx != nil || p == nil || p.call == nil || p.selection == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if _, err := p.call.ActorFor(p.request); err != nil {
		return nil, err
	}
	selection := p.selection.Context
	in := selection.Operation
	return withOwnershipScopeValue(s, ctx, []uuid.UUID{in.SpaceID}, func(local *SpaceStore) (*JoinLegacyDecisionProposal, error) {
		saved, err := local.loadLegacyJoinSelection(ctx, in, selection.Origin)
		if err != nil {
			return nil, err
		}
		sha, err := saved.Context.SHA256()
		if err != nil {
			return nil, err
		}
		expected, err := selection.SHA256()
		if err != nil || sha != expected {
			return nil, ErrJoinAttemptConflict
		}
		if saved.Operation.State != "RESERVED" || saved.Operation.AttemptID == nil {
			return nil, ErrJoinAttemptUnavailable
		}
		a, err := scanJoinAttempt(local.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1`, *saved.Operation.AttemptID))
		if err != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		if a.Input.SpaceID != in.SpaceID || a.Input.AccountID != in.ActorAccountID || a.Input.ProfileID != in.ActorProfileID || !sameOptionalJoinAttempt(a.Input.InviteID, selection.InviteID) {
			return nil, ErrJoinAttemptConflict
		}
		if a.State != "COLLECTING" || a.submission != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		if err = local.localJoinAttemptGates(ctx, a.Input); err != nil {
			return nil, err
		}
		current, err := local.GetEntryPolicy(ctx, in.SpaceID)
		if err != nil {
			return nil, err
		}
		if current.Version != a.PolicyVersion || current.Version != selection.PolicyVersion || !bytes.Equal(joinDigest("policy", current.Policy), a.policyDigest) || !bytes.Equal(a.policyDigest, selection.PolicySHA256) || !bytes.Equal(joinBinding(a.Input, a.PolicyVersion, a.policyDigest), a.binding) {
			return nil, ErrJoinAttemptDenied
		}
		policy, err := entrypolicy.Parse(a.policyRaw)
		if err != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		if policy.RequirePhone && a.Input.Guest {
			return nil, ErrJoinAttemptDenied
		}
		if policy.RequirePhone {
			return nil, ErrJoinAttemptUnavailable
		}
		if policy.RequireCaptcha || len(policy.Questions) != 0 {
			return nil, ErrLegacyJoinInputRequired
		}
		canonical, err := canonicalJoinAnswers(policy, nil)
		if err != nil {
			return nil, err
		}
		a.submission = joinDigest("submission", json.RawMessage(canonical))
		c := JoinLegacyDecisionContext{Kind: JoinLegacyReady, OperationOrigin: selection.Origin, SpaceID: in.SpaceID, AttemptID: a.ID, AttemptBindingSHA256: slices.Clone(a.binding), SubmissionSHA256: slices.Clone(a.submission), ActorAccountID: in.ActorAccountID, ActorProfileID: in.ActorProfileID, OperationID: in.OperationID, RequestMethod: in.Method, RequestSHA256: slices.Clone(in.RequestSHA256)}
		if policy.RequireManualApproval {
			c.Kind = JoinLegacyPending
		} else {
			c.ReadyEvidenceSHA256 = a.evidence()
		}
		if _, err = c.SHA256(); err != nil {
			return nil, err
		}
		var now time.Time
		if err = local.db().QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return nil, err
		}
		if !now.Before(a.CollectionDeadline) {
			return nil, ErrJoinAttemptDenied
		}
		if _, err = p.call.ActorFor(p.request); err != nil {
			return nil, err
		}
		return &JoinLegacyDecisionProposal{start: p, decisionContext: c, input: a.Input, policyVersion: a.PolicyVersion}, nil
	})
}

func (p *JoinLegacyDecisionProposal) OwnerRequest() (*authv1.ObserveSpaceAdmissionSubjectRequest, error) {
	if p == nil || p.start == nil || p.start.call == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	actor, err := p.start.call.ActorFor(p.start.request)
	if err != nil {
		return nil, err
	}
	c := p.decisionContext
	sha, err := c.SHA256()
	if err != nil {
		return nil, err
	}
	if actor.AccountID != c.ActorAccountID.String() || actor.ProfileID != c.ActorProfileID.String() || actor.RPC != c.RequestMethod {
		return nil, ErrJoinAttemptUnavailable
	}
	epoch := uint64(actor.SessionEpoch)
	return &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SUBMIT, SubjectRole: authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT, AccountId: actor.AccountID, ProfileId: actor.ProfileID, SpaceId: c.SpaceID.String(), AdmissionBindingSha256: sha[:], InteractiveSessionEpoch: &epoch}, nil
}
func (p *JoinLegacyDecisionProposal) Seal(subject *admissionauthority.VerifiedSubject) (*VerifiedLegacyJoinDecision, error) {
	expected, err := p.OwnerRequest()
	if err != nil {
		return nil, err
	}
	observation, err := subject.Transcript(expected)
	if err != nil {
		return nil, err
	}
	basis := JoinLegacyDecisionBasis{Context: p.decisionContext, ApplicantObservationSHA256: slices.Clone(observation.ObservationSHA256[:])}
	if _, err = basis.SHA256(); err != nil {
		return nil, err
	}
	return &VerifiedLegacyJoinDecision{proposal: p, subject: subject, basis: basis}, nil
}
func (d *VerifiedLegacyJoinDecision) currentApplicant() (admissionauthority.SubjectIdentity, error) {
	if d == nil || d.proposal == nil || d.subject == nil {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	expected, err := d.proposal.OwnerRequest()
	if err != nil {
		return admissionauthority.SubjectIdentity{}, err
	}
	observation, err := d.subject.Transcript(expected)
	if err != nil || !bytes.Equal(observation.ObservationSHA256[:], d.basis.ApplicantObservationSHA256) {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	return observation.Identity, nil
}

// CommitLegacyJoinDecision records the true legacy Context2, A transition and
// Q together. All remote observations are already complete; only private pure
// checks execute under the actor/op then Space locks. This is not membership.
func (s *SpaceStore) CommitLegacyJoinDecision(ctx context.Context, d *VerifiedLegacyJoinDecision) (*JoinSubmissionRecord, error) {
	if ctx == nil || s == nil || s.Pool == nil || s.tx != nil || d == nil || d.proposal == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if _, err := d.currentApplicant(); err != nil {
		return nil, err
	}
	p := d.proposal
	c := p.decisionContext
	selection := p.start.selection.Context
	in := selection.Operation
	sha, err := d.basis.SHA256()
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(d.basis)
	if err != nil {
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
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{in.SpaceID}); err != nil {
		return nil, err
	}
	saved, err := local.loadLegacyJoinSelection(ctx, in, selection.Origin)
	if err != nil {
		return nil, err
	}
	if saved.Operation.State != "RESERVED" || saved.Operation.AttemptID == nil || *saved.Operation.AttemptID != c.AttemptID {
		return nil, ErrJoinAttemptUnavailable
	}
	actual, err := saved.Context.SHA256()
	if err != nil {
		return nil, err
	}
	wanted, err := selection.SHA256()
	if err != nil || actual != wanted {
		return nil, ErrJoinAttemptConflict
	}
	identity, err := d.currentApplicant()
	if err != nil {
		return nil, err
	}
	if p.input.Guest || identity.AccountType == "guest" {
		effective := p.input
		effective.Guest = true
		if err = local.localJoinAttemptGates(ctx, effective); err != nil {
			return nil, err
		}
	}
	hookRan := false
	var requestID *uuid.UUID
	hook := func(ctx context.Context, scoped *SpaceStore, a *joinAttemptPrivate) error {
		if a.ID != c.AttemptID || a.PolicyVersion != p.policyVersion || !bytes.Equal(joinDigest("input", a.Input), joinDigest("input", p.input)) || !bytes.Equal(a.binding, c.AttemptBindingSHA256) || !bytes.Equal(a.submission, c.SubmissionSHA256) || !bytes.Equal(a.EvidenceSHA256, c.ReadyEvidenceSHA256) || a.approvedAccount != nil || a.approvedProfile != nil || a.approvalOperation != nil {
			return ErrJoinAttemptUnavailable
		}
		if (c.Kind != JoinLegacyPending || a.State != "PENDING") && (c.Kind != JoinLegacyReady || a.State != "READY") {
			return ErrJoinAttemptUnavailable
		}
		tag, err := scoped.db().Exec(ctx, `UPDATE space_join_operations SET state='DECIDED',decision_basis=$3,decision_basis_sha256=$4 WHERE actor_profile_id=$1 AND operation_id=$2 AND state='RESERVED' AND selected_attempt_id=$5 AND request_sha256=$6 AND basis_version=2 AND operation_origin=$7 AND legacy_selection_sha256=$8`, in.ActorProfileID, in.OperationID, raw, sha[:], c.AttemptID, c.RequestSHA256, c.OperationOrigin, wanted[:])
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrJoinAttemptConflict
		}
		if a.State == "PENDING" {
			group, err := scoped.AttachJoinRequestAttempt(ctx, a.ID)
			if err != nil {
				return err
			}
			id := group.ID
			requestID = &id
			// Legacy pending is a final public result for this original key,
			// independent of any later approval/native publication operation.
			// SpaceMembership's empty protobuf is the canonical pending body.
			tag, err = scoped.db().Exec(ctx, `UPDATE space_join_operations SET state='COMPLETED',outcome_code=9,outcome_proto=$3 WHERE actor_profile_id=$1 AND operation_id=$2 AND state='DECIDED' AND decision_basis_sha256=$4`, in.ActorProfileID, in.OperationID, []byte{}, sha[:])
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return ErrJoinAttemptConflict
			}
		}
		hookRan = true
		return nil
	}
	guard := func() error { _, err := d.currentApplicant(); return err }
	a, err := local.submitJoinAttempt(ctx, p.input, c.AttemptID, nil, hook, guard)
	if err != nil {
		return nil, err
	}
	if !hookRan {
		return nil, ErrJoinAttemptUnavailable
	}
	var fresh bool
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()<collection_deadline FROM space_join_attempts WHERE id=$1`, c.AttemptID).Scan(&fresh); err != nil {
		return nil, err
	}
	if !fresh {
		return nil, ErrJoinAttemptDenied
	}
	if err = guard(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &JoinSubmissionRecord{Attempt: *a, DecisionBasisSHA256: slices.Clone(sha[:]), RequestID: requestID}, nil
}
