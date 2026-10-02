package grpcsvc

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
)

type gameActionMembership struct{ allowedProfile uuid.UUID }

func (g gameActionMembership) EnsureMember(_ context.Context, _ uuid.UUID, profileID uuid.UUID) error {
	if profileID != g.allowedProfile {
		return store.ErrNotChatMember
	}
	return nil
}

func (gameActionMembership) DMOtherProfileID(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (gameActionMembership) OtherMemberProfileIDs(context.Context, uuid.UUID, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

func (gameActionMembership) MemberRole(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "", nil
}

func TestResolveGameActionRequiresGISPrincipalAndCurrentMessagingMembership(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applyBaseMessagingMigrations(t, ctx, pool)
	chatID, author, member, messageID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedGroupChat(t, ctx, pool, chatID, author, member)
	card := fmt.Sprintf(`{"schema_version":1,"revision":"3","title":"Relic","safe_summary":"A relic","actions":[{"action_id":"%s","action_type":"relic.inspect","label":"Inspect","arguments_json":"{}","state_version":"s-3","expires_at":"%s"}]}`, uuid.NewString(), time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano))
	_, err := pool.Exec(ctx, `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,type,attachments,mentions,client_message_id,ghost_only,is_e2e)
		VALUES($1,$2,'group',$3,'Relic found','regular','[]'::jsonb,'[]'::jsonb,$4,false,false)`, messageID, chatID, author, uuid.New())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO message_game_cards(message_id,app_id,environment_id,installation_id,bot_id,card_json,card_sha256,actions_enabled)
		VALUES($1,$2,$3,$4,$5,$6::jsonb,$7,false)`, messageID, uuid.New(), uuid.New(), uuid.New(), uuid.New(), card, fmt.Sprintf("%x", sha256.Sum256([]byte(card))))
	require.NoError(t, err)
	service := &MessagingGRPC{Messages: &store.MessagesStore{Pool: pool}, ChatGuard: gameActionMembership{allowedProfile: member}}
	request := &messagingv1.ResolveGameActionRequest{MessageId: messageID.String(), ProfileId: member.String()}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	ctx = principal.WithVerified(ctx, principal.Principal{Kind: "service", Issuer: "gameintegration", Subject: "service:gameintegration", Audience: "messaging", RPC: messagingv1.MessagingService_ResolveGameAction_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash})
	response, err := service.ResolveGameAction(ctx, request)
	require.NoError(t, err)
	require.Equal(t, messageID.String(), response.GetMessage().GetId())
	require.Equal(t, "s-3", response.GetMessage().GetGameCard().GetActions()[0].GetStateVersion())
	foreign := &messagingv1.ResolveGameActionRequest{MessageId: messageID.String(), ProfileId: uuid.NewString()}
	foreignHash, err := principal.RequestHash(foreign)
	require.NoError(t, err)
	foreignCtx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "gameintegration", Subject: "service:gameintegration", Audience: "messaging", RPC: messagingv1.MessagingService_ResolveGameAction_FullMethodName, RequestID: uuid.NewString(), RequestHash: foreignHash})
	_, err = service.ResolveGameAction(foreignCtx, foreign)
	require.Equal(t, codes.NotFound, status.Code(err), "membership loss/foreign profile is concealed")
	badCtx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "bot", Subject: "service:bot", Audience: "messaging", RPC: messagingv1.MessagingService_ResolveGameAction_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash})
	_, err = service.ResolveGameAction(badCtx, request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
