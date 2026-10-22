package chatcreation

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/principal"
)

// AuthFloorClient is the one protected Auth read consumed by Chat creation.
type AuthFloorClient interface {
	GetChatSessionEpochFloor(context.Context, *authv1.GetChatSessionEpochFloorRequest, ...grpc.CallOption) (*authv1.GetChatSessionEpochFloorResponse, error)
}

type AuthFloorChecker struct {
	Client AuthFloorClient
	Issuer *principal.Issuer
}

func (c *AuthFloorChecker) CheckSessionEpoch(ctx context.Context, account string, epoch int64) error {
	if c == nil || c.Client == nil || c.Issuer == nil {
		return ErrDependencyUnavailable
	}
	if _, err := CanonicalID(account); err != nil || epoch <= 0 {
		return ErrInitiator
	}
	// This hook runs after the delegated signature/body checks. Use its request
	// identity only; replace all outgoing metadata with Chat's own credential.
	incoming, err := principal.IncomingMetadata(ctx)
	if err != nil {
		return ErrInitiator
	}
	if _, err = CanonicalID(incoming.RequestID); err != nil {
		return ErrInitiator
	}
	req := &authv1.GetChatSessionEpochFloorRequest{AccountId: account}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return ErrDependencyUnavailable
	}
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "auth", RPC: authv1.AuthService_GetChatSessionEpochFloor_FullMethodName, RequestID: incoming.RequestID, RequestHash: hash})
	if err != nil {
		return ErrDependencyUnavailable
	}
	call, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	call = metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", incoming.RequestID))
	response, err := c.Client.GetChatSessionEpochFloor(call, req)
	if call.Err() != nil {
		return errors.Join(ErrDependencyUnavailable, call.Err())
	}
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return ErrInitiator
		}
		return errors.Join(ErrDependencyUnavailable, err)
	}
	if response == nil || HasUnknown(response) || response.GetSessionEpochFloor() <= 0 {
		return ErrDependencyUnavailable
	}
	if epoch < response.GetSessionEpochFloor() {
		return ErrInitiator
	}
	return nil
}
