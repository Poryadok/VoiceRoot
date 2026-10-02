package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func requireMessageReferenceEntitlement(ctx context.Context, guard ChatGuard, messageID, profileID uuid.UUID) error {
	checker, ok := guard.(MessageHistoryEntitlement)
	if !ok {
		return status.Error(codes.Unavailable, "message history entitlement unavailable")
	}
	allowed, err := checker.MessageReadEntitledForMessage(ctx, messageID, profileID)
	if err != nil {
		return status.Error(codes.Unavailable, "message history entitlement unavailable")
	}
	if !allowed {
		return status.Error(codes.PermissionDenied, "message attachment access denied")
	}
	return nil
}
