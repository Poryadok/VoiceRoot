package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

const jsStreamVoiceEvents = "voice_events"

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

	switch p := e.GetPayload().(type) {
	case *eventsv1.VoiceStreamEvent_CallIncoming:
		ev := p.CallIncoming
		if ev == nil || ev.GetRoomId() == "" || ev.GetCalleeProfileId() == "" {
			return nil, fanoutEnvelope{}, false
		}
		d, err := json.Marshal(map[string]any{
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
		return []string{ev.GetCalleeProfileId()}, fanoutEnvelope{Op: "call_incoming", D: d}, true
	case *eventsv1.VoiceStreamEvent_CallAccepted:
		ev := p.CallAccepted
		if ev == nil || ev.GetRoomId() == "" || len(ev.GetProfileIds()) == 0 {
			return nil, fanoutEnvelope{}, false
		}
		d, err := json.Marshal(map[string]any{
			"room_id":                ev.GetRoomId(),
			"chat_id":                ev.GetChatId(),
			"accepted_by_profile_id": ev.GetAcceptedByProfileId(),
			"profile_ids":            ev.GetProfileIds(),
			"media_kind":             ev.GetMediaKind(),
			"livekit_room_name":      ev.GetLivekitRoomName(),
		})
		if err != nil {
			return nil, fanoutEnvelope{}, false
		}
		return ev.GetProfileIds(), fanoutEnvelope{Op: "call_accepted", D: d}, true
	case *eventsv1.VoiceStreamEvent_CallDeclined:
		ev := p.CallDeclined
		if ev == nil || ev.GetRoomId() == "" || len(ev.GetProfileIds()) == 0 {
			return nil, fanoutEnvelope{}, false
		}
		d, err := json.Marshal(map[string]any{
			"room_id":                ev.GetRoomId(),
			"chat_id":                ev.GetChatId(),
			"declined_by_profile_id": ev.GetDeclinedByProfileId(),
			"profile_ids":            ev.GetProfileIds(),
		})
		if err != nil {
			return nil, fanoutEnvelope{}, false
		}
		return ev.GetProfileIds(), fanoutEnvelope{Op: "call_declined", D: d}, true
	case *eventsv1.VoiceStreamEvent_CallMissed:
		ev := p.CallMissed
		if ev == nil || ev.GetRoomId() == "" {
			return nil, fanoutEnvelope{}, false
		}
		d, err := json.Marshal(map[string]any{
			"room_id":              ev.GetRoomId(),
			"chat_id":              ev.GetChatId(),
			"initiator_profile_id": ev.GetInitiatorProfileId(),
			"callee_profile_id":    ev.GetCalleeProfileId(),
		})
		if err != nil {
			return nil, fanoutEnvelope{}, false
		}
		return compactProfiles(ev.GetInitiatorProfileId(), ev.GetCalleeProfileId()), fanoutEnvelope{Op: "call_missed", D: d}, true
	case *eventsv1.VoiceStreamEvent_CallEnded:
		ev := p.CallEnded
		if ev == nil || ev.GetRoomId() == "" || len(ev.GetProfileIds()) == 0 {
			return nil, fanoutEnvelope{}, false
		}
		d, err := json.Marshal(map[string]any{
			"room_id":             ev.GetRoomId(),
			"duration_seconds":    ev.GetDurationSeconds(),
			"profile_ids":         ev.GetProfileIds(),
			"reason":              ev.GetReason(),
			"ended_by_profile_id": ev.GetEndedByProfileId(),
		})
		if err != nil {
			return nil, fanoutEnvelope{}, false
		}
		return ev.GetProfileIds(), fanoutEnvelope{Op: "call_ended", D: d}, true
	case *eventsv1.VoiceStreamEvent_VoiceStateChanged:
		ev := p.VoiceStateChanged
		if ev == nil || ev.GetRoomId() == "" || len(ev.GetProfileIds()) == 0 {
			return nil, fanoutEnvelope{}, false
		}
		d, err := json.Marshal(map[string]any{
			"room_id":     ev.GetRoomId(),
			"profile_id":  ev.GetProfileId(),
			"is_muted":    ev.GetIsMuted(),
			"is_deafened": ev.GetIsDeafened(),
			"is_video_on": ev.GetIsVideoOn(),
			"profile_ids": ev.GetProfileIds(),
		})
		if err != nil {
			return nil, fanoutEnvelope{}, false
		}
		return ev.GetProfileIds(), fanoutEnvelope{Op: "voice_state_update", D: d}, true
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

func subscribeVoiceEvents(js nats.JetStreamContext, hub *wsHub, instanceID string) (*nats.Subscription, error) {
	durable := voiceConsumerDurableName(instanceID)
	handler := func(msg *nats.Msg) {
		if profileIDs, fe, ok := voiceEventBytesToFanout(msg.Data); ok {
			for _, profileID := range compactProfiles(profileIDs...) {
				hub.broadcastToProfile(profileID, fe)
			}
		}
	}
	sub, err := js.Subscribe("voice.>", handler,
		nats.Durable(durable),
		nats.BindStream(jsStreamVoiceEvents),
		nats.DeliverNew(),
	)
	if err != nil {
		sub, err = js.Subscribe("", handler, nats.Bind(jsStreamVoiceEvents, durable))
		if err != nil {
			return nil, fmt.Errorf("jetstream subscribe voice.events: %w", err)
		}
	}
	return sub, nil
}

func runVoiceEventsConsumer(ctx context.Context, hub *wsHub, natsURL, instanceID string) error {
	if hub == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("voice events consumer: missing hub or NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-realtime-voice-events"),
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

	sub, err := subscribeJetStreamWithRetry(ctx, "realtime voice.events", func() (*nats.Subscription, error) {
		return subscribeVoiceEvents(js, hub, instanceID)
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil {
			log.Printf("realtime voice.events unsubscribe: %v", err)
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}
