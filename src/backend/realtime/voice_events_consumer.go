package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/natslog"
)

const jsStreamVoiceEvents = "voice_events"

type voiceEventMetadata struct {
	eventID    string
	occurredAt string
}

func parseVoiceEventMetadata(e *eventsv1.VoiceStreamEvent) (voiceEventMetadata, bool) {
	if e == nil || uuid.Validate(e.GetEventId()) != nil {
		return voiceEventMetadata{}, false
	}
	occurredAt := e.GetOccurredAt()
	if occurredAt == nil || occurredAt.CheckValid() != nil {
		return voiceEventMetadata{}, false
	}
	return voiceEventMetadata{
		eventID:    e.GetEventId(),
		occurredAt: occurredAt.AsTime().UTC().Format(time.RFC3339Nano),
	}, true
}

func (m voiceEventMetadata) marshal(payload map[string]any) ([]byte, error) {
	payload["event_id"] = m.eventID
	payload["occurred_at"] = m.occurredAt
	return json.Marshal(payload)
}

func voiceConsumerDurableName(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "rt_" + strings.ReplaceAll(id, "-", "") + "_voice"
}

func voiceEventBytesToFanout(data []byte) (profileIDs []string, env fanoutEnvelope, ok bool) {
	var e eventsv1.VoiceStreamEvent
	if err := proto.Unmarshal(data, &e); err != nil {
		return nil, fanoutEnvelope{}, false
	}
	metadata, validMetadata := parseVoiceEventMetadata(&e)
	if !validMetadata {
		return nil, fanoutEnvelope{}, false
	}

	switch p := e.GetPayload().(type) {
	case *eventsv1.VoiceStreamEvent_CallIncoming:
		ev := p.CallIncoming
		if ev == nil || ev.GetRoomId() == "" || ev.GetCalleeProfileId() == "" {
			return nil, fanoutEnvelope{}, false
		}
		recipients := compactProfiles(ev.GetCalleeProfileId())
		if len(recipients) == 0 {
			return nil, fanoutEnvelope{}, false
		}
		d, err := metadata.marshal(map[string]any{
			"room_id":              ev.GetRoomId(),
			"chat_id":              ev.GetChatId(),
			"initiator_profile_id": ev.GetInitiatorProfileId(),
			"callee_profile_id":    ev.GetCalleeProfileId(),
			"media_kind":           ev.GetMediaKind(),
			"livekit_room_name":    ev.GetLivekitRoomName(),
			"expires_at":           ev.GetExpiresAt().AsTime().UTC().Format(time.RFC3339),
		})
		if err != nil {
			return nil, fanoutEnvelope{}, false
		}
		return recipients, fanoutEnvelope{Op: "call_incoming", D: d}, true
	case *eventsv1.VoiceStreamEvent_CallAccepted:
		ev := p.CallAccepted
		if ev == nil || ev.GetRoomId() == "" {
			return nil, fanoutEnvelope{}, false
		}
		recipients := compactProfiles(ev.GetProfileIds()...)
		if len(recipients) == 0 {
			return nil, fanoutEnvelope{}, false
		}
		d, err := metadata.marshal(map[string]any{
			"room_id":                ev.GetRoomId(),
			"chat_id":                ev.GetChatId(),
			"accepted_by_profile_id": ev.GetAcceptedByProfileId(),
			"profile_ids":            recipients,
			"media_kind":             ev.GetMediaKind(),
			"livekit_room_name":      ev.GetLivekitRoomName(),
		})
		if err != nil {
			return nil, fanoutEnvelope{}, false
		}
		return recipients, fanoutEnvelope{Op: "call_accepted", D: d}, true
	case *eventsv1.VoiceStreamEvent_CallDeclined:
		ev := p.CallDeclined
		if ev == nil || ev.GetRoomId() == "" {
			return nil, fanoutEnvelope{}, false
		}
		recipients := compactProfiles(ev.GetProfileIds()...)
		if len(recipients) == 0 {
			return nil, fanoutEnvelope{}, false
		}
		d, err := metadata.marshal(map[string]any{
			"room_id":                ev.GetRoomId(),
			"chat_id":                ev.GetChatId(),
			"declined_by_profile_id": ev.GetDeclinedByProfileId(),
			"profile_ids":            recipients,
		})
		if err != nil {
			return nil, fanoutEnvelope{}, false
		}
		return recipients, fanoutEnvelope{Op: "call_declined", D: d}, true
	case *eventsv1.VoiceStreamEvent_CallMissed:
		ev := p.CallMissed
		if ev == nil || ev.GetRoomId() == "" {
			return nil, fanoutEnvelope{}, false
		}
		recipients := compactProfiles(ev.GetInitiatorProfileId(), ev.GetCalleeProfileId())
		if len(recipients) == 0 {
			return nil, fanoutEnvelope{}, false
		}
		d, err := metadata.marshal(map[string]any{
			"room_id":              ev.GetRoomId(),
			"chat_id":              ev.GetChatId(),
			"initiator_profile_id": ev.GetInitiatorProfileId(),
			"callee_profile_id":    ev.GetCalleeProfileId(),
		})
		if err != nil {
			return nil, fanoutEnvelope{}, false
		}
		return recipients, fanoutEnvelope{Op: "call_missed", D: d}, true
	case *eventsv1.VoiceStreamEvent_CallEnded:
		ev := p.CallEnded
		if ev == nil || ev.GetRoomId() == "" {
			return nil, fanoutEnvelope{}, false
		}
		recipients := compactProfiles(ev.GetProfileIds()...)
		if len(recipients) == 0 {
			return nil, fanoutEnvelope{}, false
		}
		d, err := metadata.marshal(map[string]any{
			"room_id":             ev.GetRoomId(),
			"duration_seconds":    ev.GetDurationSeconds(),
			"profile_ids":         recipients,
			"reason":              ev.GetReason(),
			"ended_by_profile_id": ev.GetEndedByProfileId(),
		})
		if err != nil {
			return nil, fanoutEnvelope{}, false
		}
		return recipients, fanoutEnvelope{Op: "call_ended", D: d}, true
	case *eventsv1.VoiceStreamEvent_VoiceStateChanged:
		ev := p.VoiceStateChanged
		if ev == nil || ev.GetRoomId() == "" {
			return nil, fanoutEnvelope{}, false
		}
		recipients := compactProfiles(ev.GetProfileIds()...)
		if len(recipients) == 0 {
			return nil, fanoutEnvelope{}, false
		}
		payload := map[string]any{
			"room_id":     ev.GetRoomId(),
			"profile_id":  ev.GetProfileId(),
			"profile_ids": recipients,
		}
		if ev.IsMuted != nil {
			payload["is_muted"] = ev.GetIsMuted()
		}
		if ev.IsDeafened != nil {
			payload["is_deafened"] = ev.GetIsDeafened()
		}
		if ev.IsVideoOn != nil {
			payload["is_video_on"] = ev.GetIsVideoOn()
		}
		if ev.IsCommander != nil {
			payload["is_commander"] = ev.GetIsCommander()
		}
		if ev.HandRaised != nil {
			payload["hand_raised"] = ev.GetHandRaised()
		}
		if ev.HasFloor != nil {
			payload["has_floor"] = ev.GetHasFloor()
		}
		if ev.IsBroadcasting != nil {
			payload["is_broadcasting"] = ev.GetIsBroadcasting()
		}
		d, err := metadata.marshal(payload)
		if err != nil {
			return nil, fanoutEnvelope{}, false
		}
		return recipients, fanoutEnvelope{Op: "voice_state_update", D: d}, true
	case *eventsv1.VoiceStreamEvent_ScreenShareStarted:
		ev := p.ScreenShareStarted
		if ev == nil || ev.GetRoomId() == "" || ev.GetProfileId() == "" {
			return nil, fanoutEnvelope{}, false
		}
		recipients := compactProfiles(ev.GetProfileIds()...)
		if len(recipients) == 0 {
			return nil, fanoutEnvelope{}, false
		}
		d, err := metadata.marshal(map[string]any{
			"room_id":    ev.GetRoomId(),
			"profile_id": ev.GetProfileId(),
			"stream_id":  ev.GetStreamId(),
		})
		if err != nil {
			return nil, fanoutEnvelope{}, false
		}
		return recipients, fanoutEnvelope{Op: "screen_share_started", D: d}, true
	case *eventsv1.VoiceStreamEvent_ScreenShareStopped:
		ev := p.ScreenShareStopped
		if ev == nil || ev.GetRoomId() == "" || ev.GetProfileId() == "" {
			return nil, fanoutEnvelope{}, false
		}
		recipients := compactProfiles(ev.GetProfileIds()...)
		if len(recipients) == 0 {
			return nil, fanoutEnvelope{}, false
		}
		d, err := metadata.marshal(map[string]any{
			"room_id":    ev.GetRoomId(),
			"profile_id": ev.GetProfileId(),
			"stream_id":  ev.GetStreamId(),
		})
		if err != nil {
			return nil, fanoutEnvelope{}, false
		}
		return recipients, fanoutEnvelope{Op: "screen_share_stopped", D: d}, true
	case *eventsv1.VoiceStreamEvent_CallStarted:
		ev := p.CallStarted
		if ev == nil || ev.GetRoomId() == "" {
			return nil, fanoutEnvelope{}, false
		}
		recipients := compactProfiles(ev.GetProfileIds()...)
		if len(recipients) == 0 {
			return nil, fanoutEnvelope{}, false
		}
		payload := map[string]any{
			"room_id":              ev.GetRoomId(),
			"chat_id":              ev.GetChatId(),
			"initiator_profile_id": ev.GetInitiatorProfileId(),
			"callee_profile_id":    ev.GetCalleeProfileId(),
			"profile_ids":          recipients,
			"media_kind":           ev.GetMediaKind(),
			"livekit_room_name":    ev.GetLivekitRoomName(),
		}
		if ev.GetRoomType() != "" {
			payload["room_type"] = ev.GetRoomType()
		}
		if ev.GetRoomType() == "voice_room" && uuid.Validate(ev.GetVoiceRoomId()) == nil && uuid.Validate(ev.GetSpaceId()) == nil {
			payload["voice_room_id"] = ev.GetVoiceRoomId()
			payload["space_id"] = ev.GetSpaceId()
		}
		d, err := metadata.marshal(payload)
		if err != nil {
			return nil, fanoutEnvelope{}, false
		}
		return recipients, fanoutEnvelope{Op: "call_started", D: d}, true
	case *eventsv1.VoiceStreamEvent_VoiceMemberJoined:
		ev := p.VoiceMemberJoined
		if ev == nil || ev.GetRoomId() == "" || ev.GetJoinedProfileId() == "" {
			return nil, fanoutEnvelope{}, false
		}
		notify := compactProfiles(ev.GetNotifyProfileIds()...)
		if len(notify) == 0 {
			return nil, fanoutEnvelope{}, false
		}
		snapshot := compactProfiles(append(append([]string(nil), notify...), ev.GetJoinedProfileId())...)
		d, err := metadata.marshal(map[string]any{
			"room_id":           ev.GetRoomId(),
			"voice_room_id":     ev.GetVoiceRoomId(),
			"space_id":          ev.GetSpaceId(),
			"joined_profile_id": ev.GetJoinedProfileId(),
			"profile_ids":       snapshot,
		})
		if err != nil {
			return nil, fanoutEnvelope{}, false
		}
		return notify, fanoutEnvelope{Op: "voice_member_joined", D: d}, true
	default:
		return nil, fanoutEnvelope{}, false
	}
}

func compactProfiles(ids ...string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func voiceEventLogAttrs(data []byte) []slog.Attr {
	var env eventsv1.VoiceStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return nil
	}
	return []slog.Attr{slog.String("event_id", env.GetEventId())}
}

func subscribeVoiceEvents(js nats.JetStreamContext, hub *wsHub, instanceID string, logger *slog.Logger) (*nats.Subscription, error) {
	durable := voiceConsumerDurableName(instanceID)
	handler := func(msg *nats.Msg) {
		consumeVoiceEventMessage(msg, hub, logger, func(message *nats.Msg) error {
			return message.Ack()
		})
	}
	sub, err := js.Subscribe("voice.>", handler, nats.Bind(jsStreamVoiceEvents, durable), nats.ManualAck())
	if err != nil {
		return nil, fmt.Errorf("bind pre-provisioned voice.events consumer %q: %w", durable, err)
	}
	return sub, nil
}

func consumeVoiceEventMessage(msg *nats.Msg, hub *wsHub, logger *slog.Logger, ack func(*nats.Msg) error) {
	defer func() {
		if err := ack(msg); err != nil && logger != nil {
			logger.Warn("voice event ack failed", slog.String("error", err.Error()))
		}
	}()
	attrs := voiceEventLogAttrs(msg.Data)
	if profileIDs, fe, ok := voiceEventBytesToFanout(msg.Data); ok {
		natslog.LogConsume(logger, msg, slog.LevelInfo, "voice event consumed", attrs...)
		for _, profileID := range compactProfiles(profileIDs...) {
			hub.broadcastToProfile(profileID, fe, logger, natslog.RequestIDFromMsg(msg))
		}
		return
	}
	natslog.LogConsume(logger, msg, slog.LevelWarn, "unknown voice event payload", attrs...)
}

func runVoiceEventsConsumer(ctx context.Context, hub *wsHub, natsURL, instanceID string, logger *slog.Logger) error {
	if hub == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("voice events consumer: missing hub or NATS URL")
	}
	nc, err := nats.Connect(natsURL, natsConnectOptions("voice-realtime-voice-events")...)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	defer func() { _ = nc.Drain() }()

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}

	sub, err := subscribeJetStreamWithRetry(ctx, "realtime voice.events", func() (*nats.Subscription, error) {
		return subscribeVoiceEvents(js, hub, instanceID, logger)
	})
	if err != nil {
		return err
	}
	markRealtimeConsumerBound(ctx)
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("voice.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}
