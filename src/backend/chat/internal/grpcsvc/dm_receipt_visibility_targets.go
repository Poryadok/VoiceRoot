package grpcsvc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

const (
	dmReceiptVisibilityDefaultPageSize = 100
	dmReceiptVisibilityMaxPageSize     = 500
)

// ListDMReceiptVisibilityTargets is an internal Messaging-only operation. It
// has no end-user or Gateway path because it exposes a profile's DM graph.
func (s *ChatGRPC) ListDMReceiptVisibilityTargets(ctx context.Context, req *chatv1.ListDMReceiptVisibilityTargetsRequest) (*chatv1.ListDMReceiptVisibilityTargetsResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	if !authctx.IsInternalCaller(ctx, "messaging") {
		return nil, status.Error(codes.PermissionDenied, "messaging service identity required")
	}
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	pageSize := int(req.GetPage().GetPageSize())
	if pageSize <= 0 {
		pageSize = dmReceiptVisibilityDefaultPageSize
	}
	if pageSize > dmReceiptVisibilityMaxPageSize {
		pageSize = dmReceiptVisibilityMaxPageSize
	}
	targets, next, err := s.DM.ListDMReceiptVisibilityTargets(ctx, profileID, req.GetPage().GetCursor(), pageSize)
	if err != nil {
		if errors.Is(err, store.ErrInvalidDMReceiptVisibilityCursor) {
			return nil, status.Error(codes.InvalidArgument, "invalid page cursor")
		}
		return nil, status.Error(codes.Unavailable, "dm receipt visibility targets unavailable")
	}
	out := make([]*chatv1.DMReceiptVisibilityTarget, 0, len(targets))
	for _, target := range targets {
		out = append(out, &chatv1.DMReceiptVisibilityTarget{ChatId: target.ChatID.String(), PeerProfileId: target.PeerProfileID.String()})
	}
	return &chatv1.ListDMReceiptVisibilityTargetsResponse{Targets: out, NextCursor: next}, nil
}
