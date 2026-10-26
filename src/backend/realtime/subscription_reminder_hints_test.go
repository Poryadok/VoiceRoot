package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	voicejwt "voice/backend/pkg/jwt"
	"voice/backend/pkg/reminderhint"
)

const reminderTarget = "22222222-2222-4222-8222-222222222222"

func reminderFixture() reminderhint.Hint {
	h := reminderhint.Hint{Version: 1, TargetProfileID: reminderTarget, AggregateKind: "PERSONAL", AggregateID: "11111111-1111-4111-8111-111111111111", GraceRevision: "7", Day: 3, Plan: "premium", WindowEnd: time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)}
	h.NotificationID = reminderhint.LogicalID(h.AggregateKind, h.AggregateID, h.GraceRevision, h.Day)
	return h
}

func TestReminderHintRealNATSWebSocketBoundary(t *testing.T) {
	server := startRealtimeJSTestServer(t)
	nc, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: reminderhint.Stream, Subjects: []string{reminderhint.Subject}})
	require.NoError(t, err)
	const instance = "reminder-test"
	_, err = js.AddConsumer(reminderhint.Stream, &nats.ConsumerConfig{Durable: reminderHintDurable(instance), FilterSubject: reminderhint.Subject, DeliverSubject: realtimeConsumerDeliverSubject(instance, "subscription_reminder"), AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverNewPolicy})
	require.NoError(t, err)
	hub := newWSHub()
	sub, err := subscribeReminderHints(js, hub, instance)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })
	tokens := staticTokenValidator{"target": voicejwt.Claims{UserID: "payer", ProfileID: reminderTarget}, "other": voicejwt.Claims{UserID: "payer", ProfileID: "33333333-3333-4333-8333-333333333333"}}
	srv := httptest.NewServer(newServiceHandler(serviceName, tokens, nil, hub, nil, instance, readinessDeps{}))
	t.Cleanup(srv.Close)
	dial := func(token, profile string) *websocket.Conn {
		headers := wsUpgradeHeaders(token)
		headers.Set("X-Profile-Id", profile)
		conn, _, e := websocket.DefaultDialer.Dial(wsEndpoint(t, srv), headers)
		require.NoError(t, e)
		t.Cleanup(func() { _ = conn.Close() })
		require.Equal(t, "hello", readReminderWS(t, conn).Op)
		return conn
	}
	target := dial("target", reminderTarget)
	other := dial("other", tokens["other"].ProfileID)
	hint := reminderFixture()
	raw, err := json.Marshal(hint)
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		ack, e := js.Publish(reminderhint.Subject, raw)
		require.NoError(t, e)
		require.NotZero(t, ack.Sequence)
		frame := readReminderWS(t, target)
		require.Equal(t, "notification", frame.Op)
		require.Contains(t, string(frame.D), hint.NotificationID)
		require.NotContains(t, string(frame.D), hint.AggregateID)
	}
	// A later same-connection heartbeat is a barrier after consumed hints. A
	// different profile on the payer account must not receive either reminder.
	require.Eventually(t, func() bool {
		info, e := js.ConsumerInfo(reminderhint.Stream, reminderHintDurable(instance))
		return e == nil && info.AckFloor.Stream == 2
	}, time.Second, 5*time.Millisecond)
	require.NoError(t, other.WriteJSON(map[string]any{"op": "heartbeat"}))
	require.Equal(t, "heartbeat_ack", readReminderWS(t, other).Op)
	hint.WindowEnd = time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano)
	raw, _ = json.Marshal(hint)
	_, err = js.Publish(reminderhint.Subject, raw)
	require.NoError(t, err)
	_, err = js.Publish(reminderhint.Subject, []byte(`{"schema_version":1,"target_profile_id":"bad"}`))
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		info, e := js.ConsumerInfo(reminderhint.Stream, reminderHintDurable(instance))
		return e == nil && info.AckFloor.Stream == 4
	}, time.Second, 5*time.Millisecond)
	require.NoError(t, target.WriteJSON(map[string]any{"op": "heartbeat"}))
	require.Equal(t, "heartbeat_ack", readReminderWS(t, target).Op)
	// Offline consumption is finite and does not create a replay on reconnect.
	require.NoError(t, target.Close())
	hint = reminderFixture()
	raw, _ = json.Marshal(hint)
	_, err = js.Publish(reminderhint.Subject, raw)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		info, e := js.ConsumerInfo(reminderhint.Stream, reminderHintDurable(instance))
		return e == nil && info.AckFloor.Stream == 5
	}, time.Second, 5*time.Millisecond)
	again := dial("target", reminderTarget)
	require.NoError(t, again.WriteJSON(map[string]any{"op": "heartbeat"}))
	require.Equal(t, "heartbeat_ack", readReminderWS(t, again).Op)
}

func readReminderWS(t *testing.T, c *websocket.Conn) wsEnvelope {
	t.Helper()
	require.NoError(t, c.SetReadDeadline(time.Now().Add(3*time.Second)))
	var frame wsEnvelope
	require.NoError(t, c.ReadJSON(&frame))
	return frame
}

func TestReminderHintRevokedSessionDoesNotReachWebSocket(t *testing.T) {
	now := time.Now().UTC()
	claims := epochEnforcementClaims(now)
	claims.ProfileID = reminderTarget
	floor := &epochEnforcementFloor{byAccount: map[string]epochEnforcementFloorResult{claims.UserID: {minimum: 7}}}
	hub := newWSHub()
	srv := httptest.NewServer(newEpochEnforcementHandler(&epochEnforcementValidator{claims: claims}, floor, func() time.Time { return now }, true, hub, nil, nil))
	t.Cleanup(srv.Close)
	conn := epochEnforcementDial(t, srv, epochEnforcementHeaders("verified-token", reminderTarget))
	t.Cleanup(func() { _ = conn.Close() })
	require.Equal(t, "hello", epochEnforcementRead(t, conn).Op)
	floor.set(claims.UserID, epochEnforcementFloorResult{minimum: 8})
	hint := reminderFixture()
	raw, _ := json.Marshal(hint)
	require.True(t, dispatchReminderHint(hub, reminderhint.Subject, raw, now))
	epochEnforcementRequireRevokedClose(t, conn)
	require.False(t, dispatchReminderHint(hub, reminderhint.Subject+".forged", raw, now))
}

func TestReminderHintPublishRetryIsOnlyBrokerAcceptance(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: reminderhint.Stream, Subjects: []string{reminderhint.Subject}, Storage: nats.FileStorage, MaxAge: 7 * 24 * time.Hour})
	require.NoError(t, err)
	hint := reminderFixture()
	first, err := reminderhint.Publish(context.Background(), js, hint, time.Now())
	require.NoError(t, err)
	retry, err := reminderhint.Publish(context.Background(), js, hint, time.Now())
	require.NoError(t, err)
	require.Equal(t, first.Sequence, retry.Sequence)
	require.True(t, retry.Duplicate)
	// No consumer or socket exists. Positive PubAck proves nothing about client
	// receipt and cannot be a Notification insertion-completion receipt.
	info, err := js.StreamInfo(reminderhint.Stream)
	require.NoError(t, err)
	require.EqualValues(t, 1, info.State.Msgs)
	hint.WindowEnd = time.Now().UTC().Format(time.RFC3339Nano)
	_, err = reminderhint.Publish(context.Background(), js, hint, time.Now())
	require.ErrorIs(t, err, reminderhint.ErrExpired)
}

func TestReminderHintCanonicalNATSPermissions(t *testing.T) {
	// Apply the real manifest's service grants to an isolated broker. Fixture
	// passwords authenticate only this local test; no runtime credentials issue.
	var acl struct {
		Services map[string]struct {
			Publish   []string `yaml:"publish"`
			Subscribe []string `yaml:"subscribe"`
		} `yaml:"services"`
	}
	raw, err := os.ReadFile("../../../deploy/nats/acl-intent.yaml")
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(raw, &acl))
	users := []*natsserver.User{{Username: "fixture-admin", Password: "fixture-only"}}
	for _, name := range []string{"notification", "realtime", "voice"} {
		g := acl.Services[name]
		users = append(users, &natsserver.User{Username: name, Password: "fixture-only", Permissions: &natsserver.Permissions{Publish: &natsserver.SubjectPermission{Allow: g.Publish}, Subscribe: &natsserver.SubjectPermission{Allow: g.Subscribe}}})
	}
	s, err := natsserver.NewServer(&natsserver.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir(), Users: users})
	require.NoError(t, err)
	go s.Start()
	require.True(t, s.ReadyForConnections(time.Second))
	t.Cleanup(s.Shutdown)
	connect := func(name string, denied chan error) *nats.Conn {
		nc, e := nats.Connect(s.ClientURL(), nats.UserInfo(name, "fixture-only"), nats.CustomInboxPrefix("_INBOX.voice."+name), nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, e error) {
			if denied != nil {
				denied <- e
			}
		}))
		require.NoError(t, e)
		t.Cleanup(nc.Close)
		return nc
	}
	admin := connect("fixture-admin", nil)
	js, err := admin.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: reminderhint.Stream, Subjects: []string{reminderhint.Subject}, Storage: nats.FileStorage, MaxAge: 7 * 24 * time.Hour})
	require.NoError(t, err)
	_, err = js.AddConsumer(reminderhint.Stream, &nats.ConsumerConfig{Durable: reminderHintDurable("realtime-1"), FilterSubject: reminderhint.Subject, DeliverSubject: realtimeConsumerDeliverSubject("realtime-1", "subscription_reminder"), AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverNewPolicy})
	require.NoError(t, err)
	consumer := connect("realtime", nil)
	cjs, err := consumer.JetStream()
	require.NoError(t, err)
	hub := newWSHub()
	target := hub.attachAccountConn("realtime-1", "acl-target", "payer", reminderTarget, 4)
	t.Cleanup(func() { hub.unregisterConn(target) })
	sub, err := subscribeReminderHints(cjs, hub, "realtime-1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })
	pubDenied := make(chan error, 8)
	publisher := connect("notification", pubDenied)
	pjs, err := publisher.JetStream()
	require.NoError(t, err)
	_, err = reminderhint.Publish(context.Background(), pjs, reminderFixture(), time.Now())
	require.NoError(t, err)
	select {
	case frame := <-target.fanout:
		require.Equal(t, "notification", frame.Op)
	case <-time.After(time.Second):
		t.Fatal("canonical ACL blocked reminder")
	}
	denied := make(chan error, 8)
	wrong := connect("voice", denied)
	require.NoError(t, wrong.Publish(reminderhint.Subject, []byte("forged")))
	require.NoError(t, wrong.FlushTimeout(time.Second))
	select {
	case err := <-denied:
		require.ErrorContains(t, err, "Permissions Violation")
	case <-time.After(time.Second):
		t.Fatal("wrong publisher was not denied")
	}
	require.NoError(t, publisher.Publish(reminderhint.Subject+".forged", []byte("adjacent")))
	require.NoError(t, publisher.FlushTimeout(time.Second))
	select {
	case err := <-pubDenied:
		require.ErrorContains(t, err, "Permissions Violation")
	case <-time.After(time.Second):
		t.Fatal("adjacent publisher was not denied")
	}
	// The actual broker also refuses direct domain subscriptions for Realtime.
	rtDenied := make(chan error, 8)
	rt := connect("realtime", rtDenied)
	_, err = rt.Subscribe(reminderhint.Subject, func(*nats.Msg) {})
	require.NoError(t, err)
	require.NoError(t, rt.FlushTimeout(time.Second))
	select {
	case err := <-rtDenied:
		require.ErrorContains(t, err, "Permissions Violation")
	case <-time.After(time.Second):
		t.Fatal("direct domain subscription was not denied")
	}
}

func TestReminderHintRejectsConsumerBindingDrift(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: reminderhint.Stream, Subjects: []string{reminderhint.Subject}})
	require.NoError(t, err)
	_, err = subscribeReminderHints(js, newWSHub(), "binding")
	require.Error(t, err)
	_, err = js.ConsumerInfo(reminderhint.Stream, reminderHintDurable("binding"))
	require.ErrorIs(t, err, nats.ErrConsumerNotFound, "runtime must not provision its consumer")
	_, err = js.AddConsumer(reminderhint.Stream, &nats.ConsumerConfig{Durable: reminderHintDurable("binding"), FilterSubject: reminderhint.Subject, DeliverSubject: realtimeConsumerDeliverSubject("binding", "subscription_reminder"), AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverAllPolicy})
	require.NoError(t, err)
	_, err = subscribeReminderHints(js, newWSHub(), "binding")
	require.ErrorContains(t, err, "incompatible configuration")
}
