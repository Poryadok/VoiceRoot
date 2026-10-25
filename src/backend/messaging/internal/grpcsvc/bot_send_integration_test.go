package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"os"
	"path/filepath"
	"testing"
	"time"
	botv1 "voice.app/voice/bot/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/messaging/internal/botsendprincipal"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spacecommunication"
	"voice/backend/role/permissions"
)

type botSendTestVerifier struct{ key *rsa.PublicKey }

func (v botSendTestVerifier) Verify(ctx context.Context, token, rpc, id, hash string) (principal.Principal, error) {
	return principal.VerifyService(ctx, token, principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "messaging", ExpectedRPC: rpc, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return v.key, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }})
}
func TestBotSendWritesWithoutViewAndRetainsOwnerChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL send/outbox/receipt pipeline")
	}
	ctx := t.Context()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applyBaseMessagingMigrations(t, ctx, pool)
	applyModerationSchemasForMessagingTest(t, ctx, pool)
	b := botSendBinding()
	chat, actor, space := uuid.MustParse(b.ChatId), uuid.MustParse(b.ActorProfileId), uuid.MustParse(b.SpaceId)
	seedGroupChat(t, ctx, pool, chat, uuid.New(), uuid.New())
	_, err := pool.Exec(ctx, `UPDATE chats SET space_id=$2 WHERE id=$1`, chat, space)
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	botIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	communication := &communicationStateStub{state: spacecommunication.Clear}
	policy := &store.SQLChatThreadPolicy{Pool: pool}
	svc := &MessagingGRPC{Messages: &store.MessagesStore{Pool: pool}, ChatThreadPolicy: policy, CommunicationStates: communication, RolePermissions: selectiveRolePerms{deny: map[string]bool{permissions.TextChatSendMessages: true}}}
	calls := 0
	revokeAt := 0
	expireDuringWrite := false
	authority := &BotSendAuthority{Issuer: issuer, Scope: policy, Bot: botSendOwnerFunc(func(callback context.Context, r *botv1.CheckBotMessageSendAuthorityRequest) (*botv1.CheckBotMessageSendAuthorityResponse, error) {
		calls++
		if expireDuringWrite && calls == 2 {
			deadline, ok := callback.Deadline()
			require.True(t, ok)
			require.Less(t, time.Until(deadline), 2*time.Second)
			<-callback.Done()
		}
		return &botv1.CheckBotMessageSendAuthorityResponse{Binding: r.Binding, Allowed: revokeAt == 0 || calls < revokeAt}, nil
	}), Space: botSendSpaceFunc(func(context.Context, *spacev1.GetMemberAdmissionAuthorityRequest) (*spacev1.GetMemberAdmissionAuthorityResponse, error) {
		return botSendObservation(b), nil
	})}
	receiver := &BotSendGRPC{Messaging: svc, Authority: authority}
	interceptor := botsendprincipal.UnaryServerInterceptor(botSendTestVerifier{&key.PublicKey})
	send := func(req *messagingv1.SendBotMessageRequest) (*messagingv1.SendBotMessageResponse, error) {
		hash, e := principal.RequestHash(req)
		require.NoError(t, e)
		id := uuid.NewString()
		rpc := messagingv1.MessagingService_SendBotMessage_FullMethodName
		token, e := botIssuer.IssueService(principal.ServiceInput{Audience: "messaging", RPC: rpc, RequestID: id, RequestHash: hash})
		require.NoError(t, e)
		incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
		value, e := interceptor(incoming, req, &grpc.UnaryServerInfo{FullMethod: rpc}, func(c context.Context, r any) (any, error) {
			return receiver.SendBotMessage(c, r.(*messagingv1.SendBotMessageRequest))
		})
		if e != nil {
			return nil, e
		}
		return value.(*messagingv1.SendBotMessageResponse), nil
	}
	request := &messagingv1.SendBotMessageRequest{Binding: b, Message: &messagingv1.SendMessageRequest{Chat: chatGroupRef(chat), Content: "Bot with SEND but no VIEW", ClientMessageId: proto.String(uuid.NewString())}}
	response, err := send(request)
	require.NoError(t, err)
	require.Equal(t, actor.String(), response.Message.SenderProfileId)
	require.Equal(t, 2, calls)
	replay, err := send(request)
	require.NoError(t, err)
	require.Equal(t, response.Message.Id, replay.Message.Id)
	changed := proto.Clone(request).(*messagingv1.SendBotMessageRequest)
	changed.Message.Content = "different request at same key"
	_, err = send(changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	oldBinding := proto.Clone(b)
	b.InstallationId = uuid.NewString()
	b.InstallOperationId = uuid.NewString()
	b.InstallationGeneration++
	_, err = send(request)
	require.Equal(t, codes.AlreadyExists, status.Code(err), "a current reinstall cannot reuse the old installation receipt")
	proto.Reset(b)
	proto.Merge(b, oldBinding)
	_, err = svc.SendMessage(metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-profile-id", actor.String())), request.Message)
	require.Error(t, err, "ordinary send must retain its membership gate")
	for _, name := range []string{"prewrite_revoked", "timeout", "posted_as_chat", "deferred", "foreign_chat", "reinstalled", "malformed_json"} {
		t.Run(name, func(t *testing.T) {
			denied := proto.Clone(request).(*messagingv1.SendBotMessageRequest)
			denied.Message.ClientMessageId = proto.String(uuid.NewString())
			calls = 0
			revokeAt = 0
			switch name {
			case "prewrite_revoked":
				revokeAt = 2
			case "timeout":
				communication.state = spacecommunication.TimedOut
			case "posted_as_chat":
				denied.Message.PostedAsChat = proto.Bool(true)
			case "deferred":
				denied.Message.DeliverySchedule = &messagingv1.SendMessageRequest_SendWhenOnline{SendWhenOnline: true}
			case "malformed_json":
				denied.Message.MentionsJson = "["
			case "foreign_chat":
				denied.Binding.ChatId = uuid.NewString()
			case "reinstalled":
				denied.Binding.InstallationGeneration++
			}
			_, e := send(denied)
			require.Error(t, e)
			communication.state = spacecommunication.Clear
		})
	}
	calls = 0
	revokeAt = 0
	expireDuringWrite = true
	botIssuer, err = principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return time.Now().Add(-28 * time.Second) }})
	require.NoError(t, err)
	expiring := proto.Clone(request).(*messagingv1.SendBotMessageRequest)
	expiring.Message.ClientMessageId = proto.String(uuid.NewString())
	_, err = send(expiring)
	require.Error(t, err)
	require.Equal(t, 2, calls, "expiry must be observed in final callback after locks")
	var messages, events int
	require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM messages),(SELECT count(*) FROM message_event_outbox)`).Scan(&messages, &events))
	require.Equal(t, 1, messages)
	require.Equal(t, 1, events)
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/messaging_db/000036_bot_send_receipt.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.Error(t, err, "rollback must refuse to discard or relabel live Bot receipts")
	var receiptRPC string
	require.NoError(t, pool.QueryRow(ctx, `SELECT request_rpc FROM message_send_receipts WHERE message_id=$1`, response.Message.Id).Scan(&receiptRPC))
	require.Equal(t, messagingv1.MessagingService_SendBotMessage_FullMethodName, receiptRPC)
	_, err = pool.Exec(ctx, `UPDATE message_send_receipts SET request_rpc='/unrecognized'`)
	require.Error(t, err, "failed rollback must retain the exact RPC constraint")
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `DELETE FROM message_send_receipts`)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, string(down))
	require.NoError(t, err, "rollback is allowed when no Bot receipts need preservation")
	require.NoError(t, tx.Rollback(ctx))
	_, err = receiver.SendBotMessage(ctx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
