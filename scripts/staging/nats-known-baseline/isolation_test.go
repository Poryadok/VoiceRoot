package main

import (
	"net"
	"strconv"
	"testing"
)

func TestIsolatedPodInterfaces(t *testing.T) {
	valid := []net.Interface{
		{Name: "lo", Flags: net.FlagUp | net.FlagLoopback},
		{Name: "eth0", Flags: net.FlagUp | net.FlagBroadcast},
	}
	if !isolatedPodInterfaces(valid) {
		t.Fatal("standard pod loopback plus eth0 must pass")
	}
	for name, interfaces := range map[string][]net.Interface{
		"extra interface":      append(append([]net.Interface(nil), valid...), net.Interface{Name: "eth1", Flags: net.FlagUp}),
		"down pod interface":   {{Name: "lo", Flags: net.FlagUp | net.FlagLoopback}, {Name: "eth0"}},
		"loopback renamed":     {{Name: "lo", Flags: net.FlagUp}, {Name: "eth0", Flags: net.FlagUp}},
		"unexpected interface": {{Name: "lo", Flags: net.FlagUp | net.FlagLoopback}, {Name: "ens5", Flags: net.FlagUp}},
	} {
		t.Run(name, func(t *testing.T) {
			if isolatedPodInterfaces(interfaces) {
				t.Fatal("unexpected pod network accepted")
			}
		})
	}
}

func TestKernelUIDPhaseBoundary(t *testing.T) {
	tests := []struct {
		phase string
		uid   int
		want  bool
	}{
		{"census", 10000, false},
		{"census", 65532, true},
		{"seed", 65532, true},
		{"verify-closed", 65532, true},
		{"drain", 65532, true},
		{"verify-drained", 65532, true},
		{"seed", 10000, false},
		{"verify-closed", 10000, false},
		{"drain", 10000, false},
		{"verify-drained", 10000, false},
		{"census", 10001, false},
		{"unknown", 10000, false},
		{"unknown", 65532, false},
	}
	for _, tt := range tests {
		t.Run(tt.phase+"/"+strconv.Itoa(tt.uid), func(t *testing.T) {
			if got := kernelUIDAllowed(tt.phase, tt.uid); got != tt.want {
				t.Fatalf("kernelUIDAllowed(%q, %d) = %t, want %t", tt.phase, tt.uid, got, tt.want)
			}
		})
	}
}

func TestRunKernelRejectsUnknownPhaseBeforeIdentityCheck(t *testing.T) {
	err := runKernel([]string{"--phase", "unknown"})
	if err == nil || err.Error() != "phase_invalid" {
		t.Fatalf("unknown phase returned %v, want phase_invalid before identity checks", err)
	}
}
