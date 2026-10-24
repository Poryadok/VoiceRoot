package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
)

var ordinaryRoleSubjects = []string{"role.created", "role.updated", "role.deleted", "role.assigned", "role.revoked", "role.chat_override_set", "role.chat_override_removed", "role.voice_override_set", "role.voice_override_removed"}

func ordinaryRoleFixture(t *testing.T, subject string) (*eventsv1.RoleStreamEvent, *eventsv1.RoleMutation) {
	t.Helper()
	payload := &eventsv1.RoleMutation{SpaceId: "space-1", RoleId: "role-1", ProfileId: "profile-1", Name: "Роль \"one\"", ChatId: "chat-1", VoiceRoomId: "voice-1", ChangedFields: []string{"name", "permissions"}}
	event := &eventsv1.RoleStreamEvent{EventId: "11111111-1111-4111-8111-111111111111", OccurredAt: timestamppb.New(time.Unix(1700000000, 0))}
	field := event.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(strings.ReplaceAll(subject, ".", "_")))
	require.NotNil(t, field)
	event.ProtoReflect().Set(field, protoreflect.ValueOfMessage(payload.ProtoReflect()))
	return event, payload
}

func TestOrdinaryRoleWireActualHubKeepsRecipientBoundary(t *testing.T) {
	server := startRealtimeJSTestServer(t)
	connection, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(connection.Close)
	js, err := connection.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: jsStreamRoleEvents, Subjects: []string{"role.>"}})
	require.NoError(t, err)
	const instance = "role-wire-boundary"
	preprovisionRealtimeConsumer(t, js, jsStreamRoleEvents, roleConsumerDurableName(instance), "role.>")
	hub := newWSHub()
	selected := hub.attachConn("inst", "selected", "profile-1", 32)
	otherDevice := hub.attachConn("inst", "other-device", "profile-1", 32)
	chatPeer := hub.attachConn("inst", "chat-peer", "profile-2", 32)
	unrelated := hub.attachConn("inst", "unrelated", "profile-3", 32)
	require.True(t, hub.addChat(selected, "chat-1"))
	require.True(t, hub.addChat(chatPeer, "chat-1"))
	require.True(t, hub.addChat(unrelated, "chat-other"))
	subscription, err := subscribeRoleEvents(js, hub, instance, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subscription.Unsubscribe() })
	for _, subject := range ordinaryRoleSubjects {
		event, payload := ordinaryRoleFixture(t, subject)
		binary, marshalErr := proto.Marshal(event)
		require.NoError(t, marshalErr)
		legacy, marshalErr := json.Marshal(roleEventJSON{SpaceID: payload.SpaceId, RoleID: payload.RoleId, ProfileID: payload.ProfileId, Name: payload.Name, ChatID: payload.ChatId, VoiceRoomID: payload.VoiceRoomId})
		require.NoError(t, marshalErr)
		for _, data := range [][]byte{legacy, binary} {
			_, publishErr := js.Publish(subject, data)
			require.NoError(t, publishErr)
		}
	}
	policy, err := proto.Marshal(&eventsv1.RoleStreamEvent{EventId: "22222222-2222-4222-8222-222222222222", OccurredAt: timestamppb.Now(), Payload: &eventsv1.RoleStreamEvent_VoiceRoomPolicyInvalidated{VoiceRoomPolicyInvalidated: &eventsv1.VoiceRoomPolicyInvalidated{SpaceId: "space-1", PolicyEpoch: 1}}})
	require.NoError(t, err)
	_, err = js.Publish("role.voice_policy_invalidated", policy)
	require.NoError(t, err)
	barrier, err := json.Marshal(roleEventJSON{SpaceID: "space-1", RoleID: "role-1", ChatID: "chat-1", Name: "ordered-barrier"})
	require.NoError(t, err)
	_, err = js.Publish("role.chat_override_removed", barrier)
	require.NoError(t, err)
	for _, target := range []struct {
		name   string
		frames <-chan fanoutEnvelope
		want   int
	}{{"selected", selected.fanout, 8}, {"chat peer", chatPeer.fanout, 4}} {
		count := 0
		for {
			select {
			case frame := <-target.frames:
				var data map[string]string
				require.NoError(t, json.Unmarshal(frame.D, &data))
				require.Equal(t, "role_update", frame.Op)
				require.Len(t, data, 7)
				if data["name"] == "ordered-barrier" {
					require.Equal(t, target.want, count, target.name)
					goto drained
				}
				count++
			case <-time.After(5 * time.Second):
				t.Fatalf("%s did not receive ordered barrier", target.name)
			}
		}
	drained:
	}
	for index := 0; index < 4; index++ {
		select {
		case frame := <-otherDevice.fanout:
			var data map[string]string
			require.NoError(t, json.Unmarshal(frame.D, &data))
			require.Contains(t, []string{"role.assigned", "role.revoked"}, data["subject"])
		case <-time.After(time.Second):
			t.Fatal("profile mutation missed second device")
		}
	}
	select {
	case frame := <-otherDevice.fanout:
		t.Fatalf("non-profile event reached second device: %+v", frame)
	default:
	}
	select {
	case frame := <-unrelated.fanout:
		t.Fatalf("ordinary Role event reached unrelated connection: %+v", frame)
	default:
	}
}

func TestOrdinaryRoleWirePreservesRecipientsAndWSShape(t *testing.T) {
	for index, subject := range ordinaryRoleSubjects {
		t.Run(subject, func(t *testing.T) {
			event, payload := ordinaryRoleFixture(t, subject)
			require.EqualValues(t, 13+index, event.ProtoReflect().WhichOneof(event.ProtoReflect().Descriptor().Oneofs().ByName("payload")).Number())
			binary, err := proto.Marshal(event)
			require.NoError(t, err)
			legacy, err := json.Marshal(map[string]any{"space_id": payload.SpaceId, "role_id": payload.RoleId, "profile_id": payload.ProfileId, "name": payload.Name, "chat_id": payload.ChatId, "voice_room_id": payload.VoiceRoomId, "changed_fields": payload.ChangedFields, "future_property": true})
			require.NoError(t, err)
			future := protowire.AppendTag(append([]byte(nil), binary...), 99, protowire.VarintType)
			future = protowire.AppendVarint(future, 1)
			for encoding, data := range map[string][]byte{"legacy": legacy, "binary": binary, "unknown field": future} {
				t.Run(encoding, func(t *testing.T) {
					profileID, chatID, envelope, ok := roleEventToFanout(subject, data)
					require.True(t, ok)
					wantProfile, wantChat := "", ""
					switch subject {
					case "role.assigned", "role.revoked":
						wantProfile = payload.ProfileId
					case "role.chat_override_set", "role.chat_override_removed":
						wantChat = payload.ChatId
					}
					require.Equal(t, wantProfile, profileID)
					require.Equal(t, wantChat, chatID)
					require.Equal(t, "role_update", envelope.Op)
					var body map[string]string
					require.NoError(t, json.Unmarshal(envelope.D, &body))
					require.Equal(t, map[string]string{"space_id": payload.SpaceId, "role_id": payload.RoleId, "profile_id": payload.ProfileId, "name": payload.Name, "chat_id": payload.ChatId, "voice_room_id": payload.VoiceRoomId, "subject": subject}, body)
				})
			}
		})
	}
}

func TestOrdinaryRoleWireRejectsSubjectArmMismatch(t *testing.T) {
	for _, actual := range ordinaryRoleSubjects {
		event, _ := ordinaryRoleFixture(t, actual)
		data, err := proto.Marshal(event)
		require.NoError(t, err)
		for _, subject := range append(append([]string{}, ordinaryRoleSubjects...), "role.voice_policy_invalidated", "role.future") {
			if subject == actual {
				continue
			}
			t.Run(actual+"_on_"+subject, func(t *testing.T) {
				_, _, _, ok := roleEventToFanout(subject, data)
				require.False(t, ok)
			})
		}
	}
}

func TestOrdinaryRoleWireRejectsInvalidMetadataAndJSONEnvelope(t *testing.T) {
	event, payload := ordinaryRoleFixture(t, "role.assigned")
	invalid := map[string][]byte{"null": []byte("null"), "array": []byte("[]"), "empty object": []byte("{}"), "envelope masquerading as flat": []byte(`{"eventId":"id","space_id":"space-1","profile_id":"profile-1"}`), "oneof masquerading as flat": []byte(`{"role_assigned":{},"space_id":"space-1","profile_id":"profile-1"}`), "truncated": {0x0a, 0xff}}
	for name, mutate := range map[string]func(*eventsv1.RoleStreamEvent){"missing id": func(e *eventsv1.RoleStreamEvent) { e.EventId = "" }, "invalid id": func(e *eventsv1.RoleStreamEvent) { e.EventId = "not-uuid" }, "missing time": func(e *eventsv1.RoleStreamEvent) { e.OccurredAt = nil }, "invalid time": func(e *eventsv1.RoleStreamEvent) { e.OccurredAt.Nanos = 1000000000 }, "missing payload": func(e *eventsv1.RoleStreamEvent) { e.Payload = nil }, "missing profile": func(e *eventsv1.RoleStreamEvent) { e.GetRoleAssigned().ProfileId = "" }, "generic arm": func(e *eventsv1.RoleStreamEvent) {
		e.Payload = &eventsv1.RoleStreamEvent_RoleAssignmentChanged{RoleAssignmentChanged: &eventsv1.RoleAssignmentChanged{SpaceId: payload.SpaceId, ProfileId: payload.ProfileId, RoleIds: []string{payload.RoleId}}}
	}, "policy arm": func(e *eventsv1.RoleStreamEvent) {
		e.Payload = &eventsv1.RoleStreamEvent_VoiceRoomPolicyInvalidated{VoiceRoomPolicyInvalidated: &eventsv1.VoiceRoomPolicyInvalidated{SpaceId: payload.SpaceId, PolicyEpoch: 1}}
	}} {
		copy := proto.Clone(event).(*eventsv1.RoleStreamEvent)
		mutate(copy)
		data, err := proto.Marshal(copy)
		require.NoError(t, err)
		invalid[name] = data
	}
	for name, data := range invalid {
		t.Run(name, func(t *testing.T) { _, _, _, ok := roleEventToFanout("role.assigned", data); require.False(t, ok) })
	}
}
