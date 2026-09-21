package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/space/internal/ownershiprecovery"
	"voice/backend/space/internal/store"
)

func TestNewOwnershipRecoveryCoordinator_MissingProtectedDependenciesFailsClosed(t *testing.T) {
	binding := store.OwnershipBinding{ProtocolVersion: 2, SpaceID: uuid.New(), AccountID: uuid.New(), ActorProfileID: uuid.New(), NewOwnerProfileID: uuid.New(), OperationID: uuid.New(), SessionEpoch: 1, ProofDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	for _, service := range []*SpaceGRPC{nil, {}, {Store: &store.SpaceStore{}}} {
		_, err := NewOwnershipRecoveryCoordinator(service).Execute(context.Background(), binding, "proof")
		require.ErrorIs(t, err, ownershiprecovery.ErrCoordinatorNotConfigured)
	}
}
