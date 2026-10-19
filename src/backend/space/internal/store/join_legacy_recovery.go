package store

import (
	"bytes"
	"context"
	"slices"

	"github.com/jackc/pgx/v5"
)

// RecoverLegacyJoinDecision returns only original immutable coordinator input.
// It is not current membership, eligibility or permission authority.
func (s *SpaceStore) RecoverLegacyJoinDecision(ctx context.Context, p *JoinLegacyStartProposal) (*JoinSubmissionRecovery, error) {
	if ctx == nil || s == nil || s.Pool == nil || s.tx != nil || p == nil || p.call == nil || p.selection == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if _, err := p.call.ActorFor(p.request); err != nil {
		return nil, err
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	local := *s
	local.tx = tx
	op := p.selection.Context.Operation
	saved, err := local.loadLegacyJoinSelection(ctx, op, p.selection.Context.Origin)
	if err != nil {
		return nil, err
	}
	actual, err := saved.Context.SHA256()
	expected, expectedErr := p.selection.Context.SHA256()
	if err != nil || expectedErr != nil || actual != expected || saved.Operation.State == "RESERVED" || saved.Operation.AttemptID == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	var sha []byte
	if err = tx.QueryRow(ctx, `SELECT decision_basis_sha256 FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID).Scan(&sha); err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	history, err := local.loadLegacyJoinDecision(ctx, sha, false)
	if err != nil {
		return nil, err
	}
	c := history.basis.Context
	if c.Kind != JoinLegacyReady || c.OperationOrigin != saved.Context.Origin || c.SpaceID != op.SpaceID || c.AttemptID != *saved.Operation.AttemptID || c.ActorAccountID != op.ActorAccountID || c.ActorProfileID != op.ActorProfileID || c.OperationID != op.OperationID || c.RequestMethod != op.Method || !bytes.Equal(c.RequestSHA256, op.RequestSHA256) {
		return nil, ErrJoinAttemptUnavailable
	}
	in, err := local.loadReadyLegacyJoinDecisionInput(ctx, sha)
	if err != nil {
		return nil, err
	}
	// The immutable preselection and original A must still describe the same
	// exact InviteID and policy. Current policy is deliberately not consulted.
	a, err := scanJoinAttempt(tx.QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1`, c.AttemptID))
	if err != nil || a.PolicyVersion != saved.Context.PolicyVersion || !bytes.Equal(a.policyDigest, saved.Context.PolicySHA256) || !sameOptionalJoinAttempt(a.Input.InviteID, saved.Context.InviteID) {
		return nil, ErrJoinAttemptUnavailable
	}
	result := &JoinSubmissionRecovery{AttemptID: c.AttemptID, DecisionKind: c.Kind, DecisionBasisSHA256: slices.Clone(sha)}
	if history.publication != nil {
		if in.OperationID != *history.publication {
			return nil, ErrJoinAttemptUnavailable
		}
		publication := *history.publication
		result.PublicationOperationID = &publication
	}
	if _, err = p.call.ActorFor(p.request); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}
