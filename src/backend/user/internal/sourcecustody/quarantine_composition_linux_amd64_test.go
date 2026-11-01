//go:build linux && amd64

package sourcecustody

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sys/unix"
)

// This is solely a disposable-VM experiment, not a source launcher. In
// particular an in-memory consume result is not rollback-resistant authority.
// The only candidate is the currently running, self-built test executable.
func compositionSelf(t *testing.T) (*os.File, [32]byte) {
	t.Helper()
	self, err := os.Open("/proc/self/exe")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = self.Close() })
	data, err := io.ReadAll(io.LimitReader(self, sealedELFMaxBytes+1))
	if err != nil || len(data) > sealedELFMaxBytes {
		t.Fatal("cannot read self fixture")
	}
	digest := sha256.Sum256(data)
	var pin unix.Stat_t
	if unix.Fstat(int(self.Fd()), &pin) != nil {
		t.Fatal("cannot pin self fixture")
	}
	sealed, err := prepareSealedStaticELF(self, digest, pin)
	if err != nil {
		t.Fatalf("seal self fixture: %v", err)
	}
	t.Cleanup(func() { _ = sealed.Close() })
	return sealed, digest
}

// startCompositionFixture invokes Start only directly after this invocation's
// successful consume. Its start counter observes attempts, not payload output.
// The caller keeps the sealed FD and owns cleanup of the fresh cgroup.
type compositionReport struct {
	cleanupErr error
	goSent     bool
}

func startCompositionFixture(ctx context.Context, client *quarantineClient, grant quarantineGrant, sealed *os.File, owned *owner, beforeStart func(), afterReady func(), starts *atomic.Int32, report *compositionReport) (resultErr error) {
	if ctx == nil || ctx.Err() != nil || client.prepare(ctx, grant) != nil {
		return errUnavailable
	}
	if beforeStart != nil {
		beforeStart()
	}
	if ctx.Err() != nil || grant.challenge.expires <= time.Now().Unix() {
		return errUnavailable
	}
	cmd := exec.Command("/proc/self/exe", "-test.run=^TestQuarantineCompositionBootstrap$")
	cmd.Env = []string{"VOICE_COMPOSITION_BOOTSTRAP=1"}
	cmd.ExtraFiles = []*os.File{sealed}
	cmd.SysProcAttr = &syscall.SysProcAttr{UseCgroupFD: true, CgroupFD: owned.child}
	cmd.WaitDelay = 3 * time.Second
	childIn, in, err := os.Pipe()
	if err != nil {
		return errUnavailable
	}
	defer childIn.Close()
	defer in.Close()
	out, childOut, err := os.Pipe()
	if err != nil {
		return errUnavailable
	}
	defer out.Close()
	defer childOut.Close()
	cmd.Stdin = childIn
	cmd.Stdout = childOut
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	// This is the launch linearization point. Cancellation after the check can
	// race Start; only owned-subtree cleanup, not zero execution, is promised.
	if ctx.Err() != nil || grant.challenge.expires <= time.Now().Unix() {
		return errUnavailable
	}
	starts.Add(1)
	if cmd.Start() != nil {
		return errUnavailable
	}
	_ = childIn.Close()
	_ = childOut.Close()
	var finished chan error
	startWait := func() {
		if finished == nil {
			finished = make(chan error, 1)
			go func() { finished <- cmd.Wait() }()
		}
	}
	waited := false
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if owned.stop(cleanup) != nil {
			report.cleanupErr = errUnavailable
		}
		if !waited {
			startWait()
			select {
			case err := <-finished:
				if err != nil && resultErr == nil {
					resultErr = errUnavailable
				}
			case <-time.After(3 * time.Second):
				report.cleanupErr = errUnavailable
			}
		}
	}()
	reader := bufio.NewReader(out)
	read := func() (string, error) {
		ch := make(chan string, 1)
		go func() { s, _ := reader.ReadString('\n'); ch <- s }()
		select {
		case s := <-ch:
			return s, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	line, err := read()
	if err != nil || line != "BOOTSTRAP_READY\n" {
		return errUnavailable
	}
	if afterReady != nil {
		afterReady()
	}
	if ctx.Err() != nil || grant.challenge.expires <= time.Now().Unix() {
		return errUnavailable
	}
	if _, err := io.WriteString(in, "go\n"); err != nil {
		return errUnavailable
	}
	report.goSent = true
	line, err = read()
	if err != nil || line != "FD_EXEC_PAYLOAD\n" {
		return errUnavailable
	}
	startWait()
	select {
	case err := <-finished:
		waited = true
		if err != nil {
			return errUnavailable
		}
	case <-ctx.Done():
		return errUnavailable
	}
	return nil
}

// The bootstrap accepts only its own immutable sealed ELF and has no path or
// shell fallback. The go handshake is fixture control, not launch authority.
func TestQuarantineCompositionBootstrap(t *testing.T) {
	if os.Getenv("VOICE_COMPOSITION_BOOTSTRAP") != "1" {
		return
	}
	f := os.NewFile(3, "fixture")
	if f == nil {
		t.Fatal("missing fixture")
	}
	seals, err := unix.FcntlInt(f.Fd(), unix.F_GET_SEALS, 0)
	if err != nil || seals&sealedELFRequiredSeals != sealedELFRequiredSeals {
		t.Fatal("unsealed fixture")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(io.LimitReader(f, sealedELFMaxBytes+1))
	if err != nil || len(data) > sealedELFMaxBytes {
		t.Fatal("fixture read")
	}
	self, err := os.ReadFile("/proc/self/exe")
	if err != nil || sha256.Sum256(self) != sha256.Sum256(data) {
		t.Fatal("fixture differs from self")
	}
	fmt.Println("BOOTSTRAP_READY")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil || line != "go\n" {
		t.Fatal("bad fixture handshake")
	}
	argv, err := syscall.SlicePtrFromStrings([]string{"voice-static-fixture", "-test.run=^TestFDExecStaticFixturePayload$"})
	if err != nil {
		t.Fatal(err)
	}
	env, err := syscall.SlicePtrFromStrings([]string{"VOICE_FD_FIXTURE_PAYLOAD=1"})
	if err != nil {
		t.Fatal(err)
	}
	empty := []byte{0}
	_, _, errno := unix.Syscall6(unix.SYS_EXECVEAT, f.Fd(), uintptr(unsafe.Pointer(&empty[0])), uintptr(unsafe.Pointer(&argv[0])), uintptr(unsafe.Pointer(&env[0])), uintptr(unix.AT_EMPTY_PATH), 0)
	runtime.KeepAlive(argv)
	runtime.KeepAlive(env)
	runtime.KeepAlive(empty)
	runtime.KeepAlive(f)
	t.Fatalf("execveat: %v", errno)
}

func TestQuarantineCompositionDisposableVM(t *testing.T) {
	if os.Getenv("VOICE_DISPOSABLE_VM_QUARANTINE_COMPOSITION") != "1" {
		t.Skip("requires exclusively owned disposable VM and ledger")
	}
	dsn := os.Getenv("VOICE_TEST_QUARANTINE_DSN")
	if dsn == "" {
		t.Fatal("missing disposable ledger")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var database string
	if pool.QueryRow(ctx, "SELECT current_database()").Scan(&database) != nil || database != "voice_quarantine_test" {
		t.Fatal("wrong disposable database")
	}
	_, err = pool.Exec(ctx, `CREATE TABLE public.quarantine_generation_ledger (source bytea PRIMARY KEY, generation bigint NOT NULL, challenge bytea NOT NULL, expires bigint NOT NULL, state text NOT NULL, grant_hash bytea)`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = pool.Exec(cleanup, `DROP TABLE public.quarantine_generation_ledger`)
	})
	root, err := os.Open("/sys/fs/cgroup/voice-fixture")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	sealed, digest := compositionSelf(t)
	for _, mode := range []string{"success", "invalid", "lost", "pre-canceled", "expired-handoff", "cancel-after-start"} {
		t.Run(mode, func(t *testing.T) {
			owned, err := newOwner(root)
			if err != nil {
				t.Fatal(err)
			}
			var report compositionReport
			defer func() {
				if report.cleanupErr == nil {
					owned.close()
				}
			}()
			pub, priv, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			issuerPub, issuerPriv, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			var scope quarantineScope
			if _, err := rand.Read(scope.source[:]); err != nil {
				t.Fatal(err)
			}
			scope.generation, scope.host, scope.executable = 1, sha256.Sum256(pub), digest
			scope.boot[0], scope.runtime[0], scope.data[0] = 1, 1, 1
			a, err := newQuarantineAuthority(pool, scope, pub, issuerPriv)
			if err != nil {
				t.Fatal(err)
			}
			ch, err := a.begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			g, err := a.issue(ctx, ch, ed25519.Sign(priv, challengeProofBytes(ch)))
			if err != nil {
				t.Fatal(err)
			}
			if mode == "invalid" {
				g.signature[0] ^= 1
			}
			var transport quarantineTransport = a
			if mode == "lost" {
				transport = lostConsumeResponse{a}
			}
			client := newQuarantineClient(scope, issuerPub, quarantineTestSigner{priv}, transport)
			call, stop := context.WithCancel(ctx)
			defer stop()
			if mode == "pre-canceled" {
				stop()
			}
			var before, after func()
			if mode == "expired-handoff" {
				before = func() { time.Sleep(time.Until(time.Unix(g.challenge.expires, 0)) + 10*time.Millisecond) }
			}
			if mode == "cancel-after-start" {
				after = stop
			}
			var starts atomic.Int32
			err = startCompositionFixture(call, client, g, sealed, owned, before, after, &starts, &report)
			if report.cleanupErr != nil {
				t.Fatal("owned subtree cleanup or child wait was not established")
			}
			var state string
			if pool.QueryRow(ctx, `SELECT state FROM public.quarantine_generation_ledger WHERE source=$1`, scope.source[:]).Scan(&state) != nil {
				t.Fatal("cannot inspect fixture ledger")
			}
			if mode == "success" || mode == "lost" || mode == "expired-handoff" || mode == "cancel-after-start" {
				if state != "CONSUMED" {
					t.Fatalf("unexpected ledger state: %s", state)
				}
			} else if state != "ISSUED" {
				t.Fatalf("denied grant changed ledger: %s", state)
			}
			if mode == "success" {
				if err != nil || starts.Load() != 1 {
					t.Fatalf("successful composition: starts=%d err=%v", starts.Load(), err)
				}
			} else if mode == "cancel-after-start" {
				if !errors.Is(err, errUnavailable) || starts.Load() != 1 || report.goSent {
					t.Fatalf("post-start cancellation: starts=%d err=%v", starts.Load(), err)
				}
			} else if !errors.Is(err, errUnavailable) || starts.Load() != 0 {
				t.Fatalf("denied composition: starts=%d err=%v", starts.Load(), err)
			}
		})
	}
}
