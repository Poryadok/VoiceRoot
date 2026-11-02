//go:build linux

package sourcecustody

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestRejectOrdinaryDirectory(t *testing.T) {
	f, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err = newOwner(f); err == nil {
		t.Fatal("ordinary filesystem accepted")
	}
}

func TestPopulatedMustBeUniqueAndValid(t *testing.T) {
	for _, input := range []string{"", "frozen 0\n", "populated 2\n", "populated 0\npopulated 1\n", "populated 0 extra\n"} {
		if _, err := parsePopulated(input); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	for input, want := range map[string]bool{"populated 0\nfrozen 0\n": false, "populated 1\nfrozen 0\n": true} {
		got, err := parsePopulated(input)
		if err != nil || got != want {
			t.Fatalf("%q: got %v, %v", input, got, err)
		}
	}
}

// This opt-in test is for a disposable VM and an independently delegated root.
// It creates and signals only its own children; it is not fence qualification.
func TestOwnedCgroupInDisposableVM(t *testing.T) {
	root := os.Getenv("VOICE_TEST_CGROUP_ROOT")
	if root == "" {
		t.Skip("requires disposable VM delegated root")
	}
	f, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	a, err := newOwner(f)
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	b, err := newOwner(f)
	if err != nil {
		t.Fatal(err)
	}
	defer b.close()
	type child struct {
		cmd *exec.Cmd
		in  io.WriteCloser
		out *bufio.Reader
	}
	start := func(o *owner) child {
		cmd := exec.Command("/proc/self/exe", "-test.run=^TestCgroupSleeper$")
		cmd.Env = append(os.Environ(), "VOICE_TEST_CGROUP_SLEEPER=1")
		cmd.SysProcAttr = &syscall.SysProcAttr{UseCgroupFD: true, CgroupFD: o.child}
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		reader := bufio.NewReader(stdout)
		line, err := reader.ReadString('\n')
		if err != nil || line != "ready\n" {
			t.Fatalf("helper did not start: %q, %v", line, err)
		}
		return child{cmd: cmd, in: stdin, out: reader}
	}
	p, q := start(a), start(b)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.stop(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.cmd.Wait(); err == nil {
		t.Fatal("owned process survived kill")
	}
	if _, err := io.WriteString(q.in, "check\n"); err != nil {
		t.Fatalf("sibling request: %v", err)
	}
	if line, err := q.out.ReadString('\n'); err != nil || line != "alive\n" {
		t.Fatalf("sibling response: %q, %v", line, err)
	}
	blocked := exec.Command("/proc/self/exe", "-test.run=^TestCgroupSleeper$")
	blocked.SysProcAttr = &syscall.SysProcAttr{UseCgroupFD: true, CgroupFD: a.child}
	if err := blocked.Start(); err == nil {
		_ = blocked.Process.Kill()
		_ = blocked.Wait()
		t.Fatal("new process entered restricted child")
	}
	if err := b.stop(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCgroupSleeper(t *testing.T) {
	if os.Getenv("VOICE_TEST_CGROUP_SLEEPER") != "1" {
		return
	}
	fmt.Println("ready")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil || line != "check\n" {
		return
	}
	fmt.Println("alive")
	time.Sleep(30 * time.Second)
}
