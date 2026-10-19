package lfpconsentwire

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/lfpconsent"
)

func TestGeneratedActorMatchesCanonicalCodec(t *testing.T) {
	for _, kind := range []uint64{lfpconsent.PublishOffer, lfpconsent.Respond, lfpconsent.Accept, lfpconsent.Decline} {
		for _, accountType := range []uint64{lfpconsent.Regular, lfpconsent.Guest} {
			a := lfpconsent.Actor{ActionKind: kind, ActorKind: lfpconsent.HumanDelegatedUser, AccountType: accountType, AccountID: uuid.New(), ProfileID: uuid.New(), SessionEpoch: 9, OperationID: uuid.New(), Issuer: "gateway", Audience: "story", FullRPC: lfpconsent.RespondRPC, PrincipalRequestSHA256: [32]byte{3}}
			switch kind {
			case lfpconsent.PublishOffer:
				a.FullRPC = lfpconsent.PublishOfferRPC
			case lfpconsent.Accept, lfpconsent.Decline:
				a.Audience = "matchmaking"
				a.FullRPC = lfpconsent.DecideRPC
			}
			b, h, e := lfpconsent.EncodeActor(a)
			if e != nil {
				t.Fatal(e)
			}
			m := &storyv1.LfpActorProvenanceMaterial{ProtocolVersion: 1, ActionKind: storyv1.LfpActorActionKind(a.ActionKind), ActorKind: storyv1.LfpActorKind(a.ActorKind), AccountType: storyv1.LfpActorAccountType(a.AccountType), AccountId: a.AccountID[:], ProfileId: a.ProfileID[:], SessionEpoch: a.SessionEpoch, ActionOperationId: a.OperationID[:], Issuer: a.Issuer, Audience: a.Audience, FullRpc: a.FullRPC, PrincipalRequestSha256: a.PrincipalRequestSHA256[:]}
			generated, e := proto.MarshalOptions{Deterministic: true}.Marshal(m)
			if e != nil || !bytes.Equal(b, generated) {
				t.Fatalf("actor descriptor drift: %v", e)
			}
			got, hh, e := lfpconsent.DecodeActor(generated)
			if e != nil || got != a || hh != h {
				t.Fatal("generated actor roundtrip differs")
			}
		}
	}
}
