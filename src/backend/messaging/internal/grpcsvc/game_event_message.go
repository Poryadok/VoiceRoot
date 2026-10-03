package grpcsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	"voice/backend/messaging/internal/messageid"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"

	chatv1 "voice.app/voice/chat/v1"
	gameintegrationv1 "voice.app/voice/gameintegration/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

var gameEventDigestRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
var gameEventTypeRE = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9_]*)+$`)

// SendGameEventMessage accepts only a GIS-verified immutable intent from Bot.
// Messaging then applies current chat membership, chat type, send permission,
// and moderation policy before writing its own durable message row.
func (s *MessagingGRPC) SendGameEventMessage(ctx context.Context, req *messagingv1.SendGameEventMessageRequest) (*messagingv1.SendGameEventMessageResponse, error) {
	if req == nil || req.GetIntent() == nil {
		return nil, status.Error(codes.InvalidArgument, "verified game event intent is required")
	}
	verified, ok := principal.FromContext(ctx)
	hash, hashErr := principal.RequestHash(req)
	if !ok || hashErr != nil || verified.Kind != "service" || verified.Issuer != "bot" || verified.Subject != "service:bot" ||
		verified.Audience != "messaging" || verified.RPC != messagingv1.MessagingService_SendGameEventMessage_FullMethodName ||
		verified.RequestID == "" || verified.RequestHash != hash || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return nil, status.Error(codes.PermissionDenied, "authenticated Bot service principal required")
	}
	// This method has authenticated the Bot service principal and the Bot-owned
	// sender profile. Mark only this downstream Chat lookup as a trusted internal
	// caller; Chat still evaluates current membership for that exact profile.
	md, _ := metadata.FromIncomingContext(ctx)
	trustedMD := md.Copy()
	if trustedMD == nil {
		trustedMD = metadata.MD{}
	}
	trustedMD.Set("x-voice-internal-caller", "messaging")
	ctx = metadata.NewIncomingContext(ctx, trustedMD)
	if err := validateVerifiedGameEventIntent(req.GetIntent(), req.GetSenderProfileId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	cardJSON, err := encodeGameCard(req.GetIntent())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "game card is invalid")
	}
	if s == nil || s.Messages == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	intent := req.GetIntent()
	chatID, _ := uuid.Parse(eventChatID(intent))
	senderID, _ := uuid.Parse(req.GetSenderProfileId())
	if isNilDependency(s.ChatGuard) {
		return nil, status.Error(codes.Unavailable, "chat membership unavailable")
	}
	if err := s.ChatGuard.EnsureMember(ctx, chatID, senderID); err != nil {
		if errors.Is(err, store.ErrNotChatMember) {
			return nil, status.Error(codes.PermissionDenied, "bot is not a chat member")
		}
		if s.Logger != nil {
			s.Logger.ErrorContext(ctx, "game event chat membership check failed", slog.String("error", err.Error()))
		}
		return nil, status.Error(codes.Unavailable, "chat membership unavailable")
	}
	chatType, err := s.resolveAuthoritativeChatType(ctx, chatID, senderID)
	if err != nil {
		return nil, err
	}
	if chatType == chatv1.ChatType_CHAT_TYPE_DM {
		return nil, status.Error(codes.PermissionDenied, "game events cannot be sent to direct messages")
	}
	if err := s.checkSpaceSendPermission(ctx, chatID, senderID); err != nil {
		return nil, err
	}
	if s.Moderation != nil {
		if err := s.Moderation.EnsureCanSend(ctx, chatID, senderID); err != nil {
			if errors.Is(err, store.ErrMemberTimedOut) {
				return nil, status.Error(codes.PermissionDenied, "bot is timed out in this chat")
			}
			if errors.Is(err, store.ErrSlowModeActive) {
				return nil, status.Error(codes.ResourceExhausted, "slow mode is active")
			}
			return nil, status.Error(codes.Unavailable, "chat moderation unavailable")
		}
	}
	if s.PlatformMod != nil {
		if err := s.PlatformMod.CheckMessageAllowed(ctx, senderID, chatID, intent.GetFallbackText()); err != nil {
			return nil, status.Error(codes.PermissionDenied, "game event message rejected by moderation")
		}
	}

	clientMessageID, _ := uuid.Parse(intent.GetClientMessageId())
	messageID, err := messageid.NewMessageID()
	if err != nil {
		return nil, status.Error(codes.Internal, "message id generation failed")
	}
	var expiresAt *time.Time
	if intent.GetExpiresAt() != nil {
		value := intent.GetExpiresAt().AsTime()
		expiresAt = &value
	}
	rowData := store.MessageRow{
		ID: messageID, ChatID: chatID, ChatType: chatTypeName(chatType), SenderProfileID: senderID,
		Content: intent.GetFallbackText(), Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]",
		ClientMessageID: &clientMessageID, ContentType: "text",
	}
	if cardJSON != nil {
		rowData.GameCardJSON = cardJSON
		rowData.GameAppID, rowData.GameEnvironmentID = uuidPtr(intent.GetAppId()), uuidPtr(intent.GetEnvironmentId())
		rowData.GameInstallationID, rowData.GameBotID = uuidPtr(intent.GetInstallationId()), uuidPtr(intent.GetBotId())
		rowData.GameCharacterBindingID = optionalUUIDValue(intent.GetCharacterBindingId())
		rowData.GameCardActionsEnabled = false
	}
	row, expired, inserted, err := s.Messages.InsertGameEventMessage(ctx, rowData, expiresAt)
	if err != nil {
		if errors.Is(err, store.ErrGameEventMessageConflict) {
			return nil, status.Error(codes.AlreadyExists, "game event idempotency conflict")
		}
		return nil, status.Error(codes.Unavailable, "game event message persistence unavailable")
	}
	if expired {
		return &messagingv1.SendGameEventMessageResponse{Status: gameintegrationv1.GameEventPublicationStatus_GAME_EVENT_PUBLICATION_STATUS_EXPIRED}, nil
	}
	if inserted && s.MessageEvents != nil {
		if err := s.MessageEvents.PublishGameEventMessageSent(ctx, row.ID.String(), row.ChatID.String(), row.SenderProfileID.String(), intent.GetAppId(), intent.GetEnvironmentId()); err != nil {
			s.logPublishError(ctx, "message.sent", err, slog.String("message_id", row.ID.String()), slog.String("chat_id", row.ChatID.String()))
		}
	}
	messageIDString := row.ID.String()
	return &messagingv1.SendGameEventMessageResponse{
		Status:    gameintegrationv1.GameEventPublicationStatus_GAME_EVENT_PUBLICATION_STATUS_PUBLISHED,
		MessageId: &messageIDString,
	}, nil
}

func validateVerifiedGameEventIntent(intent *gameintegrationv1.VerifiedGameEventIntent, sender string) error {
	if hasProtoUnknowns(intent.ProtoReflect()) {
		return fmt.Errorf("verified game event intent contains unknown fields")
	}
	for name, value := range map[string]string{
		"operation_id": intent.GetOperationId(), "app_id": intent.GetAppId(),
		"app_owner_account_id": intent.GetAppOwnerAccountId(), "environment_id": intent.GetEnvironmentId(),
		"installation_id": intent.GetInstallationId(), "bot_id": intent.GetBotId(), "event_id": intent.GetEventId(),
		"client_message_id": intent.GetClientMessageId(), "sender_profile_id": sender,
	} {
		parsed, err := uuid.Parse(value)
		if err != nil || parsed.String() != value {
			return fmt.Errorf("%s must be a canonical UUID", name)
		}
	}
	if (intent.GetSchemaVersion() != 1 && intent.GetSchemaVersion() != 2) || intent.GetAuthorityRevision() == 0 ||
		!gameEventDigestRE.MatchString(intent.GetPayloadHash()) || !gameEventTypeRE.MatchString(intent.GetEventType()) ||
		strings.TrimSpace(intent.GetFallbackText()) == "" || len([]rune(intent.GetFallbackText())) > 4000 {
		return fmt.Errorf("verified game event intent is invalid")
	}
	if (intent.GetSchemaVersion() == 1 && intent.GetCard() != nil) || (intent.GetSchemaVersion() == 2 && intent.GetCard() == nil) {
		return fmt.Errorf("game card schema does not match event schema")
	}
	if card := intent.GetCard(); card != nil {
		if card.GetSchemaVersion() != 1 || card.GetRevision() == 0 || card.GetTitle() == "" || card.GetSafeSummary() == "" ||
			utf8.RuneCountInString(card.GetTitle()) > 256 || utf8.RuneCountInString(card.GetSafeSummary()) > 1000 ||
			gameEventContainsURL(card.GetTitle()) || gameEventContainsURL(card.GetSafeSummary()) || len(card.GetFacts()) > 16 ||
			len(card.GetActions()) > 8 || len(card.GetMediaReferenceIds()) > 12 {
			return fmt.Errorf("game card exceeds contract bounds")
		}
		for _, fact := range card.GetFacts() {
			if fact == nil || fact.GetLabel() == "" || fact.GetValue() == "" || utf8.RuneCountInString(fact.GetLabel()) > 128 ||
				utf8.RuneCountInString(fact.GetValue()) > 512 || gameEventContainsURL(fact.GetLabel()) || gameEventContainsURL(fact.GetValue()) {
				return fmt.Errorf("game card fact is invalid")
			}
		}
		seen := make(map[string]bool, len(card.GetActions()))
		for _, action := range card.GetActions() {
			if action == nil || !isCanonicalUUID(action.GetActionId()) || !gameEventTypeRE.MatchString(action.GetActionType()) || action.GetLabel() == "" ||
				utf8.RuneCountInString(action.GetLabel()) > 128 || gameEventContainsURL(action.GetLabel()) || seen[action.GetActionId()] ||
				!canonicalJSONArgument(action.GetArgumentsJson()) || utf8.RuneCountInString(action.GetStateVersion()) > 256 ||
				(action.GetExpiresAt() != nil && action.GetExpiresAt().CheckValid() != nil) {
				return fmt.Errorf("game card action is invalid")
			}
			seen[action.GetActionId()] = true
		}
		for _, id := range card.GetMediaReferenceIds() {
			if !isCanonicalUUID(id) {
				return fmt.Errorf("game card media reference is invalid")
			}
		}
	}
	if intent.GetCharacterBindingId() != "" {
		if !isCanonicalUUID(intent.GetCharacterBindingId()) {
			return fmt.Errorf("character binding attribution is invalid")
		}
	}
	if intent.GetOccurredAt() == nil || intent.GetOccurredAt().CheckValid() != nil ||
		(intent.GetExpiresAt() != nil && (intent.GetExpiresAt().CheckValid() != nil || !intent.GetExpiresAt().AsTime().After(intent.GetOccurredAt().AsTime()))) {
		return fmt.Errorf("verified game event timestamps are invalid")
	}
	switch recipient := intent.GetRecipient().(type) {
	case *gameintegrationv1.VerifiedGameEventIntent_Binding:
		if recipient.Binding == nil || recipient.Binding.GetBindingId() == "" || !isCanonicalUUID(recipient.Binding.GetBindingId()) || !isCanonicalUUID(recipient.Binding.GetResolvedChatId()) {
			return fmt.Errorf("binding event recipient is invalid")
		}
	case *gameintegrationv1.VerifiedGameEventIntent_DirectChat:
		if recipient.DirectChat == nil || !isCanonicalUUID(recipient.DirectChat.GetChatId()) {
			return fmt.Errorf("direct chat event recipient is invalid")
		}
	default:
		return fmt.Errorf("exactly one game event recipient is required")
	}
	return nil
}

func encodeGameCard(intent *gameintegrationv1.VerifiedGameEventIntent) (*string, error) {
	if intent == nil || intent.GetCard() == nil {
		return nil, nil
	}
	raw, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(intent.GetCard())
	if err != nil {
		return nil, err
	}
	value := string(raw)
	return &value, nil
}

func canonicalJSONArgument(value string) bool {
	if value == "" || len(value) > 4*1024 || !strings.HasPrefix(value, "{") {
		return false
	}
	canonical, err := jsoncanonicalizer.Transform([]byte(value))
	if err != nil || !bytes.Equal(canonical, []byte(value)) {
		return false
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(canonical, &object); err != nil || object == nil {
		return false
	}
	return true
}

func gameEventContainsURL(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "http://") || strings.Contains(lower, "https://")
}

func uuidPtr(value string) *uuid.UUID {
	parsed, _ := uuid.Parse(value)
	return &parsed
}

func optionalUUIDValue(value string) *uuid.UUID {
	if value == "" {
		return nil
	}
	parsed, _ := uuid.Parse(value)
	return &parsed
}

func isCanonicalUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.String() == value
}

func eventChatID(intent *gameintegrationv1.VerifiedGameEventIntent) string {
	if binding := intent.GetBinding(); binding != nil {
		return binding.GetResolvedChatId()
	}
	return intent.GetDirectChat().GetChatId()
}

func hasProtoUnknowns(message protoreflect.Message) bool {
	if len(message.GetUnknown()) != 0 {
		return true
	}
	unknown := false
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.Kind() != protoreflect.MessageKind && field.Kind() != protoreflect.GroupKind {
			return true
		}
		if field.IsList() {
			list := value.List()
			for i := 0; i < list.Len(); i++ {
				if hasProtoUnknowns(list.Get(i).Message()) {
					unknown = true
					return false
				}
			}
			return true
		}
		if field.IsMap() {
			return true
		}
		if hasProtoUnknowns(value.Message()) {
			unknown = true
			return false
		}
		return true
	})
	return unknown
}
