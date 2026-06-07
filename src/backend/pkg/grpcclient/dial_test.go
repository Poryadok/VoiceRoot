package grpcclient

import "testing"

func TestDialTarget(t *testing.T) {
	t.Parallel()
	if got := DialTarget(""); got != "" {
		t.Fatalf("empty: got %q", got)
	}
	if got := DialTarget("chat:9090"); got != "dns:///chat:9090" {
		t.Fatalf("host:port: got %q", got)
	}
	if got := DialTarget("passthrough:///bufnet"); got != "passthrough:///bufnet" {
		t.Fatalf("scheme preserved: got %q", got)
	}
}
