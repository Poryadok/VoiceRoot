package main

import (
	"context"
	"encoding/json"
	"github.com/nats-io/nats.go"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProvisionedCloneMatchesBootstrapContract(t *testing.T) {
	b := newBundle(t)
	s := testHub(t, b, filepath.Join(t.TempDir(), "data"))
	if e := setupClone(s.ClientURL(), b.dir); e != nil {
		t.Fatal(e)
	}
	p, e := connectProof(s.ClientURL(), b.dir, "bootstrap")
	if e != nil {
		t.Fatal(e)
	}
	defer p.nc.Close()
	var si nats.StreamInfo
	if e = request(p, "$JS.API.STREAM.INFO.social_events", nil, &si); e != nil {
		t.Fatal(e)
	}
	if si.Config.MaxAge != 168*time.Hour {
		t.Fatal("clone must retain deployed 168h MaxAge")
	}
	for _, c := range probeConsumers {
		var ci nats.ConsumerInfo
		if e = request(p, "$JS.API.CONSUMER.INFO.social_events."+c.durable, nil, &ci); e != nil {
			t.Fatal(e)
		}
		if ci.Config.DeliverPolicy != nats.DeliverNewPolicy || ci.Config.AckWait != 30*time.Second || ci.Config.MaxAckPending != 1000 {
			t.Fatal("clone must use deployed DeliverNew and unmodified server defaults")
		}
	}
}
func TestForgedRuntimeMarkerCannotReachAmbientBroker(t *testing.T) {
	t.Setenv("NATS_POPULATED_PROOF_RUNTIME", "isolated-internal-v1")
	e := run([]string{"runtime", "setup", filepath.Join(t.TempDir(), "arbitrary-credentials")})
	if e == nil || e.Error() != "runtime_arguments_invalid" {
		t.Fatalf("public caller must reject arbitrary path before dialing/reading it: %v", e)
	}
}

type regressionDocker struct {
	work, helper, network, volume, owner, wrong string
	args                                        [][]string
}

func (d *regressionDocker) run(_ context.Context, args ...string) ([]byte, error) {
	d.args = append(d.args, append([]string{}, args...))
	if args[0] == "image" {
		return []byte("sha256:test\n"), nil
	}
	if args[0] == "volume" && args[1] == "inspect" {
		v := map[string]any{"Name": d.volume, "Driver": "local", "Scope": "local", "Mountpoint": "/var/lib/docker/volumes/" + d.volume + "/_data", "Labels": map[string]string{"voice.populated-proof.owner": d.owner}, "Options": map[string]string{}}
		switch d.wrong {
		case "owner":
			v["Labels"] = map[string]string{"voice.populated-proof.owner": "foreign"}
		case "missing-owner":
			v["Labels"] = nil
		case "driver":
			v["Driver"] = "nfs"
		case "bind-options":
			v["Options"] = map[string]string{"type": "none", "o": "bind", "device": "/retained-source"}
		}
		return json.Marshal(v)
	}
	if args[0] != "inspect" {
		return nil, nil
	}
	mounts := []map[string]any{}
	for _, entry := range [][2]string{{"/etc/nats/hub.conf", "hub.conf"}, {"/etc/nats/jwt/operator.jwt", "jwt/operator.jwt"}, {"/etc/nats/tls/tls.crt", "tls/tls.crt"}, {"/etc/nats/tls/tls.key", "tls/tls.key"}, {"/etc/nats/tls/ca.crt", "tls/ca.crt"}} {
		mounts = append(mounts, map[string]any{"Type": "bind", "Source": filepath.Join(d.work, filepath.FromSlash(entry[1])), "Destination": entry[0], "RW": false})
	}
	mounts = append(mounts, map[string]any{"Type": "volume", "Name": d.volume, "Source": "/var/lib/docker/volumes/" + d.volume + "/_data", "Destination": "/data", "RW": true})
	switch d.wrong {
	case "volume":
		mounts[5]["Name"] = "existing-populated-source"
	case "source":
		mounts[5]["Source"] = "/retained-source"
	case "duplicate":
		mounts[4] = mounts[3]
	case "omitted":
		mounts = mounts[:5]
	}
	raw := map[string]any{"Id": strings.Repeat("a", 64), "HostConfig": map[string]any{"ReadonlyRootfs": true, "NetworkMode": d.network, "CapDrop": []string{"ALL"}, "SecurityOpt": []string{"no-new-privileges"}}, "NetworkSettings": map[string]any{"Networks": map[string]any{d.network: map[string]any{"IPAddress": "172.24.0.2", "NetworkID": strings.Repeat("b", 64)}}}, "Mounts": mounts, "Config": map[string]any{"Image": natsImage, "Hostname": d.owner + "-hub"}, "Image": "sha256:test"}
	return json.Marshal(raw)
}
func storageFixture(t *testing.T, wrong string) (*sandbox, *regressionDocker) {
	t.Helper()
	work := t.TempDir()
	d := &regressionDocker{work: work, helper: filepath.Join(work, "helper"), network: "owned", volume: "owned-data", owner: "owned", wrong: wrong}
	return &sandbox{docker: d, owner: d.owner, network: d.network, networkID: strings.Repeat("b", 64), volume: d.volume, work: work, helper: d.helper}, d
}
func TestActualOwnedLocalVolumeAndExactHubHolderAccepted(t *testing.T) {
	s, _ := storageFixture(t, "")
	if e := s.launch("owned-hub", "voice-nats", "hub.conf", ""); e != nil {
		t.Fatalf("valid inspected owned holder rejected: %v", e)
	}
}
func TestActualForeignVolumeAndDuplicateMountRejectBeforeSetup(t *testing.T) {
	for _, wrong := range []string{"volume", "owner", "missing-owner", "driver", "bind-options", "source", "duplicate", "omitted"} {
		t.Run(wrong, func(t *testing.T) {
			s, d := storageFixture(t, wrong)
			if e := s.launch("owned-hub", "voice-nats", "hub.conf", ""); e == nil {
				t.Fatal("unsafe actual storage/mount identity accepted")
			}
			for _, args := range d.args {
				if args[0] == "start" {
					t.Fatal("unowned mount/volume broker started before inspection")
				}
			}
		})
	}
}
func TestHubNeverReceivesWholeCredentialDirectory(t *testing.T) {
	s, d := storageFixture(t, "")
	if e := s.launch("owned-hub", "voice-nats", "hub.conf", ""); e != nil {
		t.Fatal(e)
	}
	found := false
	for _, args := range d.args {
		if args[0] != "create" {
			continue
		}
		found = true
		line := strings.Join(args, " ")
		for _, forbidden := range []string{"src=" + s.work + ",dst=/etc/nats,readonly", "dst=/proof,", "dst=/run/voice-populated", "dst=/var/run/nats/"} {
			if strings.Contains(line, forbidden) {
				t.Fatal("hub received diagnostic credentials/helper/context")
			}
		}
	}
	if !found {
		t.Fatal("no inspected hub created")
	}
}
