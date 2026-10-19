package consumer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/analytics/internal/adapters"
)

// This uses the real broker's source-defined publish/subscribe permissions. The
// separate hosted canonical proof owns JWT/TLS/bootstrap-script execution.
func TestParticipantFactsRealJetStreamACLAndAnalyticsNoop(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..")
	type grant struct {
		Publish   []string `yaml:"publish"`
		Subscribe []string `yaml:"subscribe"`
	}
	var policy struct {
		Services  map[string]grant `yaml:"services"`
		Bootstrap grant            `yaml:"bootstrap"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "deploy", "nats", "acl-intent.yaml"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(raw, &policy))
	var manifest struct {
		Streams []struct {
			Name     string   `yaml:"name"`
			Subjects []string `yaml:"subjects"`
			MaxAge   string   `yaml:"max_age"`
		} `yaml:"streams"`
	}
	raw, err = os.ReadFile(filepath.Join(root, "deploy", "nats", "jetstream-publisher-streams.yaml"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(raw, &manifest))
	var subjects []string
	var maxAge time.Duration
	for _, stream := range manifest.Streams {
		if stream.Name == "voice_events" {
			subjects = stream.Subjects
			maxAge, err = time.ParseDuration(stream.MaxAge)
			require.NoError(t, err)
		}
	}
	require.Contains(t, subjects, "voice.participant_joined")
	require.Contains(t, subjects, "voice.participant_left")
	var legacy []string
	for _, subject := range subjects {
		if subject != "voice.participant_joined" && subject != "voice.participant_left" {
			legacy = append(legacy, subject)
		}
	}
	users := []*server.User{}
	for _, actor := range []string{"voice", "analytics", "bootstrap"} {
		g := policy.Services[actor]
		if actor == "bootstrap" {
			g = policy.Bootstrap
		}
		users = append(users, &server.User{Username: actor, Password: "disposable-fixture", Permissions: &server.Permissions{Publish: &server.SubjectPermission{Allow: g.Publish}, Subscribe: &server.SubjectPermission{Allow: g.Subscribe}}})
	}
	broker, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir(), Users: users})
	require.NoError(t, err)
	go broker.Start()
	require.True(t, broker.ReadyForConnections(5*time.Second))
	t.Cleanup(broker.Shutdown)
	connect := func(actor string) (*nats.Conn, nats.JetStreamContext) {
		nc, e := nats.Connect(broker.ClientURL(), nats.UserInfo(actor, "disposable-fixture"), nats.CustomInboxPrefix("_INBOX.voice."+actor+".reply"), nats.ErrorHandler(func(*nats.Conn, *nats.Subscription, error) {}))
		require.NoError(t, e)
		t.Cleanup(nc.Close)
		js, e := nc.JetStream()
		require.NoError(t, e)
		return nc, js
	}
	bootstrapConnection, bootstrap := connect("bootstrap")
	_, voice := connect("voice")
	_, err = bootstrap.AddStream(&nats.StreamConfig{Name: "voice_events", Subjects: legacy, Retention: nats.LimitsPolicy, Storage: nats.FileStorage, MaxAge: maxAge, MaxMsgs: 731, Description: "retained-extra-config"})
	require.NoError(t, err)
	const durable = "analytics_v2_voice"
	// Match the bootstrap's exact unfiltered CREATE API, rather than a client
	// optimization that appends the filter to the API subject and needs new ACLs.
	create, err := json.Marshal(map[string]any{"stream_name": "voice_events", "action": "create", "config": &nats.ConsumerConfig{Name: durable, Durable: durable, DeliverSubject: "_INBOX.voice.analytics." + durable, DeliverGroup: durable, FilterSubject: "voice.>", DeliverPolicy: nats.DeliverNewPolicy, AckPolicy: nats.AckExplicitPolicy}})
	require.NoError(t, err)
	reply, err := bootstrapConnection.Request("$JS.API.CONSUMER.CREATE.voice_events."+durable, create, 2*time.Second)
	require.NoError(t, err)
	var response struct {
		Error *struct {
			Description string `json:"description"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(reply.Data, &response))
	require.Nil(t, response.Error)
	legacyEnvelope := &eventsv1.VoiceStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.Now(), Payload: &eventsv1.VoiceStreamEvent_CallStarted{CallStarted: &eventsv1.CallStarted{RoomId: uuid.NewString()}}}
	payload, err := proto.Marshal(legacyEnvelope)
	require.NoError(t, err)
	_, err = voice.Publish("voice.call_started", payload)
	require.NoError(t, err)
	before, err := bootstrap.StreamInfo("voice_events")
	require.NoError(t, err)
	consumerBefore, err := bootstrap.ConsumerInfo("voice_events", durable)
	require.NoError(t, err)
	updated := before.Config
	updated.Subjects = append([]string(nil), subjects...)
	_, err = bootstrap.UpdateStream(&updated)
	require.NoError(t, err)
	after, err := bootstrap.StreamInfo("voice_events")
	require.NoError(t, err)
	consumerAfter, err := bootstrap.ConsumerInfo("voice_events", durable)
	require.NoError(t, err)
	require.Equal(t, before.State, after.State)
	require.Equal(t, consumerBefore.Created, consumerAfter.Created)
	require.Equal(t, consumerBefore.Config, consumerAfter.Config)
	require.Equal(t, consumerBefore.AckFloor, consumerAfter.AckFloor)
	require.Equal(t, consumerBefore.NumPending, consumerAfter.NumPending)
	actualConfig := after.Config
	actualConfig.Subjects = before.Config.Subjects
	require.Equal(t, before.Config, actualConfig, "only subjects may change")
	nc, analytics := connect("analytics")
	sub, err := analytics.QueueSubscribeSync("voice.>", durable, nats.Bind("voice_events", durable), nats.ManualAck())
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })
	require.NoError(t, nc.Flush())
	old, err := sub.NextMsg(5 * time.Second)
	require.NoError(t, err)
	require.Equal(t, "voice.call_started", old.Subject)
	require.NoError(t, old.AckSync())
	runner := &Runner{Mapper: adapters.Mapper{HashKey: "participant-compatibility"}} // nil buffer deliberately proves no append
	require.NotNil(t, runner.Mapper.FromVoice(legacyEnvelope), "legacy call_started analytics remains mapped")
	for _, subject := range []string{"voice.participant_joined", "voice.participant_left"} {
		envelope := &eventsv1.VoiceStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.Now()}
		if strings.HasSuffix(subject, "joined") {
			envelope.Payload = &eventsv1.VoiceStreamEvent_VoiceParticipantJoined{VoiceParticipantJoined: &eventsv1.VoiceParticipantJoined{RoomId: uuid.NewString(), ProfileId: uuid.NewString()}}
		} else {
			envelope.Payload = &eventsv1.VoiceStreamEvent_VoiceParticipantLeft{VoiceParticipantLeft: &eventsv1.VoiceParticipantLeft{RoomId: uuid.NewString(), ProfileId: uuid.NewString()}}
		}
		payload, err = proto.Marshal(envelope)
		require.NoError(t, err)
		ack, e := voice.Publish(subject, payload)
		require.NoError(t, e)
		require.Equal(t, "voice_events", ack.Stream)
		message, e := sub.NextMsg(5 * time.Second)
		require.NoError(t, e)
		require.Equal(t, subject, message.Subject)
		require.Nil(t, runner.Mapper.FromVoice(envelope))
		require.NoError(t, runner.handleVoiceProto(message.Data, message))
	}
	require.NoError(t, nc.Flush())
	require.Eventually(t, func() bool {
		info, e := bootstrap.ConsumerInfo("voice_events", durable)
		return e == nil && info.NumAckPending == 0 && info.NumPending == 0 && info.AckFloor.Stream == 3
	}, 5*time.Second, 10*time.Millisecond, "unsupported facts must ACK without buffer persistence")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = voice.Publish("voice.participant_changed", []byte("unreviewed"), nats.Context(ctx))
	require.Error(t, err, "exact actor grant cannot publish a neighbouring subject")
	final, err := bootstrap.StreamInfo("voice_events")
	require.NoError(t, err)
	require.EqualValues(t, 3, final.State.Msgs)
	require.EqualValues(t, 3, final.State.LastSeq)
}
