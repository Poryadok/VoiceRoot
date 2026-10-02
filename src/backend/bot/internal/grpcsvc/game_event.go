package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"

	"voice/backend/bot/internal/manifest"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"

	botv1 "voice.app/voice/bot/v1"
	gameintegrationv1 "voice.app/voice/gameintegration/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

// PublishGameEvent is the Bot-owned policy boundary between GIS and Messaging.
// It rechecks live ownership, send scope, and the current chat whitelist before
// forwarding the exact GIS intent with Bot's actor profile as sender.
func (s *BotGRPC) PublishGameEvent(ctx context.Context, req *botv1.PublishGameEventRequest) (*botv1.PublishGameEventResponse, error) {
	if req == nil || req.GetIntent() == nil {
		return nil, status.Error(codes.InvalidArgument, "verified game event intent is required")
	}
	verified, ok := principal.FromContext(ctx)
	hash, hashErr := principal.RequestHash(req)
	if !ok || hashErr != nil || verified.Kind != "service" || verified.Issuer != "gameintegration" || verified.Subject != "service:gameintegration" ||
		verified.Audience != "bot" || verified.RPC != botv1.BotService_PublishGameEvent_FullMethodName ||
		verified.RequestID == "" || verified.RequestHash != hash || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return nil, status.Error(codes.PermissionDenied, "authenticated Game Integration service principal required")
	}
	intent := req.GetIntent()
	if len(intent.ProtoReflect().GetUnknown()) != 0 || hasEventRecipientUnknowns(intent.ProtoReflect()) ||
		!canonicalUUID(intent.GetOperationId()) || !canonicalUUID(intent.GetAppId()) || !canonicalUUID(intent.GetAppOwnerAccountId()) ||
		!canonicalUUID(intent.GetEnvironmentId()) || !canonicalUUID(intent.GetInstallationId()) || !canonicalUUID(intent.GetBotId()) ||
		!canonicalUUID(intent.GetEventId()) || !canonicalUUID(intent.GetClientMessageId()) ||
		(intent.GetCharacterBindingId() != "" && !canonicalUUID(intent.GetCharacterBindingId())) ||
		(intent.GetSchemaVersion() != 1 && intent.GetSchemaVersion() != 2) ||
		(intent.GetSchemaVersion() == 1 && intent.GetCard() != nil) || (intent.GetSchemaVersion() == 2 && intent.GetCard() == nil) ||
		strings.TrimSpace(intent.GetFallbackText()) == "" || len([]rune(intent.GetFallbackText())) > 4000 {
		return nil, status.Error(codes.InvalidArgument, "verified game event intent is invalid")
	}
	switch recipient := intent.GetRecipient().(type) {
	case *gameintegrationv1.VerifiedGameEventIntent_Binding:
		if recipient.Binding == nil || !canonicalUUID(recipient.Binding.GetBindingId()) || !canonicalUUID(recipient.Binding.GetResolvedChatId()) {
			return nil, status.Error(codes.InvalidArgument, "binding recipient is invalid")
		}
	case *gameintegrationv1.VerifiedGameEventIntent_DirectChat:
		if recipient.DirectChat == nil || !canonicalUUID(recipient.DirectChat.GetChatId()) {
			return nil, status.Error(codes.InvalidArgument, "direct chat recipient is invalid")
		}
	default:
		return nil, status.Error(codes.InvalidArgument, "exactly one game event recipient is required")
	}
	if s == nil || s.Store == nil || s.Messaging == nil || s.PrincipalIssuer == nil {
		return nil, status.Error(codes.FailedPrecondition, "Bot game event delivery is not configured")
	}
	botID, _ := uuid.Parse(intent.GetBotId())
	chatIDString := eventRecipientChat(intent)
	chatID, err := uuid.Parse(chatIDString)
	if err != nil || chatID.String() != chatIDString {
		return nil, status.Error(codes.InvalidArgument, "resolved game event chat is invalid")
	}
	bot, err := s.Store.GetBotByID(ctx, botID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, status.Error(codes.PermissionDenied, "game event bot is unavailable")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "bot authority unavailable")
	}
	appOwner, _ := uuid.Parse(intent.GetAppOwnerAccountId())
	if bot.Status != "live" || bot.OwnerAccountID != appOwner || bot.ActorProfileID == uuid.Nil {
		return nil, status.Error(codes.PermissionDenied, "game event bot is unavailable")
	}
	scopes, err := manifest.ParseScopeSetJSON(bot.ScopesJSON, true)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "game event bot send scope is unavailable")
	}
	if _, allowed := scopes["TEXT_CHAT_SEND_MESSAGES"]; !allowed {
		return nil, status.Error(codes.PermissionDenied, "bot lacks TEXT_CHAT_SEND_MESSAGES scope")
	}
	whitelisted, err := s.Store.IsChatWhitelisted(ctx, botID, chatID)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "bot chat authorization unavailable")
	}
	if !whitelisted {
		return nil, status.Error(codes.PermissionDenied, "bot is not enabled in this chat")
	}
	forward := &messagingv1.SendGameEventMessageRequest{Intent: intent, SenderProfileId: bot.ActorProfileID.String()}
	requestHash, err := principal.RequestHash(forward)
	if err != nil {
		return nil, status.Error(codes.Internal, "game event request binding failed")
	}
	requestID := intent.GetOperationId()
	token, err := s.PrincipalIssuer.IssueService(principal.ServiceInput{
		Audience: "messaging", RPC: messagingv1.MessagingService_SendGameEventMessage_FullMethodName,
		RequestID: requestID, RequestHash: requestHash,
	})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Bot service principal unavailable")
	}
	callCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token, "x-request-id", requestID)
	response, err := s.Messaging.SendGameEventMessage(callCtx, forward)
	if err != nil {
		return nil, err
	}
	if response == nil || response.GetStatus() == gameintegrationv1.GameEventPublicationStatus_GAME_EVENT_PUBLICATION_STATUS_UNSPECIFIED {
		return nil, status.Error(codes.Unavailable, "Messaging returned an invalid game event result")
	}
	return &botv1.PublishGameEventResponse{Status: response.GetStatus(), MessageId: response.MessageId}, nil
}

func canonicalUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func eventRecipientChat(intent *gameintegrationv1.VerifiedGameEventIntent) string {
	if binding := intent.GetBinding(); binding != nil {
		return binding.GetResolvedChatId()
	}
	return intent.GetDirectChat().GetChatId()
}

func hasEventRecipientUnknowns(message protoreflect.Message) bool {
	unknown := false
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.Kind() != protoreflect.MessageKind && field.Kind() != protoreflect.GroupKind {
			return true
		}
		if field.IsList() {
			list := value.List()
			for i := 0; i < list.Len(); i++ {
				if len(list.Get(i).Message().GetUnknown()) != 0 {
					unknown = true
					return false
				}
			}
			return true
		}
		if field.IsMap() {
			return true
		}
		if len(value.Message().GetUnknown()) != 0 {
			unknown = true
			return false
		}
		return true
	})
	return unknown
}
