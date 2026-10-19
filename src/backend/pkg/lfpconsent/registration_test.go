package lfpconsent

import (
	"bytes"
	"encoding/hex"
	"github.com/google/uuid"
	"testing"
)

func registrationFixture() Registration {
	return Registration{
		OperationID:   uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		ConsentID:     uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		ConsentSHA256: [32]byte{1}, RegistrationID: uuid.MustParse("00000000-0000-0000-0000-000000000003"),
		Generation: 4, CapturedSearchIntentSHA256: [32]byte{2},
	}
}

func TestRegistrationImmutableCapture(t *testing.T) {
	r := registrationFixture()
	b, h, e := EncodeRegistration(r)
	if e != nil {
		t.Fatal("valid original capture rejected")
	}
	const golden = "08011210000000000000000000000000000000011a1000000000000000000000000000000002222001000000000000000000000000000000000000000000000000000000000000002a100000000000000000000000000000000330043a200200000000000000000000000000000000000000000000000000000000000000"
	if !bytes.Equal(b, mustHex(golden)) || hex.EncodeToString(h[:]) != "88a8c5e0c852420b669ac2c3d8fe91fbdd37af1ce70fc6552a76b91eb04c7cc0" {
		t.Fatalf("independent registration golden changed: %x %x", b, h)
	}
	got, gh, e := DecodeRegistration(b)
	if e != nil || got != r || gh != h {
		t.Fatal("registration roundtrip mismatch")
	}
	for _, change := range []func(*Registration){
		func(r *Registration) { r.OperationID[15]++ }, func(r *Registration) { r.ConsentID[15]++ },
		func(r *Registration) { r.ConsentSHA256[0]++ }, func(r *Registration) { r.RegistrationID[15]++ },
		func(r *Registration) { r.Generation++ }, func(r *Registration) { r.CapturedSearchIntentSHA256[0]++ },
	} {
		other := r
		change(&other)
		_, oh, e := EncodeRegistration(other)
		if e != nil || oh == h {
			t.Fatal("capture identity not bound")
		}
	}
	for name, data := range map[string][]byte{"empty": nil, "duplicate": append(bytes.Clone(b), b[:2]...), "truncated": b[:len(b)-1], "unknown": append(bytes.Clone(b), 0x48, 1), "reorder": append(bytes.Clone(b[2:]), b[:2]...), "overlong": append([]byte{8, 0x81, 0}, b[2:]...)} {
		t.Run(name, func(t *testing.T) {
			r, h, e := DecodeRegistration(data)
			if e == nil || r != (Registration{}) || h != [32]byte{} {
				t.Fatal("alternate wire or partial registration escaped")
			}
		})
	}
}

func TestRegistrationRejectsMissingCapture(t *testing.T) {
	for _, change := range []func(*Registration){func(r *Registration) { r.OperationID = uuid.Nil }, func(r *Registration) { r.ConsentID = uuid.Nil }, func(r *Registration) { r.ConsentSHA256 = [32]byte{} }, func(r *Registration) { r.RegistrationID = uuid.Nil }, func(r *Registration) { r.Generation = 0 }, func(r *Registration) { r.CapturedSearchIntentSHA256 = [32]byte{} }} {
		r := registrationFixture()
		change(&r)
		b, h, e := EncodeRegistration(r)
		if e == nil || b != nil || h != [32]byte{} {
			t.Fatal("missing immutable capture accepted")
		}
	}
}
