package principalgrpc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestMatchmakingPermissionExactSpaceBoundary(t *testing.T) {
	for _, mode := range []string{"valid", "voice_issuer", "node", "actor", "ordinary", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			req := &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "SPACE_MANAGE_MATCHMAKING"}
			if mode == "node" {
				room := uuid.NewString()
				req.VoiceRoomId = &room
			}
			if mode == "unknown" {
				req.ProtoReflect().SetUnknown([]byte{0x78, 1})
			}
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer fixture", "x-request-id", uuid.NewString()))
			interceptor := StrictUnaryInterceptor(verifierFunc(func(_ context.Context, _ string, method, id, hash string) (principal.Principal, error) {
				p := principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role", RPC: method, RequestID: id, RequestHash: hash}
				if mode == "voice_issuer" {
					p.Issuer = "voice"
					p.Subject = "service:voice"
				}
				if mode == "actor" {
					p.ProfileID = uuid.NewString()
				}
				return p, nil
			}))
			if mode == "ordinary" {
				interceptor = OwnershipUnaryInterceptor(nil)
			}
			called := false
			_, err := interceptor(ctx, req, &grpc.UnaryServerInfo{FullMethod: rolev1.RoleService_CheckPermission_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
			if mode == "valid" {
				require.NoError(t, err)
				require.True(t, called)
			} else {
				require.Error(t, err)
				require.False(t, called)
			}
		})
	}
}
