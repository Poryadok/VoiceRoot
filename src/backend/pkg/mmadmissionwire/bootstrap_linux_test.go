//go:build linux

package mmadmissionwire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func init() {
	mode := os.Getenv("MM_BOOTSTRAP_NEGATIVE_CHILD")
	if mode == "" {
		return
	}
	if mode == "untrusted-peer" {
		pair, e := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
		if e != nil {
			os.Exit(31)
		}
		f := os.NewFile(uintptr(pair[0]), "untrusted-bootstrap")
		raw, e := net.FileConn(f)
		_ = f.Close()
		defer syscall.Close(pair[1])
		if e != nil {
			os.Exit(32)
		}
		defer raw.Close()
		conn, ok := raw.(*net.UnixConn)
		if !ok {
			os.Exit(33)
		}
		peer, e := PeerCredentials(conn)
		if e != nil || peer.UID != 65532 || privateBootstrapDriver(conn) == nil {
			os.Exit(34)
		}
	} else {
		if mode == "missing" {
			_ = syscall.Close(NativeBootstrapFD)
		}
		var d DescriptorV1
		if json.NewDecoder(os.Stdin).Decode(&d) != nil {
			os.Exit(35)
		}
		descriptor, _ := json.Marshal(d)
		if _, e := LoadNativeReadbackClient(context.Background(), descriptor, time.Second); !errors.Is(e, ErrInvalid) {
			os.Exit(36)
		}
		if _, _, e := syscall.Syscall(syscall.SYS_FCNTL, NativeBootstrapFD, syscall.F_GETFD, 0); e != syscall.EBADF {
			os.Exit(37)
		}
		if _, e := LoadNativeReadbackClient(context.Background(), descriptor, time.Second); !errors.Is(e, ErrInvalid) {
			os.Exit(38)
		}
	}
	fmt.Println("PRIVATE_BOOTSTRAP_REJECTED")
	os.Exit(0)
}
func TestActualPrivateBootstrapRejectsMissingRegularAndUntrustedFD(t *testing.T) {
	if os.Getenv("MM_BOOTSTRAP_FD_TEST") != "1" {
		t.Skip("requires own isolated root Linux runner and accessible test binary")
	}
	if os.Geteuid() != 0 {
		t.Fatal("owned driver runner must be root; helpers remain65532")
	}
	binary, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	d, _, _ := testObservation()
	descriptor, _ := json.Marshal(d)
	for _, mode := range []string{"missing", "regular-file", "untrusted-peer"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "-test.run=^$")
			cmd.Env = append(os.Environ(), "MM_BOOTSTRAP_NEGATIVE_CHILD="+mode)
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65532, Gid: 65532}}
			cmd.Stdin = strings.NewReader(string(descriptor))
			if mode == "regular-file" {
				f, e := os.CreateTemp(t.TempDir(), "not-a-bootstrap-socket")
				if e != nil {
					t.Fatal(e)
				}
				defer f.Close()
				cmd.ExtraFiles = []*os.File{f}
			}
			out, e := cmd.CombinedOutput()
			if e != nil || string(out) != "PRIVATE_BOOTSTRAP_REJECTED\n" {
				t.Fatalf("%s rejected incorrectly: %v %s", mode, e, out)
			}
		})
	}
}
