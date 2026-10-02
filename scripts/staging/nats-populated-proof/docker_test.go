//go:build dockerproof

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nats-io/nats.go"
)

// Run explicitly with -tags=dockerproof. This is a mandatory independent
// integration target, not a skipped branch of the ordinary unit suite.
func TestDockerPopulatedProof(t *testing.T) {
	b := newBundle(t)
	b.m.Kind = "fixture"
	b.save(t)
	input, e := loadInput(b.dir, b.acl, "", "", true, b.now)
	if e != nil {
		t.Fatal(e)
	}
	helper := filepath.Join(t.TempDir(), "proof-linux")
	cmd := exec.Command("go", "build", "-o", helper, ".")
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	if e = cmd.Run(); e != nil {
		t.Fatal("isolated helper build failed")
	}
	if e = os.Chmod(helper, 0700); e != nil {
		t.Fatal("fixture helper mode failed")
	}
	// The source is a separate populated broker with independent fixture keys,
	// payloads and ACK floors. It is never passed to the clone orchestrator.
	source := newBundle(t)
	sourceHub := testHub(t, source, filepath.Join(t.TempDir(), "retained-source"))
	if e = setupClone(sourceHub.ClientURL(), source.dir); e != nil {
		t.Fatal(e)
	}
	if e = exerciseClone(sourceHub.ClientURL(), source.dir); e != nil {
		t.Fatal(e)
	}
	admin := testCredential(t, source.account, grant{Publish: []string{">"}, Subscribe: []string{">"}}, nil)
	adminPath := filepath.Join(t.TempDir(), "source-only.creds")
	writeTest(t, adminPath, admin)
	nc, e := nats.Connect(sourceHub.ClientURL(), nats.UserCredentials(adminPath))
	if e != nil {
		t.Fatal("source fixture connect failed")
	}
	defer nc.Close()
	js, e := nc.JetStream()
	if e != nil {
		t.Fatal("source fixture JetStream failed")
	}
	before := sourceSnapshot(t, js)
	for _, fault := range []string{"", "exercise", "cleanup"} {
		t.Run("source-preserved-"+fault, func(t *testing.T) {
			d := &faultDocker{fault: fault, t: t}
			e := runSandbox(input, d, helper)
			if fault == "" && e != nil {
				t.Fatalf("fixture Docker proof: %v", e)
			}
			if fault != "" && e == nil {
				t.Fatal("injected failure produced successful evidence")
			}
			if fault == "cleanup" && e.Error() != "sandbox_cleanup_failed" {
				t.Fatalf("cleanup must veto success: %v", e)
			}
			if after := sourceSnapshot(t, js); after != before {
				t.Fatal("source stream/message/config/consumer ACK state changed")
			}
		})
	}
}

type faultDocker struct {
	fault string
	t     *testing.T
}

func (d *faultDocker) run(ctx context.Context, args ...string) ([]byte, error) {
	if d.fault == "exercise" && len(args) > 1 && args[0] == "exec" && args[len(args)-1] == "exercise" {
		return nil, failure("fixture_exercise_fault")
	}
	raw, e := (realDocker{}).run(ctx, args...)
	// Fixture-only diagnostics expose classification, never log bytes or credentials.
	if e == nil && len(args) > 1 && args[0] == "start" {
		state, stateErr := exec.CommandContext(ctx, "docker", "inspect", args[1], "--format", "{{json .State}}").Output()
		var status struct {
			Running  bool
			ExitCode int
		}
		if stateErr == nil && json.Unmarshal(state, &status) == nil && !status.Running {
			logs, _ := exec.CommandContext(ctx, "docker", "logs", args[1]).CombinedOutput()
			d.t.Logf("fixture stopped: exit=%d permissionDenied=%t", status.ExitCode, strings.Contains(string(logs), "permission denied"))
		}
	}
	// Remove first, then simulate an ambiguous cleanup result. Inspection still
	// runs, all owned resources are removed, and uncertainty vetoes evidence.
	if d.fault == "cleanup" && len(args) > 1 && args[0] == "network" && args[1] == "rm" {
		return nil, failure("fixture_cleanup_fault")
	}
	return raw, e
}
func sourceSnapshot(t *testing.T, js nats.JetStreamContext) string {
	t.Helper()
	si, e := js.StreamInfo("social_events")
	if e != nil {
		t.Fatal("source stream inspection failed")
	}
	state := struct {
		Config    nats.StreamConfig
		State     nats.StreamState
		Payloads  []string
		Consumers []struct {
			Config              nats.ConsumerConfig
			Delivered, AckFloor nats.SequenceInfo
			Pending             int
			Remaining           uint64
		}
	}{Config: si.Config, State: si.State}
	for seq := uint64(1); seq <= si.State.LastSeq; seq++ {
		msg, e := js.GetMsg("social_events", seq)
		if e != nil {
			t.Fatal("source payload inspection failed")
		}
		state.Payloads = append(state.Payloads, hash(msg.Data))
	}
	for _, name := range []string{"rt_realtime1_social", "rt_realtime1_friend_request", "rt_realtime1_friend_removed"} {
		ci, e := js.ConsumerInfo("social_events", name)
		if e != nil {
			t.Fatal("source consumer inspection failed")
		}
		state.Consumers = append(state.Consumers, struct {
			Config              nats.ConsumerConfig
			Delivered, AckFloor nats.SequenceInfo
			Pending             int
			Remaining           uint64
		}{ci.Config, ci.Delivered, ci.AckFloor, ci.NumAckPending, ci.NumPending})
	}
	return hash(must(json.Marshal(state)))
}
