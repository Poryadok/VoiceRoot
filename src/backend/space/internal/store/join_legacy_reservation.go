package store

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"slices"
)

type JoinLegacySelectedAttemptView struct {
	AttemptID uuid.UUID
	State     string
}

// ReadLegacyJoinSelectedAttempt is routing data only. The actual decision or
// resume factory still reloads and validates state before any owner execution.
func (s *SpaceStore) ReadLegacyJoinSelectedAttempt(ctx context.Context, p *JoinLegacyStartProposal) (*JoinLegacySelectedAttemptView, error) {
	if ctx == nil || s == nil || s.Pool == nil || s.tx != nil || p == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if _, err := p.Reservation(); err != nil {
		return nil, err
	}
	c := p.selection.Context
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	local := *s
	local.tx = tx
	saved, err := local.loadLegacyJoinSelection(ctx, c.Operation, c.Origin)
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
	a, err := scanJoinAttempt(tx.QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1`, *saved.Operation.AttemptID))
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	in := c.Operation
	if a.Input.SpaceID != in.SpaceID || a.Input.AccountID != in.ActorAccountID || a.Input.ProfileID != in.ActorProfileID || !sameOptionalJoinAttempt(a.Input.InviteID, c.InviteID) {
		return nil, ErrJoinAttemptConflict
	}
	if a.PolicyVersion != c.PolicyVersion || !bytes.Equal(a.policyDigest, c.PolicySHA256) || !bytes.Equal(joinBinding(a.Input, a.PolicyVersion, a.policyDigest), a.binding) || a.approvedAccount != nil || a.approvedProfile != nil || a.approvalOperation != nil || a.publication != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if (a.State != "COLLECTING" && a.State != "PENDING") || (a.State == "COLLECTING" && a.submission != nil) || (a.State == "PENDING" && !validJoinDecisionDigest(a.submission)) {
		return nil, ErrJoinAttemptUnavailable
	}
	if _, err = p.call.ActorFor(p.request); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &JoinLegacySelectedAttemptView{AttemptID: a.ID, State: a.State}, nil
}

// Reservation returns a copy of the constructor's immutable routing snapshot.
// It is not a current state read or admission/eligibility capability. Every
// subsequent writer rereads its exact operation under the established locks.
func (p *JoinLegacyStartProposal) Reservation() (JoinOperation, error) {
	if p == nil || p.call == nil || p.request == nil || p.selection == nil {
		return JoinOperation{}, ErrJoinAttemptUnavailable
	}
	actor, err := p.call.ActorFor(p.request)
	op := p.selection.Operation
	if err != nil {
		return JoinOperation{}, err
	}
	if actor.AccountID != op.Input.ActorAccountID.String() || actor.ProfileID != op.Input.ActorProfileID.String() || actor.RPC != op.Input.Method {
		return JoinOperation{}, ErrJoinAttemptUnavailable
	}
	op.Input.RequestSHA256 = slices.Clone(op.Input.RequestSHA256)
	if op.AttemptID != nil {
		id := *op.AttemptID
		op.AttemptID = &id
	}
	return op, nil
}
