package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func stateU(dst []byte, value uint64) []byte { return binary.AppendUvarint(dst, value) }
func stateI(dst []byte, value int64) []byte  { return binary.AppendVarint(dst, value) }
func fullStateFixture() []byte {
	b := []byte{22, 2}
	for _, n := range []uint64{1, 1, 3, 2, 1} {
		b = stateU(b, n)
	}
	b = stateI(b, 1700000000)
	b = stateU(b, 1)
	b = stateU(b, 1)
	b = stateI(b, 0)
	b = stateU(b, 1)
	b = stateU(b, 1)
	return stateU(b, 1)
}

func TestDurableStateActualPopulatedPinnedNativeFiles(t *testing.T) {
	raw, err := os.ReadFile("testdata/native-durable-states.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Rows []struct {
			Phase, Path string
			Data        []byte
		}
	}
	if json.Unmarshal(raw, &fixture) != nil || len(fixture.Rows) != 88 {
		t.Fatal("physical fixture incomplete")
	}
	protected := 0
	for _, row := range fixture.Rows {
		s, err := decodeNativeConsumerState(row.Data)
		if err != nil {
			t.Fatal(row.Path, err)
		}
		if strings.Contains(row.Path, "/rollout_extra_durable/") {
			if s.AckFloor.Consumer != 1 || s.AckFloor.Stream != 1 || s.Delivered.Consumer != 3 || s.Delivered.Stream != 2 || len(s.Pending) != 1 || s.Pending[2].ConsumerSequence != 2 || s.Redelivered[2] != 1 {
				t.Fatal("actual populated durable maps lost", s)
			}
			protected++
		}
	}
	if protected != 2 {
		t.Fatal("actual before/after protected consumer missing")
	}
}

func TestFullDurableConsumerState(t *testing.T) {
	s, err := decodeNativeConsumerState(fullStateFixture())
	if err != nil {
		t.Fatal(err)
	}
	if s.AckFloor.Consumer != 1 || s.AckFloor.Stream != 1 || s.Delivered.Consumer != 3 || s.Delivered.Stream != 2 {
		t.Fatal("sequence state lost", s)
	}
	p, ok := s.Pending[2]
	if !ok || p.ConsumerSequence != 2 || p.TimestampNS != 1700000000000000000 || s.Redelivered[2] != 1 {
		t.Fatal("full protected map lost", s)
	}
	var out bytes.Buffer
	if err = nativeConsumerState(nil, bytes.NewReader(fullStateFixture()), &out); err != nil {
		t.Fatal(err)
	}
	var decoded nativeDurableState
	if json.Unmarshal(out.Bytes(), &decoded) != nil || len(decoded.Pending) != 1 || decoded.Pending[2] != p {
		t.Fatal("CLI map serialization lost")
	}
}

func TestDurableStateRejectsMalformedAndUnbounded(t *testing.T) {
	good := fullStateFixture()
	cases := [][]byte{nil, {22}, {22, 3}, append(append([]byte{}, good...), 0), {22, 2, 0x81, 0, 1, 3, 2, 0, 0}, bytes.Repeat([]byte{0}, (256<<10)+1)}
	for n := 0; n < len(good); n++ {
		cases = append(cases, good[:n])
	}
	// Duplicate pending keys, zero redelivery count, excessive count, bad floor.
	b := []byte{22, 2, 1, 1, 3, 2, 2}
	b = stateI(b, 1700000000)
	for i := 0; i < 2; i++ {
		b = append(b, 1, 1, 0)
	}
	cases = append(cases, append(b, 0))
	z := append([]byte{}, good...)
	z[len(z)-1] = 0
	cases = append(cases, z)
	b = []byte{22, 2, 1, 1, 3, 2}
	b = stateU(b, 16385)
	cases = append(cases, b)
	cases = append(cases, []byte{22, 2, 4, 1, 3, 2, 0, 0})
	for i, raw := range cases {
		if _, err := decodeNativeConsumerState(raw); err == nil {
			t.Fatalf("malformed state %d admitted", i)
		}
	}
	var out bytes.Buffer
	if nativeConsumerState([]string{"extra"}, bytes.NewReader(good), &out) == nil || out.Len() != 0 {
		t.Fatal("unexpected CLI args admitted")
	}
}
