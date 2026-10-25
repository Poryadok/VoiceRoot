package spacemembership

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

type Client struct {
	space       spacev1.SpaceServiceClient
	role        rolev1.RoleServiceClient
	chat        chatv1.ChatScopeAuthorityServiceClient
	messaging   messagingv1.MessagingServiceClient
	Store       *store.BotStore
	Source      uuid.UUID
	issuer      *principal.Issuer
	connections []*grpc.ClientConn
}

func New(ctx context.Context, cfg Config, issuer *principal.Issuer) (result *Client, err error) {
	if ctx == nil || issuer == nil || cfg.SourceGeneration == uuid.Nil {
		return nil, ErrUnavailable
	}
	c := &Client{issuer: issuer, Source: cfg.SourceGeneration}
	defer func() {
		if err != nil {
			_ = c.Close()
		}
	}()
	for _, endpoint := range []Endpoint{cfg.Space, cfg.Role, cfg.Chat, cfg.Messaging} {
		tlsConfig, e := endpoint.TLS()
		if e != nil {
			return nil, e
		}
		conn, e := grpc.NewClient(grpcclient.DialTarget(endpoint.Address), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		if e != nil {
			return nil, ErrUnavailable
		}
		c.connections = append(c.connections, conn)
		conn.Connect()
		ready, cancel := context.WithTimeout(ctx, grpcclient.DialTimeoutFromEnv())
		e = grpcclient.WaitForReady(ready, conn)
		cancel()
		if e != nil {
			return nil, ErrUnavailable
		}
	}
	c.space = spacev1.NewSpaceServiceClient(c.connections[0])
	c.role = rolev1.NewRoleServiceClient(c.connections[1])
	c.chat = chatv1.NewChatScopeAuthorityServiceClient(c.connections[2])
	c.messaging = messagingv1.NewMessagingServiceClient(c.connections[3])
	return c, nil
}
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	for _, conn := range c.connections {
		_ = conn.Close()
	}
	c.connections = nil
	return nil
}

func canonicalID(v string) bool {
	id, err := uuid.Parse(v)
	return err == nil && id != uuid.Nil && id.String() == v
}
func validIntent(i *spacev1.BotInstallationIntent, space, actor string) bool {
	return i != nil && len(i.ProtoReflect().GetUnknown()) == 0 && canonicalID(i.GetOperationId()) && canonicalID(i.GetBotId()) && canonicalID(i.GetInstallationId()) && canonicalID(space) && canonicalID(actor) && i.GetSpaceId() == space && i.GetActorProfileId() == actor && i.GetGeneration() > 0 && i.GetGeneration() <= math.MaxInt64
}
func (c *Client) callContext(ctx context.Context, audience, method, operation string, req proto.Message) (context.Context, context.CancelFunc, error) {
	if c == nil || c.issuer == nil || !canonicalID(operation) {
		return nil, nil, ErrUnavailable
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	// Business identity is stable; IssueService generates a fresh JWT/JTI even
	// when an earlier exact operation timed out or lost its response.
	token, err := c.issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: operation, RequestHash: hash})
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	call = metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operation))
	return call, cancel, nil
}
func (c *Client) Add(ctx context.Context, request *spacev1.AddBotMemberRequest) error {
	if c == nil || c.space == nil || request == nil {
		return ErrUnavailable
	}
	req := proto.Clone(request).(*spacev1.AddBotMemberRequest)
	if len(req.ProtoReflect().GetUnknown()) != 0 || !validIntent(req.GetInstallationIntent(), req.GetSpaceId(), req.GetProfileId()) {
		return ErrUnavailable
	}
	call, cancel, err := c.callContext(ctx, "space", spacev1.SpaceService_AddBotMember_FullMethodName, req.GetInstallationIntent().GetOperationId(), req)
	if err != nil {
		return err
	}
	defer cancel()
	response, err := c.space.AddBotMember(call, req)
	if err == nil && (response == nil || len(response.ProtoReflect().GetUnknown()) != 0) {
		return ErrUnavailable
	}
	return err
}
func (c *Client) Remove(ctx context.Context, request *spacev1.RemoveBotMemberRequest) error {
	if c == nil || c.space == nil || request == nil {
		return ErrUnavailable
	}
	req := proto.Clone(request).(*spacev1.RemoveBotMemberRequest)
	if len(req.ProtoReflect().GetUnknown()) != 0 || !validIntent(req.GetInstallationIntent(), req.GetSpaceId(), req.GetProfileId()) || !canonicalID(req.GetInstallOperationId()) || req.GetInstallOperationId() == req.GetInstallationIntent().GetOperationId() {
		return ErrUnavailable
	}
	call, cancel, err := c.callContext(ctx, "space", spacev1.SpaceService_RemoveBotMember_FullMethodName, req.GetInstallationIntent().GetOperationId(), req)
	if err != nil {
		return err
	}
	defer cancel()
	response, err := c.space.RemoveBotMember(call, req)
	if err == nil && (response == nil || len(response.ProtoReflect().GetUnknown()) != 0) {
		return ErrUnavailable
	}
	return err
}
func (c *Client) CheckManageBots(ctx context.Context, space, profile uuid.UUID) (bool, error) {
	if c == nil || c.role == nil || space == uuid.Nil || profile == uuid.Nil {
		return false, ErrUnavailable
	}
	req := &rolev1.CheckPermissionRequest{SpaceId: space.String(), ProfileId: profile.String(), PermissionName: "SPACE_MANAGE_BOTS"}
	call, cancel, err := c.callContext(ctx, "role", rolev1.RoleService_CheckPermission_FullMethodName, uuid.NewString(), req)
	if err != nil {
		return false, err
	}
	defer cancel()
	resp, err := c.role.CheckPermission(call, req)
	if err != nil {
		return false, err
	}
	if resp == nil || len(resp.ProtoReflect().GetUnknown()) != 0 {
		return false, ErrUnavailable
	}
	return resp.GetAllowed(), nil
}
