package store

import "slices"

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
