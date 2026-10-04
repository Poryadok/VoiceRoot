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
	"time"

	"github.com/nats-io/nats.go"
)

// The root controller verifies actual Docker ownership/mounts before starting
// this unprivileged kernel in the broker's network-none namespace. The kernel
// independently refuses every interface except loopback and every caller URL.
func isolatedKernel() bool {
	if runtime.GOOS != "linux" || os.Geteuid() != 65532 {
		return false
	}
	interfaces, e := net.Interfaces()
	if e != nil || len(interfaces) != 1 {
		return false
	}
	return interfaces[0].Flags&net.FlagLoopback != 0
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

func readSnapshot() (fixtureSnapshot, error) {
	b, e := privateInput("fixture.json")
	if e != nil {
		return fixtureSnapshot{}, e
	}
	var s fixtureSnapshot
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&s) != nil || d.Decode(new(any)) != io.EOF || !validSnapshot(s) {
		return fixtureSnapshot{}, fixedError("fixture_snapshot_invalid")
	}
	return s, nil
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
	if flags.Parse(args) != nil || flags.NArg() != 0 || !isolatedKernel() {
		return fixedError("kernel_isolation_invalid")
	}
	if *phase != "seed" && *phase != "verify-closed" && *phase != "drain" && *phase != "verify-drained" && *phase != "census" {
		return fixedError("phase_invalid")
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
	s, e := readSnapshot()
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
	if e := runKernel(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e.Error())
		os.Exit(1)
	}
	fmt.Println("KNOWN_NATS_KERNEL=PASS")
}
