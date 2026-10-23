package principalgrpc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestWorkloadDecisionExactScopeAndIdentity(t *testing.T) {
	for issuer, permissions := range map[string][]string{
		"messaging": {"TEXT_CHAT_SEND_MESSAGES", "TEXT_CHAT_PIN_MESSAGES", "TEXT_CHAT_ADD_REACTIONS", "TEXT_CHAT_MENTION_ALL_IN_CHAT", "TEXT_CHAT_MENTION_ALL_ONLINE", "TEXT_CHAT_CREATE_THREADS", "TEXT_CHAT_SEND_IN_THREADS"},
		"voice":     {"VOICE_JOIN", "VOICE_SPEAK", "VOICE_SCREEN_SHARE", "VOICE_MUTE_OTHERS", "VOICE_MOVE_OTHERS"},
	} {
		for _, permission := range permissions {
			t.Run(permission, func(t *testing.T) {
				req := &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: permission}
				if issuer == "messaging" {
					req.Chat = &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()}
				} else {
					room := uuid.NewString()
					req.VoiceRoomId = &room
				}
				hash, err := principal.RequestHash(req)
				require.NoError(t, err)
				p := principal.Principal{Kind: "service", Issuer: issuer, Subject: "service:" + issuer, Audience: "role", RPC: rolev1.RoleService_CheckPermission_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, ExpiresAt: time.Now().Add(time.Minute)}
				require.Equal(t, issuer, WorkloadDecisionIssuer(permission))
				require.NoError(t, RequireWorkloadDecision(principal.WithVerified(context.Background(), p), req))
				require.Error(t, RequireWorkloadDecision(context.Background(), req))
				for name, change := range map[string]func(*principal.Principal){
					"wrong_issuer": func(p *principal.Principal) { p.Issuer = "space" }, "actor": func(p *principal.Principal) { p.ProfileID = uuid.NewString() },
					"bot": func(p *principal.Principal) { p.BotID = uuid.NewString() }, "expired": func(p *principal.Principal) { p.ExpiresAt = time.Now().Add(-time.Second) },
					"hash": func(p *principal.Principal) { p.RequestHash = "wrong" }, "method": func(p *principal.Principal) { p.RPC = "other" },
				} {
					t.Run(name, func(t *testing.T) {
						bad := p
						change(&bad)
						require.Error(t, RequireWorkloadDecision(principal.WithVerified(context.Background(), bad), req))
					})
				}
				for name, change := range map[string]func(*rolev1.CheckPermissionRequest){
					"missing_scope": func(r *rolev1.CheckPermissionRequest) { r.Chat = nil; r.VoiceRoomId = nil },
					"mixed_scope": func(r *rolev1.CheckPermissionRequest) {
						r.Chat = &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()}
						room := uuid.NewString()
						r.VoiceRoomId = &room
					},
					"unknown":          func(r *rolev1.CheckPermissionRequest) { r.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01}) },
					"wrong_permission": func(r *rolev1.CheckPermissionRequest) { r.PermissionName = "SPACE_MANAGE_ROLES" },
					"invalid_profile":  func(r *rolev1.CheckPermissionRequest) { r.ProfileId = "other" },
				} {
					t.Run(name, func(t *testing.T) {
						bad := proto.Clone(req).(*rolev1.CheckPermissionRequest)
						change(bad)
						bound := p
						bound.RequestHash, err = principal.RequestHash(bad)
						require.NoError(t, err)
						require.Error(t, RequireWorkloadDecision(principal.WithVerified(context.Background(), bound), bad))
					})
				}
			})
		}
	}
	require.Empty(t, WorkloadDecisionIssuer("TEXT_CHAT_VIEW"))
	require.Empty(t, WorkloadDecisionIssuer("VOICE_VIDEO"))
}
