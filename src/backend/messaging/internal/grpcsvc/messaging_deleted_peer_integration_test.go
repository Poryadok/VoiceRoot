package grpcsvc

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"

	"voice/backend/messaging/internal/store"
)

// testDeletedAccountChecker is the Auth S2S contract expected by T056 P3. It
// deliberately lives only in tests until Messaging wires the production adapter.
type testDeletedAccountChecker interface {
	DeletedAmong(context.Context, []uuid.UUID) (map[uuid.UUID]struct{}, error)
}

type recordingDeletedAccounts struct {
	mu      sync.Mutex
	deleted map[uuid.UUID]struct{}
	err     error
	calls   int
}

func (c *recordingDeletedAccounts) DeletedAmong(_ context.Context, accountIDs []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	out := make(map[uuid.UUID]struct{})
	for _, accountID := range accountIDs {
		if _, ok := c.deleted[accountID]; ok {
			out[accountID] = struct{}{}
		}
	}
	return out, nil
}

func (c *recordingDeletedAccounts) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

type nonMemberGuard struct{}

func (nonMemberGuard) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error {
	return store.ErrNotChatMember
}

func (nonMemberGuard) DMOtherProfileID(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, errors.New("DM peer lookup must not run for a non-member")
}

func (nonMemberGuard) OtherMemberProfileIDs(context.Context, uuid.UUID, uuid.UUID) ([]uuid.UUID, error) {
	return nil, errors.New("member lookup must not run for a non-member")
}

func (nonMemberGuard) MemberRole(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "", store.ErrNotChatMember
}

func applyDeletedPeerMessagingMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000003_groups.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000005_thread_settings.up.sql"))
	applyBaseMessagingMigrations(t, ctx, pool)
}

func messageCountForDeletedPeerTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool, chatID uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE chat_id = $1`, chatID).Scan(&count))
	return count
}

func sendDeletedPeerTestMessage(t *testing.T, ctx context.Context, client messagingv1.MessagingServiceClient, accountID, profileID uuid.UUID, chat *chatv1.ChatRef, content string, clientMessageID *string) *messagingv1.Message {
	t.Helper()
	resp, err := client.SendMessage(withProfileCtx(ctx, accountID, profileID), &messagingv1.SendMessageRequest{
		Chat:            chat,
		Content:         content,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
		ClientMessageId: clientMessageID,
	})
	require.NoError(t, err)
	return resp.GetMessage()
}

func TestMessagingDeletedPeer_SendMessageDeniedInBothDirectionsWithoutWritesOrEvents(t *testing.T) {
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
		UserProfiles:    profileAcctMap{profA: acctA, profB: acctB},
		DeletedAccounts: deleted,
		MessageEvents:   events,
	})

	for _, sender := range []struct {
		account uuid.UUID
		profile uuid.UUID
		content string
	}{{acctA, profA, "survivor to deleted peer"}, {acctB, profB, "deleted peer to survivor"}} {
		_, err := client.SendMessage(withProfileCtx(ctx, sender.account, sender.profile), &messagingv1.SendMessageRequest{
			Chat: chatDMRef(chatID), Content: sender.content, AttachmentsJson: "[]", MentionsJson: "[]",
		})
		require.Equal(t, codes.PermissionDenied, status.Code(err), "both DM directions must close after either account is deleted")
	}

	require.Equal(t, 0, messageCountForDeletedPeerTest(t, ctx, pool, chatID))
	require.Zero(t, events.eventCount(), "denied sends must not publish message events")
	require.GreaterOrEqual(t, deleted.callCount(), 2, "each valid DM attempt must consult Auth deletion state")
}

func TestMessagingDeletedPeer_ForwardMessageWithCommentaryDeniedBeforeWritesOrEvents(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyDeletedPeerMessagingMigrations(t, ctx, pool)

	sourceChat, targetChat := uuid.New(), uuid.New()
	profA, profB, profSource := uuid.New(), uuid.New(), uuid.New()
	acctA, acctB, acctSource := uuid.New(), uuid.New(), uuid.New()
	seedDMChat(t, ctx, pool, targetChat, profA, profB)
	seedGroupChat(t, ctx, pool, sourceChat, profA, profB)
	_, err := pool.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member')`, sourceChat, profSource)
	require.NoError(t, err)

	deleted := &recordingDeletedAccounts{deleted: map[uuid.UUID]struct{}{acctB: {}}}
	events := &spyMessageEvents{}
	client, _ := startMessagingServerWired(t, pool, messagingWire{
		UserProfiles:    profileAcctMap{profA: acctA, profB: acctB, profSource: acctSource},
		DeletedAccounts: deleted,
		MessageEvents:   events,
	})
	original := sendDeletedPeerTestMessage(t, ctx, client, acctA, profA, chatGroupRef(sourceChat), "forward source", nil)
	events.reset()

	commentary := "must never be inserted"
	for _, forwarder := range []struct {
		account uuid.UUID
		profile uuid.UUID
	}{{acctA, profA}, {acctB, profB}} {
		_, err := client.ForwardMessage(withProfileCtx(ctx, forwarder.account, forwarder.profile), &messagingv1.ForwardMessageRequest{
			SourceMessageId: original.GetId(), TargetChat: chatDMRef(targetChat), Commentary: &commentary,
		})
		require.Equal(t, codes.PermissionDenied, status.Code(err), "both DM directions must be denied before commentary or forwarded-message insertion")
	}

	require.Equal(t, 0, messageCountForDeletedPeerTest(t, ctx, pool, targetChat), "commentary and forwarded message must both be absent")
	require.Zero(t, events.eventCount(), "denied forwards must not publish message events")
}

func TestMessagingDeletedPeer_NonMemberDeniedBeforeDeletedAccountLookup(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyDeletedPeerMessagingMigrations(t, ctx, pool)

	deleted := &recordingDeletedAccounts{deleted: map[uuid.UUID]struct{}{uuid.New(): {}}}
	events := &spyMessageEvents{}
	client, _ := startMessagingServerWired(t, pool, messagingWire{
		ChatGuard:       nonMemberGuard{},
		DeletedAccounts: deleted,
		MessageEvents:   events,
	})

	chatID := uuid.New()
	_, err := client.SendMessage(withProfileCtx(ctx, uuid.New(), uuid.New()), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "nonmember", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Zero(t, deleted.callCount(), "membership denial must not call the Auth deletion checker")
	require.Equal(t, 0, messageCountForDeletedPeerTest(t, ctx, pool, chatID))
	require.Zero(t, events.eventCount())
}

func TestMessagingDeletedPeer_MissingOrFailingCheckerFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name    string
		checker testDeletedAccountChecker
	}{
		{name: "missing checker"},
		{name: "checker unavailable", checker: &recordingDeletedAccounts{err: errors.New("auth unavailable")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool := startPostgresForTest(t, ctx)
			applyDeletedPeerMessagingMigrations(t, ctx, pool)

			chatID := uuid.New()
			profA, profB := uuid.New(), uuid.New()
			acctA, acctB := uuid.New(), uuid.New()
			seedDMChat(t, ctx, pool, chatID, profA, profB)
			events := &spyMessageEvents{}
			client, _ := startMessagingServerWired(t, pool, messagingWire{
				UserProfiles:    profileAcctMap{profA: acctA, profB: acctB},
				DeletedAccounts: tc.checker,
				MessageEvents:   events,
			})

			_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
				Chat: chatDMRef(chatID), Content: "must fail closed", AttachmentsJson: "[]", MentionsJson: "[]",
			})
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Equal(t, 0, messageCountForDeletedPeerTest(t, ctx, pool, chatID))
			require.Zero(t, events.eventCount())
		})
	}
}

func TestMessagingDeletedPeer_IdempotentReplayDeniedAfterDeletion(t *testing.T) {
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
		UserProfiles:    profileAcctMap{profA: acctA, profB: acctB},
		DeletedAccounts: deleted,
		MessageEvents:   events,
	})

	clientMessageID := uuid.New().String()
	original := sendDeletedPeerTestMessage(t, ctx, client, acctA, profA, chatDMRef(chatID), "before deletion", &clientMessageID)
	require.NotEmpty(t, original.GetId())
	require.Equal(t, 1, messageCountForDeletedPeerTest(t, ctx, pool, chatID))
	require.Equal(t, 1, events.eventCount())

	deleted.mu.Lock()
	deleted.deleted[acctB] = struct{}{}
	deleted.mu.Unlock()
	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "before deletion", AttachmentsJson: "[]", MentionsJson: "[]", ClientMessageId: &clientMessageID,
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err), "deletion must win over a prior idempotency success")
	require.Equal(t, 1, messageCountForDeletedPeerTest(t, ctx, pool, chatID), "replay may not create another row")
	require.Equal(t, 1, events.eventCount(), "replay after deletion may not publish another event")
}

func TestMessagingDeletedPeer_GroupAndChannelDoNotConsultChecker(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyDeletedPeerMessagingMigrations(t, ctx, pool)

	groupID, channelID := uuid.New(), uuid.New()
	profA, profB := uuid.New(), uuid.New()
	acctA, acctB := uuid.New(), uuid.New()
	seedGroupChat(t, ctx, pool, groupID, profA, profB)
	seedChannelChat(t, ctx, pool, channelID, profA)
	_, err := pool.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member')`, channelID, profB)
	require.NoError(t, err)

	deleted := &recordingDeletedAccounts{deleted: map[uuid.UUID]struct{}{acctB: {}}}
	events := &spyMessageEvents{}
	client, _ := startMessagingServerWired(t, pool, messagingWire{
		UserProfiles:    profileAcctMap{profA: acctA, profB: acctB},
		DeletedAccounts: deleted,
		MessageEvents:   events,
	})

	sendDeletedPeerTestMessage(t, ctx, client, acctA, profA, chatGroupRef(groupID), "group remains available", nil)
	sendDeletedPeerTestMessage(t, ctx, client, acctA, profA, chatChannelRef(channelID), "channel remains available", nil)
	require.Zero(t, deleted.callCount(), "non-DM sends must not invoke the deleted-account checker")
	require.Equal(t, 1, messageCountForDeletedPeerTest(t, ctx, pool, groupID))
	require.Equal(t, 1, messageCountForDeletedPeerTest(t, ctx, pool, channelID))
	require.Equal(t, 2, events.eventCount())
}
