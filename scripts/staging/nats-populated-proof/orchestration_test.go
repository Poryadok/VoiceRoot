package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type cleanupDocker struct {
	commands        [][]string
	fault, leftover string
}

func (d *cleanupDocker) run(_ context.Context, args ...string) ([]byte, error) {
	d.commands = append(d.commands, append([]string{}, args...))
	joined := strings.Join(args, " ")
	if d.fault != "" && strings.Contains(joined, d.fault) {
		return nil, errors.New("fixture fault")
	}
	if d.leftover != "" && strings.HasPrefix(joined, d.leftover+" ls") {
		return []byte("owned-resource-remains"), nil
	}
	return nil, nil
}
func TestCleanupFailureVetoesEvidenceAndAttemptsEveryOwnedResource(t *testing.T) {
	for _, fault := range []string{"rm -f", "volume rm", "network rm", "container ls", "volume ls", "network ls"} {
		t.Run(fault, func(t *testing.T) {
			d := &cleanupDocker{fault: fault}
			s := &sandbox{docker: d, owner: "owned", network: "owned", volume: "owned-data", containers: []string{"owned-hub", "owned-leaf"}, work: filepath.Join(t.TempDir(), "owned-copy")}
			if os.Mkdir(s.work, 0700) != nil {
				t.Fatal("fixture failed")
			}
			writeTest(t, filepath.Join(s.work, "opaque-copy"), []byte("disposable"))
			if e := s.cleanup(); e == nil || e.Error() != "sandbox_cleanup_failed" {
				t.Fatalf("cleanup fault must veto success: %v", e)
			}
			if len(d.commands) != 7 {
				t.Fatalf("must attempt all removals and inspect all classes: %d", len(d.commands))
			}
			if _, e := os.Stat(s.work); !os.IsNotExist(e) {
				t.Fatal("owned opaque copies retained after fault")
			}
		})
	}
}
func TestLeftoversAndUnavailableInventoryCannotProveCleanup(t *testing.T) {
	for _, kind := range []string{"container", "volume", "network"} {
		t.Run(kind, func(t *testing.T) {
			d := &cleanupDocker{leftover: kind}
			s := &sandbox{docker: d, owner: "owned", network: "owned", volume: "owned-data"}
			if e := s.cleanup(); e == nil {
				t.Fatal("leftover resource accepted")
			}
		})
	}
}
func TestCleanupPreservesUnownedPopulatedStorage(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source-retained")
	owned := filepath.Join(root, "owned-copy")
	if os.Mkdir(source, 0700) != nil || os.Mkdir(owned, 0700) != nil {
		t.Fatal("fixture directories failed")
	}
	sentinel := []byte("messages/stream-config/consumer-ACK-state")
	writeTest(t, filepath.Join(source, "state"), sentinel)
	s := &sandbox{docker: &cleanupDocker{}, owner: "owned", network: "owned", volume: "owned-data", work: owned}
	if e := s.cleanup(); e != nil {
		t.Fatal(e)
	}
	if got := must(os.ReadFile(filepath.Join(source, "state"))); string(got) != string(sentinel) {
		t.Fatal("unowned populated source was changed")
	}
}
