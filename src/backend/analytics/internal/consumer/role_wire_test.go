package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/analytics/internal/adapters"
	"voice/backend/analytics/internal/buffer"
	"voice/backend/analytics/internal/store"
	"voice/backend/pkg/analyticshash"
)

var roleWireSubjects = []string{"role.created", "role.updated", "role.deleted", "role.assigned", "role.revoked", "role.chat_override_set", "role.chat_override_removed", "role.voice_override_set", "role.voice_override_removed"}

const roleWireProfile = "30000000-0000-4000-8000-000000000003"

func roleWireSource(t *testing.T) (nats.JetStreamContext, *nats.Subscription) {
	t.Helper()
	server := startAnalyticsJetStream(t)
	connection, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(connection.Close)
	js, err := connection.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "role_events", Subjects: []string{"role.>"}})
	require.NoError(t, err)
	const durable = "role_wire"
	provisionAnalyticsDurable(t, js, "role_events", "role.>", durable, durable)
	subscription, err := js.QueueSubscribeSync("role.>", durable, nats.Bind("role_events", durable), nats.ManualAck())
	require.NoError(t, err)
	t.Cleanup(func() { _ = subscription.Unsubscribe() })
	return js, subscription
}

func TestRoleWireRetainedBacklogRebindAndPersistenceAck(t *testing.T) {
	server := startAnalyticsJetStream(t)
	connection, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(connection.Close)
	js, err := connection.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "role_events", Subjects: []string{"role.>"}, Storage: nats.FileStorage})
	require.NoError(t, err)
	const durable = "role_backlog"
	_, err = js.AddConsumer("role_events", &nats.ConsumerConfig{Durable: durable, DeliverSubject: "_INBOX.voice.analytics." + durable, DeliverGroup: durable, FilterSubject: "role.>", DeliverPolicy: nats.DeliverNewPolicy, AckPolicy: nats.AckExplicitPolicy, AckWait: 300 * time.Millisecond})
	require.NoError(t, err)
	legacy := []byte(`{"space_id":"10000000-0000-4000-8000-000000000001","role_id":"20000000-0000-4000-8000-000000000002","profile_id":"30000000-0000-4000-8000-000000000003"}`)
	typed, binary := roleWireFixture(t, "role.assigned")
	_, err = js.Publish("role.assigned", legacy)
	require.NoError(t, err)
	_, err = js.Publish("role.assigned", binary)
	require.NoError(t, err)
	policy, err := proto.Marshal(&eventsv1.RoleStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.Now(), Payload: &eventsv1.RoleStreamEvent_VoiceRoomPolicyInvalidated{VoiceRoomPolicyInvalidated: &eventsv1.VoiceRoomPolicyInvalidated{SpaceId: "10000000-0000-4000-8000-000000000001", PolicyEpoch: 7}}})
	require.NoError(t, err)
	_, err = js.Publish("role.voice_policy_invalidated", policy)
	require.NoError(t, err)
	info, err := js.StreamInfo("role_events")
	require.NoError(t, err)
	require.EqualValues(t, 3, info.State.Msgs, "mixed bytes are retained before reader binding")
	firstReader, err := js.QueueSubscribeSync("role.>", durable, nats.Bind("role_events", durable), nats.ManualAck())
	require.NoError(t, err)
	first, err := firstReader.NextMsg(3 * time.Second)
	require.NoError(t, err)
	firstMetadata, err := first.Metadata()
	require.NoError(t, err)
	require.EqualValues(t, 1, firstMetadata.Sequence.Stream)
	var persisted []store.EventRow
	failing := buffer.New(1, time.Hour, func(context.Context, []store.EventRow) error { return errors.New("persistence unavailable") }, nil)
	require.NoError(t, (&Runner{Mapper: adapters.Mapper{HashKey: "role-wire-test"}, Buffer: failing}).handleRoleMsg(first))
	require.NoError(t, connection.Flush())
	consumer, err := js.ConsumerInfo("role_events", durable)
	require.NoError(t, err)
	require.Zero(t, consumer.AckFloor.Stream, "failed persistence cannot ACK the retained record")
	require.NoError(t, firstReader.Unsubscribe())
	connection.Close()

	restartedConnection, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(restartedConnection.Close)
	restartedJS, err := restartedConnection.JetStream()
	require.NoError(t, err)
	restartedReader, err := restartedJS.QueueSubscribeSync("role.>", durable, nats.Bind("role_events", durable), nats.ManualAck())
	require.NoError(t, err)
	t.Cleanup(func() { _ = restartedReader.Unsubscribe() })
	accumulator := buffer.New(1, time.Hour, func(_ context.Context, rows []store.EventRow) error {
		persisted = append(persisted, rows...)
		return nil
	}, nil)
	runner := &Runner{Mapper: adapters.Mapper{HashKey: "role-wire-test"}, Buffer: accumulator}
	seen := map[uint64]bool{}
	redelivered := false
	for attempt := 0; len(seen) < 3 && attempt < 8; attempt++ {
		message, nextErr := restartedReader.NextMsg(3 * time.Second)
		require.NoError(t, nextErr)
		metadata, metadataErr := message.Metadata()
		require.NoError(t, metadataErr)
		redelivered = redelivered || metadata.NumDelivered > 1
		require.NoError(t, runner.handleRoleMsg(message))
		seen[metadata.Sequence.Stream] = true
	}
	require.Len(t, seen, 3)
	require.True(t, redelivered)
	require.Eventually(t, func() bool {
		state, stateErr := restartedJS.ConsumerInfo("role_events", durable)
		return stateErr == nil && state.AckFloor.Stream == 3 && state.NumAckPending == 0
	}, 3*time.Second, 10*time.Millisecond)
	require.Len(t, persisted, 2, "policy notice produces no ordinary analytics row")
	legacyID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("voice.analytics.jetstream.v1\x00role_events\x001")).String()
	rows := map[string]store.EventRow{}
	for _, row := range persisted {
		rows[row.EventID] = row
	}
	require.Contains(t, rows, legacyID)
	require.Contains(t, rows, typed.EventId)
	require.Equal(t, typed.OccurredAt.AsTime(), rows[typed.EventId].Timestamp)
	for _, row := range rows {
		require.Equal(t, analyticshash.ID("role-wire-test", roleWireProfile), row.ProfileIDHashed)
		require.JSONEq(t, `{"space_id":"10000000-0000-4000-8000-000000000001","role_id":"20000000-0000-4000-8000-000000000002"}`, row.PropertiesJSON)
	}
	// A writer may roll back to JSON while these dual readers remain running.
	_, err = restartedJS.Publish("role.assigned", legacy)
	require.NoError(t, err)
	rollback, err := restartedReader.NextMsg(3 * time.Second)
	require.NoError(t, err)
	require.NoError(t, runner.handleRoleMsg(rollback))
	require.Len(t, persisted, 3)
	require.NotEqual(t, legacyID, persisted[2].EventID)
	require.Eventually(t, func() bool {
		state, stateErr := restartedJS.ConsumerInfo("role_events", durable)
		return stateErr == nil && state.AckFloor.Stream == 4 && state.NumAckPending == 0
	}, 3*time.Second, 10*time.Millisecond)
}

func roleWireFixture(t *testing.T, subject string) (*eventsv1.RoleStreamEvent, []byte) {
	t.Helper()
	event := &eventsv1.RoleStreamEvent{EventId: uuid.NewSHA1(uuid.NameSpaceOID, []byte(subject)).String(), OccurredAt: timestamppb.New(time.Unix(1700000000, 0))}
	mutation := &eventsv1.RoleMutation{SpaceId: "10000000-0000-4000-8000-000000000001", RoleId: "20000000-0000-4000-8000-000000000002", ProfileId: roleWireProfile, Name: "private name", ChatId: "40000000-0000-4000-8000-000000000004", VoiceRoomId: "50000000-0000-4000-8000-000000000005", ChangedFields: []string{"name"}}
	field := event.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(strings.ReplaceAll(subject, ".", "_")))
	require.NotNil(t, field)
	event.ProtoReflect().Set(field, protoreflect.ValueOfMessage(mutation.ProtoReflect()))
	data, err := proto.Marshal(event)
	require.NoError(t, err)
	return event, data
}

func TestRoleWireRunnerPreservesTypedAndLegacyIdentity(t *testing.T) {
	js, subscription := roleWireSource(t)
	for _, subject := range roleWireSubjects {
		t.Run(subject, func(t *testing.T) {
			event, binary := roleWireFixture(t, subject)
			legacy, err := json.Marshal(map[string]string{"space_id": "10000000-0000-4000-8000-000000000001", "role_id": "20000000-0000-4000-8000-000000000002", "profile_id": roleWireProfile, "name": "private name", "chat_id": "40000000-0000-4000-8000-000000000004", "voice_room_id": "50000000-0000-4000-8000-000000000005"})
			require.NoError(t, err)
			for encoding, data := range map[string][]byte{"binary": binary, "legacy": legacy} {
				t.Run(encoding, func(t *testing.T) {
					var rows []store.EventRow
					accumulator := buffer.New(1, time.Hour, func(_ context.Context, batch []store.EventRow) error { rows = append(rows, batch...); return nil }, nil)
					runner := &Runner{Mapper: adapters.Mapper{HashKey: "role-wire-test"}, Buffer: accumulator}
					_, err := js.Publish(subject, data)
					require.NoError(t, err)
					message, err := subscription.NextMsg(3 * time.Second)
					require.NoError(t, err)
					metadata, err := message.Metadata()
					require.NoError(t, err)
					require.NoError(t, runner.handleRoleMsg(message))
					require.NoError(t, runner.handleRoleMsg(message))
					require.Len(t, rows, 2)
					wantID := event.EventId
					if encoding == "legacy" {
						wantID = uuid.NewSHA1(uuid.NameSpaceURL, []byte("voice.analytics.jetstream.v1\x00role_events\x00"+strconv.FormatUint(metadata.Sequence.Stream, 10))).String()
					}
					for _, row := range rows {
						require.Equal(t, wantID, row.EventID)
						require.Equal(t, "role_"+strings.TrimPrefix(subject, "role."), row.EventType)
						require.Equal(t, "role", row.SourceService)
						require.Equal(t, analyticshash.ID("role-wire-test", roleWireProfile), row.ProfileIDHashed)
						require.JSONEq(t, `{"space_id":"10000000-0000-4000-8000-000000000001","role_id":"20000000-0000-4000-8000-000000000002"}`, row.PropertiesJSON)
						if encoding == "binary" {
							require.Equal(t, event.OccurredAt.AsTime(), row.Timestamp)
						}
					}
				})
			}
		})
	}
}

func TestRoleWireRunnerRejectsNonOrdinaryJSON(t *testing.T) {
	js, subscription := roleWireSource(t)
	for _, data := range []string{"null", "[]", "{}", `{"eventId":"id","space_id":"space-1"}`, `{"role_assigned":{},"space_id":"space-1"}`} {
		t.Run(data, func(t *testing.T) {
			var rows []store.EventRow
			accumulator := buffer.New(1, time.Hour, func(_ context.Context, batch []store.EventRow) error { rows = append(rows, batch...); return nil }, nil)
			runner := &Runner{Mapper: adapters.Mapper{}, Buffer: accumulator}
			_, err := js.Publish("role.assigned", []byte(data))
			require.NoError(t, err)
			message, err := subscription.NextMsg(3 * time.Second)
			require.NoError(t, err)
			require.NoError(t, runner.handleRoleMsg(message))
			require.Empty(t, rows)
		})
	}
}
