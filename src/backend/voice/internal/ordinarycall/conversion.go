package ordinarycall

import (
	"context"
)

// QuiesceConversionSource consumes an immutable, trusted reservation snapshot.
// It runs only AFTER the conversion reservation commits and never releases a
// tuple-only fence or routes a native source through the legacy Redis writer.
func (c *Coordinator) QuiesceConversionSource(ctx context.Context, b MediaBinding) error {
	if !c.ready() {
		return ErrUnavailable
	}
	_, err := c.Journal.Mutate(ctx, b.RoomID, func(r *Room) error {
		if err := conversionBindingMatches(r, b); err != nil {
			return err
		}
		m := r.Members[b.ProfileID.String()]
		switch {
		case r.Stage == StagePreparing:
			return r.AbortAdmission(r.Intent.OperationID, c.at(r))
		case r.Stage == StageClosing || r.Stage == StageAborting || r.Stage == StageClosed:
			return nil
		case m.State == MemberPending:
			return r.AbortAdmission(m.FenceOperationID, c.at(r))
		case m.State == MemberLeft || m.State == MemberAborted || m.State == MemberLeaving:
			return nil
		case r.Intent.Group:
			return r.DecideLeave(b.ProfileID, c.at(r))
		default:
			return r.DecideEnd(b.ProfileID, "sdk_conversion", c.at(r))
		}
	})
	if err != nil {
		return err
	}
	if err := c.Reconcile(ctx, b.RoomID); err != nil {
		return err
	}
	r, err := c.Journal.Load(ctx, b.RoomID)
	if err != nil {
		return err
	}
	if err := conversionBindingMatches(r, b); err != nil {
		return err
	}
	m := r.Members[b.ProfileID.String()]
	if r.HasPendingClose() || m.State != MemberLeft && m.State != MemberAborted {
		return ErrCleanupPending
	}
	return nil
}

func conversionBindingMatches(r *Room, b MediaBinding) error {
	if r == nil || r.Intent.RoomID != b.RoomID || r.Intent.OperationID != b.RoomOperationID {
		return ErrStaleGeneration
	}
	m, ok := r.Members[b.ProfileID.String()]
	if !ok || m.AccountID != b.AccountID || m.ProfileID != b.ProfileID || m.Generation != b.Generation ||
		m.FenceOperationID != b.FenceOperationID || m.MediaIdentity != b.MediaIdentity {
		return ErrStaleGeneration
	}
	return nil
}
