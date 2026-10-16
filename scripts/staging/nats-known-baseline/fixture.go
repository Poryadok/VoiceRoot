package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
)

const fixtureStream = "social_events"
const fixtureSubject = "social.user_blocked"
const fixtureDurable = "rt_realtime1_social"

type record struct {
	EventID       string `json:"event_id"`
	Subject       string `json:"subject"`
	Sequence      uint64 `json:"sequence"`
	PayloadSHA256 string `json:"payload_sha256"`
	HeadersSHA256 string `json:"headers_sha256"`
}

type fixtureSnapshot struct {
	Schema               string   `json:"schema"`
	Records              []record `json:"records"`
	StreamConfigSHA256   string   `json:"stream_config_sha256"`
	ConsumerConfigSHA256 string   `json:"consumer_config_sha256"`
	StreamLast           uint64   `json:"stream_last"`
	MessageCount         uint64   `json:"message_count"`
	AckConsumer          uint64   `json:"ack_consumer"`
	AckStream            uint64   `json:"ack_stream"`
	DeliveredConsumer    uint64   `json:"delivered_consumer"`
	DeliveredStream      uint64   `json:"delivered_stream"`
	AckPending           int      `json:"ack_pending"`
	Pending              uint64   `json:"pending"`
}

func digest(b []byte) string       { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func canonicalDigest(v any) string { b, _ := json.Marshal(v); return digest(b) }
func fixedError(code string) error { return errors.New(code) }

func info(nc *nats.Conn) (*nats.StreamInfo, *nats.ConsumerInfo, error) {
	js, e := nc.JetStream(nats.MaxWait(2 * time.Second))
	if e != nil {
		return nil, nil, fixedError("fixture_connection_failed")
	}
	si, e := js.StreamInfo(fixtureStream)
	if e != nil {
		return nil, nil, fixedError("fixture_stream_info_failed")
	}
	ci, e := js.ConsumerInfo(fixtureStream, fixtureDurable)
	if e != nil {
		return nil, nil, fixedError("fixture_consumer_info_failed")
	}
	return si, ci, nil
}

func state(nc *nats.Conn) (fixtureSnapshot, error) {
	si, ci, e := info(nc)
	if e != nil {
		return fixtureSnapshot{}, e
	}
	return fixtureSnapshot{Schema: "known-nats-fixture-v1", StreamConfigSHA256: canonicalDigest(si.Config), ConsumerConfigSHA256: canonicalDigest(ci.Config), StreamLast: si.State.LastSeq, MessageCount: si.State.Msgs, AckConsumer: ci.AckFloor.Consumer, AckStream: ci.AckFloor.Stream, DeliveredConsumer: ci.Delivered.Consumer, DeliveredStream: ci.Delivered.Stream, AckPending: ci.NumAckPending, Pending: ci.NumPending}, nil
}

func subscribe(nc *nats.Conn) (*nats.Subscription, error) {
	js, e := nc.JetStream(nats.MaxWait(2 * time.Second))
	if e != nil {
		return nil, fixedError("fixture_connection_failed")
	}
	s, e := js.SubscribeSync(fixtureSubject, nats.Bind(fixtureStream, fixtureDurable), nats.ManualAck())
	if e != nil {
		return nil, fixedError("fixture_bind_failed")
	}
	if nc.FlushTimeout(2*time.Second) != nil {
		return nil, fixedError("fixture_bind_flush_failed")
	}
	return s, nil
}

func fixtureMessage(ordinal int) *nats.Msg {
	// Deliberately synthetic: caller must keep this broker physically isolated
	// from every business service until its owned storage is retired.
	body, _ := json.Marshal(struct {
		EventID string `json:"event_id"`
		Fixture string `json:"fixture"`
		Ordinal int    `json:"ordinal"`
	}{EventID: "voice-nats-known-backup-" + string(rune('0'+ordinal)), Fixture: "isolated-cold-backup-v1", Ordinal: ordinal})
	var bodyID struct {
		EventID string `json:"event_id"`
	}
	_ = json.Unmarshal(body, &bodyID)
	msg := nats.NewMsg(fixtureSubject)
	msg.Data = body
	msg.Header.Set(nats.MsgIdHdr, bodyID.EventID)
	return msg
}

func publish(nc *nats.Conn, ordinal int) (record, error) {
	msg := fixtureMessage(ordinal)
	js, e := nc.JetStream(nats.MaxWait(2 * time.Second))
	if e != nil {
		return record{}, fixedError("fixture_connection_failed")
	}
	ack, e := js.PublishMsg(msg)
	if e != nil || ack.Stream != fixtureStream || ack.Sequence != uint64(ordinal) || ack.Duplicate {
		return record{}, fixedError("fixture_publish_failed")
	}
	return record{EventID: msg.Header.Get(nats.MsgIdHdr), Subject: fixtureSubject, Sequence: ack.Sequence, PayloadSHA256: digest(msg.Data), HeadersSHA256: canonicalDigest(msg.Header)}, nil
}

func checkMessage(msg *nats.Msg, want record) error {
	m, e := msg.Metadata()
	if e != nil || m.Stream != fixtureStream || m.Sequence.Stream != want.Sequence || msg.Subject != want.Subject || msg.Header.Get(nats.MsgIdHdr) != want.EventID || digest(msg.Data) != want.PayloadSHA256 || canonicalDigest(msg.Header) != want.HeadersSHA256 {
		return fixedError("fixture_record_mismatch")
	}
	return nil
}

func seedFixture(admin, pub, rt *nats.Conn) (fixtureSnapshot, error) {
	before, e := state(admin)
	if e != nil {
		return fixtureSnapshot{}, e
	}
	if before.MessageCount != 0 || before.StreamLast != 0 || before.AckStream != 0 || before.DeliveredStream != 0 || before.Pending != 0 || before.AckPending != 0 {
		return fixtureSnapshot{}, fixedError("fixture_source_not_empty")
	}
	s, e := subscribe(rt)
	if e != nil {
		return fixtureSnapshot{}, e
	}
	defer s.Unsubscribe()
	records := make([]record, 0, 3)
	for i := 1; i <= 2; i++ {
		r, e := publish(pub, i)
		if e != nil {
			return fixtureSnapshot{}, e
		}
		records = append(records, r)
		msg, e := s.NextMsg(2 * time.Second)
		if e != nil {
			return fixtureSnapshot{}, fixedError("fixture_delivery_failed")
		}
		if e = checkMessage(msg, r); e != nil {
			return fixtureSnapshot{}, e
		}
		if i == 1 && msg.AckSync(nats.AckWait(2*time.Second)) != nil {
			return fixtureSnapshot{}, fixedError("fixture_ack_failed")
		}
	}
	if s.Unsubscribe() != nil || rt.FlushTimeout(2*time.Second) != nil {
		return fixtureSnapshot{}, fixedError("fixture_unbind_failed")
	}
	rt.Close()
	// Wait for actual server-side interest removal, not a caller assertion.
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, ci, e := info(admin)
		if e != nil {
			return fixtureSnapshot{}, e
		}
		if !ci.PushBound {
			break
		}
		if time.Now().After(deadline) {
			return fixtureSnapshot{}, fixedError("fixture_interest_not_removed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	r, e := publish(pub, 3)
	if e != nil {
		return fixtureSnapshot{}, e
	}
	records = append(records, r)
	after, e := state(admin)
	if e != nil {
		return fixtureSnapshot{}, e
	}
	after.Records = records
	if after.AckStream != 1 || after.DeliveredStream != 2 || after.AckPending != 1 || after.Pending != 1 || after.MessageCount != 3 {
		return fixtureSnapshot{}, fixedError("fixture_seed_state_mismatch")
	}
	return after, nil
}

func validSnapshot(w fixtureSnapshot) bool {
	if w.Schema != "known-nats-fixture-v1" || len(w.Records) != 3 || w.MessageCount != 3 || w.StreamLast != 3 || w.AckConsumer != 1 || w.AckStream != 1 || w.DeliveredConsumer != 2 || w.DeliveredStream != 2 || w.AckPending != 1 || w.Pending != 1 {
		return false
	}
	for i, r := range w.Records {
		msg := fixtureMessage(i + 1)
		if r.Sequence != uint64(i+1) || r.Subject != fixtureSubject || r.EventID != msg.Header.Get(nats.MsgIdHdr) || r.PayloadSHA256 != digest(msg.Data) || r.HeadersSHA256 != canonicalDigest(msg.Header) {
			return false
		}
	}
	return true
}

func verifyClosedFixture(admin *nats.Conn, w fixtureSnapshot) error {
	if !validSnapshot(w) {
		return fixedError("fixture_snapshot_invalid")
	}
	got, e := state(admin)
	if e != nil {
		return e
	}
	got.Records = w.Records
	if canonicalDigest(got) != canonicalDigest(w) {
		return fixedError("fixture_closed_state_mismatch")
	}
	return nil
}

func drainFixture(admin, rt *nats.Conn, w fixtureSnapshot) error {
	if !validSnapshot(w) {
		return fixedError("fixture_snapshot_invalid")
	}
	s, e := subscribe(rt)
	if e != nil {
		return e
	}
	defer s.Unsubscribe()
	seen := map[uint64]bool{}
	deadline := time.Now().Add(40 * time.Second)
	for len(seen) < 2 {
		if time.Now().After(deadline) {
			return fixedError("fixture_drain_timeout")
		}
		msg, e := s.NextMsg(time.Second)
		if e == nats.ErrTimeout {
			continue
		}
		if e != nil {
			return fixedError("fixture_drain_failed")
		}
		m, e := msg.Metadata()
		if e != nil || m.Sequence.Stream < 2 || m.Sequence.Stream > 3 || seen[m.Sequence.Stream] {
			return fixedError("fixture_drain_identity_failed")
		}
		if e = checkMessage(msg, w.Records[m.Sequence.Stream-1]); e != nil {
			return e
		}
		if m.Sequence.Stream == 2 && m.NumDelivered < 2 {
			return fixedError("fixture_redelivery_not_preserved")
		}
		if msg.AckSync(nats.AckWait(2*time.Second)) != nil {
			return fixedError("fixture_ack_failed")
		}
		seen[m.Sequence.Stream] = true
	}
	return verifyDrainedFixture(admin, w)
}

func verifyDrainedFixture(admin *nats.Conn, w fixtureSnapshot) error {
	if !validSnapshot(w) {
		return fixedError("fixture_snapshot_invalid")
	}
	got, e := state(admin)
	if e != nil {
		return e
	}
	// Three unique records produce four deliveries: seq2 is delivered once
	// before capture and redelivered after restore. Last stream delivery may
	// be seq2 or seq3, while the final ACK stream floor covers all three.
	if got.StreamConfigSHA256 != w.StreamConfigSHA256 || got.ConsumerConfigSHA256 != w.ConsumerConfigSHA256 || got.StreamLast != 3 || got.MessageCount != 3 || got.AckStream != 3 || got.AckConsumer != 4 || got.DeliveredConsumer != 4 || (got.DeliveredStream != 2 && got.DeliveredStream != 3) || got.Pending != 0 || got.AckPending != 0 {
		return fixedError("fixture_drained_state_mismatch")
	}
	return nil
}
