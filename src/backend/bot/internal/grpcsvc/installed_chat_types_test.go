package grpcsvc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/dispatch"
	"voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
)

type installedChatLookup struct {
	chatv1.ChatServiceClient
	lookup func(context.Context, *chatv1.GetChatRequest) (*chatv1.GetChatResponse, error)
}

func (c installedChatLookup) GetChat(ctx context.Context, req *chatv1.GetChatRequest, _ ...grpc.CallOption) (*chatv1.GetChatResponse, error) {
	return c.lookup(ctx, req)
}

func TestListInstalledBots_ActualChatTypes(t *testing.T) {
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "installedtypes", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	space, profile := uuid.New(), uuid.New()
	ids := make([]uuid.UUID, 8)
	for i := range ids {
		ids[i] = uuid.New()
	}
	bot, _, err := st.CreateBot(ctx, uuid.New(), "TypedBot", "", `[]`, uuid.New())
	require.NoError(t, err)
	_, err = installSendFixture(t, st, ctx, bot.ID, space, profile, ids)
	require.NoError(t, err)
	svc := grpcsvc.NewBotGRPC(st, dispatch.NewHub())
	request := &botv1.ListInstalledBotsRequest{SpaceId: space.String()}
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-profile-id", profile.String()))

	t.Run("missing owner client never guesses a channel", func(t *testing.T) {
		got, err := svc.ListInstalledBots(ctx, request)
		require.NoError(t, err)
		require.Len(t, got.GetInstalledBots(), 1)
		require.Len(t, got.GetInstalledBots()[0].GetAllowedChats(), len(ids))
		for _, ref := range got.GetInstalledBots()[0].GetAllowedChats() {
			require.Nil(t, ref.Type)
		}
	})

	groupType := chatv1.ChatType_CHAT_TYPE_GROUP
	spaceValue, foreignSpace := space.String(), uuid.NewString()
	lookupCalls := make(map[string]int)
	svc.Chat = installedChatLookup{lookup: func(callCtx context.Context, req *chatv1.GetChatRequest) (*chatv1.GetChatResponse, error) {
		md, _ := metadata.FromOutgoingContext(callCtx)
		require.Equal(t, []string{profile.String()}, md.Get("x-voice-profile-id"))
		require.Empty(t, md.Get("x-voice-internal"), "hydration cannot invent elevated authority")
		lookupCalls[req.GetChatId()]++
		chat := &chatv1.Chat{Id: req.GetChatId(), SpaceId: &spaceValue, Type: groupType}
		switch req.GetChatId() {
		case ids[1].String():
			chat.Type = chatv1.ChatType_CHAT_TYPE_CHANNEL
		case ids[2].String():
			return &chatv1.GetChatResponse{}, nil
		case ids[3].String():
			return nil, status.Error(codes.PermissionDenied, "private owner detail")
		case ids[4].String():
			chat.Id = uuid.NewString()
		case ids[5].String():
			chat.SpaceId = &foreignSpace
		case ids[6].String():
			chat.Type = chatv1.ChatType_CHAT_TYPE_DM
		case ids[7].String():
			return nil, status.Error(codes.Unavailable, "private owner outage")
		}
		return &chatv1.GetChatResponse{Chat: chat}, nil
	}}
	for _, want := range []chatv1.ChatType{chatv1.ChatType_CHAT_TYPE_GROUP, chatv1.ChatType_CHAT_TYPE_CHANNEL} {
		groupType = want
		got, err := svc.ListInstalledBots(ctx, request)
		require.NoError(t, err)
		require.Len(t, got.GetInstalledBots(), 1)
		refs := got.GetInstalledBots()[0].GetAllowedChats()
		require.Len(t, refs, len(ids))
		byID := make(map[string]*chatv1.ChatRef)
		for _, ref := range refs {
			byID[ref.GetId()] = ref
		}
		require.Equal(t, want, byID[ids[0].String()].GetType())
		require.Equal(t, chatv1.ChatType_CHAT_TYPE_CHANNEL, byID[ids[1].String()].GetType())
		for _, id := range ids[2:] {
			require.Contains(t, byID, id.String())
			require.Nil(t, byID[id.String()].Type, "unresolved/foreign/unsupported Chat must keep type absent")
		}
	}
	for _, id := range ids {
		require.Equal(t, 2, lookupCalls[id.String()], "each request observes fresh Chat-owned type")
	}
	second, _, err := st.CreateBot(ctx, uuid.New(), "SharedChatBot", "", `[]`, uuid.New())
	require.NoError(t, err)
	_, err = installSendFixture(t, st, ctx, second.ID, space, profile, ids[:1])
	require.NoError(t, err)
	lookupCalls = make(map[string]int)
	got, err := svc.ListInstalledBots(ctx, request)
	require.NoError(t, err)
	for _, installed := range got.GetInstalledBots() {
		for _, ref := range installed.GetAllowedChats() {
			if ref.GetId() == ids[0].String() {
				require.Equal(t, groupType, ref.GetType())
			}
		}
	}
	require.Len(t, got.GetInstalledBots(), 2)
	require.Equal(t, 1, lookupCalls[ids[0].String()], "one owner lookup per distinct Chat per request")
}
