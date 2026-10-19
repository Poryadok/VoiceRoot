package ordinarycall

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Journal interface {
	Create(context.Context, *Room) error
	Load(context.Context, uuid.UUID) (*Room, error)
	Mutate(context.Context, uuid.UUID, func(*Room) error) (*Room, error)
	MarkProjected(context.Context, uuid.UUID, uint64) error
}

type Media interface {
	EnsureRoom(context.Context, string) error
	CloseRoom(context.Context, string) error
	RemoveParticipant(context.Context, string, string) error
	ListParticipantIdentities(context.Context, string) ([]string, error)
}

// Projection must apply the exact journal version monotonically and preserve
// ownership when a newer call already owns the profile/chat projection indexes.
type Projection interface {
	Apply(context.Context, *Room) error
}

// Coordinator performs network work only outside native journal transactions.
// Authorization and post-reservation checks belong to the protected handler;
// they are passed as a bounded check, never run inside a Mutate callback.
type Coordinator struct {
	Journal       Journal
	Media         Media
	Projection    Projection
	EffectTimeout time.Duration
	Now           func() time.Time
}

func (c *Coordinator) ready() bool {
	return c != nil && c.Journal != nil && c.Media != nil && c.Projection != nil
}
func (c *Coordinator) now() time.Time {
	if c.Now != nil {
		return c.Now().UTC()
	}
	return time.Now().UTC()
}
func (c *Coordinator) timeout() time.Duration {
	if c.EffectTimeout > 0 && c.EffectTimeout <= 30*time.Second {
		return c.EffectTimeout
	}
	return 5 * time.Second
}

func (c *Coordinator) at(r *Room) time.Time {
	if !r.TransactionTime.IsZero() {
		return r.TransactionTime
	}
	return c.now()
}

func (c *Coordinator) Start(ctx context.Context, intent CreateIntent, check func(context.Context) error) (*Room, error) {
	if !c.ready() {
		return nil, ErrUnavailable
	}
	r, err := NewPendingRoom(intent, c.now())
	if err != nil {
		return nil, err
	}
	if err = c.Journal.Create(ctx, r); err != nil {
		return nil, err
	}
	effectCtx, cancel := context.WithTimeout(ctx, c.timeout())
	err = c.Media.EnsureRoom(effectCtx, r.LiveKitRoomName)
	cancel()
	if err == nil {
		err = ctx.Err()
	}
	if err == nil && check != nil {
		err = check(ctx)
	}
	if err != nil {
		return nil, errors.Join(err, c.abortAndRepair(r.Intent.RoomID, intent.OperationID))
	}
	r, err = c.Journal.Mutate(ctx, intent.RoomID, func(room *Room) error {
		if err := room.RecordEnsureResult(intent.OperationID, true, c.at(room)); err != nil {
			return err
		}
		return room.CommitAdmission(intent.OperationID, c.at(room))
	})
	if err != nil {
		// Abort is a journal CAS: if COMMIT won despite an unknown response,
		// AbortAdmission is a no-op and cannot compensate the accepted call.
		return nil, errors.Join(err, c.abortAndRepair(intent.RoomID, intent.OperationID))
	}
	if err = c.project(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// Admit executes a group Join or DM Accept using the same pending/finalize seam.
// begin must be a pure model mutation which captures the exact generations.
func (c *Coordinator) Admit(ctx context.Context, roomID, operation uuid.UUID, begin func(*Room) error, check func(context.Context) error) (*Room, error) {
	if !c.ready() {
		return nil, ErrUnavailable
	}
	if _, err := c.Journal.Mutate(ctx, roomID, begin); err != nil {
		return nil, err
	}
	err := ctx.Err()
	if err == nil && check != nil {
		err = check(ctx)
	}
	if err != nil {
		return nil, errors.Join(err, c.abortAndRepair(roomID, operation))
	}
	r, err := c.Journal.Mutate(ctx, roomID, func(r *Room) error { return r.CommitAdmission(operation, c.at(r)) })
	if err != nil {
		return nil, errors.Join(err, c.abortAndRepair(roomID, operation))
	}
	if err = c.project(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

func (c *Coordinator) abortAndRepair(roomID, operation uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*c.timeout())
	defer cancel()
	if _, err := c.Journal.Mutate(ctx, roomID, func(r *Room) error { return r.AbortAdmission(operation, c.at(r)) }); err != nil {
		return err
	}
	return c.Reconcile(ctx, roomID)
}

// Change commits a terminal/leave decision before any SDK side effect. A later
// failure is recoverable cleanup, never permission to restore membership.
func (c *Coordinator) Change(ctx context.Context, roomID uuid.UUID, change func(*Room) error) (*Room, error) {
	if !c.ready() {
		return nil, ErrUnavailable
	}
	if _, err := c.Journal.Mutate(ctx, roomID, change); err != nil {
		return nil, err
	}
	if err := c.Reconcile(ctx, roomID); err != nil {
		return nil, err
	}
	return c.Journal.Load(ctx, roomID)
}

func (c *Coordinator) project(ctx context.Context, r *Room) error {
	if err := c.Projection.Apply(ctx, r); err != nil {
		return err
	}
	return c.Journal.MarkProjected(ctx, r.Intent.RoomID, r.Version)
}

// Reconcile repairs an interrupted operation. Preparing calls are never resumed
// to READY after a process restart; they compensate their immutable SDK name.
// Workers invoke this with a fresh bounded context and persisted scheduling lease.
func (c *Coordinator) Reconcile(ctx context.Context, roomID uuid.UUID) error {
	if !c.ready() {
		return ErrUnavailable
	}
	r, err := c.Journal.Load(ctx, roomID)
	if err != nil {
		return err
	}
	if r.Stage == StagePreparing {
		r, err = c.Journal.Mutate(ctx, roomID, func(r *Room) error {
			if r.Stage != StagePreparing {
				return nil
			}
			return r.AbortAdmission(r.Intent.OperationID, c.at(r))
		})
		if err != nil {
			return err
		}
	}
	if r.Stage == StageRinging && !c.at(r).Before(r.ExpiresAt) {
		r, err = c.Journal.Mutate(ctx, roomID, func(r *Room) error {
			if r.Stage != StageRinging || c.at(r).Before(r.ExpiresAt) {
				return nil
			}
			return r.DecideTimeout(c.at(r))
		})
		if err != nil {
			return err
		}
	}
	// A crashed Join/Accept cannot retain pending capacity/account ownership
	// forever. The same bound as unfinished Start permits in-flight checks to
	// finish; the terminal/committed CAS still wins if it completed meanwhile.
	for _, admission := range r.Admissions {
		if admission.State != AdmissionPending || admission.Kind == AdmissionStart ||
			c.at(r).Before(admission.CreatedAt.Add(30*time.Second)) {
			continue
		}
		r, err = c.Journal.Mutate(ctx, roomID, func(r *Room) error {
			current, exists := r.Admissions[admission.ID.String()]
			if !exists || current.State != AdmissionPending || c.at(r).Before(current.CreatedAt.Add(30*time.Second)) {
				return nil
			}
			return r.AbortAdmission(admission.ID, c.at(r))
		})
		if err != nil {
			return err
		}
	}
	if closeEffect, ok := r.PendingClose(); ok {
		effectCtx, cancel := context.WithTimeout(ctx, c.timeout())
		err = c.Media.CloseRoom(effectCtx, r.LiveKitRoomName)
		cancel()
		if err != nil {
			return err
		}
		r, err = c.Journal.Mutate(ctx, roomID, func(r *Room) error { return r.CompleteRoomCleanup(closeEffect.ID, closeEffect.Revision, c.at(r)) })
		if err != nil {
			return err
		}
	} else if r.Stage == StageClosed {
		// Permanent tombstones continue reconciling exact retired names: an
		// old bearer or delayed SDK create may reappear after confirmed deletion.
		effectCtx, cancel := context.WithTimeout(ctx, c.timeout())
		err = c.Media.CloseRoom(effectCtx, r.LiveKitRoomName)
		cancel()
		if err != nil {
			return err
		}
	} else if r.Stage == StageActive {
		for _, effect := range r.Effects {
			if effect.Applied || effect.Kind != EffectRemoveMember {
				continue
			}
			effectCtx, cancel := context.WithTimeout(ctx, c.timeout())
			err = c.Media.RemoveParticipant(effectCtx, r.LiveKitRoomName, effect.Participant)
			cancel()
			if err != nil {
				return err
			}
			r, err = c.Journal.Mutate(ctx, roomID, func(r *Room) error { return r.CompleteMemberCleanup(effect.ProfileID, effect.Generation, c.at(r)) })
			if err != nil {
				return err
			}
		}
	}
	if r.Stage == StageActive || r.Stage == StageRinging {
		if err = c.reconcileMediaIdentities(ctx, r); err != nil {
			return err
		}
	}
	r, err = c.Journal.Load(ctx, roomID)
	if err != nil {
		return err
	}
	return c.project(ctx, r)
}

func (c *Coordinator) reconcileMediaIdentities(ctx context.Context, observed *Room) error {
	effectCtx, cancel := context.WithTimeout(ctx, c.timeout())
	identities, err := c.Media.ListParticipantIdentities(effectCtx, observed.LiveKitRoomName)
	cancel()
	if err != nil {
		return err
	}
	for _, identity := range identities {
		// An observation may precede a concurrent commit. Never use the old
		// room snapshot to classify a current SDK participant for removal.
		current, err := c.Journal.Load(ctx, observed.Intent.RoomID)
		if err != nil {
			return err
		}
		allowed := false
		if current.Stage == StageActive {
			for _, member := range current.Members {
				if member.State == MemberJoined && member.MediaIdentity == identity {
					allowed = true
					break
				}
			}
		}
		if allowed {
			continue
		}
		effectCtx, cancel := context.WithTimeout(ctx, c.timeout())
		err = c.Media.RemoveParticipant(effectCtx, current.LiveKitRoomName, identity)
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}
