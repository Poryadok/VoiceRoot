package main

import (
	"encoding/json"
	"log/slog"

	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

// profileFanout is a personal WebSocket delivery target (op "notification").
type profileFanout struct {
	ProfileID string
	Envelope  fanoutEnvelope
}

func inAppNotificationFanouts(data []byte, chatMemberProfileIDs []string, reactionMessageAuthorProfileID string) ([]profileFanout, bool) {
	var e eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &e); err != nil {
		return nil, false
	}
	switch p := e.GetPayload().(type) {
	case *eventsv1.MessageStreamEvent_MessageSent:
		return newMessageNotificationFanouts(p.MessageSent, chatMemberProfileIDs)
	case *eventsv1.MessageStreamEvent_ReactionAdded:
		return reactionNotificationFanouts(p.ReactionAdded, chatMemberProfileIDs, reactionMessageAuthorProfileID)
	case *eventsv1.MessageStreamEvent_MentionAdded:
		return mentionNotificationFanouts(p.MentionAdded)
	default:
		return nil, false
	}
}

func mentionNotificationFanouts(ma *eventsv1.MentionAdded) ([]profileFanout, bool) {
	if ma == nil || ma.GetChatId() == "" || ma.GetMessageId() == "" {
		return nil, false
	}
	senderID := ma.GetSenderProfileId()
	var fanouts []profileFanout
	for _, profileID := range ma.GetMentionedProfileIds() {
		if profileID == "" || profileID == senderID {
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

func newMessageNotificationFanouts(ms *eventsv1.MessageSent, chatMemberProfileIDs []string) ([]profileFanout, bool) {
	if ms == nil || ms.GetChatId() == "" || ms.GetMessageId() == "" {
		return nil, false
	}
	senderID := ms.GetSenderProfileId()
	var fanouts []profileFanout
	for _, profileID := range chatMemberProfileIDs {
		if profileID == "" || profileID == senderID {
			continue
		}
		d, err := json.Marshal(map[string]string{
			"type":              "new_message",
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

func reactionNotificationFanouts(ra *eventsv1.ReactionAdded, chatMemberProfileIDs []string, reactionMessageAuthorProfileID string) ([]profileFanout, bool) {
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

func dispatchMessageStreamEvent(hub *wsHub, data []byte, logger *slog.Logger, requestID string) {
	if ma := mentionAddedFromBytes(data); ma != nil {
		dispatchMentionAdded(hub, ma, data, logger, requestID)
		return
	}
	chatID, fe, ok := messageEventBytesToFanout(data)
	if !ok || chatID == "" {
		return
	}
	fanouts, notifyOk := inAppNotificationFanouts(data, hub.profileIDsSubscribedToChat(chatID), "")
	notifyFirst := isReactionAddedEvent(data)
	if notifyFirst && notifyOk {
		for _, f := range fanouts {
			hub.broadcastToProfile(f.ProfileID, f.Envelope, logger, requestID)
		}
	}
	hub.broadcastToChat(chatID, fe, logger, requestID)
	if !notifyFirst && notifyOk {
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

func dispatchMentionAdded(hub *wsHub, ma *eventsv1.MentionAdded, data []byte, logger *slog.Logger, requestID string) {
	senderID := ma.GetSenderProfileId()
	for _, profileID := range ma.GetMentionedProfileIds() {
		if profileID == "" || profileID == senderID {
			continue
		}
		d, err := json.Marshal(map[string]string{
			"chat_id":    ma.GetChatId(),
			"message_id": ma.GetMessageId(),
			"user_id":    profileID,
		})
		if err != nil {
			continue
		}
		hub.broadcastToProfile(profileID, fanoutEnvelope{Op: "mention", D: d}, logger, requestID)
	}
	fanouts, ok := inAppNotificationFanouts(data, nil, "")
	if ok {
		for _, f := range fanouts {
			hub.broadcastToProfile(f.ProfileID, f.Envelope, logger, requestID)
		}
	}
}

func isReactionAddedEvent(data []byte) bool {
	var e eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &e); err != nil {
		return false
	}
	_, ok := e.GetPayload().(*eventsv1.MessageStreamEvent_ReactionAdded)
	return ok
}
