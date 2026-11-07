package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"runtime/debug"
	"strconv"
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

func TestIdentityMatchesPinnedTestcontainersChild(t *testing.T) {
	if os.Getenv("VOICE_CI_SESSION_CHILD") == "1" {
		if err := json.NewEncoder(os.Stdout).Encode(testcontainers.SessionID()); err != nil {
			t.Fatal(err)
		}
		return
	}
	identity, err := sessionForParent(int32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestIdentityMatchesPinnedTestcontainersChild$")
	child.Env = append(os.Environ(), "VOICE_CI_SESSION_CHILD=1")
	output, err := child.Output()
	if err != nil {
		t.Fatal(err)
	}
	var actual string
	if err := json.NewDecoder(bytes.NewReader(output)).Decode(&actual); err != nil {
		t.Fatalf("decode child session: %v, output %q", err, output)
	}
	if identity.Session != actual || identity.ParentPID != int32(os.Getpid()) || identity.CreatedMS <= 0 {
		t.Fatalf("predicted identity %#v, actual session %q", identity, actual)
	}
}

func TestRejectInvalidParent(t *testing.T) {
	for _, pid := range []int32{0, -1} {
		if _, err := sessionForParent(pid); err == nil {
			t.Fatal("accepted invalid PID " + strconv.Itoa(int(pid)))
		}
	}
}

func TestDependencyVersionsFailClosed(t *testing.T) {
	good := &debug.BuildInfo{Deps: []*debug.Module{
		{Path: "github.com/testcontainers/testcontainers-go", Version: "v0.35.0"},
		{Path: "github.com/shirou/gopsutil/v3", Version: "v3.23.12"},
	}}
	if err := validateDependencies(good); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []*debug.BuildInfo{
		nil, {}, {Deps: good.Deps[:1]},
		{Deps: []*debug.Module{{Path: "github.com/testcontainers/testcontainers-go", Version: "v0.36.0"}, good.Deps[1]}},
		{Deps: []*debug.Module{{Path: "github.com/testcontainers/testcontainers-go", Version: "v0.35.0", Replace: &debug.Module{Path: "local"}}, good.Deps[1]}},
	} {
		if err := validateDependencies(bad); err == nil {
			t.Fatalf("accepted dependency drift: %#v", bad)
		}
	}
}
