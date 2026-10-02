package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

// profileFanout is a personal WebSocket delivery target (op "notification").
type profileFanout struct {
	ProfileID string
	Envelope  fanoutEnvelope
}

// sharedChatRecipientAllowSet makes one one-way block decision per account
// and maps it back to all live profiles/tabs for that account.
func sharedChatRecipientAllowSet(hub *wsHub, chatID, senderProfileID string, profileIDs []string, logger *slog.Logger) map[string]bool {
	allowed := make(map[string]bool, len(profileIDs))
	if hub == nil {
		return allowed
	}
	for _, profileID := range profileIDs {
		if profileID == senderProfileID && profileID != "" {
			allowed[profileID] = true
		}
	}
	policy, ok := hub.subscriptionChecker.(messageSentBlockPolicy)
	if hub.subscriptionChecker == nil {
		for _, profileID := range profileIDs {
			allowed[profileID] = true
		}
		return allowed
	}
	if !ok {
		if logger != nil {
			logger.Warn("message block policy is not configured", slog.String("chat_id", chatID))
		}
		return allowed
	}
	senderAccountID, err := policy.MessageSenderAccount(context.Background(), senderProfileID)
	if err != nil {
		if logger != nil {
			logger.Warn("message sender account lookup failed", slog.String("chat_id", chatID), slog.Any("error", err))
		}
		return allowed
	}
	shared, err := policy.MessageChatIsShared(context.Background(), chatID, senderAccountID, senderProfileID)
	if err != nil {
		if logger != nil {
			logger.Warn("message chat type lookup failed", slog.String("chat_id", chatID), slog.Any("error", err))
		}
		return allowed
	}
	if !shared {
		for _, profileID := range profileIDs {
			allowed[profileID] = true
		}
		return allowed
	}
	profilesByAccount := make(map[string][]string)
	hub.mu.RLock()
	for _, profileID := range profileIDs {
		if profileID == "" || profileID == senderProfileID {
			continue
		}
		for reg := range hub.byProfile[profileID] {
			if accountID := canonicalUUID(reg.accountID); accountID != "" {
				profilesByAccount[accountID] = append(profilesByAccount[accountID], profileID)
				break
			}
		}
	}
	hub.mu.RUnlock()
	type decision struct {
		accountID string
		allow     bool
	}
	jobs := make(chan string)
	results := make(chan decision, len(profilesByAccount))
	workerCount := 16
	if len(profilesByAccount) < workerCount {
		workerCount = len(profilesByAccount)
	}
	var workers sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for accountID := range jobs {
				if accountID == senderAccountID {
					results <- decision{accountID, true}
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), chatSubscriptionCheckTimeout)
				blocked, lookupErr := policy.MessageRecipientBlocked(ctx, accountID, senderAccountID)
				cancel()
				if lookupErr != nil && logger != nil {
					logger.Warn("message recipient block lookup failed", slog.String("chat_id", chatID), slog.String("recipient_account_id", accountID), slog.Any("error", lookupErr))
				}
				results <- decision{accountID, lookupErr == nil && !blocked}
			}
		}()
	}
	go func() {
		for accountID := range profilesByAccount {
			jobs <- accountID
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()
	for result := range results {
		for _, profileID := range profilesByAccount[result.accountID] {
			allowed[profileID] = result.allow
		}
	}
	return allowed
}

func filterProfileFanouts(fanouts []profileFanout, allowed map[string]bool) []profileFanout {
	filtered := make([]profileFanout, 0, len(fanouts))
	for _, fanout := range fanouts {
		if allowed[fanout.ProfileID] {
			filtered = append(filtered, fanout)
		}
	}
	return filtered
}

func messageSentFromBytes(data []byte) *eventsv1.MessageSent {
	var event eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &event); err != nil {
		return nil
	}
	payload, ok := event.GetPayload().(*eventsv1.MessageStreamEvent_MessageSent)
	if !ok || payload.MessageSent == nil {
		return nil
	}
	return payload.MessageSent
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
		if recipientStates != nil && recipientStates[profileID].InboxBucket == "declined" {
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
	var recipientIDs []string
	if hub != nil {
		recipientIDs = hub.profileIDsSubscribedToChat(chatID)
	}
	if lookupOK {
		if hub.memberInboxLister != nil {
			// A new DM request recipient has no chat subscription yet. Add only
			// authoritative request-bucket members; keep ordinary notification
			// delivery scoped to existing chat subscriptions.
			seen := make(map[string]struct{}, len(recipientIDs))
			for _, profileID := range recipientIDs {
				seen[profileID] = struct{}{}
			}
			for profileID, state := range recipientStates {
				if state.InboxBucket == "requests" {
					if _, exists := seen[profileID]; !exists {
						recipientIDs = append(recipientIDs, profileID)
					}
				}
			}
		}
		fanouts, notifyOK = inAppNotificationFanouts(data, recipientIDs, "", recipientStates)
	}
	var archiveFanouts []profileFanout
	if lookupOK {
		archiveFanouts = archiveActivityFanouts(data, recipientStates)
	}
	var allowedProfiles map[string]bool
	var senderProfileID string
	if messageSent := messageSentFromBytes(data); messageSent != nil {
		senderProfileID = messageSent.GetSenderProfileId()
	} else if mentionAdded := mentionAddedFromBytes(data); mentionAdded != nil {
		senderProfileID = mentionAdded.GetSenderProfileId()
	}
	if messageSentFromBytes(data) != nil || mentionAddedFromBytes(data) != nil {
		candidates := append([]string(nil), recipientIDs...)
		for _, fanout := range fanouts {
			candidates = append(candidates, fanout.ProfileID)
		}
		for _, fanout := range archiveFanouts {
			candidates = append(candidates, fanout.ProfileID)
		}
		allowedProfiles = sharedChatRecipientAllowSet(hub, chatID, senderProfileID, candidates, logger)
		fanouts = filterProfileFanouts(fanouts, allowedProfiles)
		archiveFanouts = filterProfileFanouts(archiveFanouts, allowedProfiles)
	}
	for _, f := range archiveFanouts {
		hub.broadcastToProfile(f.ProfileID, f.Envelope, logger, requestID)
	}
	if mentionAddedFromBytes(data) != nil {
		if lookupOK {
			dispatchMentionAdded(hub, mentionAddedFromBytes(data), recipientStates, allowedProfiles, logger, requestID)
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
	if messageSentFromBytes(data) != nil {
		hub.broadcastToChatFiltered(chatID, fe, allowedProfiles, logger, requestID)
	} else {
		hub.broadcastToChat(chatID, fe, logger, requestID)
	}
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

func dispatchMentionAdded(hub *wsHub, ma *eventsv1.MentionAdded, recipientStates map[string]chatMemberDeliveryState, allowedProfiles map[string]bool, logger *slog.Logger, requestID string) {
	senderID := ma.GetSenderProfileId()
	for _, profileID := range ma.GetMentionedProfileIds() {
		if profileID == "" || profileID == senderID {
			continue
		}
		if allowedProfiles != nil && !allowedProfiles[profileID] {
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

func isReactionAddedEvent(data []byte) bool {
	var e eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &e); err != nil {
		return false
	}
	_, ok := e.GetPayload().(*eventsv1.MessageStreamEvent_ReactionAdded)
	return ok
}
