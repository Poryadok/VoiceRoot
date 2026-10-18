package s2s

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type installationClientFunc func(context.Context, *botv1.CheckBotInstallationScopeRequest) (*botv1.CheckBotInstallationScopeResponse, error)

func (f installationClientFunc) CheckBotInstallationScope(ctx context.Context, req *botv1.CheckBotInstallationScopeRequest, _ ...grpc.CallOption) (*botv1.CheckBotInstallationScopeResponse, error) {
	return f(ctx, req)
}

func TestBotInstallationDecisionBindsActualServiceAndRejectsSubstitution(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	binding := &spacev1.SpaceChatCreationBinding{ProtocolVersion: 1, OperationId: uuid.NewString(), CreatorProfileId: uuid.NewString(), SpaceId: uuid.NewString(), ChatId: uuid.NewString(), ChatType: chatv1.ChatType_CHAT_TYPE_GROUP, NodeId: uuid.NewString(), AuditEventId: uuid.NewString(), CreateRequestSha256: make([]byte, 32), Initiator: &spacev1.SpaceChatCreationBinding_Bot{Bot: &spacev1.SpaceChatCreationBot{BotId: uuid.NewString(), InstallationId: uuid.NewString(), BotScope: "TEXT_CHAT_CREATE_IN_SPACE"}}}
	for _, scenario := range []string{"allowed", "denied", "changed bot", "changed installation", "changed actor", "unknown field", "nil response", "authentication error"} {
		t.Run(scenario, func(t *testing.T) {
			client := installationClientFunc(func(ctx context.Context, req *botv1.CheckBotInstallationScopeRequest) (*botv1.CheckBotInstallationScopeResponse, error) {
				md, ok := metadata.FromOutgoingContext(ctx)
				require.True(t, ok)
				require.Len(t, md, 2)
				hash, err := principal.RequestHash(req)
				require.NoError(t, err)
				p, err := principal.VerifyService(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "bot", ExpectedRPC: botv1.BotService_CheckBotInstallationScope_FullMethodName, ExpectedRequestID: binding.OperationId, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
				require.NoError(t, err)
				require.Equal(t, "service:space", p.Subject)
				require.Equal(t, binding.SpaceId, req.SpaceId)
				require.Equal(t, binding.CreatorProfileId, req.ActorProfileId)
				require.Equal(t, binding.GetBot().InstallationId, req.InstallationId)
				out := &botv1.CheckBotInstallationScopeResponse{Allowed: true, BotId: req.BotId, SpaceId: req.SpaceId, ActorProfileId: req.ActorProfileId, InstallationId: req.InstallationId, BotScope: req.BotScope}
				switch scenario {
				case "denied":
					out.Allowed = false
				case "changed bot":
					out.BotId = uuid.NewString()
				case "changed installation":
					out.InstallationId = uuid.NewString()
				case "changed actor":
					out.ActorProfileId = uuid.NewString()
				case "unknown field":
					out.ProtoReflect().SetUnknown([]byte{0x78, 1})
				case "nil response":
					return nil, nil
				case "authentication error":
					return nil, status.Error(codes.PermissionDenied, "wrong listener")
				}
				return out, nil
			})
			decision := &BotInstallationDecision{Issuer: issuer, Client: client}
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", uuid.NewString()))
			code := status.Code(decision.CheckCurrentBotInstallation(ctx, binding))
			expected := codes.Unavailable
			if scenario == "allowed" {
				expected = codes.OK
			}
			if scenario == "denied" {
				expected = codes.PermissionDenied
			}
			require.Equal(t, expected, code)
		})
	}
}
