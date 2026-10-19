package lfpconsent

import (
	"bytes"
	"crypto/sha256"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protowire"
)

// Registration is the original MM-owned search-intent capture identity.
// Its validity never means that a currently pending or accepted party exists.
type Registration struct {
	OperationID, ConsentID     uuid.UUID
	ConsentSHA256              [32]byte
	RegistrationID             uuid.UUID
	Generation                 uint64
	CapturedSearchIntentSHA256 [32]byte
}

const RegistrationDomain = "voice.mm.lfp-consent-registration.v1\x00"

// EncodeRegistration returns exactly deterministic LfpConsentRegistration
// fields1-7, with its self-hash field8 absent. It does not mint MM provenance.
func EncodeRegistration(r Registration) ([]byte, [32]byte, error) {
	if r.OperationID == uuid.Nil || r.ConsentID == uuid.Nil || r.ConsentSHA256 == [32]byte{} ||
		r.RegistrationID == uuid.Nil || r.Generation == 0 || r.CapturedSearchIntentSHA256 == [32]byte{} {
		return nil, [32]byte{}, ErrInvalid
	}
	b := number(nil, 1, ProtocolVersion)
	b = blob(b, 2, r.OperationID[:])
	b = blob(b, 3, r.ConsentID[:])
	b = blob(b, 4, r.ConsentSHA256[:])
	b = blob(b, 5, r.RegistrationID[:])
	b = number(b, 6, r.Generation)
	b = blob(b, 7, r.CapturedSearchIntentSHA256[:])
	h := sha256.New()
	_, _ = h.Write([]byte(RegistrationDomain))
	_, _ = h.Write(b)
	var hash [32]byte
	copy(hash[:], h.Sum(nil))
	return b, hash, nil
}

func DecodeRegistration(data []byte) (Registration, [32]byte, error) {
	if len(data) == 0 || len(data) > MaxMaterialBytes {
		return Registration{}, [32]byte{}, ErrInvalid
	}
	var r Registration
	rest := data
	for field := protowire.Number(1); field <= 7; field++ {
		tag, typ, n := protowire.ConsumeTag(rest)
		if n < 0 || tag != field {
			return Registration{}, [32]byte{}, ErrInvalid
		}
		rest = rest[n:]
		if field == 1 || field == 6 {
			if typ != protowire.VarintType {
				return Registration{}, [32]byte{}, ErrInvalid
			}
			v, n := protowire.ConsumeVarint(rest)
			if n < 0 {
				return Registration{}, [32]byte{}, ErrInvalid
			}
			rest = rest[n:]
			if field == 1 {
				if v != ProtocolVersion {
					return Registration{}, [32]byte{}, ErrInvalid
				}
			} else {
				r.Generation = v
			}
		} else {
			if typ != protowire.BytesType {
				return Registration{}, [32]byte{}, ErrInvalid
			}
			v, n := protowire.ConsumeBytes(rest)
			if n < 0 {
				return Registration{}, [32]byte{}, ErrInvalid
			}
			rest = rest[n:]
			var dst []byte
			switch field {
			case 2:
				dst = r.OperationID[:]
			case 3:
				dst = r.ConsentID[:]
			case 4:
				dst = r.ConsentSHA256[:]
			case 5:
				dst = r.RegistrationID[:]
			case 7:
				dst = r.CapturedSearchIntentSHA256[:]
			}
			if len(v) != len(dst) {
				return Registration{}, [32]byte{}, ErrInvalid
			}
			copy(dst, v)
		}
	}
	canonical, h, e := EncodeRegistration(r)
	if e != nil || len(rest) != 0 || !bytes.Equal(canonical, data) {
		return Registration{}, [32]byte{}, ErrInvalid
	}
	return r, h, nil
}
