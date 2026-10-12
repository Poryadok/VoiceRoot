package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	gameintegrationv1 "voice.app/voice/gameintegration/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
)

func TestSendGameEventMessageRequiresVerifiedBotPrincipal(t *testing.T) {
	request := validGameEventRequest()
	service := &MessagingGRPC{}
	_, err := service.SendGameEventMessage(context.Background(), request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	ctx := gameEventPrincipalContext(t, request, func(p *principal.Principal) { p.Issuer = "gateway" })
	_, err = service.SendGameEventMessage(ctx, request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	ctx = gameEventPrincipalContext(t, request, nil)
	_, err = service.SendGameEventMessage(ctx, request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "verified Bot call proceeds to dependency readiness")
}

func TestSendGameEventMessageValidatesImmutableIntent(t *testing.T) {
	request := validGameEventRequest()
	require.NoError(t, validateVerifiedGameEventIntent(request.GetIntent(), request.GetSenderProfileId()))

	request.Intent.PayloadHash = "not-a-sha256"
	require.Error(t, validateVerifiedGameEventIntent(request.GetIntent(), request.GetSenderProfileId()))

	request = validGameEventRequest()
	request.Intent.Recipient = nil
	require.Error(t, validateVerifiedGameEventIntent(request.GetIntent(), request.GetSenderProfileId()))

	request = validGameEventRequest()
	request.Intent.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	require.Error(t, validateVerifiedGameEventIntent(request.GetIntent(), request.GetSenderProfileId()))
}

func TestSendGameEventMessageValidatesAndEncodesV2Card(t *testing.T) {
	request := validGameEventRequest()
	request.Intent.SchemaVersion = 2
	characterBindingID := uuid.NewString()
	request.Intent.CharacterBindingId = &characterBindingID
	request.Intent.Card = &gameintegrationv1.GameCard{SchemaVersion: 1, Revision: 1, Title: "Relic found", SafeSummary: "A relic was found",
		Facts:             []*gameintegrationv1.GameCardFact{{Label: "Location", Value: "North gate"}},
		Actions:           []*gameintegrationv1.GameCardAction{{ActionId: uuid.NewString(), ActionType: "relic.inspect", Label: "Inspect", ArgumentsJson: `{"target":"relic"}`}},
		MediaReferenceIds: []string{uuid.NewString()}}
	require.NoError(t, validateVerifiedGameEventIntent(request.GetIntent(), request.GetSenderProfileId()))
	raw, err := encodeGameCard(request.GetIntent())
	require.NoError(t, err)
	require.NotNil(t, raw)
	require.Contains(t, *raw, `"safe_summary":"A relic was found"`)

	request.Intent.Card.Actions[0].ArgumentsJson = `{"z":1,"a":2}`
	require.Error(t, validateVerifiedGameEventIntent(request.GetIntent(), request.GetSenderProfileId()), "action arguments must use canonical JSON ordering")
	request = validGameEventRequest()
	request.Intent.SchemaVersion = 2
	request.Intent.Card = &gameintegrationv1.GameCard{SchemaVersion: 1, Revision: 1, Title: "https://unsafe.example", SafeSummary: "summary"}
	require.Error(t, validateVerifiedGameEventIntent(request.GetIntent(), request.GetSenderProfileId()))
}

func TestSendGameEventMessagePersistsCardAndTrustedAttribution(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000003_groups.up.sql")
	applyBaseMessagingMigrations(t, ctx, pool)

	chatID, senderID, memberID := uuid.New(), uuid.New(), uuid.New()
	seedGroupChat(t, ctx, pool, chatID, senderID, memberID)
	service := startMessagingDirect(t, pool)
	request := validGameEventRequest()
	request.Intent.SchemaVersion = 2
	characterBindingID := uuid.NewString()
	request.Intent.CharacterBindingId = &characterBindingID
	request.Intent.Recipient = &gameintegrationv1.VerifiedGameEventIntent_DirectChat{DirectChat: &gameintegrationv1.DirectChatEventRecipient{ChatId: chatID.String()}}
	request.Intent.Card = &gameintegrationv1.GameCard{SchemaVersion: 1, Revision: 3, Title: "Relic found", SafeSummary: "A relic was found",
		Facts:             []*gameintegrationv1.GameCardFact{{Label: "Location", Value: "North gate"}},
		Actions:           []*gameintegrationv1.GameCardAction{{ActionId: uuid.NewString(), ActionType: "relic.inspect", Label: "Inspect", ArgumentsJson: `{"target":"relic"}`}},
		MediaReferenceIds: []string{uuid.NewString()}}
	request.SenderProfileId = senderID.String()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	serviceCtx := principal.WithVerified(ctx, principal.Principal{Kind: "service", Issuer: "bot", Subject: "service:bot", Audience: "messaging",
		RPC: messagingv1.MessagingService_SendGameEventMessage_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash})
	response, err := service.SendGameEventMessage(serviceCtx, request)
	require.NoError(t, err)
	require.Equal(t, gameintegrationv1.GameEventPublicationStatus_GAME_EVENT_PUBLICATION_STATUS_PUBLISHED, response.GetStatus())
	events := storedOutboxEvents(t, pool, "message.sent")
	require.Len(t, events, 1)
	require.Equal(t, request.GetIntent().GetAppId(), events[0].GetMessageSent().GetGameApplicationId())
	require.Equal(t, request.GetIntent().GetEnvironmentId(), events[0].GetMessageSent().GetGameEnvironmentId())

	row, err := (&store.MessagesStore{Pool: pool}).GetMessageByID(ctx, uuid.MustParse(response.GetMessageId()))
	require.NoError(t, err)
	require.Equal(t, request.GetIntent().GetFallbackText(), row.Content)
	require.Equal(t, request.GetIntent().GetAppId(), row.GameAppID.String())
	require.Equal(t, request.GetIntent().GetEnvironmentId(), row.GameEnvironmentID.String())
	require.Equal(t, request.GetIntent().GetInstallationId(), row.GameInstallationID.String())
	require.Equal(t, request.GetIntent().GetBotId(), row.GameBotID.String())
	require.Equal(t, request.GetIntent().GetCharacterBindingId(), row.GameCharacterBindingID.String())
	require.False(t, row.GameCardActionsEnabled)
	require.Contains(t, *row.GameCardJSON, "relic.inspect")
}

func validGameEventRequest() *messagingv1.SendGameEventMessageRequest {
	now := time.Now().UTC().Truncate(time.Second)
	return &messagingv1.SendGameEventMessageRequest{
		Intent: &gameintegrationv1.VerifiedGameEventIntent{
			OperationId: uuid.NewString(), AppId: uuid.NewString(), AppOwnerAccountId: uuid.NewString(),
			EnvironmentId: uuid.NewString(), InstallationId: uuid.NewString(), BotId: uuid.NewString(),
			EventId: uuid.NewString(), PayloadHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Recipient:         &gameintegrationv1.VerifiedGameEventIntent_DirectChat{DirectChat: &gameintegrationv1.DirectChatEventRecipient{ChatId: uuid.NewString()}},
			AuthorityRevision: 1, SchemaVersion: 1, OccurredAt: timestamppb.New(now), ExpiresAt: timestamppb.New(now.Add(time.Minute)),
			FallbackText: "A game event occurred.", ClientMessageId: uuid.NewString(), EventType: "match.completed",
		},
		SenderProfileId: uuid.NewString(),
	}
}

func gameEventPrincipalContext(t *testing.T, req *messagingv1.SendGameEventMessageRequest, edit func(*principal.Principal)) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	verified := principal.Principal{Kind: "service", Issuer: "bot", Subject: "service:bot", Audience: "messaging",
		RPC: messagingv1.MessagingService_SendGameEventMessage_FullMethodName, RequestID: "request-1", RequestHash: hash}
	if edit != nil {
		edit(&verified)
	}
	return principal.WithVerified(context.Background(), verified)
}
