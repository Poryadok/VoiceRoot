package config

import (
	"testing"
)

func TestSplitCSV(t *testing.T) {
	if got := SplitCSV(""); got != nil {
		t.Fatalf("empty: got %#v", got)
	}
	if got := SplitCSV(" a , , b "); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %#v", got)
	}
}

func TestTrimEnv(t *testing.T) {
	t.Setenv("PKG_TRIM_TEST", "  x  ")
	if TrimEnv("PKG_TRIM_TEST") != "x" {
		t.Fatal()
	}
}

func TestLoadJSONEnv(t *testing.T) {
	t.Setenv("PKG_JSON_TEST", `{"k":"v"}`)
	var m map[string]string
	LoadJSONEnv("PKG_JSON_TEST", &m, func(name string, err error) {
		t.Fatalf("unexpected log: %s %v", name, err)
	})
	if m["k"] != "v" {
		t.Fatalf("%v", m)
	}

	var called bool
	t.Setenv("PKG_JSON_BAD", `not-json`)
	LoadJSONEnv("PKG_JSON_BAD", &m, func(name string, err error) {
		called = true
		if name != "PKG_JSON_BAD" || err == nil {
			t.Fatalf("bad callback")
		}
	})
	if !called {
		t.Fatal("expected invalid callback")
	}
}
