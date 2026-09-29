package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func startLiveACLProofServer(t *testing.T, allowAdjacent, allowCreate, allowAck bool) (*nats.Conn, *nats.Conn, *nats.Conn, *nats.Conn) {
	t.Helper()
	rtPub := []string{"$JS.API.CONSUMER.INFO.social_events.rt_realtime1_friend_request"}
	if allowAck {
		rtPub = append(rtPub, "$JS.ACK.social_events.rt_realtime1_friend_request.>")
	}
	if allowCreate {
		rtPub = append(rtPub, "$JS.API.CONSUMER.CREATE.social_events.rt_realtime1_friend_request")
	}
	rtSub := []string{"_INBOX.voice.realtime.>", "_INBOX.voice.realtime1.friend_request"}
	if allowAdjacent {
		rtSub = append(rtSub, "_INBOX.voice.realtime1.friend_request_adjacent")
	}
	leafPub := append(append([]string{}, rtPub...), "$JS.ACK.social_events.rt_realtime1_friend_request.>")
	opts := &server.Options{
		Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir(),
		Users: []*server.User{
			{Username: "admin", Password: "admin"},
			{Username: "realtime", Password: "realtime", Permissions: &server.Permissions{
				Publish: &server.SubjectPermission{Allow: rtPub}, Subscribe: &server.SubjectPermission{Allow: rtSub},
			}},
			{Username: "realtime-leaf", Password: "realtime-leaf", Permissions: &server.Permissions{
				Publish: &server.SubjectPermission{Allow: leafPub}, Subscribe: &server.SubjectPermission{Allow: rtSub},
			}},
			{Username: "proof", Password: "proof", Permissions: &server.Permissions{
				Publish:   &server.SubjectPermission{Allow: []string{"social.friend_request", "$JS.API.STREAM.INFO.social_events", "$JS.API.STREAM.MSG.GET.social_events", "$JS.API.STREAM.MSG.DELETE.social_events"}},
				Subscribe: &server.SubjectPermission{Allow: []string{"_INBOX.voice.nats-proof.>"}},
			}},
		},
	}
	s, err := server.NewServer(opts)
	if err != nil {
		t.Fatal(err)
	}
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}
	t.Cleanup(s.Shutdown)
	connect := func(name, password string, extra ...nats.Option) *nats.Conn {
		t.Helper()
		options := append([]nats.Option{nats.UserInfo(name, password), nats.Timeout(time.Second)}, extra...)
		nc, err := nats.Connect(s.ClientURL(), options...)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(nc.Close)
		return nc
	}
	admin := connect("admin", "admin")
	rt := connect("realtime-leaf", "realtime-leaf", nats.CustomInboxPrefix("_INBOX.voice.realtime"), nats.PermissionErrOnSubscribe(true))
	hubRT := connect("realtime", "realtime", nats.CustomInboxPrefix("_INBOX.voice.realtime"), nats.PermissionErrOnSubscribe(true))
	proof := connect("proof", "proof", nats.CustomInboxPrefix("_INBOX.voice.nats-proof"))
	js, err := admin.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.AddStream(&nats.StreamConfig{Name: jsStreamSocialEvents, Subjects: []string{"social.>"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := js.AddConsumer(jsStreamSocialEvents, &nats.ConsumerConfig{
		Durable:        friendRequestConsumerDurableName("realtime-1"),
		DeliverSubject: realtimeConsumerDeliverSubject("realtime-1", "friend_request"),
		FilterSubject:  "social.friend_request", DeliverPolicy: nats.DeliverNewPolicy, AckPolicy: nats.AckExplicitPolicy,
	}); err != nil {
		t.Fatal(err)
	}
	return admin, rt, hubRT, proof
}

func TestLiveACLProofSucceedsAndRemovesOnlyMarker(t *testing.T) {
	admin, rt, hubRT, proof := startLiveACLProofServer(t, false, false, true)
	if err := runNATSLiveACLProof(rt, hubRT, proof, "realtime-1"); err != nil {
		t.Fatalf("proof: %v", err)
	}
	js, _ := admin.JetStream()
	info, err := js.StreamInfo(jsStreamSocialEvents)
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != 0 {
		t.Fatalf("retained messages = %d, want zero", info.State.Msgs)
	}
}

func TestLiveACLProofRejectsAdjacentSubscriptionGrant(t *testing.T) {
	_, rt, hubRT, proof := startLiveACLProofServer(t, true, false, true)
	err := runNATSLiveACLProof(rt, hubRT, proof, "realtime-1")
	var failure *liveACLProofFailure
	if !errors.As(err, &failure) || failure.code != "adjacent_sub_unproven" {
		t.Fatalf("failure = %v, want adjacent_sub_unproven", err)
	}
}

func TestLiveACLProofRejectsConsumerCreateGrant(t *testing.T) {
	_, rt, hubRT, proof := startLiveACLProofServer(t, false, true, true)
	err := runNATSLiveACLProof(rt, hubRT, proof, "realtime-1")
	var failure *liveACLProofFailure
	if !errors.As(err, &failure) || failure.code != "consumer_create_allowed" {
		t.Fatalf("failure = %v, want consumer_create_allowed", err)
	}
}

func TestLiveACLProofRequiresEmptyStreamBeforePublishing(t *testing.T) {
	admin, rt, hubRT, proof := startLiveACLProofServer(t, false, false, true)
	js, _ := admin.JetStream()
	if _, err := js.Publish("social.friend_request", []byte("historical")); err != nil {
		t.Fatal(err)
	}
	err := runNATSLiveACLProof(rt, hubRT, proof, "realtime-1")
	var failure *liveACLProofFailure
	if !errors.As(err, &failure) || failure.code != "stream_not_empty" {
		t.Fatalf("failure = %v, want stream_not_empty", err)
	}
	info, err := js.StreamInfo(jsStreamSocialEvents)
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != 1 {
		t.Fatalf("historical messages = %d, want unchanged", info.State.Msgs)
	}
}

func TestLiveACLProofRejectsDeletedHistoricalSequence(t *testing.T) {
	admin, rt, hubRT, proof := startLiveACLProofServer(t, false, false, true)
	js, _ := admin.JetStream()
	ack, err := js.Publish("social.friend_request", []byte("historical"))
	if err != nil {
		t.Fatal(err)
	}
	if err := js.DeleteMsg(jsStreamSocialEvents, ack.Sequence); err != nil {
		t.Fatal(err)
	}
	info, err := js.StreamInfo(jsStreamSocialEvents)
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != 0 || info.State.LastSeq == 0 {
		t.Fatalf("fixture not empty with history: msgs=%d last=%d", info.State.Msgs, info.State.LastSeq)
	}
	err = runNATSLiveACLProof(rt, hubRT, proof, "realtime-1")
	var failure *liveACLProofFailure
	if !errors.As(err, &failure) || failure.code != "stream_not_empty" {
		t.Fatalf("failure = %v, want stream_not_empty", err)
	}
}

func TestLiveACLProofCleansMarkerAfterRejectedHubAck(t *testing.T) {
	admin, rt, hubRT, proof := startLiveACLProofServer(t, false, false, false)
	err := runNATSLiveACLProof(rt, hubRT, proof, "realtime-1")
	var failure *liveACLProofFailure
	if !errors.As(err, &failure) || failure.code != "hub_ack_failed" {
		t.Fatalf("failure = %v, want hub_ack_failed", err)
	}
	js, _ := admin.JetStream()
	info, err := js.StreamInfo(jsStreamSocialEvents)
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != 0 {
		t.Fatalf("retained messages after failed ACK = %d", info.State.Msgs)
	}
}

func TestLiveACLProofOutputNeverContainsUnderlyingError(t *testing.T) {
	line := liveACLProofResultLine(errors.New("private credential and payload"), "r20260930a", strings.Repeat("a", 64))
	if line != "NATS_LIVE_ACL_PROOF=FAIL code=internal_error" {
		t.Fatalf("unsafe failure line: %q", line)
	}
}
