// Package creatorauthority owns Space's fresh creator session observation.
package creatorauthority

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/principal"
)

var (
	ErrUnavailable    = errors.New("creator session authority unavailable")
	ErrInvalidActor   = errors.New("invalid creator actor")
	ErrSessionRevoked = errors.New("creator session revoked")
)

type Client interface {
	GetSpaceSessionEpochFloor(context.Context, *authv1.GetSpaceSessionEpochFloorRequest, ...grpc.CallOption) (*authv1.GetSpaceSessionEpochFloorResponse, error)
}

// Checker uses only Auth's exact Space caller surface. It has no positive cache,
// Redis-only fallback, or dependency on the Role/Voice epoch endpoints.
type Checker struct {
	Client Client
	Issuer *principal.Issuer
}

func (c *Checker) CheckEpoch(ctx context.Context, account string, epoch int64) error {
	id, err := uuid.Parse(account)
	if err != nil || id == uuid.Nil || id.String() != account || epoch <= 0 {
		return ErrInvalidActor
	}
	if c == nil || c.Client == nil || c.Issuer == nil {
		return ErrUnavailable
	}
	if ctx.Err() != nil {
		return ErrUnavailable
	}
	request := &authv1.GetSpaceSessionEpochFloorRequest{AccountId: account}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return ErrUnavailable
	}
	requestID := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "auth", RPC: authv1.AuthService_GetSpaceSessionEpochFloor_FullMethodName, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return ErrUnavailable
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response, err := c.Client.GetSpaceSessionEpochFloor(ctx, request)
	if err != nil || ctx.Err() != nil || response == nil || len(response.ProtoReflect().GetUnknown()) != 0 || response.SessionEpochFloor <= 0 {
		return ErrUnavailable
	}
	if epoch < response.SessionEpochFloor {
		return ErrSessionRevoked
	}
	return nil
}
