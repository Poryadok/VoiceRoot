//go:build linux

package sourcecustody

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestOwnedCgroupPostgresBeforeCommit is an opt-in disposable-VM experiment.
// It does not test a startup race, a sent COMMIT, restart denial, or fencing.
// The VM must supply a private cgroup root and an owned, disposable filesystem.
func TestOwnedCgroupPostgresBeforeCommit(t *testing.T) {
	if os.Getenv("VOICE_DISPOSABLE_VM_POSTGRES_TEST") != "1" {
		t.Skip("requires the isolated offline PostgreSQL VM fixture")
	}
	const rootPath = "/sys/fs/cgroup/voice-fixture"
	const parentDir = "/mnt/voice-pg"
	const postgres = "/usr/libexec/postgresql17/postgres"
	const initdb = "/usr/libexec/postgresql17/initdb"
	const psql = "/usr/libexec/postgresql17/psql"
	const port = "55432"
	if os.Geteuid() != 0 {
		t.Fatal("fixture supervisor must run as guest root")
	}
	account, err := user.Lookup("pgfixture")
	if err != nil {
		t.Fatal(err)
	}
	uid, err := strconv.ParseUint(account.Uid, 10, 32)
	if err != nil || uid == 0 {
		t.Fatal("invalid fixture uid")
	}
	gid, err := strconv.ParseUint(account.Gid, 10, 32)
	if err != nil || gid == 0 {
		t.Fatal("invalid fixture gid")
	}
	credential := &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	root, err := os.Open(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	a, err := newOwner(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.stop(ctx); err != nil {
			t.Errorf("fixture cgroup cleanup: %v", err)
		}
		a.close()
	}()
	dataRoot, err := os.MkdirTemp(parentDir, "owned-postgres-")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chown(dataRoot, int(uid), int(gid)); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(dataRoot, "data")
	sock := filepath.Join(dataRoot, "sock")
	if err = os.Mkdir(sock, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Chown(sock, int(uid), int(gid)); err != nil {
		t.Fatal(err)
	}
	cleanEnv := []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "HOME=/tmp", "LC_ALL=C", "PAGER=cat", "PSQL_PAGER=cat"}
	initCtx, initCancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer initCancel()
	init := exec.CommandContext(initCtx, initdb, "-D", data, "-U", "pgfixture", "--auth=trust", "--no-instructions")
	init.WaitDelay = 3 * time.Second
	init.Env = cleanEnv
	init.SysProcAttr = &syscall.SysProcAttr{UseCgroupFD: true, CgroupFD: a.child, Credential: credential}
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("fixture initdb: %v: %s", err, out)
	}
	sentinelCtx, sentinelCancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer sentinelCancel()
	sentinel := exec.CommandContext(sentinelCtx, "/proc/self/exe", "-test.run=^TestCgroupSleeper$")
	sentinel.Env = append(append([]string{}, cleanEnv...), "VOICE_TEST_CGROUP_SLEEPER=1")
	sentinelIn, err := sentinel.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	sentinelOut, err := sentinel.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = sentinel.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sentinel.Process.Kill(); _ = sentinel.Wait() })
	sentinelReader := bufio.NewReader(sentinelOut)
	readSentinel := func(want string) {
		result := make(chan string, 1)
		go func() { line, _ := sentinelReader.ReadString('\n'); result <- line }()
		select {
		case line := <-result:
			if line != want {
				t.Fatalf("sentinel response %q, want %q", line, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("sentinel response timeout")
		}
	}
	readSentinel("ready\n")
	start := func(o *owner) (<-chan error, *exec.Cmd) {
		cmd := exec.Command(postgres, "-D", data, "-c", "listen_addresses=", "-k", sock, "-p", port)
		cmd.Env = cleanEnv
		cmd.SysProcAttr = &syscall.SysProcAttr{UseCgroupFD: true, CgroupFD: o.child, Credential: credential}
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatalf("clone into owned cgroup: %v", err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		t.Cleanup(func() {
			if cmd.Process.Kill() == nil {
				select {
				case <-done:
				case <-time.After(5 * time.Second):
				}
			}
		})
		return done, cmd
	}
	query := func(ctx context.Context, text, app string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, psql, "-X", "-q", "-P", "pager=off", "-h", sock, "-p", port, "-U", "pgfixture", "-d", "postgres", "-At", "-v", "ON_ERROR_STOP=1", "-c", text)
		cmd.WaitDelay = 3 * time.Second
		cmd.Env = append(append([]string{}, cleanEnv...), "PGAPPNAME="+app)
		return cmd
	}
	runQuery := func(text, app string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return query(ctx, text, app).CombinedOutput()
	}
	ready := func(done <-chan error) {
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			select {
			case err := <-done:
				t.Fatalf("postmaster exited before readiness: %v", err)
			default:
			}
			if out, err := runQuery("SELECT 1", "voice_fixture_ready"); err == nil && strings.TrimSpace(string(out)) == "1" {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("postmaster readiness timeout")
	}
	done, server := start(a)
	ready(done)
	procs, err := a.control("cgroup.procs", false, "")
	if err != nil || !strings.Contains("\n"+procs, "\n"+strconv.Itoa(server.Process.Pid)+"\n") {
		t.Fatalf("postmaster not observed in owned cgroup: %v", err)
	}
	if out, err := runQuery("CREATE TABLE voice_cgroup_probe(id integer NOT NULL)", "voice_fixture_setup"); err != nil {
		t.Fatalf("create fixture table: %v: %s", err, out)
	}
	txCtx, txCancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer txCancel()
	tx := query(txCtx, "BEGIN; INSERT INTO voice_cgroup_probe VALUES (42); SELECT pg_sleep(120)", "voice_cgroup_tx_test")
	if err = tx.Start(); err != nil {
		t.Fatal(err)
	}
	txDone := make(chan error, 1)
	go func() { txDone <- tx.Wait() }()
	t.Cleanup(func() {
		if tx.Process.Kill() == nil {
			select {
			case <-txDone:
			case <-time.After(5 * time.Second):
			}
		}
	})
	deadline := time.Now().Add(30 * time.Second)
	confirmed := false
	for time.Now().Before(deadline) {
		select {
		case err := <-txDone:
			t.Fatalf("transaction exited before stop: %v", err)
		default:
		}
		out, err := runQuery("SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE application_name='voice_cgroup_tx_test' AND wait_event='PgSleep')", "voice_fixture_probe")
		if err == nil && strings.TrimSpace(string(out)) == "t" {
			confirmed = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !confirmed {
		t.Fatal("uncommitted transaction did not reach pg_sleep")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = a.stop(ctx); err != nil {
		t.Fatalf("stop owned cgroup: %v", err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("postmaster unexpectedly exited successfully")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("postmaster did not exit")
	}
	select {
	case err := <-txDone:
		if err == nil {
			t.Fatal("transaction client unexpectedly succeeded")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("transaction client did not fail")
	}
	if _, err = io.WriteString(sentinelIn, "check\n"); err != nil {
		t.Fatalf("sentinel request: %v", err)
	}
	readSentinel("alive\n")
	b, err := newOwner(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := b.stop(ctx); err != nil {
			t.Errorf("fixture recovery cgroup cleanup: %v", err)
		}
		b.close()
	}()
	recoveryDone, recovery := start(b)
	ready(recoveryDone)
	out, err := runQuery("SELECT count(*) FROM voice_cgroup_probe", "voice_fixture_recovery")
	if err != nil || strings.TrimSpace(string(out)) != "0" {
		t.Fatalf("uncommitted row survived: %q, %v", out, err)
	}
	if err = recovery.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-recoveryDone:
		if err != nil {
			t.Fatalf("recovery postmaster shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("recovery shutdown timeout")
	}
	t.Logf("pre-COMMIT row rolled back; unrelated sentinel %d survived", sentinel.Process.Pid)
}
