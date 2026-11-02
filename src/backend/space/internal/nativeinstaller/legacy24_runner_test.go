package nativeinstaller

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

type legacy24ShortWriter struct{}

func (legacy24ShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestLegacy24EvidenceShortWriteStops(t *testing.T) {
	if err := emitLegacy24(legacy24ShortWriter{}, "source24", &Legacy24Receipt{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short evidence write accepted: %v", err)
	}
}

func TestLegacy24RequiresOriginalCustodyBeforeConnecting(t *testing.T) {
	for _, c := range []Legacy24Config{
		{},
		{Config: Config{Output: &bytes.Buffer{}}, OperationID: "op", SnapshotID: "snapshot"},
		{Config: Config{Output: &bytes.Buffer{}}, OriginEvidencePath: "original.json", SnapshotID: "snapshot"},
	} {
		if err := RunLegacy24Upgrade(context.Background(), c); err == nil {
			t.Fatal("missing original custody accepted")
		}
	}
}
