package main

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

// Only the migrated exact Chat VIEW decision uses this protected transport.
// The ordinary client retains unrelated, separately tracked RPC migrations.
type chatRoleDecisionClient struct {
	rolev1.RoleServiceClient
	protected rolev1.RoleServiceClient
	issuer    *principal.Issuer
}

func (c *chatRoleDecisionClient) CheckPermission(ctx context.Context, req *rolev1.CheckPermissionRequest, opts ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "permission request required")
	}
	if req.GetPermissionName() != "TEXT_CHAT_VIEW" {
		if c.RoleServiceClient == nil {
			return nil, status.Error(codes.Unavailable, "ordinary Role client unavailable")
		}
		return c.RoleServiceClient.CheckPermission(ctx, req, opts...)
	}
	if c.protected == nil || c.issuer == nil {
		return nil, status.Error(codes.Unavailable, "protected Chat Role decision transport unavailable")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Role decision body")
	}
	requestID := uuid.NewString()
	token, err := c.issuer.IssueService(principal.ServiceInput{Audience: "role", RPC: rolev1.RoleService_CheckPermission_FullMethodName, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Chat Role decision signing unavailable")
	}
	// Replace caller metadata: the subject is bound request data, never an actor header.
	signed := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	return c.protected.CheckPermission(signed, req, opts...)
}

func newChatRoleDecisionClientFromEnv(ordinary rolev1.RoleServiceClient, issuer *principal.Issuer) (rolev1.RoleServiceClient, func() error, error) {
	client := &chatRoleDecisionClient{RoleServiceClient: ordinary, issuer: issuer}
	configured := false
	for _, suffix := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"} {
		if strings.TrimSpace(os.Getenv("CHAT_ROLE_DECISION_"+suffix)) != "" {
			configured = true
		}
	}
	if !configured {
		return client, func() error { return nil }, nil
	}
	if issuer == nil {
		return nil, nil, errors.New("chat Role decision transport requires Chat signing identity")
	}
	conn, err := dialChatCreationOwnerFromEnv("CHAT_ROLE_DECISION_")
	if err != nil {
		return nil, nil, err
	}
	client.protected = rolev1.NewRoleServiceClient(conn)
	return client, conn.Close, nil
}
