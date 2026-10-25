package store

import (
	"context"

	"github.com/google/uuid"
)

// SpaceAdmissionOwnership is internal workflow authority, never an operation,
// media, room or lifecycle identity. It is not serialized into public payloads.
type SpaceAdmissionOwnership struct {
	OperationID uuid.UUID
	Token       uuid.UUID
	Epoch       int64
}

// SpaceAdmissionRevocation is an exact persisted restrictive target. It is
// returned only by the trusted leave/ACL path; it grants no forward authority.
// A workflow takeover invalidates its epoch and requires a fresh DENY fence.
type SpaceAdmissionRevocation struct {
	Admission  SpaceMediaAdmission
	OwnerEpoch int64
}

func NewSpaceAdmissionOwnership(operation uuid.UUID) SpaceAdmissionOwnership {
	return SpaceAdmissionOwnership{OperationID: operation, Token: uuid.New(), Epoch: 1}
}

type admissionOwnershipContext struct{}

// WithSpaceAdmissionOwnership preserves independent tokens when an internal
// batch owns multiple operations. Child contexts never mutate their parent map.
func WithSpaceAdmissionOwnership(ctx context.Context, owner SpaceAdmissionOwnership) context.Context {
	previous, _ := ctx.Value(admissionOwnershipContext{}).(map[uuid.UUID]SpaceAdmissionOwnership)
	owners := make(map[uuid.UUID]SpaceAdmissionOwnership, len(previous)+1)
	for operation, value := range previous {
		owners[operation] = value
	}
	owners[owner.OperationID] = owner
	return context.WithValue(ctx, admissionOwnershipContext{}, owners)
}

func SpaceAdmissionOwnershipFromContext(ctx context.Context, operation uuid.UUID) (SpaceAdmissionOwnership, bool) {
	owners, _ := ctx.Value(admissionOwnershipContext{}).(map[uuid.UUID]SpaceAdmissionOwnership)
	owner, ok := owners[operation]
	return owner, ok && owner.OperationID == operation && operation != uuid.Nil && owner.Token != uuid.Nil && owner.Epoch > 0
}
