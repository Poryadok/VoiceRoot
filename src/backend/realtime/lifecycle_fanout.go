package main

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"strings"
	"time"
	"voice/backend/realtime/internal/lifecycle"
)

type chatLifecycleRunner interface {
	Run(context.Context, uuid.UUID, func(*lifecycle.Scope)) error
}

// chatOrigin is routing metadata, never authorization. Typed origin supplied
// by a chat-addressed producer must agree with any payload origin. Personal
// message notifications retain the same origin even without a subscription.
func chatOrigin(env fanoutEnvelope) (uuid.UUID, bool, bool) {
	required := strings.HasPrefix(env.Op, "message_") || strings.HasPrefix(env.Op, "chat_") || env.Op == "typing" || env.Op == "mark_read" || env.Op == "archive_activity" || env.Op == "dm_peer_deleted"
	var body struct {
		ChatID string `json:"chat_id"`
		Type   string `json:"type"`
	}
	if len(env.D) > 0 && json.Unmarshal(env.D, &body) != nil {
		return uuid.Nil, required, false
	}
	if env.Op == "notification" {
		switch body.Type {
		case "new_message", "message_request", "mention", "reaction", "reply":
			required = true
		}
	}
	raw := env.chatID
	if raw == "" {
		raw = body.ChatID
	} else if body.ChatID != "" && body.ChatID != raw {
		return uuid.Nil, true, false
	}
	if raw == "" {
		return uuid.Nil, required, !required
	}
	id, err := uuid.Parse(raw)
	return id, true, err == nil && id != uuid.Nil && id.String() == raw
}

// sendFanout owns one finite synchronous fanout across all local recipients.
// No goroutine may use its captured scope after this call settles it.
func (h *wsHub) sendFanout(targets []*connReg, env fanoutEnvelope, disconnect bool) {
	send := func(envelope fanoutEnvelope) {
		for _, reg := range targets {
			result := reg.enqueue(envelope, disconnect)
			if envelope.observeMetrics {
				observeWSFanoutEnqueue(result)
			}
		}
	}
	if h.lifecycle == nil {
		send(env)
		return
	}
	chat, required, valid := chatOrigin(env)
	if !valid {
		return
	}
	if !required {
		send(env)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = h.lifecycle.Run(ctx, chat, func(scope *lifecycle.Scope) { env.chatID = chat.String(); env.admission = scope; send(env) })
}
