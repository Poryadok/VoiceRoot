// Package targetvalidation calls resource owners using a fresh Role credential
// for every attempt. No allow decision or capability is cached.
package targetvalidation

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/roleoverrideprincipal"
)

type ChatClient interface {
	ValidateRoleOverrideTarget(context.Context, *chatv1.ValidateRoleOverrideTargetRequest, ...grpc.CallOption) (*chatv1.ValidateRoleOverrideTargetResponse, error)
}
type SpaceClient interface {
	ValidateRoleOverrideTarget(context.Context, *spacev1.ValidateRoleOverrideTargetRequest, ...grpc.CallOption) (*spacev1.ValidateRoleOverrideTargetResponse, error)
}
type Client struct {
	Chat   ChatClient
	Space  SpaceClient
	Issuer *principal.Issuer
}

func (c *Client) ValidateChat(ctx context.Context, spaceID, chatID string) error {
	if c == nil || c.Chat == nil || c.Space == nil || c.Issuer == nil {
		return status.Error(codes.Unavailable, "target validation unavailable")
	}
	if err := c.validateSpace(ctx, &spacev1.ValidateRoleOverrideTargetRequest{SpaceId: spaceID}); err != nil {
		return err
	}
	request := &chatv1.ValidateRoleOverrideTargetRequest{SpaceId: spaceID, ChatId: chatID}
	call, cancel, err := c.context(ctx, "chat", request)
	if err != nil {
		return err
	}
	defer cancel()
	response, err := c.Chat.ValidateRoleOverrideTarget(call, request)
	if err != nil {
		return err
	}
	if response == nil || len(response.ProtoReflect().GetUnknown()) != 0 {
		return status.Error(codes.Unavailable, "invalid target response")
	}
	return nil
}

func (c *Client) ValidateVoice(ctx context.Context, spaceID, roomID string) error {
	if c == nil || c.Space == nil || c.Issuer == nil {
		return status.Error(codes.Unavailable, "target validation unavailable")
	}
	return c.validateSpace(ctx, &spacev1.ValidateRoleOverrideTargetRequest{SpaceId: spaceID, VoiceRoomId: &roomID})
}

func (c *Client) validateSpace(ctx context.Context, request *spacev1.ValidateRoleOverrideTargetRequest) error {
	call, cancel, err := c.context(ctx, "space", request)
	if err != nil {
		return err
	}
	defer cancel()
	response, err := c.Space.ValidateRoleOverrideTarget(call, request)
	if err != nil {
		return err
	}
	if response == nil || len(response.ProtoReflect().GetUnknown()) != 0 {
		return status.Error(codes.Unavailable, "invalid target response")
	}
	return nil
}

func (c *Client) context(ctx context.Context, target string, request proto.Message) (context.Context, context.CancelFunc, error) {
	if ctx.Err() != nil {
		return nil, nil, status.FromContextError(ctx.Err()).Err()
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, nil, status.Error(codes.InvalidArgument, "invalid target request")
	}
	id := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: target, RPC: roleoverrideprincipal.Method(target), RequestID: id, RequestHash: hash})
	if err != nil {
		return nil, nil, status.Error(codes.Unavailable, "target credential unavailable")
	}
	call, cancel := context.WithTimeout(ctx, 2*time.Second)
	return metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), cancel, nil
}
