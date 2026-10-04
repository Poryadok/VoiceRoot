package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func broker(t *testing.T, store string) *server.Server {
	t.Helper()
	s, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, JetStream: true, StoreDir: store, NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("broker not ready")
	}
	t.Cleanup(func() { s.Shutdown(); s.WaitForShutdown() })
	return s
}

func client(t *testing.T, s *server.Server) *nats.Conn {
	t.Helper()
	nc, err := nats.Connect(s.ClientURL(), nats.Timeout(time.Second), nats.NoReconnect())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	return nc
}

func prepare(t *testing.T, nc *nats.Conn) {
	t.Helper()
	js, _ := nc.JetStream(nats.MaxWait(time.Second))
	if _, err := js.AddStream(&nats.StreamConfig{Name: "social_events", Subjects: []string{"social.user_blocked"}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy}); err != nil {
		t.Fatal(err)
	}
	if _, err := js.AddConsumer("social_events", &nats.ConsumerConfig{Durable: "rt_realtime1_social", DeliverSubject: "_INBOX.voice.realtime1.social", FilterSubject: "social.user_blocked", DeliverPolicy: nats.DeliverNewPolicy, AckPolicy: nats.AckExplicitPolicy, AckWait: time.Second}); err != nil {
		t.Fatal(err)
	}
}

// Test-only copy is performed after actual broker shutdown. Production custody
// uses the controller's bounded archive inventory/extraction, not this helper.
func copyStore(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0700)
		}
		if !d.Type().IsRegular() {
			t.Fatal("unexpected nonregular fixture")
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestColdRestoreKnownRecordsAndPendingState(t *testing.T) {
	src := t.TempDir()
	s := broker(t, src)
	admin, pub, rt := client(t, s), client(t, s), client(t, s)
	prepare(t, admin)
	snapshot, err := seedFixture(admin, pub, rt)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.AckStream != 1 || snapshot.DeliveredStream != 2 || snapshot.AckPending != 1 || snapshot.Pending != 1 || len(snapshot.Records) != 3 {
		t.Fatalf("incorrect closed state: %+v", snapshot)
	}
	admin.Close()
	pub.Close()
	rt.Close()
	s.Shutdown()
	s.WaitForShutdown()
	dst := t.TempDir()
	copyStore(t, src, dst)
	restored := broker(t, dst)
	a, r := client(t, restored), client(t, restored)
	if err := verifyClosedFixture(a, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := drainFixture(a, r, snapshot); err != nil {
		t.Fatal(err)
	}
	a.Close()
	r.Close()
	restored.Shutdown()
	restored.WaitForShutdown()
	restarted := broker(t, dst)
	if err := verifyDrainedFixture(client(t, restarted), snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreRejectsChangedPayloadDigestBeforeACK(t *testing.T) {
	s := broker(t, t.TempDir())
	a, p, r := client(t, s), client(t, s), client(t, s)
	prepare(t, a)
	snapshot, err := seedFixture(a, p, r)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Records[1].PayloadSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
	if err := drainFixture(a, client(t, s), snapshot); err == nil {
		t.Fatal("altered record accepted")
	}
	js, _ := a.JetStream()
	ci, err := js.ConsumerInfo("social_events", "rt_realtime1_social")
	if err != nil {
		t.Fatal(err)
	}
	if ci.AckFloor.Stream != 1 {
		t.Fatal("mismatched record was acknowledged")
	}
}

func TestClosedSnapshotRejectsDifferentACKPosition(t *testing.T) {
	s := broker(t, t.TempDir())
	a, p, r := client(t, s), client(t, s), client(t, s)
	prepare(t, a)
	snapshot, err := seedFixture(a, p, r)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.AckStream = 2
	if err := verifyClosedFixture(a, snapshot); err == nil {
		t.Fatal("changed ACK state accepted")
	}
}

func TestClosedSnapshotRejectsForgedKnownRecordIdentity(t *testing.T) {
	s := broker(t, t.TempDir())
	a, p, r := client(t, s), client(t, s), client(t, s)
	prepare(t, a)
	snapshot, err := seedFixture(a, p, r)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Records[1].EventID = "forged-ledger-event"
	if err := verifyClosedFixture(a, snapshot); err == nil {
		t.Fatal("closed ledger accepted a forged known record ID")
	}
}
