package store

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/google/uuid"
	"voice/backend/space/internal/entrypolicy"
)

type joinDecisionSaved struct {
	basis            JoinDecisionBasis
	operation        JoinOperation
	publication      *uuid.UUID
	publicationState *string
}

func (s *SpaceStore) loadJoinDecision(ctx context.Context, sha []byte, lock bool) (*joinDecisionSaved, error) {
	if s == nil || (s.Pool == nil && s.tx == nil) || !validJoinDecisionDigest(sha) || (lock && s.tx == nil) {
		return nil, ErrJoinAttemptUnavailable
	}
	query := `SELECT ` + joinOperationColumns + `,decision_basis,publication_operation_id,publication_state FROM space_join_operations WHERE decision_basis_sha256=$1`
	if lock {
		query += ` FOR UPDATE`
	}
	var saved joinDecisionSaved
	var raw []byte
	op := &saved.operation
	err := s.db().QueryRow(ctx, query, sha).Scan(&op.Input.SpaceID, &op.Input.ActorAccountID, &op.Input.ActorProfileID, &op.Input.OperationID, &op.Input.Method, &op.Input.RequestSHA256, &op.AttemptID, &op.State, &raw, &saved.publication, &saved.publicationState)
	if err != nil || op.AttemptID == nil || op.State == "RESERVED" || len(raw) > 4096 {
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
	if *op.AttemptID != c.AttemptID || !bytes.Equal(joinDigest("public-operation", expected), joinDigest("public-operation", op.Input)) {
		return nil, ErrJoinAttemptUnavailable
	}
	return &saved, nil
}

// LoadJoinDecisionBasis reads immutable historical data, never a current allow.
func (s *SpaceStore) LoadJoinDecisionBasis(ctx context.Context, sha []byte) (*JoinDecisionBasis, error) {
	saved, err := s.loadJoinDecision(ctx, sha, false)
	if err != nil {
		return nil, err
	}
	return &saved.basis, nil
}

// VerifyJoinDecisionBasis is scoped to the native Prepare/FIRST Publish TX.
// It must verify both journals and pin their exact immutable decision to P.
func (s *SpaceStore) VerifyJoinDecisionBasis(ctx context.Context, in JoinAttemptInput, ref JoinAttemptReference, sha []byte, version int64, publication uuid.UUID) error {
	if s == nil || s.tx == nil || !validJoinInput(in) || ref.AttemptID == uuid.Nil || !validJoinDecisionDigest(ref.EvidenceSHA256) || publication == uuid.Nil || version <= 0 {
		return ErrJoinAttemptUnavailable
	}
	if err := lockAndCheckOwnershipScope(ctx, s.tx, []uuid.UUID{in.SpaceID}); err != nil {
		return err
	}
	saved, err := s.loadJoinDecision(ctx, sha, true)
	if err != nil {
		return err
	}
	c := saved.basis.Context
	if c.Kind == JoinDecisionSubmitPending {
		return ErrJoinAttemptUnavailable
	}
	if c.AttemptID != ref.AttemptID || c.SpaceID != in.SpaceID || !bytes.Equal(c.ReadyEvidenceSHA256, ref.EvidenceSHA256) {
		return ErrJoinAttemptConflict
	}
	a, err := scanJoinAttempt(s.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1 FOR UPDATE`, ref.AttemptID))
	if err != nil {
		return ErrJoinAttemptUnavailable
	}
	if !bytes.Equal(joinDigest("input", a.Input), joinDigest("input", in)) || version != a.PolicyVersion {
		return ErrJoinAttemptConflict
	}
	if !bytes.Equal(c.AttemptBindingSHA256, a.binding) || !bytes.Equal(c.SubmissionSHA256, a.submission) || !bytes.Equal(c.ReadyEvidenceSHA256, a.EvidenceSHA256) {
		return ErrJoinAttemptUnavailable
	}
	policy, err := entrypolicy.Parse(a.policyRaw)
	if err != nil {
		return ErrJoinAttemptUnavailable
	}
	switch c.Kind {
	case JoinDecisionSubmitReady:
		if policy.RequireManualApproval || c.ActorAccountID != in.AccountID || c.ActorProfileID != in.ProfileID || a.approvedAccount != nil || a.approvedProfile != nil || a.approvalOperation != nil {
			return ErrJoinAttemptUnavailable
		}
	case JoinDecisionApproveReady:
		if !policy.RequireManualApproval || a.approvedAccount == nil || a.approvedProfile == nil || a.approvalOperation == nil || *a.approvedAccount != c.ActorAccountID || *a.approvedProfile != c.ActorProfileID || *a.approvalOperation != c.OperationID {
			return ErrJoinAttemptUnavailable
		}
		parent, err := s.loadJoinDecision(ctx, c.SubmitBasisSHA256, false)
		if err != nil {
			return err
		}
		p := parent.basis.Context
		if p.Kind != JoinDecisionSubmitPending || parent.publication != nil || p.SpaceID != c.SpaceID || p.AttemptID != c.AttemptID || p.ActorAccountID != in.AccountID || p.ActorProfileID != in.ProfileID || !bytes.Equal(p.AttemptBindingSHA256, c.AttemptBindingSHA256) || !bytes.Equal(p.SubmissionSHA256, c.SubmissionSHA256) {
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
	if err = s.VerifyJoinAttemptAdmission(ctx, in, ref, version, publication); err != nil {
		return err
	}
	if saved.publication != nil {
		return nil
	}
	result, err := s.db().Exec(ctx, `UPDATE space_join_operations SET publication_operation_id=$2,publication_state='PREPARING' WHERE decision_basis_sha256=$1 AND publication_operation_id IS NULL`, sha, publication)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrJoinAttemptConflict
	}
	return nil
}

// MarkJoinDecisionBasisPublished is first-publication bookkeeping only. Public
// responses require the actual saved native outcome, not this status alone.
func (s *SpaceStore) MarkJoinDecisionBasisPublished(ctx context.Context, ref JoinAttemptReference, sha []byte, publication uuid.UUID) error {
	if s == nil || s.tx == nil || ref.AttemptID == uuid.Nil || !validJoinDecisionDigest(ref.EvidenceSHA256) || publication == uuid.Nil {
		return ErrJoinAttemptUnavailable
	}
	saved, err := s.loadJoinDecision(ctx, sha, false)
	if err != nil {
		return err
	}
	if err = lockAndCheckOwnershipScope(ctx, s.tx, []uuid.UUID{saved.basis.Context.SpaceID}); err != nil {
		return err
	}
	saved, err = s.loadJoinDecision(ctx, sha, true)
	if err != nil {
		return err
	}
	if saved.basis.Context.AttemptID != ref.AttemptID || !bytes.Equal(saved.basis.Context.ReadyEvidenceSHA256, ref.EvidenceSHA256) || saved.publication == nil || *saved.publication != publication || saved.publicationState == nil || *saved.publicationState != "PREPARING" {
		return ErrJoinAttemptUnavailable
	}
	result, err := s.db().Exec(ctx, `UPDATE space_join_operations SET publication_state='PUBLISHED' WHERE decision_basis_sha256=$1 AND publication_operation_id=$2 AND publication_state='PREPARING' AND EXISTS(SELECT 1 FROM space_join_attempts WHERE id=$3 AND state='PUBLISHED' AND publication_operation_id=$2 AND evidence_sha256=$4)`, sha, publication, ref.AttemptID, ref.EvidenceSHA256)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrJoinAttemptUnavailable
	}
	return nil
}
