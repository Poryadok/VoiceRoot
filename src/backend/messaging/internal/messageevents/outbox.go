package messageevents

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

// OutboxEvent is the immutable broker request stored with its domain mutation.
// Payload is the exact deterministic protobuf wire representation; Headers
// contains only contract headers and never request-scoped tracing metadata.
type OutboxEvent struct {
	EventID    uuid.UUID
	Subject    string
	MessageID  uuid.UUID
	ChatID     uuid.UUID
	Payload    []byte
	Headers    map[string]string
	LeaseToken uuid.UUID
	Attempts   int64
}

func newOutboxEvent(subject string, envelope *eventsv1.MessageStreamEvent, headers map[string]string) (OutboxEvent, error) {
	return encodeOutboxEvent(subject, envelope, headers)
}

func NewMessageSentOutbox(messageID, chatID, senderProfileID string, hasMentions bool, threadParentID string, isE2E bool, contentType string, sendSilent bool) (OutboxEvent, error) {
	sent := &eventsv1.MessageSent{MessageId: messageID, ChatId: chatID, SenderProfileId: senderProfileID, HasMentions: hasMentions, ThreadParentId: ptrIfNonEmpty(threadParentID), IsE2E: isE2E, SendSilent: sendSilent}
	if value := strings.TrimSpace(contentType); value != "" {
		sent.ContentType = &value
	}
	return newOutboxEvent(subjectMessageSent, &eventsv1.MessageStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.New(time.Now().UTC()), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: sent}}, messageSentHeaderMap(threadParentID))
}

func NewGameEventMessageSentOutbox(messageID, chatID, senderProfileID, applicationID, environmentID string) (OutboxEvent, error) {
	sent := &eventsv1.MessageSent{MessageId: messageID, ChatId: chatID, SenderProfileId: senderProfileID, ContentType: ptrIfNonEmpty("text"), GameApplicationId: &applicationID, GameEnvironmentId: &environmentID}
	return newOutboxEvent(subjectMessageSent, &eventsv1.MessageStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.New(time.Now().UTC()), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: sent}}, nil)
}

func NewMentionAddedOutbox(messageID, chatID, senderProfileID string, mentionedProfileIDs []string, sendSilent bool, gameApplicationID, gameEnvironmentID string) (OutboxEvent, error) {
	mention := &eventsv1.MentionAdded{MessageId: messageID, ChatId: chatID, SenderProfileId: senderProfileID, MentionedProfileIds: append([]string(nil), mentionedProfileIDs...), SendSilent: sendSilent}
	if value := ptrIfNonEmpty(gameApplicationID); value != nil {
		mention.GameApplicationId = value
	}
	if value := ptrIfNonEmpty(gameEnvironmentID); value != nil {
		mention.GameEnvironmentId = value
	}
	return newOutboxEvent(subjectMentionAdded, &eventsv1.MessageStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.New(time.Now().UTC()), Payload: &eventsv1.MessageStreamEvent_MentionAdded{MentionAdded: mention}}, nil)
}

func NewMessageEditedOutbox(messageID, chatID string, isE2E bool) (OutboxEvent, error) {
	payload := &eventsv1.MessageEdited{MessageId: messageID, ChatId: chatID, IsE2E: isE2E}
	return newOutboxEvent(subjectMessageEdited, &eventsv1.MessageStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.New(time.Now().UTC()), Payload: &eventsv1.MessageStreamEvent_MessageEdited{MessageEdited: payload}}, nil)
}

func NewMessageDeletedOutbox(messageID, chatID string) (OutboxEvent, error) {
	payload := &eventsv1.MessageDeleted{MessageId: messageID, ChatId: chatID}
	return newOutboxEvent(subjectMessageDeleted, &eventsv1.MessageStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.New(time.Now().UTC()), Payload: &eventsv1.MessageStreamEvent_MessageDeleted{MessageDeleted: payload}}, nil)
}

func NewMessageReadOutbox(messageID, chatID, profileID string) (OutboxEvent, error) {
	payload := &eventsv1.MessageRead{MessageId: messageID, ChatId: chatID, ProfileId: profileID}
	return newOutboxEvent(subjectMessageRead, &eventsv1.MessageStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.New(time.Now().UTC()), Payload: &eventsv1.MessageStreamEvent_MessageRead{MessageRead: payload}}, nil)
}

func NewReadReceiptRevokedOutbox(messageID, chatID, profileID, recipientProfileID string) (OutboxEvent, error) {
	payload := &eventsv1.ReadReceiptRevoked{MessageId: messageID, ChatId: chatID, ProfileId: profileID, RecipientProfileId: recipientProfileID}
	return newOutboxEvent(subjectReadReceiptRevoked, &eventsv1.MessageStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.New(time.Now().UTC()), Payload: &eventsv1.MessageStreamEvent_ReadReceiptRevoked{ReadReceiptRevoked: payload}}, nil)
}

func NewReactionAddedOutbox(messageID, chatID, profileID, messageAuthorProfileID, emoji, gameApplicationID, gameEnvironmentID string) (OutboxEvent, error) {
	payload := &eventsv1.ReactionAdded{MessageId: messageID, ChatId: chatID, ProfileId: profileID, Emoji: emoji, MessageAuthorProfileId: messageAuthorProfileID}
	if value := ptrIfNonEmpty(gameApplicationID); value != nil {
		payload.GameApplicationId = value
	}
	if value := ptrIfNonEmpty(gameEnvironmentID); value != nil {
		payload.GameEnvironmentId = value
	}
	return newOutboxEvent(subjectReactionAdded, &eventsv1.MessageStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.New(time.Now().UTC()), Payload: &eventsv1.MessageStreamEvent_ReactionAdded{ReactionAdded: payload}}, nil)
}

func NewReactionRemovedOutbox(messageID, chatID, profileID, emoji string) (OutboxEvent, error) {
	payload := &eventsv1.ReactionRemoved{MessageId: messageID, ChatId: chatID, ProfileId: profileID, Emoji: emoji}
	return newOutboxEvent(subjectReactionRemoved, &eventsv1.MessageStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.New(time.Now().UTC()), Payload: &eventsv1.MessageStreamEvent_ReactionRemoved{ReactionRemoved: payload}}, nil)
}

func NewMessagePinnedOutbox(messageID, chatID, pinnedBy string) (OutboxEvent, error) {
	payload := &eventsv1.MessagePinned{MessageId: messageID, ChatId: chatID, PinnedBy: pinnedBy}
	return newOutboxEvent(subjectMessagePinned, &eventsv1.MessageStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.New(time.Now().UTC()), Payload: &eventsv1.MessageStreamEvent_MessagePinned{MessagePinned: payload}}, nil)
}

func NewMessageUnpinnedOutbox(messageID, chatID, unpinnedBy string) (OutboxEvent, error) {
	payload := &eventsv1.MessageUnpinned{MessageId: messageID, ChatId: chatID, UnpinnedBy: unpinnedBy}
	return newOutboxEvent(subjectMessageUnpinned, &eventsv1.MessageStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.New(time.Now().UTC()), Payload: &eventsv1.MessageStreamEvent_MessageUnpinned{MessageUnpinned: payload}}, nil)
}

func NewMessageForwardedOutbox(messageID, sourceChatID, targetChatID, forwarderProfileID string) (OutboxEvent, error) {
	payload := &eventsv1.MessageForwarded{MessageId: messageID, SourceChatId: sourceChatID, TargetChatId: targetChatID, ForwarderProfileId: forwarderProfileID}
	return newOutboxEvent(subjectMessageForwarded, &eventsv1.MessageStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.New(time.Now().UTC()), Payload: &eventsv1.MessageStreamEvent_MessageForwarded{MessageForwarded: payload}}, nil)
}

func messageSentHeaderMap(threadParentID string) map[string]string {
	if value := strings.TrimSpace(threadParentID); value != "" {
		return map[string]string{natsHeaderThreadParentID: value}
	}
	return nil
}

// encodeOutboxEvent validates and deterministically serializes one event. The
// event envelope (including ID and timestamp) must be constructed once by the
// producer and persisted; dispatch retries must use the returned bytes as-is.
func encodeOutboxEvent(subject string, event *eventsv1.MessageStreamEvent, headers map[string]string) (OutboxEvent, error) {
	if event == nil || event.GetPayload() == nil || event.GetOccurredAt() == nil {
		return OutboxEvent{}, fmt.Errorf("message outbox event envelope is incomplete")
	}
	if err := event.GetOccurredAt().CheckValid(); err != nil {
		return OutboxEvent{}, fmt.Errorf("message outbox event timestamp is invalid: %w", err)
	}
	eventID, err := uuid.Parse(event.GetEventId())
	if err != nil || eventID == uuid.Nil || eventID.String() != event.GetEventId() {
		return OutboxEvent{}, fmt.Errorf("message outbox event ID must be a canonical UUID")
	}
	expectedSubject, messageID, chatID, err := eventIdentity(event)
	if err != nil {
		return OutboxEvent{}, err
	}
	if subject != expectedSubject {
		return OutboxEvent{}, fmt.Errorf("message outbox subject %q does not match event payload %q", subject, expectedSubject)
	}
	messageUUID, err := uuid.Parse(messageID)
	if err != nil {
		return OutboxEvent{}, fmt.Errorf("message outbox message ID is invalid")
	}
	chatUUID, err := uuid.Parse(chatID)
	if err != nil {
		return OutboxEvent{}, fmt.Errorf("message outbox chat ID is invalid")
	}
	payload, err := (proto.MarshalOptions{Deterministic: true}).Marshal(event)
	if err != nil {
		return OutboxEvent{}, fmt.Errorf("marshal MessageStreamEvent: %w", err)
	}
	copyHeaders := make(map[string]string, len(headers))
	for name, value := range headers {
		name = strings.TrimSpace(name)
		if name == "" || strings.ContainsAny(name, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return OutboxEvent{}, fmt.Errorf("message outbox header is invalid")
		}
		copyHeaders[name] = value
	}
	return OutboxEvent{
		EventID: eventID, Subject: subject, MessageID: messageUUID, ChatID: chatUUID,
		Payload: payload, Headers: copyHeaders,
	}, nil
}

func eventIdentity(event *eventsv1.MessageStreamEvent) (string, string, string, error) {
	switch payload := event.GetPayload().(type) {
	case *eventsv1.MessageStreamEvent_MessageSent:
		return subjectMessageSent, payload.MessageSent.GetMessageId(), payload.MessageSent.GetChatId(), nil
	case *eventsv1.MessageStreamEvent_MessageEdited:
		return subjectMessageEdited, payload.MessageEdited.GetMessageId(), payload.MessageEdited.GetChatId(), nil
	case *eventsv1.MessageStreamEvent_MessageDeleted:
		return subjectMessageDeleted, payload.MessageDeleted.GetMessageId(), payload.MessageDeleted.GetChatId(), nil
	case *eventsv1.MessageStreamEvent_MessageRead:
		return subjectMessageRead, payload.MessageRead.GetMessageId(), payload.MessageRead.GetChatId(), nil
	case *eventsv1.MessageStreamEvent_ReadReceiptRevoked:
		return subjectReadReceiptRevoked, payload.ReadReceiptRevoked.GetMessageId(), payload.ReadReceiptRevoked.GetChatId(), nil
	case *eventsv1.MessageStreamEvent_ReactionAdded:
		return subjectReactionAdded, payload.ReactionAdded.GetMessageId(), payload.ReactionAdded.GetChatId(), nil
	case *eventsv1.MessageStreamEvent_ReactionRemoved:
		return subjectReactionRemoved, payload.ReactionRemoved.GetMessageId(), payload.ReactionRemoved.GetChatId(), nil
	case *eventsv1.MessageStreamEvent_MentionAdded:
		return subjectMentionAdded, payload.MentionAdded.GetMessageId(), payload.MentionAdded.GetChatId(), nil
	case *eventsv1.MessageStreamEvent_MessagePinned:
		return subjectMessagePinned, payload.MessagePinned.GetMessageId(), payload.MessagePinned.GetChatId(), nil
	case *eventsv1.MessageStreamEvent_MessageUnpinned:
		return subjectMessageUnpinned, payload.MessageUnpinned.GetMessageId(), payload.MessageUnpinned.GetChatId(), nil
	case *eventsv1.MessageStreamEvent_MessageForwarded:
		return subjectMessageForwarded, payload.MessageForwarded.GetMessageId(), payload.MessageForwarded.GetTargetChatId(), nil
	default:
		return "", "", "", fmt.Errorf("message outbox payload type %T is not a Messaging-produced event", event.GetPayload())
	}
}
