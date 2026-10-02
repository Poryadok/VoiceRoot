package main

import (
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testHub(t *testing.T, b *testBundle, store string) *server.Server {
	t.Helper()
	config := hubConfig(b.files)
	for _, n := range []string{"operator.jwt", "tls.crt", "tls.key", "ca.crt"} {
		config = strings.ReplaceAll(config, "/etc/nats/jwt/"+n, filepath.ToSlash(filepath.Join(b.dir, n)))
		config = strings.ReplaceAll(config, "/etc/nats/tls/"+n, filepath.ToSlash(filepath.Join(b.dir, n)))
	}
	config = strings.ReplaceAll(config, "/data", filepath.ToSlash(store))
	path := filepath.Join(t.TempDir(), "server.conf")
	writeTest(t, path, []byte(config))
	opts, e := server.ProcessConfigFile(path)
	if e != nil {
		t.Fatal("fixture config parse failed")
	}
	opts.Port = -1
	opts.Host = "127.0.0.1"
	opts.HTTPPort = -1
	opts.LeafNode.Port = -1
	opts.NoLog = true
	opts.NoSigs = true
	s, e := server.NewServer(opts)
	if e != nil {
		t.Fatal("fixture server failed")
	}
	s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("fixture server not ready")
	}
	t.Cleanup(func() { s.Shutdown(); s.WaitForShutdown() })
	return s
}
func TestPopulatedExactIdentityProofAndPersistedACK(t *testing.T) {
	b := newBundle(t)
	store := filepath.Join(t.TempDir(), "owned-data")
	s := testHub(t, b, store)
	url := s.ClientURL()
	if e := setupClone(url, b.dir); e != nil {
		t.Fatalf("setup: %v", e)
	}
	if e := exerciseClone(url, b.dir); e != nil {
		t.Fatalf("exercise: %v", e)
	}
	s.Shutdown()
	s.WaitForShutdown()
	s = testHub(t, b, store)
	if e := verifyPersistedClone(s.ClientURL(), b.dir); e != nil {
		t.Fatalf("restart: %v", e)
	}
}
func TestMissingRealACKCannotPass(t *testing.T) {
	b := newBundle(t)
	g := b.p.Services["realtime"]
	var keep []string
	for _, p := range g.Publish {
		if p != "$JS.ACK.social_events.rt_realtime1_friend_removed.>" {
			keep = append(keep, p)
		}
	}
	g.Publish = keep
	b.files["realtime.creds"] = testCredential(t, b.account, g, nil)
	b.save(t)
	s := testHub(t, b, filepath.Join(t.TempDir(), "data"))
	if e := setupClone(s.ClientURL(), b.dir); e != nil {
		t.Fatalf("setup: %v", e)
	}
	if e := exerciseClone(s.ClientURL(), b.dir); e == nil {
		t.Fatal("missing exact ACK grant produced successful proof")
	}
}
func TestTimeoutIsNotAuthorizationDenial(t *testing.T) {
	b := newBundle(t)
	s := testHub(t, b, filepath.Join(t.TempDir(), "data"))
	p, e := connectProof(s.ClientURL(), b.dir, "social")
	if e != nil {
		t.Fatal(e)
	}
	defer p.nc.Close()
	if e = denied(p, func() error { return p.nc.Publish("social.friend_request", nil) }); e == nil || e.Error() != "negative_denial_not_observed" {
		t.Fatalf("allowed operation cannot prove denial: %v", e)
	}
}
func TestRuntimeRefusesAmbientEndpoint(t *testing.T) {
	os.Unsetenv("NATS_POPULATED_PROOF_RUNTIME")
	if e := runtimePhase("setup"); e == nil {
		t.Fatal("ambient process accepted as isolated proof")
	}
}

var _ = nats.ErrPermissionViolation
