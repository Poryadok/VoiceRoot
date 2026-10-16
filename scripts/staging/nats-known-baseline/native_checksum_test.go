package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestNativeChecksumFixedIdentityAndBoundedInput(t *testing.T) {
	var out bytes.Buffer
	if err := nativeChecksum([]string{"events", "ephemeral"}, strings.NewReader(`{"Name":"ephemeral"}`), &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 16 {
		t.Fatalf("checksum length: %q", out.String())
	}
	for _, args := range [][]string{{"events/other", "ephemeral"}, {"events", "*"}, {"events"}, {"events", "ephemeral", "extra"}} {
		out.Reset()
		if nativeChecksum(args, strings.NewReader("{}"), &out) == nil || out.Len() != 0 {
			t.Fatal("invalid identity accepted")
		}
	}
	out.Reset()
	if nativeChecksum([]string{"events", "ephemeral"}, strings.NewReader(strings.Repeat("x", (256<<10)+1)), &out) == nil || out.Len() != 0 {
		t.Fatal("oversize accepted")
	}
}

func TestNativeChecksumActualPinnedServerBeforeAfterMetadata(t *testing.T) {
	raw, err := os.ReadFile("testdata/native-startup-checksum.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Stream, Consumer string
		Data             []byte
		Expected         string
	}
	if err := json.Unmarshal(raw, &rows); err != nil || len(rows) != 2 {
		t.Fatal("fixture shape", err)
	}
	for _, row := range rows {
		var out bytes.Buffer
		if err := nativeChecksum([]string{row.Stream, row.Consumer}, bytes.NewReader(row.Data), &out); err != nil || out.String() != row.Expected {
			t.Fatal("actual pinned native checksum mismatch", err)
		}
	}
}
