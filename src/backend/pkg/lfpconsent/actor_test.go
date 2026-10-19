package lfpconsent

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
)

func actorFixture() Actor {
	c := fixture()
	return Actor{ActionKind: Respond, ActorKind: HumanDelegatedUser, AccountType: Guest, AccountID: c.ResponderAccountID, ProfileID: c.ResponderProfileID, SessionEpoch: c.ResponderSessionEpoch, OperationID: c.ResponseOperationID, Issuer: "gateway", Audience: "story", FullRPC: RespondRPC, PrincipalRequestSHA256: c.PrincipalRequestSHA256}
}

func TestActorGoldenAndActions(t *testing.T) {
	a := actorFixture()
	b, h, e := EncodeActor(a)
	if e != nil {
		t.Fatal(e)
	}
	const golden = "08011002180120022a100000000000000000000000000000000632100000000000000000000000000000000738094210000000000000000000000000000000024a0767617465776179520573746f72795a3d2f766f6963652e73746f72792e76312e4c6670436f6e73656e74416374696f6e536572766963652f526573706f6e64576974684c6670436f6e73656e7462200300000000000000000000000000000000000000000000000000000000000000"
	if !bytes.Equal(b, mustHex(golden)) || hex.EncodeToString(h[:]) != "0ee8c407a9c8e01f9fbb593f8b4dc36fae509ad8b3f9c57e4602205eb223b85d" {
		t.Fatalf("independent actor golden mismatch: %x %x", b, h)
	}
	got, hh, e := DecodeActor(b)
	if e != nil || got != a || hh != h {
		t.Fatalf("roundtrip: %v", e)
	}
	for _, kind := range []uint64{PublishOffer, Respond, Accept, Decline} {
		for _, accountType := range []uint64{Regular, Guest} {
			a := actorFixture()
			a.ActionKind = kind
			a.AccountType = accountType
			switch kind {
			case PublishOffer:
				a.FullRPC = PublishOfferRPC
			case Accept, Decline:
				a.Audience = "matchmaking"
				a.FullRPC = DecideRPC
			}
			b, h, e := EncodeActor(a)
			if e != nil {
				t.Fatal(e)
			}
			got, hh, e := DecodeActor(b)
			if e != nil || got != a || hh != h {
				t.Fatal("action/type omitted")
			}
		}
	}
}

func TestActorRejectsUnsupportedAndMalformed(t *testing.T) {
	for name, mutate := range map[string]func(*Actor){
		"unknownaction": func(a *Actor) { a.ActionKind = 9 }, "unknownkind": func(a *Actor) { a.ActorKind = 2 }, "unknownclass": func(a *Actor) { a.AccountType = 0 },
		"nilaccount": func(a *Actor) { a.AccountID = uuid.Nil }, "nilprofile": func(a *Actor) { a.ProfileID = uuid.Nil }, "nilepoch": func(a *Actor) { a.SessionEpoch = 0 }, "negativeepoch": func(a *Actor) { a.SessionEpoch = -1 },
		"niloperation": func(a *Actor) { a.OperationID = uuid.Nil }, "serviceissuer": func(a *Actor) { a.Issuer = "story" }, "wrongaudience": func(a *Actor) { a.Audience = "matchmaking" }, "wrongmethod": func(a *Actor) { a.FullRPC = DecideRPC }, "zeroSHA": func(a *Actor) { a.PrincipalRequestSHA256 = [32]byte{} },
	} {
		t.Run(name, func(t *testing.T) {
			a := actorFixture()
			mutate(&a)
			b, h, e := EncodeActor(a)
			if e == nil || b != nil || h != [32]byte{} {
				t.Fatal("unsupported actor accepted")
			}
		})
	}
	b, _, e := EncodeActor(actorFixture())
	if e != nil {
		t.Fatal(e)
	}
	for name, data := range map[string][]byte{"empty": nil, "truncated": b[:len(b)-1], "duplicate": append(bytes.Clone(b), b[:2]...), "unknown": append(bytes.Clone(b), 0x68, 1), "overlong": append([]byte{8, 0x81, 0}, b[2:]...), "reordered": append(append(bytes.Clone(b[2:4]), b[:2]...), b[4:]...), "huge": make([]byte, 2049)} {
		t.Run(name, func(t *testing.T) {
			a, h, e := DecodeActor(data)
			if e == nil || a != (Actor{}) || h != [32]byte{} {
				t.Fatal("noncanonical actor accepted")
			}
		})
	}
}
