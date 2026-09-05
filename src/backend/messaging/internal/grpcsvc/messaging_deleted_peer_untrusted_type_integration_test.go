package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

// untrustedChatRef deliberately permits an omitted or forged client type. The
// authoritative type is stored in chat_db.chats and must control DM policy.
func untrustedChatRef(chatID uuid.UUID, typ *chatv1.ChatType) *chatv1.ChatRef {
	return &chatv1.ChatRef{Id: chatID.String(), Type: typ}
}

// testAuthoritativeChatTypeResolver describes the required Messaging seam.
// The caller's ChatRef.type is deliberately absent from this contract.
type testAuthoritativeChatTypeResolver interface {
	ResolveChatType(context.Context, uuid.UUID, uuid.UUID) (chatv1.ChatType, error)
}

type sqlTestAuthoritativeChatTypeResolver struct{ pool *pgxpool.Pool }

func (r sqlTestAuthoritativeChatTypeResolver) ResolveChatType(ctx context.Context, chatID, _ uuid.UUID) (chatv1.ChatType, error) {
	var raw string
	if err := r.pool.QueryRow(ctx, `SELECT type FROM chats WHERE id = $1`, chatID).Scan(&raw); err != nil {
		return chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, err
	}
	switch raw {
	case "dm":
		return chatv1.ChatType_CHAT_TYPE_DM, nil
	case "group":
		return chatv1.ChatType_CHAT_TYPE_GROUP, nil
	case "channel":
		return chatv1.ChatType_CHAT_TYPE_CHANNEL, nil
	default:
		return chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, nil
	}
}

type faultingTestAuthoritativeChatTypeResolver struct {
	typ chatv1.ChatType
	err error
}

func (r faultingTestAuthoritativeChatTypeResolver) ResolveChatType(context.Context, uuid.UUID, uuid.UUID) (chatv1.ChatType, error) {
	return r.typ, r.err
}

func TestMessagingDeletedPeer_AuthoritativeDMRejectsUntrustedTypeOnSendBeforeIdempotencyOrEvents(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyDeletedPeerMessagingMigrations(t, ctx, pool)

	chatID := uuid.New()
	profA, profB := uuid.New(), uuid.New()
	acctA, acctB := uuid.New(), uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)
	deleted := &recordingDeletedAccounts{deleted: map[uuid.UUID]struct{}{acctB: {}}}
	events := &spyMessageEvents{}
	client, _ := startMessagingServerWired(t, pool, messagingWire{
		UserProfiles:               profileAcctMap{profA: acctA, profB: acctB},
		DeletedAccounts:            deleted,
		RequireDeletedAccountsSeam: true,
		MessageEvents:              events,
	})

	group := chatv1.ChatType_CHAT_TYPE_GROUP
	channel := chatv1.ChatType_CHAT_TYPE_CHANNEL
	for _, tc := range []struct {
		name string
		typ  *chatv1.ChatType
	}{
		{name: "id only", typ: nil},
		{name: "forged group", typ: &group},
		{name: "forged channel", typ: &channel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deleted.mu.Lock()
			deleted.deleted = map[uuid.UUID]struct{}{}
			deleted.mu.Unlock()
			clientMessageID := uuid.New().String()
			created := sendDeletedPeerTestMessage(t, ctx, client, acctA, profA, chatDMRef(chatID), "created before deletion", &clientMessageID)
			require.NotEmpty(t, created.GetId())
			beforeReplayRows := messageCountForDeletedPeerTest(t, ctx, pool, chatID)
			require.Positive(t, beforeReplayRows)

			deleted.mu.Lock()
			deleted.deleted[acctB] = struct{}{}
			deleted.mu.Unlock()
			deleted.resetCalls()
			events.reset()
			_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
				Chat:            untrustedChatRef(chatID, tc.typ),
				Content:         "created before deletion",
				AttachmentsJson: "[]",
				MentionsJson:    "[]",
				ClientMessageId: &clientMessageID,
			})
			requireDeletedPeerPermissionDenied(t, err, acctA, acctB)
			require.Equal(t, beforeReplayRows, messageCountForDeletedPeerTest(t, ctx, pool, chatID), "the deleted-peer gate must run before returning an idempotency replay")
			require.Zero(t, events.eventCount(), "denied writes must not publish events")
			require.NotEmpty(t, deleted.calls(), "the authoritative DM gate must consult Auth")

			freshClientMessageID := uuid.New().String()
			_, err = client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
				Chat:            untrustedChatRef(chatID, tc.typ),
				Content:         "fresh write after deletion must be denied",
				AttachmentsJson: "[]",
				MentionsJson:    "[]",
				ClientMessageId: &freshClientMessageID,
			})
			requireDeletedPeerPermissionDenied(t, err, acctA, acctB)
			require.Equal(t, beforeReplayRows, messageCountForDeletedPeerTest(t, ctx, pool, chatID), "a fresh idempotency key must not bypass the authoritative DM gate")
			require.Zero(t, events.eventCount(), "denied fresh writes must not publish events")
		})
	}
}

func TestMessagingDeletedPeer_AuthoritativeDMRejectsUntrustedTypeOnForwardBeforeCommentaryOrEvents(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyDeletedPeerMessagingMigrations(t, ctx, pool)

	targetChatID, sourceChatID := uuid.New(), uuid.New()
	profA, profB := uuid.New(), uuid.New()
	acctA, acctB := uuid.New(), uuid.New()
	seedDMChat(t, ctx, pool, targetChatID, profA, profB)
	seedGroupChat(t, ctx, pool, sourceChatID, profA, profB)
	deleted := &recordingDeletedAccounts{deleted: map[uuid.UUID]struct{}{acctB: {}}}
	events := &spyMessageEvents{}
	client, _ := startMessagingServerWired(t, pool, messagingWire{
		UserProfiles:               profileAcctMap{profA: acctA, profB: acctB},
		DeletedAccounts:            deleted,
		RequireDeletedAccountsSeam: true,
		MessageEvents:              events,
	})
	original := sendDeletedPeerTestMessage(t, ctx, client, acctA, profA, chatGroupRef(sourceChatID), "forward source", nil)
	events.reset()

	group := chatv1.ChatType_CHAT_TYPE_GROUP
	channel := chatv1.ChatType_CHAT_TYPE_CHANNEL
	for _, tc := range []struct {
		name string
		typ  *chatv1.ChatType
	}{
		{name: "id only", typ: nil},
		{name: "forged group", typ: &group},
		{name: "forged channel", typ: &channel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deleted.resetCalls()
			commentary := "must never be persisted"
			_, err := client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
				SourceMessageId: original.GetId(),
				TargetChat:      untrustedChatRef(targetChatID, tc.typ),
				Commentary:      &commentary,
			})
			requireDeletedPeerPermissionDenied(t, err, acctA, acctB)
			require.Zero(t, messageCountForDeletedPeerTest(t, ctx, pool, targetChatID), "neither commentary nor forwarded message may be inserted")
			require.Zero(t, events.eventCount(), "denied forward must not publish events")
			require.NotEmpty(t, deleted.calls(), "the authoritative target DM gate must consult Auth")
		})
	}
}

func TestMessagingGetMessages_AuthoritativeDMIgnoresUntrustedTypeAndPreservesHistory(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyDeletedPeerMessagingMigrations(t, ctx, pool)

	chatID := uuid.New()
	profA, profB := uuid.New(), uuid.New()
	acctA, acctB := uuid.New(), uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)
	deleted := &recordingDeletedAccounts{deleted: map[uuid.UUID]struct{}{}}
	events := &spyMessageEvents{}
	client, _ := startMessagingServerWired(t, pool, messagingWire{
		UserProfiles:               profileAcctMap{profA: acctA, profB: acctB},
		DeletedAccounts:            deleted,
		RequireDeletedAccountsSeam: true,
		MessageEvents:              events,
	})
	for _, content := range []string{"oldest", "middle", "newest"} {
		sendDeletedPeerTestMessage(t, ctx, client, acctA, profA, chatDMRef(chatID), content, nil)
	}
	active, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID), Page: &commonv1.CursorPageRequest{PageSize: 2},
	})
	require.NoError(t, err)
	require.Equal(t, messagingv1.DmPeerState_DM_PEER_STATE_ACTIVE, active.GetDmPeerState())
	require.Len(t, active.GetMessageList().GetMessages(), 2)
	require.True(t, active.GetMessageList().GetHasMore())
	require.NotEmpty(t, active.GetMessageList().GetNextCursor())

	deleted.mu.Lock()
	deleted.deleted[acctB] = struct{}{}
	deleted.mu.Unlock()
	deleted.resetCalls()
	events.reset()
	group := chatv1.ChatType_CHAT_TYPE_GROUP
	channel := chatv1.ChatType_CHAT_TYPE_CHANNEL
	for _, tc := range []struct {
		name string
		typ  *chatv1.ChatType
	}{
		{name: "id only", typ: nil},
		{name: "forged group", typ: &group},
		{name: "forged channel", typ: &channel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deleted.resetCalls()
			resp, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
				Chat: untrustedChatRef(chatID, tc.typ), Page: &commonv1.CursorPageRequest{PageSize: 2},
			})
			require.NoError(t, err)
			require.Equal(t, messagingv1.DmPeerState_DM_PEER_STATE_DELETED, resp.GetDmPeerState())
			require.Equal(t, active.GetMessageList(), resp.GetMessageList(), "peer state must not alter real rows, order, or cursor")
			require.Equal(t, 3, messageCountForDeletedPeerTest(t, ctx, pool, chatID), "history must not gain a synthetic deletion row")
			require.Zero(t, events.eventCount(), "history reads must not publish events")
			require.NotEmpty(t, deleted.calls(), "deleted state must use the authoritative DM type")
		})
	}
}

func TestMessagingDeletedPeer_AuthoritativeGroupAndChannelIgnoreForgedDMType(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyDeletedPeerMessagingMigrations(t, ctx, pool)

	groupID, channelID, sourceChatID := uuid.New(), uuid.New(), uuid.New()
	profA, profB := uuid.New(), uuid.New()
	acctA, acctB := uuid.New(), uuid.New()
	seedGroupChat(t, ctx, pool, groupID, profA, profB)
	seedChannelChat(t, ctx, pool, channelID, profA)
	seedGroupChat(t, ctx, pool, sourceChatID, profA, profB)
	_, err := pool.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member')`, channelID, profB)
	require.NoError(t, err)
	profiles := &recordingProfileAccounts{accountIDs: map[uuid.UUID]uuid.UUID{profA: acctA, profB: acctB}}
	deleted := &recordingDeletedAccounts{deleted: map[uuid.UUID]struct{}{acctB: {}}}
	client, _ := startMessagingServerWired(t, pool, messagingWire{
		UserProfiles:               profiles,
		DeletedAccounts:            deleted,
		RequireDeletedAccountsSeam: true,
	})
	original := sendDeletedPeerTestMessage(t, ctx, client, acctA, profA, chatGroupRef(sourceChatID), "forward source", nil)
	deleted.resetCalls()

	dm := chatv1.ChatType_CHAT_TYPE_DM
	for _, tc := range []struct {
		name         string
		chatID       uuid.UUID
		postedAsChat *bool
	}{
		{name: "group", chatID: groupID},
		{name: "channel", chatID: channelID, postedAsChat: boolPtr(true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deleted.resetCalls()
			_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
				Chat:            untrustedChatRef(tc.chatID, &dm),
				Content:         "stored non-DM type must remain available",
				AttachmentsJson: "[]",
				MentionsJson:    "[]",
				PostedAsChat:    tc.postedAsChat,
			})
			require.NoError(t, err)
			require.Empty(t, deleted.calls(), "authoritative non-DM chats must not call Auth")
			require.Empty(t, profiles.calledProfiles(), "authoritative non-DM chats must not map profiles to accounts")

			resp, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
				Chat: untrustedChatRef(tc.chatID, &dm), Page: &commonv1.CursorPageRequest{PageSize: 10},
			})
			require.NoError(t, err)
			require.Nil(t, resp.DmPeerState, "authoritative non-DM history must omit the optional DM state")
			require.Empty(t, deleted.calls(), "authoritative non-DM history must not call Auth")
			require.Empty(t, profiles.calledProfiles(), "authoritative non-DM history must not map profiles to accounts")

			commentary := "forward commentary in authoritative non-DM chat"
			_, err = client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
				SourceMessageId: original.GetId(),
				TargetChat:      untrustedChatRef(tc.chatID, &dm),
				Commentary:      &commentary,
			})
			require.NoError(t, err)
			require.Empty(t, deleted.calls(), "authoritative non-DM forwards must not call Auth")
			require.Empty(t, profiles.calledProfiles(), "authoritative non-DM forwards must not map profiles to accounts")
			require.Equal(t, 3, messageCountForDeletedPeerTest(t, ctx, pool, tc.chatID), "send, commentary, and forward rows must be retained")
			var commentaryRows, forwardRows int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE chat_id = $1 AND type = 'regular' AND content = $2`, tc.chatID, commentary).Scan(&commentaryRows))
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE chat_id = $1 AND type = 'forward'`, tc.chatID).Scan(&forwardRows))
			require.Equal(t, 1, commentaryRows)
			require.Equal(t, 1, forwardRows)
		})
	}
}

// The resolver is a required security dependency: it is not permissible to
// fall back to the caller-controlled ChatRef.type when the authoritative Chat
// lookup is missing, fails, or returns no recognized chat type.
func TestMessagingDeletedPeer_AuthoritativeChatTypeResolverFailsClosed(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyDeletedPeerMessagingMigrations(t, ctx, pool)

	targetChatID, sourceChatID := uuid.New(), uuid.New()
	profA, profB := uuid.New(), uuid.New()
	acctA, acctB := uuid.New(), uuid.New()
	seedDMChat(t, ctx, pool, targetChatID, profA, profB)
	seedGroupChat(t, ctx, pool, sourceChatID, profA, profB)
	sourceMessageID := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx, `
INSERT INTO messages (id, chat_id, chat_type, sender_profile_id, content, attachments, mentions)
VALUES ($1, $2, 'group', $3, 'source must remain unread by failed target gate', '[]', '[]')`, sourceMessageID, sourceChatID, profA)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO messages (id, chat_id, chat_type, sender_profile_id, content, attachments, mentions)
VALUES ($1, $2, 'dm', $3, 'history must not leak around failed type lookup', '[]', '[]')`, uuid.Must(uuid.NewV7()), targetChatID, profB)
	require.NoError(t, err)

	for _, tc := range []struct {
		name     string
		resolver testAuthoritativeChatTypeResolver
	}{
		{name: "missing resolver"},
		{name: "resolver error", resolver: faultingTestAuthoritativeChatTypeResolver{err: errors.New("chat unavailable")}},
		{name: "malformed type", resolver: faultingTestAuthoritativeChatTypeResolver{typ: chatv1.ChatType_CHAT_TYPE_UNSPECIFIED}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := &spyMessageEvents{}
			client, _ := startMessagingServerWired(t, pool, messagingWire{
				UserProfiles:               profileAcctMap{profA: acctA, profB: acctB},
				DeletedAccounts:            allowDeletedAccounts{},
				RequireDeletedAccountsSeam: true,
				ChatTypeResolver:           tc.resolver,
				RequireChatTypeResolver:    true,
				MessageEvents:              events,
			})

			beforeRows := messageCountForDeletedPeerTest(t, ctx, pool, targetChatID)
			_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
				Chat: untrustedChatRef(targetChatID, nil), Content: "must not write", AttachmentsJson: "[]", MentionsJson: "[]",
			})
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Equal(t, beforeRows, messageCountForDeletedPeerTest(t, ctx, pool, targetChatID))
			require.Zero(t, events.eventCount())

			commentary := "must not be inserted"
			_, err = client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
				SourceMessageId: sourceMessageID.String(), TargetChat: untrustedChatRef(targetChatID, nil), Commentary: &commentary,
			})
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Equal(t, beforeRows, messageCountForDeletedPeerTest(t, ctx, pool, targetChatID))
			require.Zero(t, events.eventCount())

			resp, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
				Chat: untrustedChatRef(targetChatID, nil), Page: &commonv1.CursorPageRequest{PageSize: 10},
			})
			require.Nil(t, resp, "type lookup failure must not leak a partial history page")
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Equal(t, beforeRows, messageCountForDeletedPeerTest(t, ctx, pool, targetChatID))
			require.Zero(t, events.eventCount())
		})
	}
}

func boolPtr(v bool) *bool { return &v }
