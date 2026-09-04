package grpcsvc

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
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
	mu         sync.Mutex
	deleted    map[uuid.UUID]struct{}
	err        error
	accountIDs [][]uuid.UUID
}

type allowDeletedAccounts struct{}

func (allowDeletedAccounts) DeletedAmong(context.Context, []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	return map[uuid.UUID]struct{}{}, nil
}

type nilMapDeletedAccounts struct{}

func (nilMapDeletedAccounts) DeletedAmong(context.Context, []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	return nil, nil
}

type typedNilChatGuard struct {
	memberErr error
}

func (g *typedNilChatGuard) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error {
	return g.memberErr
}

func (g *typedNilChatGuard) DMOtherProfileID(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, g.memberErr
}

func (g *typedNilChatGuard) OtherMemberProfileIDs(context.Context, uuid.UUID, uuid.UUID) ([]uuid.UUID, error) {
	return nil, g.memberErr
}

func (g *typedNilChatGuard) MemberRole(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "", g.memberErr
}

type typedNilProfileAccounts struct {
	accountID uuid.UUID
}

func (p *typedNilProfileAccounts) AccountIDByProfileID(context.Context, uuid.UUID) (uuid.UUID, error) {
	return p.accountID, nil
}

func (c *recordingDeletedAccounts) DeletedAmong(_ context.Context, accountIDs []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accountIDs = append(c.accountIDs, append([]uuid.UUID(nil), accountIDs...))
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

func (c *recordingDeletedAccounts) calls() [][]uuid.UUID {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([][]uuid.UUID, len(c.accountIDs))
	for i, accountIDs := range c.accountIDs {
		out[i] = append([]uuid.UUID(nil), accountIDs...)
	}
	return out
}

type failingProfileAccounts struct{ err error }

func (f failingProfileAccounts) AccountIDByProfileID(context.Context, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, f.err
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

func requireDeletedPeerPermissionDenied(t *testing.T, err error, accountIDs ...uuid.UUID) {
	t.Helper()
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	message := status.Convert(err).Message()
	require.Equal(t, "permission denied", message, "deleted-peer denial must use the generic canonical description")
	for _, forbidden := range append([]string{"deleted", "account", "profile"}, uuidStrings(accountIDs)...) {
		require.NotContains(t, strings.ToLower(message), strings.ToLower(forbidden), "PermissionDenied must not disclose deleted-peer identity or state")
	}
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
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
		UserProfiles:               profileAcctMap{profA: acctA, profB: acctB},
		DeletedAccounts:            deleted,
		RequireDeletedAccountsSeam: true,
		MessageEvents:              events,
	})

	for _, sender := range []struct {
		account uuid.UUID
		profile uuid.UUID
		content string
	}{{acctA, profA, "survivor to deleted peer"}, {acctB, profB, "deleted peer to survivor"}} {
		_, err := client.SendMessage(withProfileCtx(ctx, sender.account, sender.profile), &messagingv1.SendMessageRequest{
			Chat: chatDMRef(chatID), Content: sender.content, AttachmentsJson: "[]", MentionsJson: "[]",
		})
		requireDeletedPeerPermissionDenied(t, err, acctA, acctB)
	}

	require.Equal(t, 0, messageCountForDeletedPeerTest(t, ctx, pool, chatID))
	require.Zero(t, events.eventCount(), "denied sends must not publish message events")
	require.Equal(t, [][]uuid.UUID{{acctA, acctB}, {acctB, acctA}}, deleted.calls(), "each valid DM attempt must query Auth in sender-then-peer account order")
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
		UserProfiles:               profileAcctMap{profA: acctA, profB: acctB, profSource: acctSource},
		DeletedAccounts:            deleted,
		RequireDeletedAccountsSeam: true,
		MessageEvents:              events,
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
		requireDeletedPeerPermissionDenied(t, err, acctA, acctB)
	}

	require.Equal(t, 0, messageCountForDeletedPeerTest(t, ctx, pool, targetChat), "commentary and forwarded message must both be absent")
	require.Zero(t, events.eventCount(), "denied forwards must not publish message events")
	require.Equal(t, [][]uuid.UUID{{acctA, acctB}, {acctB, acctA}}, deleted.calls(), "forward gates must query Auth in forwarder-then-target-peer account order")
}

func TestMessagingDeletedPeer_NonMemberDeniedBeforeDeletedAccountLookup(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyDeletedPeerMessagingMigrations(t, ctx, pool)

	deleted := &recordingDeletedAccounts{deleted: map[uuid.UUID]struct{}{uuid.New(): {}}}
	events := &spyMessageEvents{}
	client, _ := startMessagingServerWired(t, pool, messagingWire{
		ChatGuard:                  nonMemberGuard{},
		DeletedAccounts:            deleted,
		RequireDeletedAccountsSeam: true,
		MessageEvents:              events,
	})

	chatID := uuid.New()
	_, err := client.SendMessage(withProfileCtx(ctx, uuid.New(), uuid.New()), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "nonmember", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Empty(t, deleted.calls(), "membership denial must not call the Auth deletion checker")
	require.Equal(t, 0, messageCountForDeletedPeerTest(t, ctx, pool, chatID))
	require.Zero(t, events.eventCount())
}

func TestMessagingDeletedPeer_MissingOrFailingCheckerFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name     string
		checker  *recordingDeletedAccounts
		profiles ProfileAccountLookup
	}{
		{name: "missing checker"},
		{
			name:    "checker unavailable",
			checker: &recordingDeletedAccounts{err: errors.New("auth unavailable")},
		},
		{
			name:     "profile account mapping unavailable",
			checker:  &recordingDeletedAccounts{},
			profiles: failingProfileAccounts{err: errors.New("user unavailable")},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool := startPostgresForTest(t, ctx)
			applyDeletedPeerMessagingMigrations(t, ctx, pool)

			sourceChat, targetChat := uuid.New(), uuid.New()
			profA, profB := uuid.New(), uuid.New()
			acctA, acctB := uuid.New(), uuid.New()
			seedDMChat(t, ctx, pool, targetChat, profA, profB)
			seedGroupChat(t, ctx, pool, sourceChat, profA, profB)
			events := &spyMessageEvents{}
			profiles := tc.profiles
			if profiles == nil {
				profiles = profileAcctMap{profA: acctA, profB: acctB}
			}
			client, _ := startMessagingServerWired(t, pool, messagingWire{
				UserProfiles:               profiles,
				DeletedAccounts:            tc.checker,
				RequireDeletedAccountsSeam: true,
				MessageEvents:              events,
			})

			_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
				Chat: chatDMRef(targetChat), Content: "must fail closed", AttachmentsJson: "[]", MentionsJson: "[]",
			})
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Equal(t, 0, messageCountForDeletedPeerTest(t, ctx, pool, targetChat))
			require.Zero(t, events.eventCount())

			original := sendDeletedPeerTestMessage(t, ctx, client, acctA, profA, chatGroupRef(sourceChat), "forward source", nil)
			events.reset()
			commentary := "must never be inserted"
			_, err = client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
				SourceMessageId: original.GetId(), TargetChat: chatDMRef(targetChat), Commentary: &commentary,
			})
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Equal(t, 0, messageCountForDeletedPeerTest(t, ctx, pool, targetChat), "unavailable checker must deny before commentary and forward writes")
			require.Zero(t, events.eventCount())

			if tc.checker != nil {
				want := [][]uuid.UUID{}
				if tc.name == "checker unavailable" {
					want = [][]uuid.UUID{{acctA, acctB}, {acctA, acctB}}
				}
				require.Equal(t, want, tc.checker.calls())
			}
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
		UserProfiles:               profileAcctMap{profA: acctA, profB: acctB},
		DeletedAccounts:            deleted,
		RequireDeletedAccountsSeam: true,
		MessageEvents:              events,
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
	requireDeletedPeerPermissionDenied(t, err, acctA, acctB)
	require.Equal(t, 1, messageCountForDeletedPeerTest(t, ctx, pool, chatID), "replay may not create another row")
	require.Equal(t, 1, events.eventCount(), "replay after deletion may not publish another event")
	require.Equal(t, [][]uuid.UUID{{acctA, acctB}, {acctA, acctB}}, deleted.calls(), "both initial send and replay must query Auth in sender-then-peer account order before returning")
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
		UserProfiles:               profileAcctMap{profA: acctA, profB: acctB},
		DeletedAccounts:            deleted,
		RequireDeletedAccountsSeam: true,
		MessageEvents:              events,
	})

	sendDeletedPeerTestMessage(t, ctx, client, acctA, profA, chatGroupRef(groupID), "group remains available", nil)
	postedAsChat := true
	_, err = client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatChannelRef(channelID),
		Content:         "channel remains available",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
		PostedAsChat:    &postedAsChat,
	})
	require.NoError(t, err)
	require.Empty(t, deleted.calls(), "non-DM sends must not invoke the deleted-account checker")
	require.Equal(t, 1, messageCountForDeletedPeerTest(t, ctx, pool, groupID))
	require.Equal(t, 1, messageCountForDeletedPeerTest(t, ctx, pool, channelID))
	require.Equal(t, 2, events.eventCount())
}

func TestMessagingDeletedPeer_NilMapCheckerResponseFailsClosedBeforeSendAndForwardWrites(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyDeletedPeerMessagingMigrations(t, ctx, pool)

	targetChat, sourceChat := uuid.New(), uuid.New()
	profA, profB := uuid.New(), uuid.New()
	acctA, acctB := uuid.New(), uuid.New()
	seedDMChat(t, ctx, pool, targetChat, profA, profB)
	seedGroupChat(t, ctx, pool, sourceChat, profA, profB)
	events := &spyMessageEvents{}
	client, _ := startMessagingServerWired(t, pool, messagingWire{
		UserProfiles:               profileAcctMap{profA: acctA, profB: acctB},
		DeletedAccounts:            nilMapDeletedAccounts{},
		RequireDeletedAccountsSeam: true,
		MessageEvents:              events,
	})

	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(targetChat), Content: "must not write", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Zero(t, messageCountForDeletedPeerTest(t, ctx, pool, targetChat))
	require.Zero(t, events.eventCount())

	original := sendDeletedPeerTestMessage(t, ctx, client, acctA, profA, chatGroupRef(sourceChat), "source", nil)
	events.reset()
	commentary := "must not write"
	_, err = client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
		SourceMessageId: original.GetId(), TargetChat: chatDMRef(targetChat), Commentary: &commentary,
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Zero(t, messageCountForDeletedPeerTest(t, ctx, pool, targetChat))
	require.Zero(t, events.eventCount())
}

func TestMessagingDeletedPeer_TypedNilDependenciesFailClosed(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyDeletedPeerMessagingMigrations(t, ctx, pool)

	targetChat, sourceChat := uuid.New(), uuid.New()
	profA, profB := uuid.New(), uuid.New()
	acctA, acctB := uuid.New(), uuid.New()
	seedDMChat(t, ctx, pool, targetChat, profA, profB)
	seedGroupChat(t, ctx, pool, sourceChat, profA, profB)

	t.Run("chat guard", func(t *testing.T) {
		var guard *typedNilChatGuard
		events := &spyMessageEvents{}
		client, _ := startMessagingServerWired(t, pool, messagingWire{
			ChatGuard:                  guard,
			UserProfiles:               profileAcctMap{profA: acctA, profB: acctB},
			DeletedAccounts:            allowDeletedAccounts{},
			RequireDeletedAccountsSeam: true,
			MessageEvents:              events,
		})
		_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
			Chat: chatDMRef(targetChat), Content: "must fail closed", AttachmentsJson: "[]", MentionsJson: "[]",
		})
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Zero(t, messageCountForDeletedPeerTest(t, ctx, pool, targetChat))
		require.Zero(t, events.eventCount())
	})

	t.Run("profile lookup forward", func(t *testing.T) {
		var profiles *typedNilProfileAccounts
		events := &spyMessageEvents{}
		client, _ := startMessagingServerWired(t, pool, messagingWire{
			UserProfiles:               profiles,
			DeletedAccounts:            allowDeletedAccounts{},
			RequireDeletedAccountsSeam: true,
			MessageEvents:              events,
		})
		original := sendDeletedPeerTestMessage(t, ctx, client, acctA, profA, chatGroupRef(sourceChat), "source", nil)
		events.reset()
		commentary := "must not write"
		_, err := client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
			SourceMessageId: original.GetId(), TargetChat: chatDMRef(targetChat), Commentary: &commentary,
		})
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Zero(t, messageCountForDeletedPeerTest(t, ctx, pool, targetChat))
		require.Zero(t, events.eventCount())
	})
}
