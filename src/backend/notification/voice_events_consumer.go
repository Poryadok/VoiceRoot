package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/presence"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
	"voice/backend/pkg/natslog"
	eventsv1 "voice.app/voice/events/v1"
)

const jsStreamVoiceEvents = "voice_events"

func runVoiceEventsConsumer(
	ctx context.Context,
	natsURL string,
	tokens *store.DeviceTokenStore,
	pusher *dispatch.PushDispatcher,
	presenceChecker presence.Checker,
	policy delivery.DeliveryPolicyLoader,
	logger *slog.Logger,
) error {
	if tokens == nil || pusher == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("voice notification consumer: missing deps")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-notification-voice"),
		nats.Timeout(10*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	defer func() { _ = nc.Drain() }()

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}

	handler := &consumer.VoiceEventHandler{Router: delivery.DecideRouting}
	callPusher := &dispatch.CallPusher{Tokens: tokens, Pusher: pusher}
	roomPusher := &dispatch.MatchmakingPusher{Tokens: tokens, Pusher: pusher}
	durable := consumer.SharedDurable("voice")

	isOnline := func(profileID string) bool {
		if presenceChecker == nil {
			return false
		}
		id, err := uuid.Parse(profileID)
		if err != nil {
			return false
		}
		online, err := presenceChecker.IsOnline(ctx, id)
		return err == nil && online
	}

	msgHandler := func(msg *nats.Msg) {
		var env eventsv1.VoiceStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "voice event unmarshal failed")
			consumer.JetStreamTermAck(msg)
			return
		}
		route, err := routeVoiceNotification(ctx, handler, presenceChecker, policy, &env, isOnline)
		if err != nil {
			consumer.JetStreamConsumeAck(msg, err)
			return
		}
		if route == nil {
			consumer.JetStreamConsumeAck(msg, nil)
			return
		}
		natslog.LogConsume(logger, msg, slog.LevelInfo, "voice notification event consumed")
		switch route.kind {
		case voiceRouteCall:
			err = callPusher.SendPush(context.Background(), route.decisions, route.payload)
		case voiceRouteRoom:
			err = roomPusher.SendPush(context.Background(), route.decisions, route.payload)
		}
		consumer.JetStreamConsumeAck(msg, err)
		if err != nil && logger != nil {
			logger.Warn("voice push failed", slog.Any("error", err))
		}
	}

	sub, err := js.Subscribe("voice.>", msgHandler,
		nats.Durable(durable),
		nats.BindStream(jsStreamVoiceEvents),
		nats.ManualAck(),
	)
	if err != nil {
		sub, err = js.Subscribe("", msgHandler, nats.Bind(jsStreamVoiceEvents, durable), nats.ManualAck())
		if err != nil {
			return fmt.Errorf("jetstream subscribe voice.events: %w", err)
		}
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("voice.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}

type voiceRouteKind int

const (
	voiceRouteCall voiceRouteKind = iota
	voiceRouteRoom
)

type voiceNotificationRoute struct {
	kind      voiceRouteKind
	decisions map[string]delivery.DeliveryDecision
	payload   push.Payload
}

type onlineFuncPresenceChecker func(string) bool

func (f onlineFuncPresenceChecker) IsOnline(_ context.Context, profileID uuid.UUID) (bool, error) {
	if f == nil {
		return false, nil
	}
	return f(profileID.String()), nil
}

func voiceEffectivePresenceChecker(presenceChecker presence.Checker, isOnline func(string) bool) presence.Checker {
	if presenceChecker != nil {
		return presenceChecker
	}
	if isOnline == nil {
		return nil
	}
	return onlineFuncPresenceChecker(isOnline)
}

func routeVoiceNotification(
	ctx context.Context,
	h *consumer.VoiceEventHandler,
	presenceChecker presence.Checker,
	policy delivery.DeliveryPolicyLoader,
	env *eventsv1.VoiceStreamEvent,
	isOnline func(profileID string) bool,
) (*voiceNotificationRoute, error) {
	if h == nil || env == nil {
		return nil, nil
	}
	pc := voiceEffectivePresenceChecker(presenceChecker, isOnline)
	switch p := env.GetPayload().(type) {
	case *eventsv1.VoiceStreamEvent_CallIncoming:
		ev := p.CallIncoming
		if ev == nil || ev.GetRoomId() == "" || ev.GetCalleeProfileId() == "" {
			return nil, nil
		}
		expiresAt := ""
		if ev.GetExpiresAt() != nil {
			expiresAt = ev.GetExpiresAt().AsTime().UTC().Format(time.RFC3339)
		}
		raw := h.HandleCallIncoming(ctx, ev, isOnline(ev.GetCalleeProfileId()))
		senderID, _ := uuid.Parse(ev.GetInitiatorProfileId())
		enriched, err := dispatch.EnrichDecisions(ctx, pc, policy, raw, senderID, ev.GetChatId(), delivery.TypeIncomingCall)
		if err != nil {
			return nil, err
		}
		return &voiceNotificationRoute{
			kind:      voiceRouteCall,
			decisions: enriched,
			payload: push.Payload{
				Data: map[string]string{
					"type":                 string(delivery.TypeIncomingCall),
					"room_id":              ev.GetRoomId(),
					"chat_id":              ev.GetChatId(),
					"initiator_profile_id": ev.GetInitiatorProfileId(),
					"callee_profile_id":    ev.GetCalleeProfileId(),
					"media_kind":           ev.GetMediaKind(),
					"livekit_room_name":    ev.GetLivekitRoomName(),
					"expires_at":           expiresAt,
				},
			},
		}, nil
	case *eventsv1.VoiceStreamEvent_VoiceMemberJoined:
		ev := p.VoiceMemberJoined
		if ev == nil || ev.GetJoinedProfileId() == "" {
			return nil, nil
		}
		raw := h.HandleVoiceMemberJoined(ctx, ev, isOnline)
		senderID, _ := uuid.Parse(ev.GetJoinedProfileId())
		enriched, err := dispatch.EnrichDecisions(ctx, pc, policy, raw, senderID, "", delivery.TypeVoiceMemberJoined)
		if err != nil {
			return nil, err
		}
		return &voiceNotificationRoute{
			kind:      voiceRouteRoom,
			decisions: enriched,
			payload: push.Payload{
				Title: "Voice room",
				Body:  "Someone joined the voice room",
				Data: map[string]string{
					"type":               string(delivery.TypeVoiceMemberJoined),
					"room_id":            ev.GetRoomId(),
					"voice_room_id":      ev.GetVoiceRoomId(),
					"space_id":           ev.GetSpaceId(),
					"joined_profile_id":  ev.GetJoinedProfileId(),
					"sender_profile_id":  ev.GetJoinedProfileId(),
				},
			},
		}, nil
	default:
		return nil, nil
	}
}
