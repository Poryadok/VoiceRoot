package lfpconsent

import (
	"bytes"
	"encoding/hex"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protowire"
	"testing"
)

func fixture() Consent {
	return Consent{
		ConsentID:             uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		ResponseOperationID:   uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		StoryID:               uuid.MustParse("00000000-0000-0000-0000-000000000003"),
		StoryIncarnation:      uuid.MustParse("00000000-0000-0000-0000-000000000004"),
		OfferRevision:         7,
		AuthorProfileID:       uuid.MustParse("00000000-0000-0000-0000-000000000005"),
		ResponderAccountID:    uuid.MustParse("00000000-0000-0000-0000-000000000006"),
		ResponderProfileID:    uuid.MustParse("00000000-0000-0000-0000-000000000007"),
		ResponderSessionEpoch: 9, ResponseType: Join,
		CriteriaSHA256: [32]byte{1}, OfferPayloadSHA256: [32]byte{2},
		CreatedAtUnixMillis: 1000, OfferExpiresAtUnixMillis: 2000,
		PrincipalRequestSHA256: [32]byte{3},
	}
}

func TestConsentRoundTripAndDomain(t *testing.T) {
	want := fixture()
	b, hash, err := Encode(want)
	if err != nil {
		t.Fatalf("valid receipt rejected: %v", err)
	}
	got, decodedHash, err := Decode(b)
	if err != nil || got != want || hash != decodedHash {
		t.Fatalf("roundtrip mismatch: %v", err)
	}
	// These bytes are fixed by the independently numbered protobuf contract.
	const prefix = "08011210000000000000000000000000000000011a1000000000000000000000000000000002"
	if !bytes.HasPrefix(b, mustHex(prefix)) {
		t.Fatalf("wire prefix changed: %x", b)
	}
	const golden = "08011210000000000000000000000000000000011a10000000000000000000000000000000022210000000000000000000000000000000032a100000000000000000000000000000000430073a10000000000000000000000000000000054210000000000000000000000000000000064a100000000000000000000000000000000750095801622001000000000000000000000000000000000000000000000000000000000000006a20020000000000000000000000000000000000000000000000000000000000000070e80778d00f8201200300000000000000000000000000000000000000000000000000000000000000"
	if !bytes.Equal(b, mustHex(golden)) || hex.EncodeToString(hash[:]) != "627f5c0f7604fb823f2e2bb3d0b1200368bc378adfd07270e8a6fd82b300a8ff" {
		t.Fatalf("independent protocol1 golden changed: %x %x", b, hash)
	}
	// Mutating returned wire cannot mutate previously decoded identity material.
	b[5] ^= 1
	if got != want {
		t.Fatal("decoder aliases caller bytes")
	}
	for _, mutate := range []func(*Consent){
		func(c *Consent) { c.ResponseOperationID[15]++ },
		func(c *Consent) { c.StoryIncarnation[15]++ },
		func(c *Consent) { c.OfferRevision++ },
		func(c *Consent) { c.ResponderAccountID[15]++ },
		func(c *Consent) { c.ResponderSessionEpoch++ },
		func(c *Consent) { c.ResponseType = Invite },
		func(c *Consent) { c.PrincipalRequestSHA256[0]++ },
	} {
		c := want
		mutate(&c)
		_, h, e := Encode(c)
		if e != nil || h == hash {
			t.Fatal("authority binding omitted")
		}
	}
}

func TestConsentInvalidSemanticMaterial(t *testing.T) {
	for name, mutate := range map[string]func(*Consent){
		"nil consent":         func(c *Consent) { c.ConsentID = uuid.Nil },
		"nil operation":       func(c *Consent) { c.ResponseOperationID = uuid.Nil },
		"nil story":           func(c *Consent) { c.StoryID = uuid.Nil },
		"nil incarnation":     func(c *Consent) { c.StoryIncarnation = uuid.Nil },
		"zero revision":       func(c *Consent) { c.OfferRevision = 0 },
		"nil author":          func(c *Consent) { c.AuthorProfileID = uuid.Nil },
		"nil account":         func(c *Consent) { c.ResponderAccountID = uuid.Nil },
		"nil responder":       func(c *Consent) { c.ResponderProfileID = uuid.Nil },
		"self response":       func(c *Consent) { c.ResponderProfileID = c.AuthorProfileID },
		"zero epoch":          func(c *Consent) { c.ResponderSessionEpoch = 0 },
		"negative epoch":      func(c *Consent) { c.ResponderSessionEpoch = -1 },
		"unknown type":        func(c *Consent) { c.ResponseType = 3 },
		"missing criteria":    func(c *Consent) { c.CriteriaSHA256 = [32]byte{} },
		"missing offer":       func(c *Consent) { c.OfferPayloadSHA256 = [32]byte{} },
		"missing principal":   func(c *Consent) { c.PrincipalRequestSHA256 = [32]byte{} },
		"no creation":         func(c *Consent) { c.CreatedAtUnixMillis = 0 },
		"expired at creation": func(c *Consent) { c.OfferExpiresAtUnixMillis = c.CreatedAtUnixMillis },
	} {
		t.Run(name, func(t *testing.T) {
			c := fixture()
			mutate(&c)
			b, h, e := Encode(c)
			if e == nil || len(b) != 0 || h != [32]byte{} {
				t.Fatal("invalid material returned authority bytes")
			}
		})
	}
}

func TestConsentRejectsAlternateAndMalformedWire(t *testing.T) {
	b, _, err := Encode(fixture())
	if err != nil {
		t.Fatal(err)
	}
	unknown := protowire.AppendTag(nil, 99, protowire.VarintType)
	unknown = protowire.AppendVarint(unknown, 1)
	cases := map[string][]byte{
		"empty": nil, "oversize": make([]byte, MaxMaterialBytes+1), "truncated": b[:len(b)-1],
		"unknown": append(bytes.Clone(b), unknown...), "duplicate": append(bytes.Clone(b), b[:2]...),
		"reordered":        append(bytes.Clone(b[2:]), b[:2]...),
		"overlong version": append([]byte{8, 0x81, 0}, b[2:]...),
		"wrong protocol":   append([]byte{8, 2}, b[2:]...),
		"wrong wire type":  append([]byte{10, 1, 1}, b[2:]...),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			c, h, e := Decode(data)
			if e == nil || c != (Consent{}) || h != [32]byte{} {
				t.Fatal("untrusted wire accepted or partial material escaped")
			}
		})
	}
}

func mustHex(s string) []byte {
	b, e := hex.DecodeString(s)
	if e != nil {
		panic(e)
	}
	return b
}
