package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWriterStopsAtSelectedEvent(t *testing.T) {
	var saved bytes.Buffer
	writer := &failureWriter{at: "trusted50", output: &saved}
	encoder := json.NewEncoder(writer)
	if err := encoder.Encode(map[string]string{"event": "migrated50"}); err != nil {
		t.Fatal(err)
	}
	before := saved.String()
	if err := encoder.Encode(map[string]string{"event": "trusted50"}); err == nil {
		t.Fatal("selected event must fail")
	}
	if saved.String() != before {
		t.Fatal("failed event was retained")
	}
	if _, err := writer.Write([]byte("not JSON")); err == nil {
		t.Fatal("malformed event must fail")
	}
}
