package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

// Controlled signed actor isolates the handler gate; real Auth floor/Gateway
// transport and actual Role decisions have their separate cross-process oracle.
func verifiedMutationContext(t *testing.T, request proto.Message, method string, account, profile uuid.UUID) context.Context {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	id := uuid.NewString()
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "bot", RPC: method, RequestID: id, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 9, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	p, err := principal.VerifyDelegatedUser(t.Context(), token, principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "bot", ExpectedRPC: method, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, SessionEpochChecker: func(context.Context, string, int64) error { return nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }})
	require.NoError(t, err)
	return principal.WithVerified(t.Context(), p)
}

func TestSetBotChatEnabledCannotUseMissingPrivateRoleAuthority(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL mutation permission boundary")
	}
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "bot-enable-private-role", "")
	paths, err := filepath.Glob("../../../migrations/bot_db/*.up.sql")
	require.NoError(t, err)
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	st := &store.BotStore{Pool: pool}
	owner, profile, space, chat := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Enable permission", "", `[]`, uuid.New())
	require.NoError(t, err)
	_, err = st.InstallInSpace(ctx, bot.ID, space, profile, []uuid.UUID{chat})
	require.NoError(t, err)
	request := &botv1.SetBotChatEnabledRequest{BotId: bot.ID.String(), SpaceId: space.String(), Chat: &chatv1.ChatRef{Id: chat.String()}, Enabled: false}
	service := NewBotGRPC(st, nil)
	_, err = service.SetBotChatEnabled(verifiedMutationContext(t, request, botv1.BotService_SetBotChatEnabled_FullMethodName, owner, profile), request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "no nil or ordinary Role fallback for an enabling/disabling mutation")
	enabled, err := st.IsChatWhitelisted(ctx, bot.ID, chat)
	require.NoError(t, err)
	require.True(t, enabled)
}

func TestMutationEntryPointsRejectRawInitiationBeforeReadingTargets(t *testing.T) {
	service := NewBotGRPC(nil, nil)
	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("x-voice-user-id", uuid.NewString(), "x-voice-profile-id", uuid.NewString(), "x-voice-session-epoch", "9"))
	for name, call := range map[string]func() error{
		"install": func() error { _, err := service.InstallBotInSpace(ctx, &botv1.InstallBotInSpaceRequest{}); return err },
		"uninstall": func() error {
			_, err := service.UninstallBotFromSpace(ctx, &botv1.UninstallBotFromSpaceRequest{})
			return err
		},
		"whitelist": func() error { _, err := service.SetChatWhitelist(ctx, &botv1.SetChatWhitelistRequest{}); return err },
		"enabled":   func() error { _, err := service.SetBotChatEnabled(ctx, &botv1.SetBotChatEnabledRequest{}); return err },
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, codes.Unauthenticated, status.Code(call()), "raw caller metadata must not reach target parsing, owner reads or effects")
		})
	}
}
