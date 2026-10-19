package control

import "context"

// Authority is the protected durable native journal, not request-supplied
// certificates. Implementations must serialize owner allocation and close with
// native resource lifecycle, preserve unknown launches/attempts indefinitely,
// and resolve actual supervisor receipts again at the durable commit boundary.
type Authority interface {
	ReadCloseIntent(context.Context, Ref) (CloseIntent, error)
	RecordWorkerFence(context.Context, Ref, Ref, Ref) error
	ConfirmClose(context.Context, Ref) (Ref, error)
	ReadCloseAcknowledgement(context.Context, Ref) (CloseAcknowledgement, error)
}

// Supervisor owns the actual sink enforcement. FenceWorker first durably seals
// all starts/enables for that launch, closes mandatory forwarding and its queues,
// kills/verifies the exact sandbox/descendants/sockets, and archives the receipt.
// A lost reply is retried with the same refs. No worker can call this interface.
// A local-lock/no-op/timeout-only implementation does not satisfy this contract.
// InspectFence rederives the exhaustive durable birth census and seal barrier,
// resolves profile/host witnesses, and reobserves actual process and forwarding
// enforcement on EVERY read. JSON/hash/name alone cannot supply these facts.
// Missing capability, retained ambiguity or unverifiable scope is an error.
type Supervisor interface {
	FenceWorker(context.Context, Ref, Ref) (Ref, error)
	InspectFence(context.Context, Ref) (FenceReceipt, error)
}

type Controller struct {
	authority   Authority
	supervisors map[ID]Supervisor
}

func New(authority Authority, supervisors map[ID]Supervisor) (*Controller, error) {
	if authority == nil || len(supervisors) == 0 {
		return nil, ErrUnavailable
	}
	copyMap := make(map[ID]Supervisor, len(supervisors))
	for id, s := range supervisors {
		if !id.Valid() || s == nil {
			return nil, ErrUnavailable
		}
		copyMap[id] = s
	}
	return &Controller{authority, copyMap}, nil
}

// Close consumes a reference only. There is no caller-owned runtime list,
// replacement owner, physical JSON, boolean fence or permissive default.
// No native transaction is held across external enforcement. Every worker
// receipt is archived before aggregate confirmation and exact replay is durable.
func (c *Controller) Close(ctx context.Context, ref Ref) (CloseAcknowledgement, error) {
	var zero CloseAcknowledgement
	if c == nil || c.authority == nil {
		return zero, ErrUnavailable
	}
	intent, err := c.authority.ReadCloseIntent(ctx, ref)
	if err != nil {
		return zero, err
	}
	if intent.Ref != ref {
		return zero, ErrConflict
	}
	if err = intent.Validate(); err != nil {
		return zero, err
	}
	fences := make([]Ref, 0, len(intent.Owners))
	for _, owner := range intent.Owners {
		s := c.supervisors[owner.Supervisor]
		if s == nil {
			return zero, ErrUnavailable
		}
		receiptRef, err := s.FenceWorker(ctx, intent.Ref, owner.Ref)
		if err != nil {
			return zero, err
		}
		receipt, err := s.InspectFence(ctx, receiptRef)
		if err != nil {
			return zero, err
		}
		if receipt.Ref != receiptRef {
			return zero, ErrConflict
		}
		if err = receipt.Validate(intent, owner); err != nil {
			return zero, err
		}
		if err = c.authority.RecordWorkerFence(ctx, intent.Ref, owner.Ref, receiptRef); err != nil {
			return zero, err
		}
		fences = append(fences, receiptRef)
	}
	ackRef, err := c.authority.ConfirmClose(ctx, intent.Ref)
	if err != nil {
		return zero, err
	}
	ack, err := c.authority.ReadCloseAcknowledgement(ctx, ackRef)
	if err != nil {
		return zero, err
	}
	if !ackRef.Valid() || ack.Ref != ackRef || ackRef.Issuer != intent.Authority || ack.Close != intent.Ref || ack.Resource != intent.Resource || ack.SealingRevision != intent.SealingRevision || ack.OwnerCensusHash != intent.OwnerCensusHash || !ack.Absence.Valid() || ack.Absence.Issuer != intent.Authority || len(ack.WorkerFences) != len(fences) {
		return zero, ErrConflict
	}
	for i, ref := range fences {
		if ack.WorkerFences[i] != ref {
			return zero, ErrConflict
		}
	}
	return ack, nil
}
