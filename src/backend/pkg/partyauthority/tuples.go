// Package partyauthority encodes private native party identity evidence. It must
// never be serialized into public Matchmaking responses, events or log fields.
package partyauthority

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"github.com/google/uuid"
)

var ErrInvalid = errors.New("invalid private party member authority")

const Version uint32 = 1

type Member struct {
	ProfileID, AccountID uuid.UUID
	SessionEpoch         int64
}

func Encode(members []Member) ([]byte, [32]byte, error) {
	if len(members) == 0 || uint64(len(members)) > uint64(^uint32(0)) || len(members) > (int(^uint(0)>>1)-8)/40 {
		return nil, [32]byte{}, ErrInvalid
	}
	accounts := make(map[uuid.UUID]bool, len(members))
	for i, m := range members {
		if m.ProfileID == uuid.Nil || m.AccountID == uuid.Nil || m.SessionEpoch <= 0 || accounts[m.AccountID] || (i > 0 && bytes.Compare(members[i-1].ProfileID[:], m.ProfileID[:]) >= 0) {
			return nil, [32]byte{}, ErrInvalid
		}
		accounts[m.AccountID] = true
	}
	raw := make([]byte, 8+40*len(members))
	binary.BigEndian.PutUint32(raw[:4], Version)
	binary.BigEndian.PutUint32(raw[4:8], uint32(len(members)))
	for i, m := range members {
		at := 8 + i*40
		copy(raw[at:at+16], m.ProfileID[:])
		copy(raw[at+16:at+32], m.AccountID[:])
		binary.BigEndian.PutUint64(raw[at+32:at+40], uint64(m.SessionEpoch))
	}
	h := sha256.New()
	_, _ = h.Write([]byte("voice.mm-party.member-tuples.v1\x00"))
	_, _ = h.Write(raw)
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	return raw, digest, nil
}
func Decode(raw []byte, expected [32]byte) ([]Member, error) {
	if len(raw) < 8 || binary.BigEndian.Uint32(raw[:4]) != Version {
		return nil, ErrInvalid
	}
	count := binary.BigEndian.Uint32(raw[4:8])
	if count == 0 || 8+uint64(count)*40 != uint64(len(raw)) {
		return nil, ErrInvalid
	}
	members := make([]Member, int(count))
	for i := range members {
		at := 8 + i*40
		copy(members[i].ProfileID[:], raw[at:at+16])
		copy(members[i].AccountID[:], raw[at+16:at+32])
		epoch := binary.BigEndian.Uint64(raw[at+32 : at+40])
		if epoch == 0 || epoch > uint64(^uint64(0)>>1) {
			return nil, ErrInvalid
		}
		members[i].SessionEpoch = int64(epoch)
	}
	canonical, digest, err := Encode(members)
	if err != nil || digest != expected || !bytes.Equal(canonical, raw) {
		return nil, ErrInvalid
	}
	return members, nil
}
