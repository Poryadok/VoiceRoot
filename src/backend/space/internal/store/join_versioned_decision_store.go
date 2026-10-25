package store

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"slices"

	"github.com/google/uuid"
	"voice/backend/space/internal/entrypolicy"
)

// ValidatedJoinDecisionBasis is historical data from the owning journal. Its
// private tag cannot be used to manufacture a current admission capability.
type ValidatedJoinDecisionBasis struct {
	version uint8
	v1      *JoinDecisionBasis
	legacy  *JoinLegacyDecisionBasis
}

func (b *ValidatedJoinDecisionBasis) Version() uint8 {
	if b == nil {
		return 0
	}
	return b.version
}
func (b *ValidatedJoinDecisionBasis) V1() (JoinDecisionBasis, error) {
	if b == nil || b.version != 1 || b.v1 == nil || b.legacy != nil {
		return JoinDecisionBasis{}, ErrJoinAttemptUnavailable
	}
	return cloneJoinV1Basis(*b.v1), nil
}
func (b *ValidatedJoinDecisionBasis) LegacyV2() (JoinLegacyDecisionBasis, error) {
	if b == nil || b.version != 2 || b.legacy == nil || b.v1 != nil {
		return JoinLegacyDecisionBasis{}, ErrJoinAttemptUnavailable
	}
	return cloneJoinLegacyBasis(*b.legacy), nil
}
func (s *SpaceStore) LoadVersionedJoinDecisionBasis(ctx context.Context, sha []byte) (*ValidatedJoinDecisionBasis, error) {
	if ctx == nil || s == nil || (s.Pool == nil && s.tx == nil) || !validJoinDecisionDigest(sha) {
		return nil, ErrJoinAttemptUnavailable
	}
	var version int16
	if err := s.db().QueryRow(ctx, `SELECT basis_version FROM space_join_operations WHERE decision_basis_sha256=$1`, sha).Scan(&version); err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	switch version {
	case 1:
		saved, err := s.loadJoinDecision(ctx, sha, false)
		if err != nil {
			return nil, err
		}
		if saved.basis.Context.Kind == JoinDecisionApproveReady {
			parent, err := s.loadJoinDecision(ctx, saved.basis.Context.SubmitBasisSHA256, false)
			if err != nil || !v1OriginalParentMatches(saved.basis.Context, parent) {
				return nil, ErrJoinAttemptUnavailable
			}
		}
		basis := cloneJoinV1Basis(saved.basis)
		return &ValidatedJoinDecisionBasis{version: 1, v1: &basis}, nil
	case 2:
		saved, err := s.loadLegacyJoinDecision(ctx, sha, false)
		if err != nil {
			return nil, err
		}
		if saved.basis.Context.Kind == JoinApproveLegacyReady {
			parent, err := s.loadLegacyJoinDecision(ctx, saved.basis.Context.PendingBasisSHA256, false)
			if err != nil || !legacyOriginalParentMatches(saved.basis.Context, parent) {
				return nil, ErrJoinAttemptUnavailable
			}
		}
		basis := cloneJoinLegacyBasis(saved.basis)
		return &ValidatedJoinDecisionBasis{version: 2, legacy: &basis}, nil
	default:
		return nil, ErrJoinAttemptUnavailable
	}
}

type legacyJoinDecisionSaved struct {
	basis            JoinLegacyDecisionBasis
	operation        JoinOperation
	publication      *uuid.UUID
	publicationState *string
}

func (s *SpaceStore) loadLegacyJoinDecision(ctx context.Context, sha []byte, lock bool) (*legacyJoinDecisionSaved, error) {
	if s == nil || (s.Pool == nil && s.tx == nil) || !validJoinDecisionDigest(sha) || (lock && s.tx == nil) {
		return nil, ErrJoinAttemptUnavailable
	}
	query := `SELECT ` + joinOperationColumns + `,basis_version,operation_origin,decision_basis,publication_operation_id,publication_state FROM space_join_operations WHERE decision_basis_sha256=$1`
	if lock {
		query += ` FOR UPDATE`
	}
	var saved legacyJoinDecisionSaved
	var version, origin int16
	var raw []byte
	op := &saved.operation
	err := s.db().QueryRow(ctx, query, sha).Scan(&op.Input.SpaceID, &op.Input.ActorAccountID, &op.Input.ActorProfileID, &op.Input.OperationID, &op.Input.Method, &op.Input.RequestSHA256, &op.AttemptID, &op.State, &version, &origin, &raw, &saved.publication, &saved.publicationState)
	if err != nil || version != 2 || op.AttemptID == nil || op.State == "RESERVED" || len(raw) > 4096 {
		return nil, ErrJoinAttemptUnavailable
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&saved.basis) != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, ErrJoinAttemptUnavailable
	}
	digest, err := saved.basis.SHA256()
	if err != nil || !bytes.Equal(digest[:], sha) {
		return nil, ErrJoinAttemptUnavailable
	}
	c := saved.basis.Context
	expected := JoinOperationInput{SpaceID: c.SpaceID, ActorAccountID: c.ActorAccountID, ActorProfileID: c.ActorProfileID, OperationID: c.OperationID, Method: c.RequestMethod, RequestSHA256: c.RequestSHA256}
	if int16(c.OperationOrigin) != origin || *op.AttemptID != c.AttemptID || !bytes.Equal(joinDigest("public-operation", expected), joinDigest("public-operation", op.Input)) {
		return nil, ErrJoinAttemptUnavailable
	}
	return &saved, nil
}

func legacyOriginalParentMatches(c JoinLegacyDecisionContext, p *legacyJoinDecisionSaved) bool {
	if p == nil {
		return false
	}
	parent := p.basis.Context
	return p.publication == nil && parent.Kind == JoinLegacyPending && parent.SpaceID == c.SpaceID && parent.AttemptID == c.AttemptID && bytes.Equal(parent.AttemptBindingSHA256, c.AttemptBindingSHA256) && bytes.Equal(parent.SubmissionSHA256, c.SubmissionSHA256)
}

func v1OriginalParentMatches(c JoinDecisionContext, p *joinDecisionSaved) bool {
	if p == nil {
		return false
	}
	parent := p.basis.Context
	return p.publication == nil && parent.Kind == JoinDecisionSubmitPending && parent.SpaceID == c.SpaceID && parent.AttemptID == c.AttemptID && bytes.Equal(parent.AttemptBindingSHA256, c.AttemptBindingSHA256) && bytes.Equal(parent.SubmissionSHA256, c.SubmissionSHA256)
}

func cloneJoinV1Basis(b JoinDecisionBasis) JoinDecisionBasis {
	c := &b.Context
	c.AttemptBindingSHA256 = slices.Clone(c.AttemptBindingSHA256)
	c.SubmissionSHA256 = slices.Clone(c.SubmissionSHA256)
	c.ReadyEvidenceSHA256 = slices.Clone(c.ReadyEvidenceSHA256)
	c.RequestSHA256 = slices.Clone(c.RequestSHA256)
	c.SubmitBasisSHA256 = slices.Clone(c.SubmitBasisSHA256)
	b.ApplicantObservationSHA256 = slices.Clone(b.ApplicantObservationSHA256)
	b.ReviewerObservationSHA256 = slices.Clone(b.ReviewerObservationSHA256)
	return b
}

func cloneJoinLegacyBasis(b JoinLegacyDecisionBasis) JoinLegacyDecisionBasis {
	c := &b.Context
	c.AttemptBindingSHA256 = slices.Clone(c.AttemptBindingSHA256)
	c.SubmissionSHA256 = slices.Clone(c.SubmissionSHA256)
	c.ReadyEvidenceSHA256 = slices.Clone(c.ReadyEvidenceSHA256)
	c.RequestSHA256 = slices.Clone(c.RequestSHA256)
	c.PendingBasisSHA256 = slices.Clone(c.PendingBasisSHA256)
	b.ApplicantObservationSHA256 = slices.Clone(b.ApplicantObservationSHA256)
	b.ReviewerObservationSHA256 = slices.Clone(b.ReviewerObservationSHA256)
	return b
}

func (s *SpaceStore) VerifyVersionedJoinDecisionBasis(ctx context.Context, in JoinAttemptInput, ref JoinAttemptReference, sha []byte, policyVersion int64, publication uuid.UUID) error {
	if s == nil || s.tx == nil || !validJoinInput(in) || ref.AttemptID == uuid.Nil || !validJoinDecisionDigest(ref.EvidenceSHA256) || policyVersion <= 0 || publication == uuid.Nil {
		return ErrJoinAttemptUnavailable
	}
	if err := lockAndCheckOwnershipScope(ctx, s.tx, []uuid.UUID{in.SpaceID}); err != nil {
		return err
	}
	var version int16
	if err := s.db().QueryRow(ctx, `SELECT basis_version FROM space_join_operations WHERE decision_basis_sha256=$1`, sha).Scan(&version); err != nil {
		return ErrJoinAttemptUnavailable
	}
	if version == 1 {
		return s.VerifyJoinDecisionBasis(ctx, in, ref, sha, policyVersion, publication)
	}
	if version != 2 {
		return ErrJoinAttemptUnavailable
	}
	saved, err := s.loadLegacyJoinDecision(ctx, sha, true)
	if err != nil {
		return err
	}
	c := saved.basis.Context
	if c.Kind == JoinLegacyPending {
		return ErrJoinAttemptUnavailable
	}
	if c.SpaceID != in.SpaceID || c.AttemptID != ref.AttemptID || !bytes.Equal(c.ReadyEvidenceSHA256, ref.EvidenceSHA256) {
		return ErrJoinAttemptConflict
	}
	a, err := scanJoinAttempt(s.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1 FOR UPDATE`, ref.AttemptID))
	if err != nil {
		return ErrJoinAttemptUnavailable
	}
	if policyVersion != a.PolicyVersion || !bytes.Equal(joinDigest("input", a.Input), joinDigest("input", in)) {
		return ErrJoinAttemptConflict
	}
	if !bytes.Equal(a.binding, c.AttemptBindingSHA256) || !bytes.Equal(a.submission, c.SubmissionSHA256) || !bytes.Equal(a.EvidenceSHA256, c.ReadyEvidenceSHA256) {
		return ErrJoinAttemptUnavailable
	}
	policy, err := entrypolicy.Parse(a.policyRaw)
	if err != nil {
		return ErrJoinAttemptUnavailable
	}
	switch c.Kind {
	case JoinLegacyReady:
		if policy.RequireManualApproval || c.ActorAccountID != in.AccountID || c.ActorProfileID != in.ProfileID || a.approvedAccount != nil || a.approvedProfile != nil || a.approvalOperation != nil {
			return ErrJoinAttemptUnavailable
		}
	case JoinApproveLegacyReady:
		if !policy.RequireManualApproval || a.approvedAccount == nil || a.approvedProfile == nil || a.approvalOperation == nil || *a.approvedAccount != c.ActorAccountID || *a.approvedProfile != c.ActorProfileID || *a.approvalOperation != c.OperationID {
			return ErrJoinAttemptUnavailable
		}
		parent, err := s.loadLegacyJoinDecision(ctx, c.PendingBasisSHA256, false)
		if err != nil || !legacyOriginalParentMatches(c, parent) || parent.basis.Context.ActorAccountID != in.AccountID || parent.basis.Context.ActorProfileID != in.ProfileID {
			return ErrJoinAttemptUnavailable
		}
	default:
		return ErrJoinAttemptUnavailable
	}
	if saved.publication != nil {
		if *saved.publication != publication {
			return ErrJoinAttemptConflict
		}
		if saved.publicationState == nil || *saved.publicationState != "PREPARING" {
			return ErrJoinAttemptUnavailable
		}
	}
	if err = s.VerifyJoinAttemptAdmission(ctx, in, ref, policyVersion, publication); err != nil {
		return err
	}
	if saved.publication != nil {
		return nil
	}
	tag, err := s.db().Exec(ctx, `UPDATE space_join_operations SET publication_operation_id=$2,publication_state='PREPARING' WHERE decision_basis_sha256=$1 AND basis_version=2 AND publication_operation_id IS NULL`, sha, publication)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrJoinAttemptConflict
	}
	return nil
}

func (s *SpaceStore) MarkVersionedJoinDecisionBasisPublished(ctx context.Context, ref JoinAttemptReference, sha []byte, publication uuid.UUID) error {
	if s == nil || s.tx == nil || ref.AttemptID == uuid.Nil || !validJoinDecisionDigest(ref.EvidenceSHA256) || !validJoinDecisionDigest(sha) || publication == uuid.Nil {
		return ErrJoinAttemptUnavailable
	}
	var version int16
	if err := s.db().QueryRow(ctx, `SELECT basis_version FROM space_join_operations WHERE decision_basis_sha256=$1`, sha).Scan(&version); err != nil {
		return ErrJoinAttemptUnavailable
	}
	if version == 1 {
		return s.MarkJoinDecisionBasisPublished(ctx, ref, sha, publication)
	}
	if version != 2 {
		return ErrJoinAttemptUnavailable
	}
	saved, err := s.loadLegacyJoinDecision(ctx, sha, false)
	if err != nil {
		return err
	}
	if err = lockAndCheckOwnershipScope(ctx, s.tx, []uuid.UUID{saved.basis.Context.SpaceID}); err != nil {
		return err
	}
	saved, err = s.loadLegacyJoinDecision(ctx, sha, true)
	if err != nil {
		return err
	}
	c := saved.basis.Context
	if c.AttemptID != ref.AttemptID || !bytes.Equal(c.ReadyEvidenceSHA256, ref.EvidenceSHA256) || saved.publication == nil || *saved.publication != publication || saved.publicationState == nil || *saved.publicationState != "PREPARING" {
		return ErrJoinAttemptUnavailable
	}
	tag, err := s.db().Exec(ctx, `UPDATE space_join_operations SET publication_state='PUBLISHED' WHERE decision_basis_sha256=$1 AND basis_version=2 AND publication_operation_id=$2 AND publication_state='PREPARING' AND EXISTS(SELECT 1 FROM space_join_attempts WHERE id=$3 AND state='PUBLISHED' AND publication_operation_id=$2 AND evidence_sha256=$4)`, sha, publication, ref.AttemptID, ref.EvidenceSHA256)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrJoinAttemptUnavailable
	}
	return nil
}
