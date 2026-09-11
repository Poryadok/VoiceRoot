package grpcsvc

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/space/internal/store"
)

func mapSpaceStoreError(err error) error {
	if errors.Is(err, store.ErrOwnershipFrozen) || errors.Is(err, store.ErrOwnershipScopeUnavailable) {
		return status.Error(codes.Unavailable, "space temporarily unavailable")
	}
	return status.Error(codes.Internal, "internal error")
}
