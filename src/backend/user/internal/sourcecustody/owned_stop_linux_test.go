//go:build linux

package sourcecustody

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestStopSequenceDispatchesIndependently(t *testing.T) {
	blocked := make(chan struct{})
	started := make(chan struct{})
	stopped := make(chan struct{})
	finished := make(chan struct{})
	var attempts atomic.Int32
	s := &stopSequence{wait: 40 * time.Millisecond,
		persist: func() error { attempts.Add(1); close(started); <-blocked; close(finished); return nil },
		stop:    func(context.Context) error { close(stopped); return nil }}
	result := make(chan error, 1)
	go func() { result <- s.execute(context.Background()) }()
	<-started
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stop waited for journal")
	}
	if err := <-result; err == nil {
		t.Fatal("unresolved persistence accepted")
	}
	if err := s.execute(context.Background()); err == nil || attempts.Load() != 1 {
		t.Fatal("poisoned sequence retried")
	}
	close(blocked)
	<-finished
	if err := s.execute(context.Background()); err == nil || attempts.Load() != 1 {
		t.Fatal("late completion cleared poison")
	}
}

func TestOwnedStopInDisposableVM(t *testing.T) {
	rootPath := os.Getenv("VOICE_TEST_CGROUP_ROOT")
	if rootPath == "" {
		t.Skip("requires disposable VM delegated root")
	}
	for _, mode := range []string{"persisted", "persistence-failed", "stale-generation"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Chmod(dir, 0700); err != nil {
				t.Fatal(err)
			}
			d, err := os.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = d.Close() }()
			record := testDenyRecord()
			if mode == "stale-generation" {
				prior, err := openDenyJournal(d)
				if err != nil {
					t.Fatal(err)
				}
				if err := prior.persistDeny(record); err != nil {
					t.Fatal(err)
				}
				prior.close()
				record.generation++
			}
			root, err := os.Open(rootPath)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = root.Close() }()
			o, err := newOwnedSourceStop(d, root, record)
			if err != nil {
				t.Fatal(err)
			}
			defer o.child.close()
			defer o.journal.close()
			if mode == "persistence-failed" {
				o.journal.fail = func(stage string) error {
					if stage == "write" {
						return errors.New("injected")
					}
					return nil
				}
			}
			cmd := exec.Command("/proc/self/exe", "-test.run=^TestCgroupSleeper$")
			cmd.Env = append(os.Environ(), "VOICE_TEST_CGROUP_SLEEPER=1")
			cmd.SysProcAttr = &syscall.SysProcAttr{UseCgroupFD: true, CgroupFD: o.child.child}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = stdin.Close() }()
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
			if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "ready\n" {
				t.Fatalf("helper readiness %q %v", line, err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err = o.stop(ctx)
			if (err != nil) != (mode != "persisted") {
				t.Fatalf("stop returned %v", err)
			}
			if err = cmd.Wait(); err == nil {
				t.Fatal("owned child survived")
			}
			if mode != "persistence-failed" && o.journal.matchesDeny(testDenyRecord()) != nil {
				t.Fatal("deny not recorded")
			}
			if o.stop(ctx) == nil {
				t.Fatal("one-shot coordinator reused")
			}
		})
	}
}

func TestStopSequenceCancellationStillDispatches(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	persisted, stopped := make(chan struct{}), make(chan struct{})
	s := &stopSequence{wait: time.Second,
		persist: func() error { close(persisted); return nil },
		stop: func(ctx context.Context) error {
			if ctx.Err() != nil {
				return errors.New("stop context canceled")
			}
			close(stopped)
			return nil
		}}
	if s.execute(ctx) == nil {
		t.Fatal("canceled operation accepted")
	}
	for _, ch := range []<-chan struct{}{persisted, stopped} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("operation not dispatched")
		}
	}
}

func TestStopSequenceFailuresAndSuccess(t *testing.T) {
	for _, tc := range []struct {
		name                string
		persistErr, stopErr bool
	}{
		{"success", false, false}, {"persist failure", true, false}, {"stop failure", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stopCalled atomic.Bool
			s := &stopSequence{wait: time.Second,
				persist: func() error {
					if tc.persistErr {
						return errors.New("persist")
					}
					return nil
				},
				stop: func(context.Context) error {
					stopCalled.Store(true)
					if tc.stopErr {
						return errors.New("stop")
					}
					return nil
				}}
			err := s.execute(context.Background())
			if (err != nil) != (tc.persistErr || tc.stopErr) || !stopCalled.Load() {
				t.Fatalf("result=%v, stopped=%v", err, stopCalled.Load())
			}
			if s.execute(context.Background()) == nil {
				t.Fatal("sequence reused")
			}
		})
	}
}

func TestStopSequenceCannotAcceptExpiredDeadline(t *testing.T) {
	s := &stopSequence{wait: time.Nanosecond,
		persist: func() error { return nil },
		stop:    func(context.Context) error { return nil }}
	if err := s.execute(context.Background()); err == nil {
		t.Fatal("expired deadline accepted")
	}
}
