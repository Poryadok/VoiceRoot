package catalogassets

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type ReconcileStore interface {
	EvictEmptySelections(context.Context, int) error
	ScanBindings(context.Context, uuid.UUID, int) ([]Binding, error)
	RetireExactBinding(context.Context, Binding) error
	PurgeCapturedProfile(context.Context, Binding) error
}
type ProfileSnapshots interface {
	ProfileStates(context.Context, []Actor) (map[Actor]ProfileState, error)
}
type Reconciler struct {
	Store    ReconcileStore
	Profiles ProfileSnapshots
	Files    FileClient
}

// Step scans only existing projections; it never creates actors or File rows.
// The cursor is process-local, so a restart revisits rows without a second
// durable identity ledger. An empty page wraps the next bounded scan.
func (r *Reconciler) Step(ctx context.Context, after uuid.UUID, limit int) (uuid.UUID, error) {
	if r == nil || r.Store == nil || r.Profiles == nil || r.Files == nil || limit < 1 || limit > 100 {
		return after, ErrActorUnknown
	}
	if err := r.Store.EvictEmptySelections(ctx, limit); err != nil {
		return after, err
	}
	bindings, err := r.Store.ScanBindings(ctx, after, limit)
	if err != nil {
		return after, err
	}
	if len(bindings) == 0 {
		return uuid.Nil, nil
	}
	if len(bindings) > limit {
		return after, ErrActorUnknown
	}
	actors := make([]Actor, 0, len(bindings))
	seen := map[uuid.UUID]Actor{}
	for _, b := range bindings {
		if previous, ok := seen[b.ProfileID]; ok {
			if previous != b.Actor {
				return after, ErrActorUnknown
			}
			continue
		}
		seen[b.ProfileID] = b.Actor
		actors = append(actors, b.Actor)
	}
	states, err := r.Profiles.ProfileStates(ctx, actors)
	if err != nil {
		return after, err
	}
	var failures []error
	for _, b := range bindings {
		if err := ctx.Err(); err != nil {
			return after, err
		}
		after = b.FileID
		switch states[b.Actor] {
		case ProfileSoftDeleted:
			// The SQL method matches the captured generation under ordered locks;
			// an old page must never close a newly admitted restored generation.
			if err := r.Store.PurgeCapturedProfile(ctx, b); err != nil {
				failures = append(failures, err)
			}
		case ProfileLive:
			state, err := r.Files.Status(ctx, b)
			if err != nil {
				failures = append(failures, err)
				continue
			}
			if !validState(state, b.Actor, b.Selection) || state.SourceID != b.SourceID || state.FileID != b.FileID {
				failures = append(failures, ErrBindingConflict)
				continue
			}
			qualifiedFence := state.TerminalFenceID != uuid.Nil && terminalState(state.Status)
			legacyCleanup := qualifiedFence && state.ActorCapture == (ActorCapture{}) && state.SelectionRevision == 0 && state.CurrentSelectionRevision == 0
			if b.Legacy && !legacyCleanup {
				failures = append(failures, ErrActorUnknown)
				continue
			}
			if !b.Legacy && (!validOrderedState(state) || state.SelectionRevision != b.SelectionRevision) && !legacyCleanup {
				failures = append(failures, ErrBindingConflict)
				continue
			}
			if state.ActorCapture != b.ActorCapture && (!qualifiedFence || state.ActorCapture != (ActorCapture{})) {
				failures = append(failures, ErrActorRetired)
				continue
			}
			if state.TerminalFenceID != uuid.Nil && !qualifiedFence {
				failures = append(failures, ErrBindingConflict)
				continue
			}
			if terminalState(state.Status) || state.CurrentSelectionRevision > state.SelectionRevision {
				if err := r.Store.RetireExactBinding(ctx, b); err != nil {
					failures = append(failures, err)
				}
			}
		default:
			// UNKNOWN/missing state is neither an admission nor erasure proof.
			failures = append(failures, ErrActorUnknown)
		}
	}
	return after, errors.Join(failures...)
}
