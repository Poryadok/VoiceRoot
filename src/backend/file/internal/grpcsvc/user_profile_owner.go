package grpcsvc

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/principal"

	userv1 "voice.app/voice/user/v1"
)

// UserProfileOwnerClient obtains the retention owner from User's protected
// listener. The signed request binds both the exact actor profile and the
// acquisition operation, so neither forwarded metadata nor a reused token can
// transfer account ownership.
type UserProfileOwnerClient struct {
	Client userv1.UserServiceClient
	Issuer *principal.Issuer
}

func (c *UserProfileOwnerClient) AccountIDByActorProfile(ctx context.Context, actorProfileID, operationID uuid.UUID) (uuid.UUID, error) {
	if c == nil || c.Client == nil || c.Issuer == nil || actorProfileID == uuid.Nil || operationID == uuid.Nil {
		return uuid.Nil, status.Error(codes.Unavailable, "user ownership client unavailable")
	}
	req := &userv1.ResolveAccountIDForProfileRequest{
		ProfileId:      actorProfileID.String(),
		ActorProfileId: actorProfileID.String(),
		OperationId:    operationID.String(),
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return uuid.Nil, status.Error(codes.Unauthenticated, "user ownership request binding failed")
	}
	requestID := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{
		Audience: "file", RPC: userv1.UserService_ResolveAccountIDForProfile_FullMethodName,
		RequestID: requestID, RequestHash: hash,
	})
	if err != nil {
		return uuid.Nil, status.Error(codes.Unauthenticated, "user ownership signing failed")
	}
	callCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	resp, err := c.Client.ResolveAccountIDForProfile(callCtx, req, grpc.WaitForReady(false))
	if err != nil {
		return uuid.Nil, err
	}
	accountID, err := uuid.Parse(resp.GetAccountId())
	if err != nil || accountID == uuid.Nil {
		return uuid.Nil, status.Error(codes.Internal, fmt.Sprintf("invalid account_id from User: %v", err))
	}
	return accountID, nil
}
