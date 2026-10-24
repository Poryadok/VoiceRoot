// Package lfpconsent defines private immutable LFP receipt material. A valid
// encoding is data, never proof of authentication or current eligibility.
package lfpconsent

import (
	"bytes"
	"crypto/sha256"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protowire"
)

var ErrInvalid = errors.New("invalid LFP consent material")

const (
	ProtocolVersion  uint64 = 1
	Join             uint64 = 1
	Invite           uint64 = 2
	MaxMaterialBytes        = 2048
)

// Consent matches voice.story.v1.LfpConsentMaterial field numbers exactly.
// Fixed arrays prevent aliases to mutable caller buffers and keep identity private.
type Consent struct {
	ConsentID, ResponseOperationID, StoryID, StoryIncarnation uuid.UUID
	OfferRevision                                             uint64
	AuthorProfileID, ResponderAccountID, ResponderProfileID   uuid.UUID
	ResponderSessionEpoch                                     int64
	ResponseType                                              uint64
	CriteriaSHA256, OfferPayloadSHA256                        [32]byte
	CreatedAtUnixMillis, OfferExpiresAtUnixMillis             int64
	PrincipalRequestSHA256                                    [32]byte
}

// ConsentDomain is independent of source/registration/request hash domains.
const ConsentDomain = "voice.story.lfp-consent-receipt.v1\x00"

func (c Consent) valid() bool {
	return c.ConsentID != uuid.Nil && c.ResponseOperationID != uuid.Nil &&
		c.StoryID != uuid.Nil && c.StoryIncarnation != uuid.Nil && c.OfferRevision > 0 &&
		c.AuthorProfileID != uuid.Nil && c.ResponderAccountID != uuid.Nil &&
		c.ResponderProfileID != uuid.Nil && c.AuthorProfileID != c.ResponderProfileID &&
		c.ResponderSessionEpoch > 0 && (c.ResponseType == Join || c.ResponseType == Invite) &&
		c.CriteriaSHA256 != [32]byte{} && c.OfferPayloadSHA256 != [32]byte{} &&
		c.PrincipalRequestSHA256 != [32]byte{} && c.CreatedAtUnixMillis > 0 &&
		c.OfferExpiresAtUnixMillis > c.CreatedAtUnixMillis
}

// Encode emits the deterministic protobuf encoding of LfpConsentMaterial.
// All fields are required by protocol1; no repeated/map/optional fields exist.
func Encode(c Consent) ([]byte, [32]byte, error) {
	if !c.valid() {
		return nil, [32]byte{}, ErrInvalid
	}
	b := make([]byte, 0, 320)
	b = number(b, 1, ProtocolVersion)
	b = blob(b, 2, c.ConsentID[:])
	b = blob(b, 3, c.ResponseOperationID[:])
	b = blob(b, 4, c.StoryID[:])
	b = blob(b, 5, c.StoryIncarnation[:])
	b = number(b, 6, c.OfferRevision)
	b = blob(b, 7, c.AuthorProfileID[:])
	b = blob(b, 8, c.ResponderAccountID[:])
	b = blob(b, 9, c.ResponderProfileID[:])
	b = number(b, 10, uint64(c.ResponderSessionEpoch))
	b = number(b, 11, c.ResponseType)
	b = blob(b, 12, c.CriteriaSHA256[:])
	b = blob(b, 13, c.OfferPayloadSHA256[:])
	b = number(b, 14, uint64(c.CreatedAtUnixMillis))
	b = number(b, 15, uint64(c.OfferExpiresAtUnixMillis))
	b = blob(b, 16, c.PrincipalRequestSHA256[:])
	return b, digest(b), nil
}

// Decode rejects unknown, duplicate, reordered, overlong and partial material.
// It returns no material/hash on failure. It does not check freshness at now:
// historical receipt equality must remain possible after natural offer expiry.
func Decode(data []byte) (Consent, [32]byte, error) {
	if len(data) == 0 || len(data) > MaxMaterialBytes {
		return Consent{}, [32]byte{}, ErrInvalid
	}
	var c Consent
	rest := data
	for field := protowire.Number(1); field <= 16; field++ {
		tag, typ, n := protowire.ConsumeTag(rest)
		if n < 0 || tag != field {
			return Consent{}, [32]byte{}, ErrInvalid
		}
		rest = rest[n:]
		switch field {
		case 1, 6, 10, 11, 14, 15:
			if typ != protowire.VarintType {
				return Consent{}, [32]byte{}, ErrInvalid
			}
			v, n := protowire.ConsumeVarint(rest)
			if n < 0 {
				return Consent{}, [32]byte{}, ErrInvalid
			}
			rest = rest[n:]
			switch field {
			case 1:
				if v != ProtocolVersion {
					return Consent{}, [32]byte{}, ErrInvalid
				}
			case 6:
				c.OfferRevision = v
			case 10:
				c.ResponderSessionEpoch = int64(v)
			case 11:
				c.ResponseType = v
			case 14:
				c.CreatedAtUnixMillis = int64(v)
			case 15:
				c.OfferExpiresAtUnixMillis = int64(v)
			}
		default:
			if typ != protowire.BytesType {
				return Consent{}, [32]byte{}, ErrInvalid
			}
			v, n := protowire.ConsumeBytes(rest)
			if n < 0 {
				return Consent{}, [32]byte{}, ErrInvalid
			}
			rest = rest[n:]
			var dst []byte
			switch field {
			case 2:
				dst = c.ConsentID[:]
			case 3:
				dst = c.ResponseOperationID[:]
			case 4:
				dst = c.StoryID[:]
			case 5:
				dst = c.StoryIncarnation[:]
			case 7:
				dst = c.AuthorProfileID[:]
			case 8:
				dst = c.ResponderAccountID[:]
			case 9:
				dst = c.ResponderProfileID[:]
			case 12:
				dst = c.CriteriaSHA256[:]
			case 13:
				dst = c.OfferPayloadSHA256[:]
			case 16:
				dst = c.PrincipalRequestSHA256[:]
			}
			if len(v) != len(dst) {
				return Consent{}, [32]byte{}, ErrInvalid
			}
			copy(dst, v)
		}
	}
	if len(rest) != 0 {
		return Consent{}, [32]byte{}, ErrInvalid
	}
	canonical, h, err := Encode(c)
	if err != nil || !bytes.Equal(canonical, data) {
		return Consent{}, [32]byte{}, ErrInvalid
	}
	return c, h, nil
}

func number(b []byte, n protowire.Number, v uint64) []byte {
	return protowire.AppendVarint(protowire.AppendTag(b, n, protowire.VarintType), v)
}
func blob(b []byte, n protowire.Number, v []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(b, n, protowire.BytesType), v)
}
func digest(b []byte) [32]byte {
	h := sha256.New()
	_, _ = h.Write([]byte(ConsentDomain))
	_, _ = h.Write(b)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
