package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/socialprincipal"
)

// GetProfileDeletionStates exists only on the protected Messaging entrypoint.
// Visibility and Auth eligibility checks must never substitute for deleted_at.
func (s *MessagingScheduledPresenceGRPC) GetProfileDeletionStates(ctx context.Context, req *userv1.GetProfileDeletionStatesRequest) (*userv1.GetProfileDeletionStatesResponse, error) {
	if err := socialprincipal.RequireMessagingPresence(ctx, userv1.UserService_GetProfileDeletionStates_FullMethodName, req); err != nil {
		return nil, err
	}
	if len(req.GetProfileIds()) < 1 || len(req.GetProfileIds()) > 500 {
		return nil, status.Error(codes.InvalidArgument, "profile_ids must contain 1..500 distinct canonical nonnil UUIDs")
	}
	ids := make([]uuid.UUID, len(req.ProfileIds))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for i, raw := range req.ProfileIds {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil || raw != id.String() {
			return nil, status.Error(codes.InvalidArgument, "invalid canonical profile_id")
		}
		if _, exists := seen[id]; exists {
			return nil, status.Error(codes.InvalidArgument, "duplicate profile_id")
		}
		seen[id] = struct{}{}
		ids[i] = id
	}
	if s == nil || s.User == nil || s.User.Profiles == nil {
		return nil, status.Error(codes.Unavailable, "profile deletion authority unavailable")
	}
	states, err := s.User.Profiles.ProfileDeletionStates(ctx, ids)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "profile deletion authority unavailable")
	}
	resp := &userv1.GetProfileDeletionStatesResponse{States: make([]*userv1.ProfileDeletionStateEntry, len(ids))}
	for i, id := range ids {
		state := userv1.ProfileDeletionState_PROFILE_DELETION_STATE_UNKNOWN
		if deleted, exists := states[id]; exists {
			state = userv1.ProfileDeletionState_PROFILE_DELETION_STATE_NOT_DELETED
			if deleted {
				state = userv1.ProfileDeletionState_PROFILE_DELETION_STATE_SOFT_DELETED
			}
		}
		resp.States[i] = &userv1.ProfileDeletionStateEntry{ProfileId: id.String(), State: state}
	}
	return resp, nil
}
