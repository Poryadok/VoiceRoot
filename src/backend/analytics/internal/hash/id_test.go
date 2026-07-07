package hash

import "testing"

func TestIDDeterministic(t *testing.T) {
	a := ID("test-key", "account-1")
	b := ID("test-key", "account-1")
	if a == "" || a != b {
		t.Fatalf("expected stable hash, got %q %q", a, b)
	}
}

func TestIDEmpty(t *testing.T) {
	if ID("k", "") != "" {
		t.Fatal("empty input must yield empty hash")
	}
}
