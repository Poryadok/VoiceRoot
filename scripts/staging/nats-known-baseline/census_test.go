package main

import (
	"strconv"
	"strings"
	"testing"

	"github.com/nats-io/nats.go"
)

func TestCanonicalCensusRejectsUnlistedStream(t *testing.T) {
	s := broker(t, t.TempDir())
	nc := client(t, s)
	js, _ := nc.JetStream()
	for _, name := range streamNames {
		if _, err := js.AddStream(&nats.StreamConfig{Name: name, Subjects: []string{name + ".>"}, Storage: nats.FileStorage}); err != nil {
			t.Fatal(err)
		}
	}
	for i, pair := range durablePairs {
		p := strings.Split(pair, "/")
		if _, err := js.AddConsumer(p[0], &nats.ConsumerConfig{Durable: p[1], DeliverSubject: "_INBOX.fixture." + strconv.Itoa(i), FilterSubject: p[0] + ".fixture", AckPolicy: nats.AckExplicitPolicy}); err != nil {
			t.Fatal(err)
		}
	}
	c, err := captureCensus(nc)
	if err != nil || len(c.Streams) != 15 || len(c.Consumers) != 42 {
		t.Fatalf("canonical census failed: %v", err)
	}
	if _, err = js.AddStream(&nats.StreamConfig{Name: "unlisted", Subjects: []string{"unlisted.>"}, Storage: nats.FileStorage}); err != nil {
		t.Fatal(err)
	}
	if _, err = captureCensus(nc); err == nil {
		t.Fatal("unlisted account stream escaped canonical census")
	}
}
