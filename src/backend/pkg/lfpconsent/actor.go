package lfpconsent

import (
	"bytes"
	"crypto/sha256"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protowire"
)

const (
	PublishOffer       uint64 = 1
	Respond            uint64 = 2
	Accept             uint64 = 3
	Decline            uint64 = 4
	HumanDelegatedUser uint64 = 1
	Regular            uint64 = 1
	Guest              uint64 = 2
	PublishOfferRPC           = "/voice.story.v1.LfpConsentActionService/PublishLfpConsentOffer"
	RespondRPC                = "/voice.story.v1.LfpConsentActionService/RespondWithLfpConsent"
	DecideRPC                 = "/voice.matchmaking.v1.MatchmakingService/DecideLfpRequest"
	ActorDomain               = "voice.lfp.actor-provenance.v1\x00"
)

// Actor is historical trusted-writer data, not independently authenticated JWT
// evidence or a current eligibility permit. It never defaults an unknown type.
type Actor struct {
	ActionKind, ActorKind, AccountType uint64
	AccountID, ProfileID               uuid.UUID
	SessionEpoch                       int64
	OperationID                        uuid.UUID
	Issuer, Audience, FullRPC          string
	PrincipalRequestSHA256             [32]byte
}

func (Actor) String() string   { return "LFP actor provenance" }
func (Actor) GoString() string { return "LFP actor provenance" }

func (a Actor) valid() bool {
	if a.ActorKind != HumanDelegatedUser || (a.AccountType != Regular && a.AccountType != Guest) || a.AccountID == uuid.Nil || a.ProfileID == uuid.Nil || a.SessionEpoch <= 0 || a.OperationID == uuid.Nil || a.Issuer != "gateway" || a.PrincipalRequestSHA256 == [32]byte{} {
		return false
	}
	switch a.ActionKind {
	case PublishOffer:
		return a.Audience == "story" && a.FullRPC == PublishOfferRPC
	case Respond:
		return a.Audience == "story" && a.FullRPC == RespondRPC
	case Accept, Decline:
		return a.Audience == "matchmaking" && a.FullRPC == DecideRPC
	default:
		return false
	}
}

func EncodeActor(a Actor) ([]byte, [32]byte, error) {
	if !a.valid() {
		return nil, [32]byte{}, ErrInvalid
	}
	b := number(nil, 1, ProtocolVersion)
	b = number(b, 2, a.ActionKind)
	b = number(b, 3, a.ActorKind)
	b = number(b, 4, a.AccountType)
	b = blob(b, 5, a.AccountID[:])
	b = blob(b, 6, a.ProfileID[:])
	b = number(b, 7, uint64(a.SessionEpoch))
	b = blob(b, 8, a.OperationID[:])
	b = blob(b, 9, []byte(a.Issuer))
	b = blob(b, 10, []byte(a.Audience))
	b = blob(b, 11, []byte(a.FullRPC))
	b = blob(b, 12, a.PrincipalRequestSHA256[:])
	return b, materialHash(ActorDomain, b), nil
}

func DecodeActor(data []byte) (Actor, [32]byte, error) {
	empty := func() (Actor, [32]byte, error) { return Actor{}, [32]byte{}, ErrInvalid }
	if len(data) == 0 || len(data) > MaxMaterialBytes {
		return empty()
	}
	var a Actor
	rest := data
	for field := protowire.Number(1); field <= 12; field++ {
		tag, typ, n := protowire.ConsumeTag(rest)
		if n < 0 || tag != field {
			return empty()
		}
		rest = rest[n:]
		if field <= 4 || field == 7 {
			if typ != protowire.VarintType {
				return empty()
			}
			v, n := protowire.ConsumeVarint(rest)
			if n < 0 {
				return empty()
			}
			rest = rest[n:]
			switch field {
			case 1:
				if v != ProtocolVersion {
					return empty()
				}
			case 2:
				a.ActionKind = v
			case 3:
				a.ActorKind = v
			case 4:
				a.AccountType = v
			case 7:
				a.SessionEpoch = int64(v)
			}
		} else {
			if typ != protowire.BytesType {
				return empty()
			}
			v, n := protowire.ConsumeBytes(rest)
			if n < 0 {
				return empty()
			}
			rest = rest[n:]
			var dst []byte
			switch field {
			case 5:
				dst = a.AccountID[:]
			case 6:
				dst = a.ProfileID[:]
			case 8:
				dst = a.OperationID[:]
			case 9:
				a.Issuer = string(v)
			case 10:
				a.Audience = string(v)
			case 11:
				a.FullRPC = string(v)
			case 12:
				dst = a.PrincipalRequestSHA256[:]
			}
			if dst != nil {
				if len(v) != len(dst) {
					return empty()
				}
				copy(dst, v)
			}
		}
	}
	b, h, e := EncodeActor(a)
	if e != nil || len(rest) != 0 || !bytes.Equal(b, data) {
		return empty()
	}
	return a, h, nil
}

func materialHash(domain string, b []byte) [32]byte {
	h := sha256.New()
	_, _ = h.Write([]byte(domain))
	_, _ = h.Write(b)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
