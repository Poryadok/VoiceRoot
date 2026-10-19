package grpcsvc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/spacecommunication"
)

type CommunicationStateResolver interface {
	ResolveMemberCommunicationState(context.Context, string, string) (spacecommunication.Snapshot, error)
}

type ChatCommunicationScope interface {
	Load(context.Context, uuid.UUID) (*store.ChatThreadPolicy, error)
}

// BeginCommunicationAction shares the ordinary destination policy with the
// authenticated SDK processor after Auth has resolved the actual sender.
func (s *MessagingGRPC) BeginCommunicationAction(ctx context.Context, chatID, profileID uuid.UUID) (context.Context, context.CancelFunc, error) {
	return s.beginCommunicationAction(ctx, chatID, profileID)
}

// beginCommunicationAction derives the destination scope from authoritative
// Chat metadata. The same monotonic deadline follows the authority RPC through
// the mutation transaction; retries must call this again.
func (s *MessagingGRPC) beginCommunicationAction(ctx context.Context, chatID, profileID uuid.UUID) (context.Context, context.CancelFunc, error) {
	var scope ChatCommunicationScope = s.ChatThreadPolicy
	if s.CommunicationScope != nil {
		scope = s.CommunicationScope
	}
	if scope == nil || isNilDependency(scope) {
		return nil, nil, status.Error(codes.Unavailable, "authoritative chat Space scope unavailable")
	}
	policy, err := scope.Load(ctx, chatID)
	if err != nil || policy == nil {
		return nil, nil, status.Error(codes.Unavailable, "authoritative chat Space scope unavailable")
	}
	switch policy.ChatType {
	case "dm", "group", "channel":
	default:
		return nil, nil, status.Error(codes.Unavailable, "authoritative chat type invalid")
	}
	if policy.SpaceID == nil {
		return ctx, func() {}, nil
	}
	if *policy.SpaceID == uuid.Nil || (policy.ChatType != "group" && policy.ChatType != "channel") {
		return nil, nil, status.Error(codes.Unavailable, "authoritative chat Space scope invalid")
	}
	if isNilDependency(s.CommunicationStates) {
		return nil, nil, status.Error(codes.Unavailable, "Space communication authority unavailable")
	}
	attempt, cancel := context.WithTimeout(ctx, 2*time.Second)
	snapshot, err := s.CommunicationStates.ResolveMemberCommunicationState(attempt, policy.SpaceID.String(), profileID.String())
	if err != nil {
		cancel()
		return nil, nil, status.Error(codes.Unavailable, "Space communication authority unavailable")
	}
	timedOut, err := snapshot.Validate(policy.SpaceID.String(), profileID.String())
	if err != nil || attempt.Err() != nil {
		cancel()
		return nil, nil, status.Error(codes.Unavailable, "Space communication decision incomplete")
	}
	if timedOut {
		cancel()
		return nil, nil, status.Error(codes.PermissionDenied, "member is timed out in this Space")
	}
	return attempt, cancel, nil
}
