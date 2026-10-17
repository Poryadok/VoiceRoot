package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

// Kubernetes supplies loopback and one pod Ethernet interface. NATS remains
// loopback-only; admission and default-deny egress policy provide the separate
// workload/network boundary. Census also runs as the NATS owner so O_NOATIME
// source access does not require CAP_FOWNER; other phases remain UID 65532.
func isolatedPodInterfaces(interfaces []net.Interface) bool {
	if len(interfaces) != 2 {
		return false
	}
	seenLoopback, seenPodEthernet := false, false
	for _, iface := range interfaces {
		switch iface.Name {
		case "lo":
			if iface.Flags&net.FlagLoopback == 0 || iface.Flags&net.FlagUp == 0 {
				return false
			}
			seenLoopback = true
		case "eth0":
			if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
				return false
			}
			seenPodEthernet = true
		default:
			return false
		}
	}
	return seenLoopback && seenPodEthernet
}

func knownKernelPhase(phase string) bool {
	return phase == "seed" || phase == "verify-closed" || phase == "drain" || phase == "verify-drained" || phase == "census"
}

func kernelUIDAllowed(phase string, uid int) bool {
	if !knownKernelPhase(phase) {
		return false
	}
	return uid == 65532
}

func isolatedKernelForPhase(phase string) bool {
	if runtime.GOOS != "linux" || !kernelUIDAllowed(phase, os.Geteuid()) {
		return false
	}
	interfaces, e := net.Interfaces()
	return e == nil && isolatedPodInterfaces(interfaces)
}

func privateInput(name string) ([]byte, error) {
	if strings.ContainsAny(name, "/\\") {
		return nil, fixedError("input_identity_invalid")
	}
	path := "/inputs/" + name
	s, e := os.Lstat(path)
	if e != nil || !s.Mode().IsRegular() || s.Mode().Perm() != 0440 || s.Size() < 1 || s.Size() > 256<<10 {
		return nil, fixedError("input_custody_invalid")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, fixedError("input_read_failed")
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, (256<<10)+1))
	if e != nil || len(b) > 256<<10 {
		return nil, fixedError("input_read_failed")
	}
	return b, nil
}

func connectRole(role string) (*nats.Conn, error) {
	if role != "bootstrap" && role != "social" && role != "realtime" {
		return nil, fixedError("role_invalid")
	}
	if _, e := privateInput(role + ".creds"); e != nil {
		return nil, e
	}
	inbox := map[string]string{"bootstrap": "_INBOX.voice.bootstrap.reply", "social": "_INBOX.voice.social", "realtime": "_INBOX.voice.realtime"}[role]
	nc, e := nats.Connect("nats://127.0.0.1:4222", nats.UserCredentials("/inputs/"+role+".creds"), nats.CustomInboxPrefix(inbox), nats.Timeout(2*time.Second), nats.NoReconnect(), nats.ErrorHandler(func(*nats.Conn, *nats.Subscription, error) {}))
	if e != nil {
		return nil, fixedError("role_connect_failed")
	}
	return nc, nil
}

func readSnapshot(sessionState bool) (fixtureSnapshot, error) {
	var b []byte
	var e error
	if sessionState {
		b, e = readSessionSnapshot("/state/kernel_seed.output")
		if e != nil {
			return fixtureSnapshot{}, fixedError("fixture_snapshot_invalid")
		}
	} else {
		b, e = privateInput("fixture.json")
		if e != nil {
			return fixtureSnapshot{}, e
		}
	}
	var s fixtureSnapshot
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&s) != nil || d.Decode(new(any)) != io.EOF || !validSnapshot(s) {
		return fixtureSnapshot{}, fixedError("fixture_snapshot_invalid")
	}
	return s, nil
}

func readSessionSnapshot(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() < 1 || info.Size() > 256<<10 {
		return nil, fixedError("fixture_snapshot_invalid")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() || int(stat.Gid) != os.Getegid() {
		return nil, fixedError("fixture_snapshot_invalid")
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fixedError("fixture_snapshot_invalid")
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, fixedError("fixture_snapshot_invalid")
	}
	raw, err := io.ReadAll(io.LimitReader(f, (256<<10)+1))
	if err != nil || len(raw) != info.Size() || len(raw) > 256<<10 {
		return nil, fixedError("fixture_snapshot_invalid")
	}
	return raw, nil
}

func writeResult(name string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil || len(b) > 256<<10 {
		return fixedError("result_encode_failed")
	}
	f, e := os.OpenFile("/out/"+name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return fixedError("result_create_failed")
	}
	if _, e = f.Write(append(b, '\n')); e != nil {
		f.Close()
		return fixedError("result_write_failed")
	}
	if f.Sync() != nil || f.Close() != nil {
		return fixedError("result_close_failed")
	}
	return nil
}

func runKernel(args []string) error {
	flags := flag.NewFlagSet("known-nats-kernel", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	phase := flags.String("phase", "", "fixed owned isolation phase")
	sessionState := flags.Bool("session-state", false, "read the durable session seed snapshot")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return fixedError("kernel_isolation_invalid")
	}
	if !knownKernelPhase(*phase) {
		return fixedError("phase_invalid")
	}
	if !isolatedKernelForPhase(*phase) {
		return fixedError("kernel_isolation_invalid")
	}
	a, e := connectRole("bootstrap")
	if e != nil {
		return e
	}
	defer a.Close()
	if *phase == "census" {
		c, e := captureCensus(a)
		if e != nil {
			return e
		}
		return writeResult("census.json", c)
	}
	if *phase == "seed" {
		p, e := connectRole("social")
		if e != nil {
			return e
		}
		defer p.Close()
		r, e := connectRole("realtime")
		if e != nil {
			return e
		}
		defer r.Close()
		s, e := seedFixture(a, p, r)
		if e != nil {
			return e
		}
		return writeResult("fixture.json", s)
	}
	s, e := readSnapshot(*sessionState)
	if e != nil {
		return e
	}
	if *phase == "verify-closed" {
		return verifyClosedFixture(a, s)
	}
	if *phase == "verify-drained" {
		return verifyDrainedFixture(a, s)
	}
	r, e := connectRole("realtime")
	if e != nil {
		return e
	}
	defer r.Close()
	return drainFixture(a, r, s)
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--retained-grants" {
		if e := retainedGrants(os.Stdin, os.Stdout); e != nil {
			fmt.Fprintln(os.Stderr, "retained_grants_refused")
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--owned-readonly" {
		if e := ownedReadonly(os.Args[2:], os.Stdin, os.Stdout); e != nil {
			fmt.Fprintln(os.Stderr, "owned_readonly_request_refused")
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--consumer-state" {
		if e := nativeConsumerState(os.Args[2:], os.Stdin, os.Stdout); e != nil {
			fmt.Fprintln(os.Stderr, "native_consumer_state_refused")
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--metadata-checksum" {
		if e := nativeChecksum(os.Args[2:], os.Stdin, os.Stdout); e != nil {
			fmt.Fprintln(os.Stderr, "metadata_checksum_refused")
			os.Exit(1)
		}
		return
	}
	if e := runKernel(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e.Error())
		os.Exit(1)
	}
	fmt.Println("KNOWN_NATS_KERNEL=PASS")
}
