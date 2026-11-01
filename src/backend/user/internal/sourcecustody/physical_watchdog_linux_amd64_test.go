//go:build linux && amd64

package sourcecustody

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// These tests inject every device operation. They never open a watchdog.
type fakeWatchdogOps struct {
	mu                       sync.Mutex
	calls                    []string
	path                     string
	flags                    int
	fd                       int
	stat                     watchdogStat
	info                     unix.WatchdogInfo
	timeout                  int
	fail                     string
	entered, release         chan struct{}
	openEntered, openRelease chan struct{}
}

func (f *fakeWatchdogOps) record(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, s)
}
func (f *fakeWatchdogOps) open(path string, flags int) (int, error) {
	f.record("open")
	f.path = path
	f.flags = flags
	if f.openEntered != nil {
		close(f.openEntered)
		<-f.openRelease
	}
	if f.fail == "open-with-fd" {
		return f.fd, errors.New("uncertain open")
	}
	if f.fail == "open" {
		return -1, errors.New("injected")
	}
	return f.fd, nil
}
func (f *fakeWatchdogOps) statFD(fd int) (watchdogStat, error) {
	f.record("stat")
	if fd != f.fd || f.fail == "stat" {
		return watchdogStat{}, errors.New("injected")
	}
	return f.stat, nil
}
func (f *fakeWatchdogOps) support(fd int) (unix.WatchdogInfo, error) {
	f.record("support")
	if fd != f.fd || f.fail == "support" {
		return unix.WatchdogInfo{}, errors.New("injected")
	}
	return f.info, nil
}
func (f *fakeWatchdogOps) getTimeout(fd int) (int, error) {
	f.record("timeout")
	if f.entered != nil {
		close(f.entered)
		<-f.release
	}
	if fd != f.fd || f.fail == "timeout" {
		return 0, errors.New("injected")
	}
	return f.timeout, nil
}

func watchdogFixture() (*physicalWatchdog, *fakeWatchdogOps) {
	var identity [32]byte
	copy(identity[:], "fixture-driver")
	p := watchdogPolicy{rdev: unix.Mkdev(10, 130), identity: identity, maxSeconds: 15}
	f := &fakeWatchdogOps{fd: 117, stat: watchdogStat{mode: unix.S_IFCHR | 0600, rdev: p.rdev}, timeout: 10}
	copy(f.info.Identity[:], identity[:])
	return newWatchdogWithOps(p, f, time.Second), f
}

func runBlockedWatchdog(t *testing.T, w *physicalWatchdog, ctx context.Context, entered <-chan struct{}, release chan struct{}) (<-chan error, func()) {
	t.Helper()
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	result, done := make(chan error, 1), make(chan struct{})
	go func() { defer close(done); _, err := w.observe(ctx); result <- err }()
	t.Cleanup(func() {
		unblock()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("watchdog test worker did not stop")
		}
	})
	select {
	case <-entered:
	case <-done:
		t.Fatalf("worker returned before blocked operation: %v", <-result)
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not reach blocked operation")
	}
	return result, unblock
}

func watchdogResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("watchdog worker did not return")
		return nil
	}
}

func TestWatchdogInjectedObservationOneShot(t *testing.T) {
	w, f := watchdogFixture()
	observation, err := w.observe(context.Background())
	if err != nil || observation.timeoutSeconds != 10 || observation.rdev != f.stat.rdev {
		t.Fatalf("observation: %+v %v", observation, err)
	}
	if f.path != "/dev/watchdog0" || f.flags != unix.O_WRONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK {
		t.Fatalf("open path/flags: %s %x", f.path, f.flags)
	}
	if len(f.calls) != 4 || f.calls[0] != "open" || f.calls[1] != "stat" || f.calls[2] != "support" || f.calls[3] != "timeout" {
		t.Fatalf("calls: %v", f.calls)
	}
	if _, err = w.observe(context.Background()); err == nil || len(f.calls) != 4 {
		t.Fatal("second device open accepted")
	}
	w.revoke()
	if w.retainedFD.Load() != int64(f.fd)+1 {
		t.Fatal("descriptor was not retained")
	}
}

func TestWatchdogInjectedFailuresBurnAttempt(t *testing.T) {
	for _, kind := range []string{"open", "open-with-fd", "stat", "wrong-mode", "wrong-rdev", "support", "wrong-identity", "malformed-identity", "timeout", "zero-timeout", "over-limit"} {
		t.Run(kind, func(t *testing.T) {
			w, f := watchdogFixture()
			switch kind {
			case "open", "open-with-fd", "stat", "support", "timeout":
				f.fail = kind
			case "wrong-mode":
				f.stat.mode = unix.S_IFREG | 0600
			case "wrong-rdev":
				f.stat.rdev++
			case "wrong-identity":
				f.info.Identity[0] ^= 1
			case "malformed-identity":
				f.info.Identity[31] = 1
			case "zero-timeout":
				f.timeout = 0
			case "over-limit":
				f.timeout = 16
			}
			if _, err := w.observe(context.Background()); err == nil {
				t.Fatal("invalid observation accepted")
			}
			calls := len(f.calls)
			if _, err := w.observe(context.Background()); err == nil || len(f.calls) != calls {
				t.Fatal("failed attempt retried")
			}
			if kind != "open" && w.retainedFD.Load() != int64(f.fd)+1 {
				t.Fatal("post-open FD not retained")
			}
		})
	}
}

func TestWatchdogCanceledBeforeOpenBurnsAttempt(t *testing.T) {
	w, f := watchdogFixture()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := w.observe(ctx); err == nil {
		t.Fatal("canceled attempt accepted")
	}
	if _, err := w.observe(context.Background()); err == nil || len(f.calls) != 0 {
		t.Fatal("canceled attempt reopened device")
	}
}

func TestWatchdogRevokeDuringBlockedReadback(t *testing.T) {
	w, f := watchdogFixture()
	f.entered = make(chan struct{})
	f.release = make(chan struct{})
	result, unblock := runBlockedWatchdog(t, w, context.Background(), f.entered, f.release)
	w.revoke()
	unblock()
	if err := watchdogResult(t, result); err == nil {
		t.Fatal("late response revived revoked owner")
	}
	if _, err := w.observe(context.Background()); err == nil {
		t.Fatal("revoked owner reused")
	}
	if w.retainedFD.Load() != int64(f.fd)+1 {
		t.Fatal("late FD not retained")
	}
}

func TestWatchdogRevokeDuringBlockedOpen(t *testing.T) {
	w, f := watchdogFixture()
	f.openEntered, f.openRelease = make(chan struct{}), make(chan struct{})
	result, unblock := runBlockedWatchdog(t, w, context.Background(), f.openEntered, f.openRelease)
	w.revoke()
	unblock()
	if err := watchdogResult(t, result); err == nil {
		t.Fatal("late open revived revoked owner")
	}
	if len(f.calls) != 1 || f.calls[0] != "open" || w.retainedFD.Load() != int64(f.fd)+1 {
		t.Fatalf("late open not safely retained: %v", f.calls)
	}
}

func TestWatchdogProcessWideClaim(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWatchdogClaimHelper$")
	cmd.WaitDelay = time.Second
	cmd.Env = append(os.Environ(), "VOICE_WATCHDOG_CLAIM_HELPER=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("claim helper: %v: %s", err, output)
	}
}

func TestWatchdogClaimHelper(t *testing.T) {
	if os.Getenv("VOICE_WATCHDOG_CLAIM_HELPER") != "1" {
		return
	}
	fixture, _ := watchdogFixture()
	if _, err := newPhysicalWatchdog(fixture.policy); err != nil {
		t.Fatal(err)
	}
	if _, err := newPhysicalWatchdog(fixture.policy); err == nil {
		t.Fatal("second process-local claim accepted")
	}
	// Neither object is observed; no real device syscall occurs in this helper.
}

func TestWatchdogCancellationDuringBlockedReadback(t *testing.T) {
	w, f := watchdogFixture()
	f.entered = make(chan struct{})
	f.release = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, unblock := runBlockedWatchdog(t, w, ctx, f.entered, f.release)
	cancel()
	unblock()
	if err := watchdogResult(t, result); err == nil {
		t.Fatal("canceled readback accepted")
	}
	if _, err := w.observe(context.Background()); err == nil {
		t.Fatal("canceled owner reused")
	}
	if w.retainedFD.Load() != int64(f.fd)+1 {
		t.Fatal("descriptor not retained")
	}
}

func TestWatchdogDeadlineExpiresBeforeReadbackReturns(t *testing.T) {
	w, f := watchdogFixture()
	w.wait = 30 * time.Second
	var clockMu sync.Mutex
	now := time.Now()
	w.now = func() time.Time { clockMu.Lock(); defer clockMu.Unlock(); return now }
	f.entered = make(chan struct{})
	f.release = make(chan struct{})
	result, unblock := runBlockedWatchdog(t, w, context.Background(), f.entered, f.release)
	clockMu.Lock()
	now = now.Add(30 * time.Second)
	clockMu.Unlock()
	unblock()
	if err := watchdogResult(t, result); err == nil {
		t.Fatal("expired readback accepted")
	}
	if w.retainedFD.Load() != int64(f.fd)+1 {
		t.Fatal("descriptor not retained")
	}
}

func TestWatchdogConcurrentAttemptsOnlyOneOpen(t *testing.T) {
	w, f := watchdogFixture()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := w.observe(context.Background()); results <- err }()
	}
	wg.Wait()
	close(results)
	wins := 0
	for err := range results {
		if err == nil {
			wins++
		}
	}
	if wins != 1 || len(f.calls) != 4 {
		t.Fatalf("wins=%d calls=%v", wins, f.calls)
	}
}
