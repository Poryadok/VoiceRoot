package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

const operationTimeout = 3 * time.Second

var probeConsumers = []struct{ durable, subject, delivery string }{
	{"rt_realtime1_friend_request", "social.friend_request", "_INBOX.voice.realtime1.friend_request"},
	{"rt_realtime1_friend_removed", "social.friend_removed", "_INBOX.voice.realtime1.friend_removed"},
}

type proofConnection struct {
	nc     *nats.Conn
	denied chan error
}

func connectProof(url, dir, service string) (*proofConnection, error) {
	p := &proofConnection{denied: make(chan error, 16)}
	nc, e := nats.Connect(url, nats.Name("populated-clone-"+service), nats.UserCredentials(filepath.Join(dir, service+".creds")), nats.CustomInboxPrefix("_INBOX.voice."+service+func() string {
		if service == "bootstrap" {
			return ".reply"
		}
		return ""
	}()), nats.Timeout(operationTimeout), nats.NoReconnect(), nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, e error) {
		select {
		case p.denied <- e:
		default:
		}
	}))
	if e != nil {
		return nil, failure("clone_connect_failed")
	}
	p.nc = nc
	return p, nil
}
func request(p *proofConnection, subject string, body any, result any) error {
	var raw []byte
	var e error
	if body != nil {
		raw, e = json.Marshal(body)
	}
	if e != nil {
		return failure("clone_request_encode_failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()
	msg, e := p.nc.RequestWithContext(ctx, subject, raw)
	if e != nil {
		return failure("clone_request_failed")
	}
	var status struct {
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(msg.Data, &status) != nil {
		return failure("clone_response_invalid")
	}
	if status.Error != nil {
		return failure("clone_api_rejected:" + subject + ":" + strconv.Itoa(status.Error.Code))
	}
	if result != nil && json.Unmarshal(msg.Data, result) != nil {
		return failure("clone_response_invalid")
	}
	return nil
}
func setupClone(url, dir string) error {
	b, e := connectProof(url, dir, "bootstrap")
	if e != nil {
		return e
	}
	defer b.nc.Close()
	config := nats.StreamConfig{Name: "social_events", Subjects: []string{"social.friend_request", "social.friend_accepted", "social.friend_removed", "social.user_blocked", "social.contacts_synced"}, Retention: nats.LimitsPolicy, Storage: nats.FileStorage, MaxAge: 168 * time.Hour, Duplicates: 24 * time.Hour}
	if e = request(b, "$JS.API.STREAM.CREATE.social_events", config, nil); e != nil {
		return e
	}
	// The required new UPDATE grant is positively exercised against a populated
	// same-name stream later; creation does not stand in for update permission.
	for _, c := range append(append([]struct{ durable, subject, delivery string }{}, probeConsumers...), struct{ durable, subject, delivery string }{"rt_realtime1_social", "social.user_blocked", "_INBOX.voice.realtime1.social"}) {
		payload := struct {
			Stream string              `json:"stream_name"`
			Action string              `json:"action"`
			Config nats.ConsumerConfig `json:"config"`
		}{"social_events", "create", nats.ConsumerConfig{Name: c.durable, Durable: c.durable, DeliverSubject: c.delivery, DeliverPolicy: nats.DeliverNewPolicy, AckPolicy: nats.AckExplicitPolicy, FilterSubject: c.subject}}
		if e = request(b, "$JS.API.CONSUMER.CREATE.social_events."+c.durable, payload, nil); e != nil {
			return e
		}
	}
	s, e := connectProof(url, dir, "social")
	if e != nil {
		return e
	}
	defer s.nc.Close()
	js, e := s.nc.JetStream()
	if e != nil {
		return failure("clone_jetstream_failed")
	}
	for _, body := range []string{"preexisting-owned-sentinel-1", "preexisting-owned-sentinel-2"} {
		ack, e := js.Publish("social.user_blocked", []byte(body), nats.AckWait(operationTimeout))
		if e != nil || ack.Stream != "social_events" {
			return failure("clone_population_failed")
		}
	}
	if e = request(b, "$JS.API.STREAM.UPDATE.social_events", config, nil); e != nil {
		return e
	}
	return verifyProvisioning(b)
}
func denied(p *proofConnection, operation func() error) error {
	for {
		select {
		case <-p.denied:
			continue
		default:
			goto drained
		}
	}
drained:
	if e := operation(); e != nil && !errors.Is(e, nats.ErrPermissionViolation) {
		return failure("negative_operation_failed")
	}
	if e := p.nc.FlushTimeout(operationTimeout); e != nil {
		return failure("negative_flush_failed")
	}
	select {
	case e := <-p.denied:
		if !errors.Is(e, nats.ErrPermissionViolation) && !strings.Contains(strings.ToLower(e.Error()), "permissions violation") {
			return failure("negative_not_authorization_denial")
		}
		return nil
	case <-time.After(operationTimeout):
		return failure("negative_denial_not_observed")
	}
}
func exerciseClone(url, dir string) error {
	b, e := connectProof(url, dir, "bootstrap")
	if e != nil {
		return e
	}
	defer b.nc.Close()
	r, e := connectProof(url, dir, "realtime")
	if e != nil {
		return e
	}
	defer r.nc.Close()
	s, e := connectProof(url, dir, "social")
	if e != nil {
		return e
	}
	defer s.nc.Close()
	var before nats.ConsumerInfo
	if e = request(b, "$JS.API.CONSUMER.INFO.social_events.rt_realtime1_social", nil, &before); e != nil {
		return e
	}
	js, e := s.nc.JetStream()
	if e != nil {
		return failure("clone_jetstream_failed")
	}
	for _, c := range probeConsumers {
		sub, e := r.nc.SubscribeSync(c.delivery)
		if e != nil {
			return failure("clone_subscription_failed")
		}
		defer sub.Unsubscribe()
		if r.nc.FlushTimeout(operationTimeout) != nil {
			return failure("clone_subscription_flush_failed")
		}
		marker := []byte("owned-populated-proof:" + c.durable)
		ack, e := js.Publish(c.subject, marker, nats.AckWait(operationTimeout))
		if e != nil || ack.Stream != "social_events" || ack.Sequence <= 2 {
			return failure("clone_puback_not_proven")
		}
		msg, e := sub.NextMsg(operationTimeout)
		if e != nil || !bytes.Equal(msg.Data, marker) || msg.Subject != c.subject {
			return failure("clone_delivery_not_proven")
		}
		// Use an actual server-issued ACK token for the wrong-profile denial. A
		// made-up ACK subject cannot prove protection of an existing delivery.
		if e = denied(s, func() error { return s.nc.Publish(msg.Reply, []byte("+ACK")) }); e != nil {
			return e
		}
		if e = msg.AckSync(nats.AckWait(operationTimeout)); e != nil {
			return failure("clone_ack_not_proven")
		}
		var ci nats.ConsumerInfo
		if e = request(r, "$JS.API.CONSUMER.INFO.social_events."+c.durable, nil, &ci); e != nil {
			return e
		}
		if ci.AckFloor.Stream != ack.Sequence || ci.NumAckPending != 0 {
			return failure("clone_ack_floor_not_proven")
		}
	}
	if e = denied(r, func() error { _, e := r.nc.SubscribeSync("_INBOX.voice.chat.forbidden"); return e }); e != nil {
		return e
	}
	if e = denied(r, func() error {
		return r.nc.Publish("$JS.API.CONSUMER.CREATE.social_events.rt_realtime1_friend_removed", []byte("{}"))
	}); e != nil {
		return e
	}
	if e = denied(s, func() error { _, e := s.nc.SubscribeSync(probeConsumers[0].delivery); return e }); e != nil {
		return e
	}
	if e = denied(b, func() error { return b.nc.Publish("social.friend_request", []byte("forbidden")) }); e != nil {
		return e
	}
	var after nats.ConsumerInfo
	if e = request(b, "$JS.API.CONSUMER.INFO.social_events.rt_realtime1_social", nil, &after); e != nil {
		return e
	}
	if before.AckFloor != after.AckFloor || before.Delivered != after.Delivered || before.NumAckPending != after.NumAckPending || before.NumPending != after.NumPending {
		return failure("populated_consumer_state_changed")
	}
	return nil
}
func verifyPersistedClone(url, dir string) error {
	b, e := connectProof(url, dir, "bootstrap")
	if e != nil {
		return e
	}
	defer b.nc.Close()
	if e = verifyProvisioning(b); e != nil {
		return e
	}
	for i, c := range probeConsumers {
		var ci nats.ConsumerInfo
		if e = request(b, "$JS.API.CONSUMER.INFO.social_events."+c.durable, nil, &ci); e != nil {
			return e
		}
		if ci.AckFloor.Stream != uint64(3+i) || ci.NumAckPending != 0 {
			return failure("persisted_ack_not_proven")
		}
	}
	var si nats.StreamInfo
	if e = request(b, "$JS.API.STREAM.INFO.social_events", nil, &si); e != nil {
		return e
	}
	if si.State.Msgs != 4 || si.State.LastSeq != 4 {
		return failure("populated_stream_not_preserved")
	}
	return nil
}

// Only fixed network-local endpoints are accepted by the production helper.
// Tests call the kernel directly with their own ephemeral loopback fixtures.
func runtimePhase(phase string) error {
	context, e := loadRuntimeContext(phase)
	if e != nil {
		return e
	}
	dir := "/var/run/nats/proof-creds"
	hubURL := "nats://" + context.HubIP + ":4222"
	switch phase {
	case "ready":
		p, e := connectProof(hubURL, dir, "bootstrap")
		if e != nil {
			return e
		}
		defer p.nc.Close()
		return request(p, "$JS.API.INFO", nil, nil)
	case "setup":
		return setupClone(hubURL, dir)
	case "exercise":
		return exerciseClone(hubURL, dir)
	case "persisted":
		return verifyPersistedClone(hubURL, dir)
	case "leaf-receive":
		return receiveLeaf("nats://127.0.0.1:4222", "/tmp/receiver-ready")
	case "leaf-publish":
		return publishLeaf("nats://127.0.0.1:4222")
	case "leaf-ready":
		if _, e := os.Stat("/tmp/receiver-ready"); e != nil {
			return failure("leaf_receiver_not_ready")
		}
		return nil
	case "leaf-persisted":
		return verifyLeafPersisted(hubURL, dir)
	default:
		return failure("runtime_phase_invalid")
	}
}

func leafConnect(url, profile string) (*nats.Conn, error) {
	nc, e := nats.Connect(url, nats.CustomInboxPrefix("_INBOX.voice."+profile), nats.Timeout(operationTimeout), nats.NoReconnect())
	if e != nil {
		return nil, failure("leaf_connect_failed")
	}
	return nc, nil
}
func receiveLeaf(url, ready string) error {
	nc, e := leafConnect(url, "realtime")
	if e != nil {
		return e
	}
	defer nc.Close()
	var subs []*nats.Subscription
	for _, c := range probeConsumers {
		sub, e := nc.SubscribeSync(c.delivery)
		if e != nil {
			return failure("leaf_subscription_failed")
		}
		subs = append(subs, sub)
	}
	if nc.FlushTimeout(operationTimeout) != nil {
		return failure("leaf_subscription_flush_failed")
	}
	if ready != "" {
		if os.WriteFile(ready, []byte("ready"), 0600) != nil {
			return failure("leaf_ready_write_failed")
		}
		defer os.Remove(ready)
	}
	for i, sub := range subs {
		msg, e := sub.NextMsg(15 * time.Second)
		if e != nil || !bytes.Equal(msg.Data, []byte("owned-leaf-proof:"+probeConsumers[i].durable)) || msg.Subject != probeConsumers[i].subject {
			return failure("leaf_delivery_not_proven")
		}
		if e = msg.AckSync(nats.AckWait(operationTimeout)); e != nil {
			return failure("leaf_ack_not_proven")
		}
	}
	return nil
}
func publishLeaf(url string) error {
	nc, e := leafConnect(url, "social")
	if e != nil {
		return e
	}
	defer nc.Close()
	js, e := nc.JetStream()
	if e != nil {
		return failure("leaf_jetstream_failed")
	}
	for i, c := range probeConsumers {
		ack, e := js.Publish(c.subject, []byte("owned-leaf-proof:"+c.durable), nats.AckWait(operationTimeout))
		if e != nil || ack.Stream != "social_events" || ack.Sequence != uint64(5+i) {
			return failure("leaf_puback_not_proven")
		}
	}
	return nil
}
func verifyLeafPersisted(url, dir string) error {
	b, e := connectProof(url, dir, "bootstrap")
	if e != nil {
		return e
	}
	defer b.nc.Close()
	if e = verifyProvisioning(b); e != nil {
		return e
	}
	for i, c := range probeConsumers {
		var ci nats.ConsumerInfo
		if e = request(b, "$JS.API.CONSUMER.INFO.social_events."+c.durable, nil, &ci); e != nil {
			return e
		}
		if ci.AckFloor.Stream != uint64(5+i) || ci.NumAckPending != 0 {
			return failure("leaf_persisted_ack_not_proven")
		}
	}
	var si nats.StreamInfo
	if e = request(b, "$JS.API.STREAM.INFO.social_events", nil, &si); e != nil {
		return e
	}
	if si.State.Msgs != 6 || si.State.LastSeq != 6 {
		return failure("leaf_populated_stream_not_preserved")
	}
	return nil
}

func verifyProvisioning(b *proofConnection) error {
	var si nats.StreamInfo
	if e := request(b, "$JS.API.STREAM.INFO.social_events", nil, &si); e != nil {
		return e
	}
	if si.Config.MaxAge != 168*time.Hour || si.Config.Storage != nats.FileStorage || si.Config.Retention != nats.LimitsPolicy || si.Config.Duplicates != 24*time.Hour || !sameSet(si.Config.Subjects, []string{"social.friend_request", "social.friend_accepted", "social.friend_removed", "social.user_blocked", "social.contacts_synced"}) {
		return failure("clone_stream_config_mismatch")
	}
	for _, c := range append(append([]struct{ durable, subject, delivery string }{}, probeConsumers...), struct{ durable, subject, delivery string }{"rt_realtime1_social", "social.user_blocked", "_INBOX.voice.realtime1.social"}) {
		var ci nats.ConsumerInfo
		if e := request(b, "$JS.API.CONSUMER.INFO.social_events."+c.durable, nil, &ci); e != nil {
			return e
		}
		if ci.Config.Name != c.durable || ci.Config.Durable != c.durable || ci.Config.DeliverPolicy != nats.DeliverNewPolicy || ci.Config.AckPolicy != nats.AckExplicitPolicy || ci.Config.AckWait != 30*time.Second || ci.Config.MaxAckPending != 1000 || ci.Config.FilterSubject != c.subject || ci.Config.DeliverSubject != c.delivery || ci.Config.DeliverGroup != "" || len(ci.Config.FilterSubjects) > 0 {
			return failure("clone_consumer_config_mismatch")
		}
	}
	return nil
}
