package s2s

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"time"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/messaging/internal/messageviews"
	"voice/backend/pkg/principal"
)

type profileDeletionClient interface {
	GetProfileDeletionStates(context.Context, *userv1.GetProfileDeletionStatesRequest, ...grpc.CallOption) (*userv1.GetProfileDeletionStatesResponse, error)
}
type UserGRPCProfileDeletion struct {
	Client profileDeletionClient
	Issuer *principal.Issuer
}

func (c *UserGRPCProfileDeletion) ProfileDeletionStates(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]messageviews.DeletionState, error) {
	if c == nil || c.Client == nil || c.Issuer == nil {
		return nil, errors.New("profile deletion authority unavailable")
	}
	if len(ids) < 1 || len(ids) > 500 {
		return nil, errors.New("invalid profile deletion batch")
	}
	request := &userv1.GetProfileDeletionStatesRequest{ProfileIds: make([]string, len(ids))}
	seen := map[uuid.UUID]bool{}
	for i, id := range ids {
		if id == uuid.Nil || seen[id] {
			return nil, errors.New("invalid profile deletion batch")
		}
		seen[id] = true
		request.ProfileIds[i] = id.String()
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, err
	}
	requestID := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "user", RPC: userv1.UserService_GetProfileDeletionStates_FullMethodName, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return nil, err
	}
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	call = metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	response, err := c.Client.GetProfileDeletionStates(call, request)
	if err != nil {
		return nil, err
	}
	if response == nil || len(response.GetStates()) != len(ids) {
		return nil, errors.New("incomplete profile deletion states")
	}
	result := make(map[uuid.UUID]messageviews.DeletionState, len(ids))
	for i, entry := range response.GetStates() {
		if entry == nil || entry.GetProfileId() != request.ProfileIds[i] {
			return nil, errors.New("invalid profile deletion states")
		}
		var state messageviews.DeletionState
		switch entry.GetState() {
		case userv1.ProfileDeletionState_PROFILE_DELETION_STATE_NOT_DELETED:
			state = messageviews.NotDeleted
		case userv1.ProfileDeletionState_PROFILE_DELETION_STATE_SOFT_DELETED:
			state = messageviews.SoftDeleted
		case userv1.ProfileDeletionState_PROFILE_DELETION_STATE_UNKNOWN:
			state = messageviews.Unknown
		default:
			return nil, errors.New("invalid profile deletion state")
		}
		result[ids[i]] = state
	}
	return result, nil
}
