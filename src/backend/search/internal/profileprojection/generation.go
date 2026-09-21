package profileprojection

import "fmt"

// GenerationState is the durable lifecycle of a Search-owned profile
// projection generation. User's v1 event contract is intentionally unaware of
// this local storage concern.
type GenerationState string

const (
	GenerationBuilding GenerationState = "building"
	GenerationReady    GenerationState = "ready"
)

// GenerationRoute is the complete serving/rollback pointer pair. It is
// persisted and changed under the database route lock; these methods keep the
// state transition rules independently testable.
type GenerationRoute struct {
	Active   uint64
	Rollback uint64
}

func (r *GenerationRoute) Promote(target uint64, states map[uint64]GenerationState) error {
	if r == nil || r.Active == 0 || target == 0 || target == r.Active {
		return fmt.Errorf("invalid generation promotion")
	}
	if states[target] != GenerationReady {
		return fmt.Errorf("generation %d is not ready", target)
	}
	r.Rollback = r.Active
	r.Active = target
	return nil
}

func (r *GenerationRoute) RollbackToPrevious() error {
	if r == nil || r.Active == 0 || r.Rollback == 0 || r.Active == r.Rollback {
		return fmt.Errorf("rollback generation unavailable")
	}
	r.Active, r.Rollback = r.Rollback, r.Active
	return nil
}
