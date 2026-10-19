package memberauthority

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type SpaceClient interface {
	GetMemberAdmissionAuthority(context.Context, *spacev1.GetMemberAdmissionAuthorityRequest, ...grpc.CallOption) (*spacev1.GetMemberAdmissionAuthorityResponse, error)
}

// Client uses an already configured protected mTLS Space connection. It has no
// ordinary-listener fallback, positive cache or access to either owner's DB.
type Client struct {
	Space  SpaceClient
	Issuer *principal.Issuer
}

func (c *Client) ObserveMember(ctx context.Context, req *spacev1.GetMemberAdmissionAuthorityRequest) (Observation, error) {
	if err := ctx.Err(); err != nil {
		return Observation{}, status.FromContextError(err).Err()
	}
	if c == nil || c.Space == nil || c.Issuer == nil || req == nil {
		return Observation{}, status.Error(codes.Unavailable, "member authority client unavailable")
	}
	started := time.Now()
	hash, err := principal.RequestHash(req)
	if err != nil {
		return Observation{}, status.Error(codes.InvalidArgument, "invalid member authority request")
	}
	requestID := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return Observation{}, status.Error(codes.Unavailable, "member authority credential unavailable")
	}
	call, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	call = metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	reply, err := c.Space.GetMemberAdmissionAuthority(call, req)
	if err != nil {
		return Observation{}, err
	}
	observation, err := Parse(req, reply, started, time.Now())
	if err != nil {
		return Observation{}, status.Error(codes.Unavailable, "invalid member authority response")
	}
	return observation, nil
}
