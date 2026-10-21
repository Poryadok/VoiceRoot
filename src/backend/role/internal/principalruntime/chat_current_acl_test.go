package principalruntime

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestChatCurrentACLActualSignedBoundary(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	f.config.JWKSURLs["chat"] = f.config.JWKSURLs["space"]
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
	req := &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "TEXT_CHAT_VIEW", Chat: &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()}}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	id := uuid.NewString()
	token := issueRuntimeToken(t, f.key, "chat", "current", "role", rolev1.RoleService_CheckPermission_FullMethodName, id, hash)
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), 2*time.Second)
	defer cancel()
	_, err = client.CheckPermission(ctx, req)
	require.NoError(t, err, "agreed exact service:chat CURRENTACL must reach the protected handler")
	p := <-recorder.principals
	require.Equal(t, "service:chat", p.Subject)
	require.EqualValues(t, 1, recorder.calls.Load())
}

func TestChatCurrentACLExactBodyAndTransport(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	f.config.JWKSURLs["chat"] = f.config.JWKSURLs["space"]
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
	base := &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "TEXT_CHAT_VIEW", Chat: &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_CHANNEL.Enum()}}
	method := rolev1.RoleService_CheckPermission_FullMethodName
	for _, name := range []string{"channel", "issuer", "audience", "rpc", "hash", "substituted_subject", "raw_actor", "duplicate_auth", "request_id", "permission", "voice", "missing_chat", "missing_type", "dm", "unknown_type", "unknown_root", "unknown_nested", "space", "profile", "chat"} {
		t.Run(name, func(t *testing.T) {
			req := proto.Clone(base).(*rolev1.CheckPermissionRequest)
			issuer, aud, rpc, id := "chat", "role", method, uuid.NewString()
			switch name {
			case "issuer":
				issuer = "space"
			case "audience":
				aud = "chat"
			case "rpc":
				rpc = rolev1.RoleService_GetEffectivePermissions_FullMethodName
			case "request_id":
				id = "not-a-canonical-operation"
			case "permission":
				req.PermissionName = "TEXT_CHAT_READ_HISTORY"
			case "voice":
				v := ""
				req.VoiceRoomId = &v
			case "missing_chat":
				req.Chat = nil
			case "missing_type":
				req.Chat.Type = nil
			case "dm":
				req.Chat.Type = chatv1.ChatType_CHAT_TYPE_DM.Enum()
			case "unknown_type":
				req.Chat.Type = chatv1.ChatType(99).Enum()
			case "unknown_root":
				req.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
			case "unknown_nested":
				req.Chat.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
			case "space":
				req.SpaceId = " " + req.SpaceId
			case "profile":
				req.ProfileId = uuid.Nil.String()
			case "chat":
				req.Chat.Id = "invalid"
			}
			hash, err := principal.RequestHash(req)
			require.NoError(t, err)
			if name == "hash" {
				hash = runtimeHash
			}
			token := issueRuntimeToken(t, f.key, issuer, "current", aud, rpc, id, hash)
			if name == "substituted_subject" {
				req.ProfileId = uuid.NewString()
			}
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)
			if name == "raw_actor" {
				md.Set("x-voice-profile-id", req.ProfileId)
			}
			if name == "duplicate_auth" {
				md.Append("authorization", "Bearer "+token)
			}
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), md), 2*time.Second)
			defer cancel()
			_, err = client.CheckPermission(ctx, req)
			if name == "channel" {
				require.NoError(t, err)
				<-recorder.principals
				fresh, cancelReplay := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), md.Copy()), 2*time.Second)
				defer cancelReplay()
				_, err = client.CheckPermission(fresh, req)
				require.NoError(t, fresh.Err())
				require.Equal(t, codes.Unauthenticated, status.Code(err))
			} else {
				require.Error(t, err)
				require.NotEqual(t, codes.Unimplemented, status.Code(err))
			}
		})
	}
	require.EqualValues(t, 1, recorder.calls.Load(), "invalid purpose/body/transport must not reach decision handler")
}

func TestChatCurrentACLRequiresExplicitTrust(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	r, err := New(context.Background(), f.config)
	require.NoError(t, err)
	defer r.Close()
	_, err = r.VerifyChatCurrentACL(context.Background(), "unused", rolev1.RoleService_CheckPermission_FullMethodName, uuid.NewString(), runtimeHash)
	require.Error(t, err)
}
