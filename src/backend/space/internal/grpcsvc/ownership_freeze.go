package grpcsvc

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/space/internal/store"
)

func mapSpaceStoreError(err error) error {
	if errors.Is(err, store.ErrMemberAuthorityUnavailable) {
		return status.Error(codes.Unavailable, "membership authority unavailable")
	}
	if errors.Is(err, store.ErrMemberAdmissionConflict) {
		return status.Error(codes.FailedPrecondition, "membership admission conflict")
	}
	if errors.Is(err, store.ErrAdmissionRequired) || errors.Is(err, store.ErrLegacyEntryPolicyWrite) || errors.Is(err, store.ErrEntryPolicyVersion) {
		return status.Error(codes.FailedPrecondition, "entry verification required")
	}
	if errors.Is(err, store.ErrEntryPolicyUnavailable) {
		return status.Error(codes.Unavailable, "entry verification unavailable")
	}
	if errors.Is(err, store.ErrOwnershipFrozen) || errors.Is(err, store.ErrLifecycleFrozen) || errors.Is(err, store.ErrOwnershipScopeUnavailable) {
		return status.Error(codes.Unavailable, "space temporarily unavailable")
	}
	return status.Error(codes.Internal, "internal error")
}
