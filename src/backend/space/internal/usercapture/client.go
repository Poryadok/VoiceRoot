// Package usercapture provides the protected Space-purpose User capture client.
package usercapture

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"time"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spaceaccountcapture"
)

var ErrUnavailable = errors.New("user account capture unavailable")

type rpcClient interface {
	CaptureSpaceMemberAccountBindings(context.Context, *userv1.CaptureSpaceMemberAccountBindingsRequest, ...grpc.CallOption) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error)
}

// Client implements Native's nonvariadic capture interface. Construction is
// restricted to the mutual-TLS transport factory.
type Client struct {
	rpc    rpcClient
	issuer *principal.Issuer
}

func (c *Client) CaptureSpaceMemberAccountBindings(ctx context.Context, request *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
	if c == nil || c.rpc == nil || c.issuer == nil || ctx == nil || ctx.Err() != nil || spaceaccountcapture.ValidateRequest(request) != nil {
		return nil, ErrUnavailable
	}
	expected := proto.Clone(request).(*userv1.CaptureSpaceMemberAccountBindingsRequest)
	hash, err := principal.RequestHash(expected)
	if err != nil {
		return nil, ErrUnavailable
	}
	id := uuid.NewString()
	token, err := c.issuer.IssueService(principal.ServiceInput{Audience: "user", RPC: userv1.UserService_CaptureSpaceMemberAccountBindings_FullMethodName, RequestID: id, RequestHash: hash})
	if err != nil {
		return nil, ErrUnavailable
	}
	started := time.Now()
	ownerCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if ownerCtx.Err() != nil {
		return nil, ErrUnavailable
	}
	ownerCtx = metadata.NewOutgoingContext(ownerCtx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	wire := proto.Clone(expected).(*userv1.CaptureSpaceMemberAccountBindingsRequest)
	response, err := c.rpc.CaptureSpaceMemberAccountBindings(ownerCtx, wire)
	if err != nil || ownerCtx.Err() != nil || time.Since(started) >= 2*time.Second || !proto.Equal(expected, wire) || spaceaccountcapture.ValidateResponse(expected, response) != nil {
		return nil, ErrUnavailable
	}
	return proto.Clone(response).(*userv1.CaptureSpaceMemberAccountBindingsResponse), nil
}
