package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	ts := httptest.NewServer(handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	b, _ := io.ReadAll(res.Body)
	if string(b) != "ok" {
		t.Fatalf("body %q", b)
	}
}
