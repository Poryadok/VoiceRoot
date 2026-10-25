package memberadmission

import (
	"bytes"
	"encoding/hex"
	"math"
	"strings"
	"testing"
)

func ensureFixture() EnsureBinding {
	digest := make([]byte, 32)
	for i := range digest {
		digest[i] = byte(i)
	}
	return EnsureBinding{ProtocolVersion: 1, SpaceID: "11111111-1111-4111-8111-111111111111", ProfileID: "22222222-2222-4222-8222-222222222222", AdmissionOperationID: "33333333-3333-4333-8333-333333333333", MembershipGeneration: 1, AdmissionBindingSHA256: digest}
}

const selectedRole = "44444444-4444-4444-8444-444444444444"

func TestReceiptBinding_RejectsUnsupportedOrAmbiguousIdentity(t *testing.T) {
	for _, change := range []string{"version", "zero generation", "overflow generation", "zero Space", "noncanonical profile", "operation", "digest"} {
		t.Run(change, func(t *testing.T) {
			b := ensureFixture()
			switch change {
			case "version":
				b.ProtocolVersion = 2
			case "zero generation":
				b.MembershipGeneration = 0
			case "overflow generation":
				b.MembershipGeneration = uint64(math.MaxInt64) + 1
			case "zero Space":
				b.SpaceID = "00000000-0000-0000-0000-000000000000"
			case "noncanonical profile":
				b.ProfileID = strings.ToUpper("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
			case "operation":
				b.AdmissionOperationID = "bad"
			case "digest":
				b.AdmissionBindingSHA256 = b.AdmissionBindingSHA256[:31]
			}
			if err := ValidateEnsure(b); err == nil {
				t.Fatal("invalid binding accepted")
			}
			if _, err := EnsureReceiptFingerprint(b, selectedRole); err == nil {
				t.Fatal("invalid binding fingerprinted")
			}
		})
	}
}

func TestEnsureReceipt_ChangingAnyBoundFieldChangesFingerprint(t *testing.T) {
	base := ensureFixture()
	original := append([]byte(nil), base.AdmissionBindingSHA256...)
	want, err := EnsureReceiptFingerprint(base, selectedRole)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"space", "profile", "operation", "generation", "digest", "role"} {
		t.Run(change, func(t *testing.T) {
			b := ensureFixture()
			role := selectedRole
			switch change {
			case "space":
				b.SpaceID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
			case "profile":
				b.ProfileID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
			case "operation":
				b.AdmissionOperationID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
			case "generation":
				b.MembershipGeneration++
			case "digest":
				b.AdmissionBindingSHA256[0]++
			case "role":
				role = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
			}
			got, err := EnsureReceiptFingerprint(b, role)
			if err != nil {
				t.Fatal(err)
			}
			if got == want {
				t.Fatal("bound field omitted from fingerprint")
			}
		})
	}
	if !bytes.Equal(base.AdmissionBindingSHA256, original) {
		t.Fatal("fingerprinting mutated caller bytes")
	}
}

func TestRemoveReceipt_ValidatesAndSeparatesOperationKind(t *testing.T) {
	e := ensureFixture()
	r := RemoveBinding{ProtocolVersion: 1, SpaceID: e.SpaceID, ProfileID: e.ProfileID, RemovalOperationID: e.AdmissionOperationID, RemovalGenerationFloor: e.MembershipGeneration, RemovalBindingSHA256: e.AdmissionBindingSHA256}
	got, err := RemoveReceiptFingerprint(r)
	if err != nil {
		t.Fatal(err)
	}
	ensure, err := EnsureReceiptFingerprint(e, selectedRole)
	if err != nil {
		t.Fatal(err)
	}
	if got == ensure {
		t.Fatal("operation kinds share a fingerprint domain")
	}
	r.RemovalGenerationFloor = 0
	if _, err := RemoveReceiptFingerprint(r); err == nil {
		t.Fatal("invalid removal floor accepted")
	}
}

func TestReceiptFingerprints_IndependentPythonVectors(t *testing.T) {
	// Generated independently with Python uuid.UUID.bytes, struct.pack('>I/Q')
	// and hashlib.sha256. These bytes are not protobuf authentication hashes.
	for _, tc := range []struct {
		generation uint64
		want       string
	}{
		{1, "78b5c727dd007f670a6f5af9c9b126a84bda81f1a6f90ca29792c77ddceb39af"},
		{math.MaxInt64, "5d23ef179da100918a64c5766efb14ec3f8fd10fb4d80ad5c3c54b798d6fbd1c"},
	} {
		b := ensureFixture()
		b.MembershipGeneration = tc.generation
		got, err := EnsureReceiptFingerprint(b, selectedRole)
		if err != nil {
			t.Fatal(err)
		}
		if hex.EncodeToString(got[:]) != tc.want {
			t.Fatalf("generation%d fingerprint=%x", tc.generation, got)
		}
	}
	e := ensureFixture()
	r := RemoveBinding{ProtocolVersion: 1, SpaceID: e.SpaceID, ProfileID: e.ProfileID, RemovalOperationID: e.AdmissionOperationID, RemovalGenerationFloor: 1, RemovalBindingSHA256: e.AdmissionBindingSHA256}
	got, err := RemoveReceiptFingerprint(r)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(got[:]) != "9c1029a264468f13cde01697ca8d35c5e9b45386e37573db2f9d9a5ea52953c7" {
		t.Fatalf("remove fingerprint=%x", got)
	}
}
