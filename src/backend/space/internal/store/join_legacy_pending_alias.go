package store

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"slices"
	"voice/backend/space/internal/admissionauthority"
)

// CompleteLegacyJoinPending aliases only the exact no-input pending winner.
// The fresh private START proof authenticates this new legacy call, but is NOT
// saved as another submission basis. Referenced observations/P remain original.
func (s *SpaceStore) CompleteLegacyJoinPending(ctx context.Context, d *VerifiedLegacyJoinStart) (*JoinSubmissionRecord, error) {
	if d == nil || d.proposal == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	return s.completeLegacyJoinPending(ctx, d.proposal, d.check, nil)
}

func (s *SpaceStore) completeLegacyJoinPending(ctx context.Context, p *JoinLegacyStartProposal, check func() (admissionauthority.SubjectIdentity, error), resume *JoinLegacyPendingResumeProposal) (*JoinSubmissionRecord, error) {
	if ctx == nil || s == nil || s.Pool == nil || s.tx != nil || p == nil || p.call == nil || p.selection == nil || check == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if _, err := p.call.ActorFor(p.request); err != nil {
		return nil, err
	}
	c := p.selection.Context
	in := c.Operation
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	if err = lockJoinPublicOperation(ctx, tx, in); err != nil {
		return nil, err
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{in.SpaceID}); err != nil {
		return nil, err
	}
	local := *s
	local.tx = tx
	saved, err := local.loadLegacyJoinSelection(ctx, in, c.Origin)
	if err != nil {
		return nil, err
	}
	actual, err := saved.Context.SHA256()
	if err != nil {
		return nil, err
	}
	expected, err := c.SHA256()
	if err != nil || actual != expected {
		return nil, ErrJoinAttemptConflict
	}
	if saved.Operation.State == "COMPLETED" {
		var code int32
		var raw, observed []byte
		if err = tx.QueryRow(ctx, `SELECT outcome_code,outcome_proto,observed_basis_sha256 FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, in.ActorProfileID, in.OperationID).Scan(&code, &raw, &observed); err != nil {
			return nil, err
		}
		if code != 9 || raw == nil || len(raw) != 0 || !validJoinDecisionDigest(observed) || saved.Operation.AttemptID == nil {
			return nil, ErrJoinAttemptUnavailable
		}
		if _, err = p.call.ActorFor(p.request); err != nil {
			return nil, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, ErrJoinAttemptPending
	}
	if saved.Operation.State != "RESERVED" || saved.Operation.AttemptID == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	a, err := scanJoinAttempt(tx.QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1 FOR UPDATE`, *saved.Operation.AttemptID))
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if a.Input.SpaceID != in.SpaceID || a.Input.AccountID != in.ActorAccountID || a.Input.ProfileID != in.ActorProfileID || !sameOptionalJoinAttempt(a.Input.InviteID, c.InviteID) {
		return nil, ErrJoinAttemptConflict
	}
	if a.State != "PENDING" || a.approvedAccount != nil || a.approvedProfile != nil || a.approvalOperation != nil || a.publication != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if resume != nil && (resume.selectionSHA != actual || resume.attempt != a.ID || !bytes.Equal(resume.binding, a.binding) || !bytes.Equal(joinBinding(a.Input, a.PolicyVersion, a.policyDigest), a.binding)) {
		return nil, ErrJoinAttemptUnavailable
	}
	identity, err := check()
	if err != nil {
		return nil, err
	}
	effective := a.Input
	effective.Guest = effective.Guest || identity.AccountType == "guest"
	if err = local.localJoinAttemptGates(ctx, effective); err != nil {
		return nil, err
	}
	policy, err := local.GetEntryPolicy(ctx, in.SpaceID)
	if err != nil {
		return nil, err
	}
	if policy.Version != a.PolicyVersion || policy.Version != c.PolicyVersion || !bytes.Equal(joinDigest("policy", policy.Policy), a.policyDigest) || !bytes.Equal(a.policyDigest, c.PolicySHA256) {
		return nil, ErrJoinAttemptDenied
	}
	if policy.Policy.RequirePhone {
		if effective.Guest {
			return nil, ErrJoinAttemptDenied
		}
		return nil, ErrJoinAttemptUnavailable
	}
	if policy.Policy.RequireCaptcha || len(policy.Policy.Questions) != 0 {
		return nil, ErrLegacyJoinInputRequired
	}
	if !policy.Policy.RequireManualApproval {
		return nil, ErrJoinAttemptUnavailable
	}
	if err = a.validateAutomaticEvidence(policy.Policy); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT decision_basis_sha256,basis_version FROM space_join_operations WHERE selected_attempt_id=$1 AND ((basis_version=1 AND decision_basis->'context'->>'kind'='1') OR (basis_version=2 AND decision_basis->'context'->>'kind'='4')) LIMIT 2`, a.ID)
	if err != nil {
		return nil, err
	}
	var original []byte
	var version int16
	count := 0
	for rows.Next() {
		count++
		if err = rows.Scan(&original, &version); err != nil {
			rows.Close()
			return nil, err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, ErrJoinAttemptUnavailable
	}
	if version == 1 {
		winner, err := local.loadJoinDecision(ctx, original, false)
		if err != nil {
			return nil, err
		}
		w := winner.basis.Context
		if winner.publication != nil || w.Kind != JoinDecisionSubmitPending || w.SpaceID != in.SpaceID || w.AttemptID != a.ID || w.ActorAccountID != a.Input.AccountID || w.ActorProfileID != a.Input.ProfileID || !bytes.Equal(w.AttemptBindingSHA256, a.binding) || !bytes.Equal(w.SubmissionSHA256, a.submission) {
			return nil, ErrJoinAttemptUnavailable
		}
	} else if version == 2 {
		winner, err := local.loadLegacyJoinDecision(ctx, original, false)
		if err != nil {
			return nil, err
		}
		w := winner.basis.Context
		if winner.publication != nil || w.Kind != JoinLegacyPending || w.SpaceID != in.SpaceID || w.AttemptID != a.ID || w.ActorAccountID != a.Input.AccountID || w.ActorProfileID != a.Input.ProfileID || !bytes.Equal(w.AttemptBindingSHA256, a.binding) || !bytes.Equal(w.SubmissionSHA256, a.submission) {
			return nil, ErrJoinAttemptUnavailable
		}
	} else {
		return nil, ErrJoinAttemptUnavailable
	}
	group, err := local.LookupJoinRequestGroupForAttempt(ctx, in.SpaceID, a.ID)
	if err != nil || group == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	tag, err := tx.Exec(ctx, `UPDATE space_join_operations SET state='COMPLETED',observed_basis_sha256=$3,outcome_code=9,outcome_proto=$4 WHERE actor_profile_id=$1 AND operation_id=$2 AND state='RESERVED' AND selected_attempt_id=$5 AND decision_basis IS NULL AND observed_basis_sha256 IS NULL AND legacy_selection_sha256=$6`, in.ActorProfileID, in.OperationID, original, []byte{}, a.ID, expected[:])
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, ErrJoinAttemptConflict
	}
	if _, err = check(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	// No collection deadline check: a successful pending review has no age TTL.
	id := group.ID
	return &JoinSubmissionRecord{Attempt: a.JoinAttempt, DecisionBasisSHA256: slices.Clone(original), RequestID: &id}, nil
}
