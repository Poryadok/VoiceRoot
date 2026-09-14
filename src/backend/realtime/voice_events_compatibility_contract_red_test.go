package main

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

var voiceCompatibilityOccurredAt = time.Date(2026, time.September, 14, 9, 8, 7, 123456789, time.UTC)

type voiceCompatibilityCase struct {
	name           string
	event          *eventsv1.VoiceStreamEvent
	wantOp         string
	wantProfileIDs []string
}

func voiceCompatibilityCases() []voiceCompatibilityCase {
	first := uuid.NewString()
	second := uuid.NewString()
	nonRecipient := uuid.NewString()
	roomID := uuid.NewString()
	chatID := uuid.NewString()
	streamID := uuid.NewString()

	withMetadata := func(event *eventsv1.VoiceStreamEvent) *eventsv1.VoiceStreamEvent {
		event.EventId = uuid.NewString()
		event.OccurredAt = timestamppb.New(voiceCompatibilityOccurredAt)
		return event
	}

	return []voiceCompatibilityCase{
		{
			name: "incoming uses callee only",
			event: withMetadata(&eventsv1.VoiceStreamEvent{
				Payload: &eventsv1.VoiceStreamEvent_CallIncoming{CallIncoming: &eventsv1.CallIncoming{
					RoomId: roomID, ChatId: chatID, InitiatorProfileId: nonRecipient,
					CalleeProfileId: first, MediaKind: "audio", ExpiresAt: timestamppb.New(voiceCompatibilityOccurredAt.Add(time.Minute)),
				}},
			}),
			wantOp:         "call_incoming",
			wantProfileIDs: []string{first},
		},
		{
			name: "accepted compacts profile ids",
			event: withMetadata(&eventsv1.VoiceStreamEvent{
				Payload: &eventsv1.VoiceStreamEvent_CallAccepted{CallAccepted: &eventsv1.CallAccepted{
					RoomId: roomID, ChatId: chatID, AcceptedByProfileId: nonRecipient,
					ProfileIds: []string{first, first, second, second}, MediaKind: "audio",
				}},
			}),
			wantOp:         "call_accepted",
			wantProfileIDs: []string{first, second},
		},
		{
			name: "declined compacts profile ids",
			event: withMetadata(&eventsv1.VoiceStreamEvent{
				Payload: &eventsv1.VoiceStreamEvent_CallDeclined{CallDeclined: &eventsv1.CallDeclined{
					RoomId: roomID, ChatId: chatID, DeclinedByProfileId: nonRecipient,
					ProfileIds: []string{first, first, second, second},
				}},
			}),
			wantOp:         "call_declined",
			wantProfileIDs: []string{first, second},
		},
		{
			name: "missed compacts equal endpoints",
			event: withMetadata(&eventsv1.VoiceStreamEvent{
				Payload: &eventsv1.VoiceStreamEvent_CallMissed{CallMissed: &eventsv1.CallMissed{
					RoomId: roomID, ChatId: chatID, InitiatorProfileId: first, CalleeProfileId: first,
				}},
			}),
			wantOp:         "call_missed",
			wantProfileIDs: []string{first},
		},
		{
			name: "ended compacts profile ids",
			event: withMetadata(&eventsv1.VoiceStreamEvent{
				Payload: &eventsv1.VoiceStreamEvent_CallEnded{CallEnded: &eventsv1.CallEnded{
					RoomId: roomID, ProfileIds: []string{first, first, second, second}, EndedByProfileId: nonRecipient,
				}},
			}),
			wantOp:         "call_ended",
			wantProfileIDs: []string{first, second},
		},
		{
			name: "started compacts profile ids",
			event: withMetadata(&eventsv1.VoiceStreamEvent{
				Payload: &eventsv1.VoiceStreamEvent_CallStarted{CallStarted: &eventsv1.CallStarted{
					RoomId: roomID, ChatId: chatID, InitiatorProfileId: nonRecipient,
					ProfileIds: []string{first, first, second, second}, MediaKind: "audio",
				}},
			}),
			wantOp:         "call_started",
			wantProfileIDs: []string{first, second},
		},
		{
			name: "state change compacts profile ids without inferring changed profile",
			event: withMetadata(&eventsv1.VoiceStreamEvent{
				Payload: &eventsv1.VoiceStreamEvent_VoiceStateChanged{VoiceStateChanged: &eventsv1.VoiceStateChanged{
					RoomId: roomID, ProfileId: nonRecipient, ProfileIds: []string{first, first, second, second},
				}},
			}),
			wantOp:         "voice_state_update",
			wantProfileIDs: []string{first, second},
		},
		{
			name: "screen share start compacts explicit profile ids without inferring sharer",
			event: withMetadata(&eventsv1.VoiceStreamEvent{
				Payload: &eventsv1.VoiceStreamEvent_ScreenShareStarted{ScreenShareStarted: &eventsv1.ScreenShareStarted{
					RoomId: roomID, ProfileId: nonRecipient, StreamId: streamID,
					ProfileIds: []string{first, first, second, second},
				}},
			}),
			wantOp:         "screen_share_started",
			wantProfileIDs: []string{first, second},
		},
		{
			name: "screen share stop compacts explicit profile ids without inferring sharer",
			event: withMetadata(&eventsv1.VoiceStreamEvent{
				Payload: &eventsv1.VoiceStreamEvent_ScreenShareStopped{ScreenShareStopped: &eventsv1.ScreenShareStopped{
					RoomId: roomID, ProfileId: nonRecipient, StreamId: streamID,
					ProfileIds: []string{first, first, second, second},
				}},
			}),
			wantOp:         "screen_share_stopped",
			wantProfileIDs: []string{first, second},
		},
		{
			name: "member join compacts notify ids without inferring joined profile",
			event: withMetadata(&eventsv1.VoiceStreamEvent{
				Payload: &eventsv1.VoiceStreamEvent_VoiceMemberJoined{VoiceMemberJoined: &eventsv1.VoiceMemberJoined{
					RoomId: roomID, VoiceRoomId: uuid.NewString(), SpaceId: uuid.NewString(),
					JoinedProfileId: nonRecipient, NotifyProfileIds: []string{first, first, second, second},
				}},
			}),
			wantOp:         "voice_member_joined",
			wantProfileIDs: []string{first, second},
		},
	}
}

func TestVoiceEventBytesToFanout_UsesCompactedAuthoritativeRecipients(t *testing.T) {
	for _, tc := range voiceCompatibilityCases() {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := proto.Marshal(tc.event)
			if err != nil {
				t.Fatal(err)
			}
			profileIDs, frame, ok := voiceEventBytesToFanout(wire)
			if !ok {
				t.Fatal("valid Voice event was discarded")
			}
			if frame.Op != tc.wantOp {
				t.Fatalf("op=%q want %q", frame.Op, tc.wantOp)
			}
			assertUniqueStringSet(t, profileIDs, tc.wantProfileIDs)
		})
	}
}

func assertUniqueStringSet(t *testing.T, got, want []string) {
	t.Helper()
	gotSet := make(map[string]struct{}, len(got))
	for _, value := range got {
		if _, duplicate := gotSet[value]; duplicate {
			t.Fatalf("values=%v contain duplicate %q", got, value)
		}
		gotSet[value] = struct{}{}
	}
	wantSet := make(map[string]struct{}, len(want))
	for _, value := range want {
		wantSet[value] = struct{}{}
	}
	if !reflect.DeepEqual(gotSet, wantSet) {
		t.Fatalf("values=%v want unique set %v", got, want)
	}
}

func TestVoiceEventBytesToFanout_PropagatesEnvelopeMetadataToEveryOperation(t *testing.T) {
	wantOccurredAt := voiceCompatibilityOccurredAt.Format(time.RFC3339Nano)
	for _, tc := range voiceCompatibilityCases() {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := proto.Marshal(tc.event)
			if err != nil {
				t.Fatal(err)
			}
			_, frame, ok := voiceEventBytesToFanout(wire)
			if !ok {
				t.Fatal("valid Voice event was discarded")
			}
			var payload map[string]any
			if err := json.Unmarshal(frame.D, &payload); err != nil {
				t.Fatal(err)
			}
			if got := payload["event_id"]; got != tc.event.GetEventId() {
				t.Fatalf("event_id=%v want %q", got, tc.event.GetEventId())
			}
			if got := payload["occurred_at"]; got != wantOccurredAt {
				t.Fatalf("occurred_at=%v want %q", got, wantOccurredAt)
			}
		})
	}
}

func TestVoiceEventBytesToFanout_ExposesExactPublicPayloadKeys(t *testing.T) {
	wantKeysByOp := map[string][]string{
		"call_incoming": {
			"event_id", "occurred_at", "room_id", "chat_id", "initiator_profile_id",
			"callee_profile_id", "media_kind", "livekit_room_name", "expires_at",
		},
		"call_accepted": {
			"event_id", "occurred_at", "room_id", "chat_id", "accepted_by_profile_id",
			"profile_ids", "media_kind", "livekit_room_name",
		},
		"call_ended": {
			"event_id", "occurred_at", "room_id", "duration_seconds", "profile_ids",
			"reason", "ended_by_profile_id",
		},
		"voice_state_update": {
			"event_id", "occurred_at", "room_id", "profile_id", "profile_ids",
		},
		"screen_share_started": {
			"event_id", "occurred_at", "room_id", "profile_id", "stream_id",
		},
		"screen_share_stopped": {
			"event_id", "occurred_at", "room_id", "profile_id", "stream_id",
		},
		"voice_member_joined": {
			"event_id", "occurred_at", "room_id", "voice_room_id", "space_id",
			"joined_profile_id", "profile_ids",
		},
	}

	for _, tc := range voiceCompatibilityCases() {
		wantKeys, covered := wantKeysByOp[tc.wantOp]
		if !covered {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			wire, err := proto.Marshal(tc.event)
			if err != nil {
				t.Fatal(err)
			}
			_, frame, ok := voiceEventBytesToFanout(wire)
			if !ok {
				t.Fatal("valid Voice event was discarded")
			}
			var payload map[string]any
			if err := json.Unmarshal(frame.D, &payload); err != nil {
				t.Fatal(err)
			}
			if tc.wantOp == "screen_share_started" || tc.wantOp == "screen_share_stopped" {
				if routingIDs, disclosed := payload["profile_ids"]; disclosed {
					t.Fatalf("screen-share payload disclosed routing profile_ids=%v", routingIDs)
				}
			}
			assertExactJSONKeys(t, payload, wantKeys)
		})
	}
}

func assertExactJSONKeys(t *testing.T, payload map[string]any, want []string) {
	t.Helper()
	wantSet := make(map[string]struct{}, len(want))
	for _, key := range want {
		wantSet[key] = struct{}{}
	}
	gotSet := make(map[string]struct{}, len(payload))
	for key := range payload {
		gotSet[key] = struct{}{}
	}
	if !reflect.DeepEqual(gotSet, wantSet) {
		t.Fatalf("payload keys=%v want exactly %v", gotSet, wantSet)
	}
}

func jsonStringSlice(t *testing.T, value any) []string {
	t.Helper()
	raw, ok := value.([]any)
	if !ok {
		t.Fatalf("value=%T want JSON array", value)
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("array item=%T want string", item)
		}
		result = append(result, text)
	}
	return result
}

func TestVoiceEventBytesToFanout_MemberJoinedBuildsSnapshotWithoutWideningRouting(t *testing.T) {
	first := uuid.NewString()
	second := uuid.NewString()
	joined := uuid.NewString()
	event := &eventsv1.VoiceStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(voiceCompatibilityOccurredAt),
		Payload: &eventsv1.VoiceStreamEvent_VoiceMemberJoined{VoiceMemberJoined: &eventsv1.VoiceMemberJoined{
			RoomId: uuid.NewString(), VoiceRoomId: uuid.NewString(), SpaceId: uuid.NewString(),
			JoinedProfileId: joined, NotifyProfileIds: []string{first, first, second},
		}},
	}
	wire, err := proto.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	profileIDs, frame, ok := voiceEventBytesToFanout(wire)
	if !ok {
		t.Fatal("valid member-joined event was discarded")
	}
	var payload map[string]any
	if err := json.Unmarshal(frame.D, &payload); err != nil {
		t.Fatal(err)
	}

	t.Run("routing remains unique notify recipients only", func(t *testing.T) {
		assertUniqueStringSet(t, profileIDs, []string{first, second})
	})
	t.Run("payload snapshot is unique notify recipients plus joined profile", func(t *testing.T) {
		assertUniqueStringSet(t, jsonStringSlice(t, payload["profile_ids"]), []string{first, second, joined})
	})
}

func TestVoiceEventBytesToFanout_DiscardsInvalidEnvelopeMetadata(t *testing.T) {
	validPayload := func() *eventsv1.VoiceStreamEvent_CallIncoming {
		return &eventsv1.VoiceStreamEvent_CallIncoming{CallIncoming: &eventsv1.CallIncoming{
			RoomId: uuid.NewString(), CalleeProfileId: uuid.NewString(),
			ExpiresAt: timestamppb.New(voiceCompatibilityOccurredAt.Add(time.Minute)),
		}}
	}
	for _, tc := range []struct {
		name       string
		eventID    string
		occurredAt *timestamppb.Timestamp
	}{
		{name: "missing event id", occurredAt: timestamppb.New(voiceCompatibilityOccurredAt)},
		{name: "malformed event id", eventID: "not-a-uuid", occurredAt: timestamppb.New(voiceCompatibilityOccurredAt)},
		{name: "missing occurred at", eventID: uuid.NewString()},
		{name: "out of range occurred at", eventID: uuid.NewString(), occurredAt: &timestamppb.Timestamp{Seconds: 253402300800}},
		{name: "invalid occurred at nanos", eventID: uuid.NewString(), occurredAt: &timestamppb.Timestamp{Seconds: voiceCompatibilityOccurredAt.Unix(), Nanos: 1_000_000_000}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := proto.Marshal(&eventsv1.VoiceStreamEvent{
				EventId: tc.eventID, OccurredAt: tc.occurredAt, Payload: validPayload(),
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, _, ok := voiceEventBytesToFanout(wire); ok {
				t.Fatal("Voice event with invalid envelope metadata must be discarded")
			}
		})
	}
}

func TestVoiceEventBytesToFanout_DoesNotInferAudienceWhenRecipientSetIsEmpty(t *testing.T) {
	roomID := uuid.NewString()
	profileID := uuid.NewString()
	streamID := uuid.NewString()
	for _, tc := range []struct {
		name  string
		event *eventsv1.VoiceStreamEvent
	}{
		{
			name: "missed call without either endpoint",
			event: &eventsv1.VoiceStreamEvent{Payload: &eventsv1.VoiceStreamEvent_CallMissed{
				CallMissed: &eventsv1.CallMissed{RoomId: roomID},
			}},
		},
		{
			name: "screen share started",
			event: &eventsv1.VoiceStreamEvent{Payload: &eventsv1.VoiceStreamEvent_ScreenShareStarted{
				ScreenShareStarted: &eventsv1.ScreenShareStarted{RoomId: roomID, ProfileId: profileID, StreamId: streamID},
			}},
		},
		{
			name: "screen share stopped",
			event: &eventsv1.VoiceStreamEvent{Payload: &eventsv1.VoiceStreamEvent_ScreenShareStopped{
				ScreenShareStopped: &eventsv1.ScreenShareStopped{RoomId: roomID, ProfileId: profileID, StreamId: streamID},
			}},
		},
		{
			name: "member joined",
			event: &eventsv1.VoiceStreamEvent{Payload: &eventsv1.VoiceStreamEvent_VoiceMemberJoined{
				VoiceMemberJoined: &eventsv1.VoiceMemberJoined{RoomId: roomID, JoinedProfileId: profileID},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.event.EventId = uuid.NewString()
			tc.event.OccurredAt = timestamppb.New(voiceCompatibilityOccurredAt)
			wire, err := proto.Marshal(tc.event)
			if err != nil {
				t.Fatal(err)
			}
			if profileIDs, _, ok := voiceEventBytesToFanout(wire); ok {
				t.Fatalf("event accepted with inferred audience %v", profileIDs)
			}
		})
	}
}

func TestVoiceEventBytesToFanout_VoiceStatePreservesOptionalFields(t *testing.T) {
	roomID := uuid.NewString()
	profileID := uuid.NewString()
	recipients := []string{profileID}
	framePayload := func(t *testing.T, state *eventsv1.VoiceStateChanged) map[string]any {
		t.Helper()
		wire, err := proto.Marshal(&eventsv1.VoiceStreamEvent{
			EventId:    uuid.NewString(),
			OccurredAt: timestamppb.New(voiceCompatibilityOccurredAt),
			Payload: &eventsv1.VoiceStreamEvent_VoiceStateChanged{
				VoiceStateChanged: state,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		_, frame, ok := voiceEventBytesToFanout(wire)
		if !ok {
			t.Fatal("valid voice state event was discarded")
		}
		var payload map[string]any
		if err := json.Unmarshal(frame.D, &payload); err != nil {
			t.Fatal(err)
		}
		return payload
	}

	t.Run("omits absent optional state", func(t *testing.T) {
		payload := framePayload(t, &eventsv1.VoiceStateChanged{
			RoomId: roomID, ProfileId: profileID, ProfileIds: recipients, IsMuted: proto.Bool(false),
		})
		if got, exists := payload["is_muted"]; !exists || got != false {
			t.Fatalf("present false is_muted=%v exists=%v", got, exists)
		}
		for _, field := range []string{
			"is_deafened", "is_video_on", "is_commander", "hand_raised", "has_floor", "is_broadcasting",
		} {
			if value, exists := payload[field]; exists {
				t.Fatalf("absent optional field %q was emitted as %v", field, value)
			}
		}
	})

	t.Run("preserves every present optional state", func(t *testing.T) {
		payload := framePayload(t, &eventsv1.VoiceStateChanged{
			RoomId:         roomID,
			ProfileId:      profileID,
			ProfileIds:     recipients,
			IsMuted:        proto.Bool(true),
			IsDeafened:     proto.Bool(false),
			IsVideoOn:      proto.Bool(true),
			IsCommander:    proto.Bool(false),
			HandRaised:     proto.Bool(true),
			HasFloor:       proto.Bool(false),
			IsBroadcasting: proto.Bool(true),
		})
		want := map[string]bool{
			"is_muted": true, "is_deafened": false, "is_video_on": true,
			"is_commander": false, "hand_raised": true, "has_floor": false, "is_broadcasting": true,
		}
		for field, wantValue := range want {
			got, exists := payload[field]
			if !exists || got != wantValue {
				t.Fatalf("%s=%v exists=%v want %v", field, got, exists, wantValue)
			}
		}
	})
}

func TestWSHubBroadcastToProfile_DeliversLifecycleEventToEveryTargetTabOnly(t *testing.T) {
	hub := newWSHub()
	targetProfileID := uuid.NewString()
	otherProfileID := uuid.NewString()
	firstTab := hub.attachConn("instance-a", "target-tab-a", targetProfileID, 1)
	secondTab := hub.attachConn("instance-b", "target-tab-b", targetProfileID, 1)
	nonTargetTab := hub.attachConn("instance-a", "other-tab", otherProfileID, 1)
	frame := fanoutEnvelope{Op: "call_ended", D: json.RawMessage(`{"event_id":"` + uuid.NewString() + `"}`)}

	hub.broadcastToProfile(targetProfileID, frame, nil, "")

	for name, tab := range map[string]*connReg{"first target tab": firstTab, "second target tab": secondTab} {
		select {
		case got := <-tab.fanout:
			if got.Op != frame.Op || !reflect.DeepEqual(got.D, frame.D) {
				t.Fatalf("%s frame=%+v want %+v", name, got, frame)
			}
		default:
			t.Fatalf("%s did not receive lifecycle event", name)
		}
		select {
		case duplicate := <-tab.fanout:
			t.Fatalf("%s received duplicate frame %+v", name, duplicate)
		default:
		}
	}
	select {
	case got := <-nonTargetTab.fanout:
		t.Fatalf("non-target tab received frame %+v", got)
	default:
	}
}
