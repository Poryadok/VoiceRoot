// Package automodevidence creates private keyed evidence at the Messaging
// commit boundary. Raw text and URLs never enter the private delivery journal.
package automodevidence

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"github.com/google/uuid"
	"hash"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

// c9 retention upper bound: max(samples 30d+10m, strikes/aliases 30d+30d).
const KeyRetirementHorizon = 60 * 24 * time.Hour

var keyID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var ErrKeyRing = errors.New("invalid or incomplete Messaging automod HMAC key ring")

type Ring struct {
	ids  []string
	keys map[string][]byte
}
type Projection struct {
	Eligibility automodv1.EvidenceEligibility
	Digests     []*automodv1.PatternDigest
	LinkCount   uint32
}

func NewRing(keys map[string][]byte) (*Ring, error) {
	if len(keys) < 2 || len(keys) > 4 {
		return nil, ErrKeyRing
	}
	ring := &Ring{keys: map[string][]byte{}}
	for id, key := range keys {
		if !keyID.MatchString(id) || len(key) != 32 {
			return nil, ErrKeyRing
		}
		for _, known := range ring.keys {
			if bytes.Equal(known, key) {
				return nil, ErrKeyRing
			}
		}
		ring.keys[id] = append([]byte(nil), key...)
		ring.ids = append(ring.ids, id)
	}
	sort.Strings(ring.ids)
	return ring, nil
}
func (r *Ring) KeyIDs() []string {
	if r == nil {
		return nil
	}
	return append([]string(nil), r.ids...)
}
func (r *Ring) KeyFingerprint(id string) ([32]byte, error) {
	if r == nil || r.keys[id] == nil {
		return [32]byte{}, ErrKeyRing
	}
	return sha256.Sum256(r.keys[id]), nil
}
func (r *Ring) Project(subject uuid.UUID, main, commentary string, e2e bool) (Projection, error) {
	if r == nil || len(r.ids) < 2 || subject == uuid.Nil || !utf8.ValidString(main) || !utf8.ValidString(commentary) {
		return Projection{}, ErrKeyRing
	}
	if e2e {
		return Projection{Eligibility: automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_E2E}, nil
	}
	fields := []string{strings.ToLower(strings.TrimSpace(main)), strings.ToLower(strings.TrimSpace(commentary))}
	if fields[0] == "" && fields[1] == "" {
		return Projection{Eligibility: automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_NO_TEXT}, nil
	}
	links := strings.Count(fields[0], "http://") + strings.Count(fields[0], "https://") + strings.Count(fields[1], "http://") + strings.Count(fields[1], "https://")
	if uint64(links) > math.MaxUint32 {
		return Projection{}, errors.New("automod link count exceeds protocol capacity")
	}
	result := Projection{Eligibility: automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_ELIGIBLE, LinkCount: uint32(links)}
	for _, id := range r.ids {
		mac := hmac.New(sha256.New, r.keys[id])
		_, _ = mac.Write([]byte("voice.automod.pattern.v1\x00"))
		_, _ = mac.Write(subject[:])
		for _, field := range fields {
			if uint64(len(field)) > math.MaxUint32 {
				return Projection{}, errors.New("automod text projection exceeds protocol capacity")
			}
			framed(mac, field)
		}
		result.Digests = append(result.Digests, &automodv1.PatternDigest{KeyId: id, HmacSha256: mac.Sum(nil)})
	}
	return result, nil
}
func framed(mac hash.Hash, field string) {
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(field)))
	_, _ = mac.Write(size[:])
	_, _ = mac.Write([]byte(field))
}
func CanRetire(lastSourceAt, now time.Time, pendingOldEvidence bool) bool {
	return !pendingOldEvidence && !lastSourceAt.IsZero() && !now.Before(lastSourceAt.Add(KeyRetirementHorizon))
}
