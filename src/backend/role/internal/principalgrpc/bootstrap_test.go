package principalgrpc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestOwnerBootstrapOrdinaryListenerClosed(t *testing.T) {
	called := false
	_, err := OwnershipUnaryInterceptor(nil)(context.Background(), &rolev1.BootstrapSpaceRolesRequest{}, &grpc.UnaryServerInfo{FullMethod: rolev1.RoleService_BootstrapSpaceRoles_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.False(t, called)
}

func TestRequireOwnerBootstrapIndependentBoundary(t *testing.T) {
	req := &rolev1.BootstrapSpaceRolesRequest{SpaceId: uuid.NewString(), OwnerProfileId: uuid.NewString()}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	p := principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role", RPC: rolev1.RoleService_BootstrapSpaceRoles_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, ExpiresAt: time.Now().Add(time.Second)}
	require.NoError(t, RequireOwnerBootstrap(principal.WithVerified(context.Background(), p), req))
	require.Equal(t, codes.Unauthenticated, status.Code(RequireOwnerBootstrap(context.Background(), req)))
	for _, name := range []string{"expired", "hash", "profile", "bot", "scope", "type"} {
		t.Run(name, func(t *testing.T) {
			v := p
			switch name {
			case "expired":
				v.ExpiresAt = time.Now()
			case "hash":
				v.RequestHash = "wrong"
			case "profile":
				v.ProfileID = req.OwnerProfileId
			case "bot":
				v.BotID = uuid.NewString()
			case "scope":
				v.BotScope = "global"
			case "type":
				v.AccountType = "regular"
			}
			require.Equal(t, codes.Unauthenticated, status.Code(RequireOwnerBootstrap(principal.WithVerified(context.Background(), v), req)))
		})
	}
}
