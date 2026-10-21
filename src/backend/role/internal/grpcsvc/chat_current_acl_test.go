package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"time"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestChatCurrentACLHandlerRechecksPurpose(t *testing.T) {
	r := &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "SPACE_MANAGE_ROLES"}
	hash, err := principal.RequestHash(r)
	require.NoError(t, err)
	p := principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "role", RPC: rolev1.RoleService_CheckPermission_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, ExpiresAt: time.Now().Add(30 * time.Second)}
	_, err = (&RoleGRPC{}).CheckPermission(principal.WithVerified(context.Background(), p), r)
	require.Equal(t, codes.PermissionDenied, status.Code(err), "verified Chat context cannot enter an unrelated decision before persistence")
}
