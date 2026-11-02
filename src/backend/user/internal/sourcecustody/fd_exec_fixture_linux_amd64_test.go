//go:build linux && amd64

package sourcecustody

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const fdFixtureMax = 128 << 20
const fdFixtureSeals = unix.F_SEAL_WRITE | unix.F_SEAL_GROW | unix.F_SEAL_SHRINK | unix.F_SEAL_SEAL

// fixtureStaticELF is deliberately limited to the test's own x86-64 ELF.
// This is not a general-purpose ELF validator or a production launch policy.
func fixtureStaticELF(data []byte) error {
	if len(data) == 0 || len(data) > fdFixtureMax {
		return fmt.Errorf("invalid fixture size")
	}
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer f.Close()
	if f.Class != elf.ELFCLASS64 || f.Data != elf.ELFDATA2LSB || f.Machine != elf.EM_X86_64 || f.Type != elf.ET_EXEC {
		return fmt.Errorf("unsupported fixture ELF")
	}
	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP || p.Type == elf.PT_DYNAMIC {
			return fmt.Errorf("dynamic fixture ELF rejected")
		}
	}
	return nil
}

// TestFDExecStaticFixture runs only in an explicitly owned disposable VM.
// The sole executed image is this self-built Go test binary, sealed in a memfd.
// It is not a source launcher, production qualification, or fencing receipt.
func TestFDExecStaticFixture(t *testing.T) {
	if os.Getenv("VOICE_DISPOSABLE_VM_FD_EXEC_TEST") != "1" {
		t.Skip("requires isolated disposable cgroup-v2 VM")
	}
	self, err := os.ReadFile("/proc/self/exe")
	if err != nil || fixtureStaticELF(self) != nil {
		t.Fatalf("self-built static fixture unavailable: %v", err)
	}
	// Rejection cases are only inspected, never executed.
	for name, input := range map[string][]byte{"script": []byte("#!/bin/sh\nexit 0\n"), "truncated": self[:16]} {
		if fixtureStaticELF(input) == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	dynamic := bytes.Clone(self)
	programOffset := binary.LittleEndian.Uint64(dynamic[32:40])
	if programOffset > uint64(len(dynamic)-4) {
		t.Fatal("invalid own ELF program header")
	}
	binary.LittleEndian.PutUint32(dynamic[programOffset:programOffset+4], uint32(elf.PT_INTERP))
	if fixtureStaticELF(dynamic) == nil {
		t.Fatal("accepted interpreter-bearing fixture")
	}

	// Only paths created by this test are changed. The opened descriptor keeps
	// referencing the original fixture after its temporary pathname is replaced.
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture")
	if err := os.WriteFile(path, self, 0600); err != nil {
		t.Fatal(err)
	}
	opened, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if err := os.Rename(path, filepath.Join(dir, "original")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("inert replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	contents, err := io.ReadAll(io.LimitReader(opened, fdFixtureMax+1))
	if err != nil || sha256.Sum256(contents) != sha256.Sum256(self) {
		t.Fatal("opened fixture changed")
	}

	fd, err := unix.MemfdCreate("voice-static-test-fixture", unix.MFD_ALLOW_SEALING|unix.MFD_CLOEXEC)
	if err != nil {
		t.Fatalf("memfd unavailable: %v", err)
	}
	mem := os.NewFile(uintptr(fd), "voice-static-test-fixture")
	defer mem.Close()
	if _, err := mem.Write(contents); err != nil {
		t.Fatal(err)
	}
	if _, err := unix.FcntlInt(mem.Fd(), unix.F_ADD_SEALS, fdFixtureSeals); err != nil {
		t.Fatalf("seal fixture: %v", err)
	}
	seals, err := unix.FcntlInt(mem.Fd(), unix.F_GET_SEALS, 0)
	if err != nil || seals&fdFixtureSeals != fdFixtureSeals {
		t.Fatal("required seals unavailable")
	}
	if _, err := unix.Pwrite(fd, []byte{0}, 0); err != unix.EPERM {
		t.Fatalf("sealed write was not denied: %v", err)
	}
	if err := unix.Ftruncate(fd, 0); err != unix.EPERM {
		t.Fatalf("sealed truncation was not denied: %v", err)
	}
	if _, err := mem.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}

	root, err := os.Open("/sys/fs/cgroup/voice-fixture")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	owned, err := newOwner(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := owned.stop(ctx); err != nil {
			t.Errorf("owned fixture cleanup: %v", err)
		}
		owned.close()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/proc/self/exe", "-test.run=^TestFDExecFixtureBootstrap$")
	cmd.Env = []string{"VOICE_FD_FIXTURE_BOOTSTRAP=1"}
	cmd.ExtraFiles = []*os.File{mem}
	cmd.SysProcAttr = &syscall.SysProcAttr{UseCgroupFD: true, CgroupFD: owned.child}
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("cgroup-bound bootstrap unavailable: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	reader := bufio.NewReader(output)
	readLine := func(want string) {
		lines := make(chan string, 1)
		go func() { line, _ := reader.ReadString('\n'); lines <- line }()
		select {
		case line := <-lines:
			if line != want {
				t.Fatalf("fixture output %q, want %q", line, want)
			}
		case <-ctx.Done():
			t.Fatal("fixture output timeout")
		}
	}
	readLine("BOOTSTRAP_READY\n")
	procs, err := owned.control("cgroup.procs", false, "")
	if err != nil || !strings.Contains("\n"+procs, "\n"+strconv.Itoa(cmd.Process.Pid)+"\n") {
		t.Fatal("bootstrap not in owned cgroup")
	}
	if _, err := io.WriteString(input, "go\n"); err != nil {
		t.Fatal(err)
	}
	readLine("FD_EXEC_PAYLOAD\n")
	if err := cmd.Wait(); err != nil {
		t.Fatalf("fixture exec failed: %v: %s", err, stderr.String())
	}
	t.Log("sealed self-built static ELF executed via FD after pathname substitution")
}

// The fully exec'd test bootstrap accepts only a sealed copy of itself. It is
// not a post-fork Go callback and cannot be directed to execute arbitrary data.
func TestFDExecFixtureBootstrap(t *testing.T) {
	if os.Getenv("VOICE_FD_FIXTURE_BOOTSTRAP") != "1" {
		return
	}
	f := os.NewFile(3, "fixture")
	if f == nil {
		t.Fatal("missing fixture fd")
	}
	seals, err := unix.FcntlInt(f.Fd(), unix.F_GET_SEALS, 0)
	if err != nil || seals&fdFixtureSeals != fdFixtureSeals {
		t.Fatal("unsealed fixture")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(io.LimitReader(f, fdFixtureMax+1))
	if err != nil || fixtureStaticELF(data) != nil {
		t.Fatal("invalid fixture")
	}
	self, err := os.ReadFile("/proc/self/exe")
	if err != nil || sha256.Sum256(data) != sha256.Sum256(self) {
		t.Fatal("fixture is not self")
	}
	fmt.Println("BOOTSTRAP_READY")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil || line != "go\n" {
		t.Fatal("invalid test handshake")
	}
	argv, err := syscall.SlicePtrFromStrings([]string{"voice-static-fixture", "-test.run=^TestFDExecStaticFixturePayload$"})
	if err != nil {
		t.Fatal(err)
	}
	envp, err := syscall.SlicePtrFromStrings([]string{"VOICE_FD_FIXTURE_PAYLOAD=1"})
	if err != nil {
		t.Fatal(err)
	}
	empty := []byte{0}
	_, _, errno := unix.Syscall6(unix.SYS_EXECVEAT, f.Fd(), uintptr(unsafe.Pointer(&empty[0])), uintptr(unsafe.Pointer(&argv[0])), uintptr(unsafe.Pointer(&envp[0])), uintptr(unix.AT_EMPTY_PATH), 0)
	runtime.KeepAlive(argv)
	runtime.KeepAlive(envp)
	runtime.KeepAlive(empty)
	runtime.KeepAlive(f)
	t.Fatalf("execveat failed: %v", errno)
}

func TestFDExecStaticFixturePayload(t *testing.T) {
	if os.Getenv("VOICE_FD_FIXTURE_PAYLOAD") == "1" {
		fmt.Println("FD_EXEC_PAYLOAD")
	}
}
