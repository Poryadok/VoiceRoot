// Standalone disposable broker client. This is not an MM inbox implementation.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

const roster = "voice.roster_membership_changed"
const durable = "matchmaking_voice_roster_v1"

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func connect(role string, denied chan error) *nats.Conn {
	file, inbox := role, "_INBOX.fixture.reply"
	switch role {
	case "voice", "matchmaking":
		file = "a3-" + role
		inbox = "_INBOX.voice." + role + ".reply"
	case "bootstrap":
		inbox = "_INBOX.voice.bootstrap.reply"
	case "social":
		inbox = "_INBOX.voice.social.reply"
	default:
		panic("fixture_role_invalid")
	}
	nc, err := nats.Connect("nats://127.0.0.1:4222", nats.UserCredentials("/inputs/"+file+".creds"),
		nats.CustomInboxPrefix(inbox), nats.Timeout(2*time.Second), nats.NoReconnect(),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			if denied != nil {
				select {
				case denied <- err:
				default:
				}
			}
		}))
	must(err)
	return nc
}

func publish(js nats.JetStreamContext, subject, id string) uint64 {
	m := nats.NewMsg(subject)
	m.Header.Set(nats.MsgIdHdr, id)
	m.Data = []byte(`{"event_id":"` + id + `","payload":"frozen-a3-native-envelope"}`)
	ack, err := js.PublishMsg(m)
	must(err)
	return ack.Sequence
}

func seed(mode string) {
	admin := connect("social", nil)
	defer admin.Close()
	js, err := admin.JetStream()
	must(err)
	// Establish the immutable reviewed predecessor without adding any Voice
	// records: pristine-empty and advanced-empty are separate native cases.
	raw, err := os.ReadFile("/inputs/predecessor.json")
	must(err)
	var predecessor struct {
		ChatSubjects []string `json:"chat_subjects"`
	}
	must(json.Unmarshal(raw, &predecessor))
	if len(predecessor.ChatSubjects) != 14 {
		panic("fixture_predecessor_shape")
	}
	chat, err := js.StreamInfo("chat_events")
	must(err)
	chat.Config.Subjects = predecessor.ChatSubjects
	_, err = js.UpdateStream(&chat.Config)
	must(err)
	_, err = js.AddConsumer("social_events", &nats.ConsumerConfig{Durable: "rt_realtime1_friend_removed", FilterSubject: "social.friend_removed", DeliverSubject: "_INBOX.voice.realtime1.friend_removed", DeliverPolicy: nats.DeliverNewPolicy, AckPolicy: nats.AckExplicitPolicy})
	must(err)
	info, err := js.StreamInfo("voice_events")
	must(err)
	if mode != "empty10" {
		info.Config.Subjects = append(info.Config.Subjects, "voice.participant_joined", "voice.participant_left")
		if mode != "empty12" {
			info.Config.Subjects = append(info.Config.Subjects, roster)
		}
		_, err = js.UpdateStream(&info.Config)
		must(err)
	}
	voice := connect("voice", nil)
	defer voice.Close()
	vjs, err := voice.JetStream()
	must(err)
	switch mode {
	case "empty10", "empty12", "empty13":
	case "other":
		for _, id := range []string{"other-first", "other-middle", "other-last"} {
			publish(vjs, "voice.state_changed", id)
		}
		must(js.DeleteMsg("voice_events", 1))
	case "advanced-empty":
		for _, id := range []string{"purge-first", "purge-middle", "purge-last"} {
			publish(vjs, "voice.state_changed", id)
		}
		must(js.PurgeStream("voice_events"))
	case "populated":
		publish(vjs, "voice.state_changed", "deleted-head-one")
		publish(vjs, "voice.state_changed", "deleted-head-two")
		publish(vjs, "voice.state_changed", "leading-nonmatch")
		publish(vjs, roster, "a3-roster-A")
		publish(vjs, "voice.state_changed", "deleted-hole")
		publish(vjs, roster, "a3-roster-B")
		publish(vjs, "voice.state_changed", "trailing-nonmatch")
		for _, seq := range []uint64{1, 2, 5} {
			must(js.DeleteMsg("voice_events", seq))
		}
	default:
		panic("fixture_seed_mode_invalid")
	}
	must(admin.Flush())
	must(voice.Flush())
}

func deniedPublish(role, subject string) {
	errorsSeen := make(chan error, 4)
	nc := connect(role, errorsSeen)
	defer nc.Close()
	must(nc.Publish(subject, []byte(`{"fixture":"denied"}`)))
	_ = nc.FlushTimeout(2 * time.Second)
	select {
	case err := <-errorsSeen:
		if !errors.Is(err, nats.ErrPermissionViolation) && !strings.Contains(strings.ToLower(err.Error()), "permissions violation") {
			panic("fixture_wrong_denial")
		}
	case <-time.After(3 * time.Second):
		panic("fixture_expected_permission_denial")
	}
}

func rights() {
	voice := connect("voice", nil)
	defer voice.Close()
	js, err := voice.JetStream()
	must(err)
	publish(js, roster, "a3-permission-probe")
	mm := connect("matchmaking", nil)
	defer mm.Close()
	mjs, err := mm.JetStream()
	must(err)
	info, err := mjs.ConsumerInfo("voice_events", durable)
	must(err)
	if info.Config.DeliverPolicy != nats.DeliverAllPolicy || info.Config.DeliverSubject != "" {
		panic("fixture_wrong_pull_config")
	}
	sub, err := mjs.PullSubscribe(roster, durable, nats.Bind("voice_events", durable))
	must(err)
	messages, err := sub.Fetch(1, nats.MaxWait(3*time.Second))
	must(err)
	if len(messages) != 1 || messages[0].Header.Get(nats.MsgIdHdr) != "a3-permission-probe" {
		panic("fixture_permission_message_changed")
	}
	must(messages[0].AckSync())
	for _, probe := range [][2]string{
		{"voice", "social.friend_removed"},
		{"matchmaking", "voice.roster_membership_changed"},
		{"matchmaking", "$JS.API.CONSUMER.CREATE.voice_events.unrelated"},
		{"matchmaking", "$JS.API.CONSUMER.INFO.voice_events.unrelated"},
		{"matchmaking", "$JS.API.CONSUMER.MSG.NEXT.voice_events.unrelated"},
		{"matchmaking", "$JS.ACK.voice_events.unrelated.1.1.1.1.1"},
		{"bootstrap", "$JS.API.STREAM.DELETE.voice_events"},
		{"bootstrap", "$JS.API.CONSUMER.CREATE.voice_events.unrelated"},
	} {
		deniedPublish(probe[0], probe[1])
	}
	fmt.Println(`{"signed_fixture_voice_publish_mm_info_next_ack":true,"unrelated_scope_denials":8,"installed_rights_claimed":false}`)
}

func delivery(mode string) {
	nc := connect("matchmaking", nil)
	defer nc.Close()
	js, err := nc.JetStream()
	must(err)
	sub, err := js.PullSubscribe(roster, durable, nats.Bind("voice_events", durable))
	must(err)
	n := 2
	if mode == "lost-ack" {
		n = 1
	}
	messages, err := sub.Fetch(n, nats.MaxWait(35*time.Second))
	must(err)
	if len(messages) != n {
		panic("fixture_incomplete_backlog")
	}
	ids := make([]string, 0, n)
	for _, message := range messages {
		id := message.Header.Get(nats.MsgIdHdr)
		if id != "a3-roster-A" && id != "a3-roster-B" {
			panic("fixture_foreign_envelope")
		}
		if string(message.Data) != `{"event_id":"`+id+`","payload":"frozen-a3-native-envelope"}` {
			panic("fixture_envelope_changed")
		}
		metadata, err := message.Metadata()
		must(err)
		expectedSequence := uint64(4)
		if id == "a3-roster-B" {
			expectedSequence = 6
		}
		if metadata.Sequence.Stream != expectedSequence || metadata.NumDelivered < 1 {
			panic("fixture_stream_sequence_changed")
		}
		if mode == "finish" && id == "a3-roster-A" {
			if metadata.NumDelivered < 2 {
				panic("fixture_lost_ack_not_redelivered")
			}
		} else if metadata.NumDelivered != 1 {
			panic("fixture_unexpected_redelivery")
		}
		if mode != "lost-ack" {
			must(message.AckSync())
		}
		ids = append(ids, id)
	}
	if mode == "all" && strings.Join(ids, ",") != "a3-roster-A,a3-roster-B" {
		panic("fixture_backlog_order_changed")
	}
	if mode == "lost-ack" && ids[0] != "a3-roster-A" {
		panic("fixture_first_delivery_changed")
	}
	if mode == "finish" && (len(ids) != 2 || ids[0] == ids[1]) {
		panic("fixture_redelivery_backlog_incomplete")
	}
	raw, _ := json.Marshal(map[string]any{"mode": mode, "ids": ids, "acked": mode != "lost-ack", "product_inbox_proof": false})
	fmt.Println(string(raw))
}

func main() {
	defer func() {
		if failure := recover(); failure != nil {
			fmt.Printf("a3-fixture-failed: %v\n", failure)
			os.Exit(1)
		}
	}()
	if len(os.Args) < 2 {
		panic("fixture_command_required")
	}
	switch os.Args[1] {
	case "seed":
		if len(os.Args) != 3 {
			panic("fixture_mode_required")
		}
		seed(os.Args[2])
	case "rights":
		rights()
	case "deliver":
		if len(os.Args) != 3 {
			panic("fixture_mode_required")
		}
		delivery(os.Args[2])
	default:
		panic("fixture_command_invalid")
	}
}
