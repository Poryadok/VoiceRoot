package lfpconsent

import (
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"voice/backend/pkg/storylfpadmission"
)

func originFixture(t *testing.T) storylfpadmission.Origin {
	t.Helper()
	o, e := storylfpadmission.NewOrigin(uuid.MustParse("00000000-0000-0000-0000-00000000000b"), uuid.MustParse("00000000-0000-0000-0000-00000000000c"), uuid.MustParse("00000000-0000-0000-0000-00000000000d"), [32]byte{4})
	if e != nil {
		t.Fatal(e)
	}
	return o
}

func TestActorEnvelopeIndependentGoldens(t *testing.T) {
	b, _, e := Encode(fixture())
	if e != nil {
		t.Fatal(e)
	}
	a, _, e := EncodeActor(actorFixture())
	if e != nil {
		t.Fatal(e)
	}
	envelope, e := NewEnvelope(b, a, originFixture(t))
	if e != nil {
		t.Fatal(e)
	}
	q := envelope.QualifiedSHA256()
	if hex.EncodeToString(q[:]) != "963207a612af029c79f2ac76a6a260560076ec05fd8de4c4846477608bf0d7ce" {
		t.Fatalf("independent qualified hash changed: %x", q)
	}
	var reg [32]byte
	copy(reg[:], mustHex("88a8c5e0c852420b669ac2c3d8fe91fbdd37af1ce70fc6552a76b91eb04c7cc0"))
	ack, e := HashActorBoundRegistration(reg, q)
	if e != nil || hex.EncodeToString(ack[:]) != "1c8414fb1d0a74fc3348db1b92f2c13a75306c187c08f1e17cdfa49d6034dd02" {
		t.Fatalf("independent ACK changed: %x %v", ack, e)
	}
	source, e := HashActorQualifiedSource([32]byte{5}, q, ack, [32]byte{6})
	if e != nil || hex.EncodeToString(source[:]) != "993ab8e0eaeae8ce25ba266f90e17fc82340b344847185480ad4cc38db56181d" {
		t.Fatalf("independent source hash changed: %x %v", source, e)
	}
	// Input/output buffers never rewrite an already constructed immutable proof.
	b[0] ^= 1
	a[0] ^= 1
	returned := envelope.ActorBytes()
	returned[0] ^= 1
	if envelope.Actor() != actorFixture() || envelope.Consent() != fixture() || envelope.ActorBytes()[0] != 8 {
		t.Fatal("mutable envelope alias")
	}
}

func TestActorEnvelopeRejectsCrossBindingAndUnknownOrigin(t *testing.T) {
	b, _, _ := Encode(fixture())
	for _, change := range []func(*Actor){func(a *Actor) { a.AccountID = uuid.New() }, func(a *Actor) { a.ProfileID = uuid.New() }, func(a *Actor) { a.SessionEpoch++ }, func(a *Actor) { a.OperationID = uuid.New() }, func(a *Actor) { a.PrincipalRequestSHA256[0]++ }, func(a *Actor) { a.ActionKind = PublishOffer; a.FullRPC = PublishOfferRPC }} {
		a := actorFixture()
		change(&a)
		ab, _, e := EncodeActor(a)
		if e != nil {
			t.Fatal(e)
		}
		got, e := NewEnvelope(b, ab, originFixture(t))
		if e == nil || got.Valid() {
			t.Fatal("mismatched sidecar accepted")
		}
	}
	a, _, _ := EncodeActor(actorFixture())
	got, e := NewEnvelope(b, a, storylfpadmission.Origin{})
	if e == nil || got.Valid() {
		t.Fatal("unknown origin accepted")
	}
}
