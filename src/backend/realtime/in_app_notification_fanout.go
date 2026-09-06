package main

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

// profileFanout is a personal WebSocket delivery target (op "notification").
type profileFanout struct {
	ProfileID string
	Envelope  fanoutEnvelope
}

func inAppNotificationFanouts(data []byte, chatMemberProfileIDs []string, reactionMessageAuthorProfileID string, recipientStates map[string]chatMemberDeliveryState) ([]profileFanout, bool) {
	var e eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &e); err != nil {
		return nil, false
	}
	switch p := e.GetPayload().(type) {
	case *eventsv1.MessageStreamEvent_MessageSent:
		return newMessageNotificationFanouts(p.MessageSent, chatMemberProfileIDs, recipientStates)
	case *eventsv1.MessageStreamEvent_ReactionAdded:
		return reactionNotificationFanouts(p.ReactionAdded, chatMemberProfileIDs, reactionMessageAuthorProfileID, recipientStates)
	case *eventsv1.MessageStreamEvent_MentionAdded:
		return mentionNotificationFanouts(p.MentionAdded, recipientStates)
	default:
		return nil, false
	}
}

func mentionNotificationFanouts(ma *eventsv1.MentionAdded, recipientStates map[string]chatMemberDeliveryState) ([]profileFanout, bool) {
	if ma == nil || ma.GetChatId() == "" || ma.GetMessageId() == "" {
		return nil, false
	}
	senderID := ma.GetSenderProfileId()
	var fanouts []profileFanout
	for _, profileID := range ma.GetMentionedProfileIds() {
		if profileID == "" || profileID == senderID {
			continue
		}
		if recipientStates != nil && recipientStates[profileID].IsArchived {
			continue
		}
		d, err := json.Marshal(map[string]string{
			"type":              "mention",
			"chat_id":           ma.GetChatId(),
			"message_id":        ma.GetMessageId(),
			"sender_profile_id": senderID,
		})
		if err != nil {
			return nil, false
		}
		fanouts = append(fanouts, profileFanout{
			ProfileID: profileID,
			Envelope:  fanoutEnvelope{Op: "notification", D: d},
		})
	}
	return fanouts, true
}

func newMessageNotificationFanouts(ms *eventsv1.MessageSent, chatMemberProfileIDs []string, recipientStates map[string]chatMemberDeliveryState) ([]profileFanout, bool) {
	if ms == nil || ms.GetChatId() == "" || ms.GetMessageId() == "" {
		return nil, false
	}
	senderID := ms.GetSenderProfileId()
	var fanouts []profileFanout
	for _, profileID := range chatMemberProfileIDs {
		if profileID == "" || profileID == senderID {
			continue
		}
		if recipientStates != nil && recipientStates[profileID].IsArchived {
			continue
		}
		notifType := "new_message"
		if recipientStates != nil && recipientStates[profileID].InboxBucket == "requests" {
			notifType = "message_request"
		}
		d, err := json.Marshal(map[string]string{
			"type":              notifType,
			"chat_id":           ms.GetChatId(),
			"message_id":        ms.GetMessageId(),
			"sender_profile_id": senderID,
		})
		if err != nil {
			return nil, false
		}
		fanouts = append(fanouts, profileFanout{
			ProfileID: profileID,
			Envelope:  fanoutEnvelope{Op: "notification", D: d},
		})
	}
	return fanouts, true
}

func reactionNotificationFanouts(ra *eventsv1.ReactionAdded, chatMemberProfileIDs []string, reactionMessageAuthorProfileID string, recipientStates map[string]chatMemberDeliveryState) ([]profileFanout, bool) {
	if ra == nil || ra.GetChatId() == "" || ra.GetMessageId() == "" || ra.GetProfileId() == "" || ra.GetEmoji() == "" {
		return nil, false
	}
	reactorID := ra.GetProfileId()
	authorID := ra.GetMessageAuthorProfileId()
	if authorID == "" {
		authorID = reactionMessageAuthorProfileID
	}
	if authorID == "" && len(chatMemberProfileIDs) == 2 {
		for _, profileID := range chatMemberProfileIDs {
			if profileID != "" && profileID != reactorID {
				authorID = profileID
				break
			}
		}
	}
	if authorID == "" || authorID == reactorID {
		return nil, true
	}
	if recipientStates != nil && recipientStates[authorID].IsArchived {
		return nil, true
	}
	d, err := json.Marshal(map[string]string{
		"type":               "reaction",
		"chat_id":            ra.GetChatId(),
		"message_id":         ra.GetMessageId(),
		"reactor_profile_id": reactorID,
		"emoji":              ra.GetEmoji(),
	})
	if err != nil {
		return nil, false
	}
	return []profileFanout{{
		ProfileID: authorID,
		Envelope:  fanoutEnvelope{Op: "notification", D: d},
	}}, true
}

// archiveActivityFanouts updates an open archived inbox for an incoming
// message without routing a notification-center row or an in-app sound. It
// deliberately uses profile fan-out, because archived chats are normally not
// chat-subscribed. Reactions and mentions do not advance unread state.
func archiveActivityFanouts(data []byte, recipientStates map[string]chatMemberDeliveryState) []profileFanout {
	if len(recipientStates) == 0 {
		return nil
	}
	var event eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &event); err != nil {
		return nil
	}
	message, ok := event.GetPayload().(*eventsv1.MessageStreamEvent_MessageSent)
	if !ok || message.MessageSent == nil {
		return nil
	}
	chatID := message.MessageSent.GetChatId()
	senderID := message.MessageSent.GetSenderProfileId()
	if chatID == "" {
		return nil
	}
	var targets []string
	for profileID, state := range recipientStates {
		if state.IsArchived && profileID != "" && profileID != senderID {
			targets = append(targets, profileID)
		}
	}
	d, err := json.Marshal(map[string]string{"chat_id": chatID})
	if err != nil {
		return nil
	}
	fanouts := make([]profileFanout, 0, len(targets))
	for _, profileID := range targets {
		fanouts = append(fanouts, profileFanout{ProfileID: profileID, Envelope: fanoutEnvelope{Op: "archive_activity", D: d}})
	}
	return fanouts
}

func dispatchMessageStreamEvent(hub *wsHub, data []byte, header nats.Header, logger *slog.Logger, requestID string) {
	chatID, fe, ok := messageEventToFanout(data, header)
	if !ok || chatID == "" {
		if mentionAddedFromBytes(data) == nil {
			return
		}
		chatID = mentionAddedFromBytes(data).GetChatId()
		if chatID == "" {
			return
		}
		// Mention has personal delivery only, but needs the same archive policy.
		fe = fanoutEnvelope{}
		ok = true
	}
	if !ok {
		return
	}
	var recipientStates map[string]chatMemberDeliveryState
	// A nil lister is a local/unit-test configuration with no Chat dependency.
	// A configured lister that returns an error is fail-safe below.
	lookupOK := hub != nil && hub.memberInboxLister == nil
	if hub != nil && hub.memberInboxLister != nil {
		if states, err := hub.memberInboxLister.RecipientDeliveryStates(context.Background(), chatID); err == nil {
			recipientStates = states
			lookupOK = true
		} else if logger != nil {
			logger.Warn("chat member inbox lookup failed", slog.String("chat_id", chatID), slog.Any("error", err))
		}
	}
	// A missing/failed Chat lookup must fail safe for personal notifications;
	// the chat-scoped message/reaction broadcast remains available.
	var fanouts []profileFanout
	notifyOK := false
	if lookupOK {
		fanouts, notifyOK = inAppNotificationFanouts(data, hub.profileIDsSubscribedToChat(chatID), "", recipientStates)
	}
	if lookupOK {
		for _, f := range archiveActivityFanouts(data, recipientStates) {
			hub.broadcastToProfile(f.ProfileID, f.Envelope, logger, requestID)
		}
	}
	if mentionAddedFromBytes(data) != nil {
		if lookupOK {
			dispatchMentionAdded(hub, mentionAddedFromBytes(data), recipientStates, logger, requestID)
		}
		if notifyOK {
			for _, f := range fanouts {
				hub.broadcastToProfile(f.ProfileID, f.Envelope, logger, requestID)
			}
		}
		return
	}
	notifyFirst := isReactionAddedEvent(data)
	if notifyFirst && notifyOK {
		for _, f := range fanouts {
			hub.broadcastToProfile(f.ProfileID, f.Envelope, logger, requestID)
		}
	}
	hub.broadcastToChat(chatID, fe, logger, requestID)
	if !notifyFirst && notifyOK {
		for _, f := range fanouts {
			hub.broadcastToProfile(f.ProfileID, f.Envelope, logger, requestID)
		}
	}
}

func mentionAddedFromBytes(data []byte) *eventsv1.MentionAdded {
	var e eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &e); err != nil {
		return nil
	}
	ma, ok := e.GetPayload().(*eventsv1.MessageStreamEvent_MentionAdded)
	if !ok || ma.MentionAdded == nil {
		return nil
	}
	return ma.MentionAdded
}

func dispatchMentionAdded(hub *wsHub, ma *eventsv1.MentionAdded, recipientStates map[string]chatMemberDeliveryState, logger *slog.Logger, requestID string) {
	senderID := ma.GetSenderProfileId()
	for _, profileID := range ma.GetMentionedProfileIds() {
		if profileID == "" || profileID == senderID {
			continue
		}
		if recipientStates[profileID].IsArchived {
			continue
		}
		d, err := json.Marshal(map[string]string{
			"chat_id":    ma.GetChatId(),
			"message_id": ma.GetMessageId(),
			"profile_id": profileID,
		})
		if err != nil {
			continue
		}
		hub.broadcastToProfile(profileID, fanoutEnvelope{Op: "mention", D: d}, logger, requestID)
	}
}

func isMessageSentEvent(data []byte) bool {
	var e eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &e); err != nil {
		return false
	}
	_, ok := e.GetPayload().(*eventsv1.MessageStreamEvent_MessageSent)
	return ok
}

func isReactionAddedEvent(data []byte) bool {
	var e eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &e); err != nil {
		return false
	}
	_, ok := e.GetPayload().(*eventsv1.MessageStreamEvent_ReactionAdded)
	return ok
}
