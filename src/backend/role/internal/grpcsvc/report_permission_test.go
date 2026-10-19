package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestReportPermissionAuthenticatesBeforeAnyStoreAccess(t *testing.T) {
	req := &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "TEXT_CHAT_VIEW", Chat: &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_CHANNEL.Enum()}}
	service := &RoleGRPC{}
	response, err := service.checkQualifiedReportPermission(context.Background(), req)
	require.Nil(t, response)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	p := principal.Principal{Kind: "service", Issuer: "messaging", Subject: "service:messaging", Audience: "role", RPC: rolev1.RoleService_CheckPermission_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash}
	response, err = service.checkQualifiedReportPermission(principal.WithVerified(context.Background(), p), req)
	require.Nil(t, response)
	require.Equal(t, codes.Unavailable, status.Code(err), "verified query cannot manufacture a provider allow")
	var missing *RoleGRPC
	response, err = missing.checkQualifiedReportPermission(principal.WithVerified(context.Background(), p), req)
	require.Nil(t, response)
	require.Equal(t, codes.Unavailable, status.Code(err))
}
