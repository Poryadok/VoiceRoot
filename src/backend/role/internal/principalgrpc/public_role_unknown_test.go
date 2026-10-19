package principalgrpc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestPublicRoleRejectsSignedNestedUnknownFields(t *testing.T) {
	for _, method := range []string{rolev1.RoleService_SetChatOverride_FullMethodName, rolev1.RoleService_GetEffectivePermissions_FullMethodName} {
		t.Run(method, func(t *testing.T) {
			chat := &chatv1.ChatRef{Id: uuid.NewString()}
			var request proto.Message = &rolev1.SetChatOverrideRequest{SpaceId: uuid.NewString(), RoleId: uuid.NewString(), Chat: chat}
			if method == rolev1.RoleService_GetEffectivePermissions_FullMethodName {
				request = &rolev1.GetEffectivePermissionsRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), Chat: chat}
			}
			account, profile, id := uuid.NewString(), uuid.NewString(), uuid.NewString()
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer exact-signed-fixture", "x-request-id", id))
			intercept := StrictUnaryInterceptor(verifierFunc(func(_ context.Context, _ string, gotMethod, gotID, hash string) (principal.Principal, error) {
				exactHash, err := principal.RequestHash(request)
				require.NoError(t, err)
				require.Equal(t, exactHash, hash, "the verified signer binds even the unknown bytes")
				return principal.Principal{Kind: "delegated_user", Issuer: "gateway", Subject: account, AccountID: account, ProfileID: profile, SessionEpoch: 1, Audience: "role", RPC: gotMethod, RequestID: gotID, RequestHash: hash}, nil
			}))
			calls := 0
			call := func() error {
				_, err := intercept(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { calls++; return nil, nil })
				return err
			}
			require.NoError(t, call(), "valid nested control must reach the handler")
			require.Equal(t, 1, calls)
			chat.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
			require.Equal(t, codes.InvalidArgument, status.Code(call()))
			require.Equal(t, 1, calls, "a valid credential does not authorize unknown nested request semantics")
		})
	}
}
