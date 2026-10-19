package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/space/internal/admissionauthority"
)

// A later keyed retry has a distinct execution binding. The persisted M45
// selection remains the original absent-A context; this is never a new basis.
func legacyPendingResumeContextSHA256(selection [32]byte, attempt uuid.UUID, binding []byte) ([32]byte, error) {
	if !validJoinDecisionDigest(selection[:]) || attempt == uuid.Nil || !validJoinDecisionDigest(binding) {
		return [32]byte{}, ErrJoinDecisionInvalid
	}
	raw := []byte("voice.space.join.legacy-pending-resume-context.v1\x00")
	raw = binary.BigEndian.AppendUint32(raw, 1)
	raw = append(raw, selection[:]...)
	raw = append(raw, attempt[:]...)
	raw = append(raw, binding...)
	return sha256.Sum256(raw), nil
}

type JoinLegacyPendingResumeProposal struct {
	start        *JoinLegacyStartProposal
	attempt      uuid.UUID
	binding      []byte
	selectionSHA [32]byte
}

type VerifiedLegacyJoinPendingResume struct {
	proposal *JoinLegacyPendingResumeProposal
	subject  *admissionauthority.VerifiedSubject
}

// PrepareLegacyJoinPendingResume reloads only the actual keyed operation's
// durable PENDING pin. It never creates/reuses A or chooses a submission winner.
func (s *SpaceStore) PrepareLegacyJoinPendingResume(ctx context.Context, p *JoinLegacyStartProposal) (*JoinLegacyPendingResumeProposal, error) {
	if ctx == nil || s == nil || s.Pool == nil || s.tx != nil || p == nil || p.selection == nil || p.call == nil || p.selection.Context.Origin != JoinOperationClientKey {
		return nil, ErrJoinAttemptUnavailable
	}
	if _, err := p.Reservation(); err != nil {
		return nil, err
	}
	c := p.selection.Context
	in := c.Operation
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err = lockJoinPublicOperation(ctx, tx, in); err != nil {
		return nil, err
	}
	local := *s
	local.tx = tx
	saved, err := local.loadLegacyJoinSelection(ctx, in, JoinOperationClientKey)
	if err != nil {
		return nil, err
	}
	expected, err := c.SHA256()
	if err != nil {
		return nil, err
	}
	actual, err := saved.Context.SHA256()
	if err != nil || actual != expected {
		return nil, ErrJoinAttemptConflict
	}
	if saved.Operation.State != "RESERVED" || saved.Operation.AttemptID == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{in.SpaceID}); err != nil {
		return nil, err
	}
	a, err := scanJoinAttempt(tx.QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1`, *saved.Operation.AttemptID))
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if a.Input.SpaceID != in.SpaceID || a.Input.AccountID != in.ActorAccountID || a.Input.ProfileID != in.ActorProfileID || !sameOptionalJoinAttempt(a.Input.InviteID, c.InviteID) {
		return nil, ErrJoinAttemptConflict
	}
	if a.State != "PENDING" || a.approvedAccount != nil || a.approvedProfile != nil || a.approvalOperation != nil || a.publication != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if a.PolicyVersion != c.PolicyVersion || !bytes.Equal(a.policyDigest, c.PolicySHA256) || !bytes.Equal(joinBinding(a.Input, a.PolicyVersion, a.policyDigest), a.binding) {
		return nil, ErrJoinAttemptUnavailable
	}
	if err = local.localJoinAttemptGates(ctx, a.Input); err != nil {
		return nil, err
	}
	policy, err := local.GetEntryPolicy(ctx, in.SpaceID)
	if err != nil {
		return nil, err
	}
	if policy.Version != c.PolicyVersion || !bytes.Equal(joinDigest("policy", policy.Policy), c.PolicySHA256) {
		return nil, ErrJoinAttemptDenied
	}
	if _, err = p.call.ActorFor(p.request); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &JoinLegacyPendingResumeProposal{start: p, attempt: a.ID, binding: slices.Clone(a.binding), selectionSHA: actual}, nil
}

func (p *JoinLegacyPendingResumeProposal) OwnerRequest() (*authv1.ObserveSpaceAdmissionSubjectRequest, error) {
	if p == nil || p.start == nil || p.start.call == nil || p.start.selection == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	actor, err := p.start.call.ActorFor(p.start.request)
	if err != nil {
		return nil, err
	}
	c := p.start.selection.Context
	selection, err := c.SHA256()
	if err != nil || selection != p.selectionSHA {
		return nil, ErrJoinAttemptUnavailable
	}
	if actor.AccountID != c.Operation.ActorAccountID.String() || actor.ProfileID != c.Operation.ActorProfileID.String() || actor.RPC != c.Operation.Method {
		return nil, ErrJoinAttemptUnavailable
	}
	hash, err := legacyPendingResumeContextSHA256(selection, p.attempt, p.binding)
	if err != nil {
		return nil, err
	}
	epoch := uint64(actor.SessionEpoch)
	return &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_START, SubjectRole: authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT, AccountId: actor.AccountID, ProfileId: actor.ProfileID, SpaceId: c.Operation.SpaceID.String(), AdmissionBindingSha256: hash[:], InteractiveSessionEpoch: &epoch}, nil
}

func (p *JoinLegacyPendingResumeProposal) Seal(subject *admissionauthority.VerifiedSubject) (*VerifiedLegacyJoinPendingResume, error) {
	expected, err := p.OwnerRequest()
	if err != nil {
		return nil, err
	}
	if _, err = subject.Check(expected); err != nil {
		return nil, err
	}
	return &VerifiedLegacyJoinPendingResume{proposal: p, subject: subject}, nil
}

func (d *VerifiedLegacyJoinPendingResume) check() (admissionauthority.SubjectIdentity, error) {
	if d == nil || d.proposal == nil || d.subject == nil {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	expected, err := d.proposal.OwnerRequest()
	if err != nil {
		return admissionauthority.SubjectIdentity{}, err
	}
	return d.subject.Check(expected)
}

func (s *SpaceStore) CompleteLegacyJoinPendingResume(ctx context.Context, d *VerifiedLegacyJoinPendingResume) (*JoinSubmissionRecord, error) {
	if d == nil || d.proposal == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	return s.completeLegacyJoinPending(ctx, d.proposal.start, d.check, d.proposal)
}
