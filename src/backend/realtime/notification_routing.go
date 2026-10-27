package main

import (
	"context"
	"errors"
	"github.com/nats-io/nats.go"
	"time"
	"voice/backend/pkg/notificationrouting"
)

type messageNotificationRouter interface {
	Route(context.Context, []byte) ([]notificationrouting.Target, error)
}
type natsMessageNotificationRouter struct{ connection *nats.Conn }

func (r natsMessageNotificationRouter) Route(ctx context.Context, event []byte) ([]notificationrouting.Target, error) {
	return notificationrouting.Request(ctx, notificationrouting.Client{Connection: r.connection}, event)
}

func (h *wsHub) setNotificationRouter(router messageNotificationRouter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.notificationRouter = router
}
func (h *wsHub) messageNotificationRouter() messageNotificationRouter {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.notificationRouter
}

// notificationPolicyFanouts keeps payload shaping in Realtime. Notification
// selects only the allowed target/type pairs for these exact source bytes.
func notificationPolicyFanouts(ctx context.Context, router messageNotificationRouter, data []byte) ([]profileFanout, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	targets, e := router.Route(ctx, data)
	if e != nil {
		return nil, e
	}
	allowed := make(map[string]bool, len(targets))
	states := make(map[string]chatMemberDeliveryState, len(targets))
	ids := make([]string, 0, len(targets))
	author := ""
	for _, target := range targets {
		allowed[target.ProfileID] = true
		ids = append(ids, target.ProfileID)
		switch {
		case messageSentFromBytes(data) != nil:
			if target.Type != "new_message" && target.Type != "message_request" {
				return nil, errors.New("notification routing type mismatch")
			}
			if target.Type == "message_request" {
				states[target.ProfileID] = chatMemberDeliveryState{InboxBucket: "requests"}
			}
		case mentionAddedFromBytes(data) != nil:
			if target.Type != "mention" {
				return nil, errors.New("notification routing type mismatch")
			}
		case isReactionAddedEvent(data):
			if target.Type != "reaction" || author != "" {
				return nil, errors.New("notification routing type mismatch")
			}
			author = target.ProfileID
		default:
			return nil, errors.New("unsupported notification event")
		}
	}
	fanouts, ok := inAppNotificationFanouts(data, ids, author, states)
	if !ok {
		return nil, errors.New("invalid notification source")
	}
	return filterProfileFanouts(fanouts, allowed), nil
}
