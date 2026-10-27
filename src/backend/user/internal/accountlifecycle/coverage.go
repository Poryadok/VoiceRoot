// Package accountlifecycle defines User-owned producer integrity checks.
// A valid digest is not authority, enumeration completeness or an admission grant.
package accountlifecycle

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"
	"unicode/utf8"
)

var ErrInvalidCoverage = errors.New("invalid account lifecycle coverage")

type Schedule struct {
	Fence, Cycle         string
	Revision             uint64
	ScheduledAt, PurgeAt time.Time
}

type Coverage struct {
	SchemaVersion uint32
	Schedule      Schedule
	Revision      uint64
	Complete      bool
	ProfileCount  uint64
	Profiles      []string
}

// Digest computes the agreed v1 integrity binding, including incomplete records.
func Digest(c Coverage) ([32]byte, error) {
	s := c.Schedule
	validText := func(s string) bool { return s != "" && uint64(len(s)) <= uint64(^uint32(0)) && utf8.ValidString(s) }
	if c.SchemaVersion != 1 || !validText(s.Fence) || !validText(s.Cycle) || s.Revision == 0 || c.Revision == 0 ||
		s.ScheduledAt.IsZero() || !s.PurgeAt.Equal(s.ScheduledAt.Add(30*24*time.Hour)) || c.ProfileCount != uint64(len(c.Profiles)) {
		return [32]byte{}, ErrInvalidCoverage
	}
	for i, id := range c.Profiles {
		if !canonicalProfileID(id) || (i > 0 && c.Profiles[i-1] >= id) {
			return [32]byte{}, ErrInvalidCoverage
		}
	}
	// Keep the agreed framing separate from any transport serialization. No raw
	// roster or full-roster digest is a permanent replay selector.
	h := sha256.New()
	_, _ = h.Write([]byte("voice.account-lifecycle.user-coverage.v1\x00"))
	u32 := func(v uint32) { var b [4]byte; binary.BigEndian.PutUint32(b[:], v); _, _ = h.Write(b[:]) }
	u64 := func(v uint64) { var b [8]byte; binary.BigEndian.PutUint64(b[:], v); _, _ = h.Write(b[:]) }
	text := func(v string) { u32(uint32(len(v))); _, _ = h.Write([]byte(v)) }
	stamp := func(v time.Time) { u64(uint64(v.Unix())); u32(uint32(v.Nanosecond())) }
	u32(c.SchemaVersion)
	text(s.Fence)
	text(s.Cycle)
	u64(s.Revision)
	stamp(s.ScheduledAt)
	stamp(s.PurgeAt)
	u64(c.Revision)
	complete := byte(0)
	if c.Complete {
		complete = 1
	}
	_, _ = h.Write([]byte{complete})
	u64(c.ProfileCount)
	for _, id := range c.Profiles {
		text(id)
	}
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	return digest, nil
}

// ValidateComplete binds a complete record to an independently accepted schedule.
// Callers must still establish protected authority and complete closure provenance.
func ValidateComplete(c Coverage, expected Schedule, digest [32]byte) error {
	s := c.Schedule
	if !c.Complete || s.Fence != expected.Fence || s.Cycle != expected.Cycle || s.Revision != expected.Revision ||
		!s.ScheduledAt.Equal(expected.ScheduledAt) || !s.PurgeAt.Equal(expected.PurgeAt) {
		return ErrInvalidCoverage
	}
	actual, err := Digest(c)
	if err != nil || actual != digest {
		return ErrInvalidCoverage
	}
	return nil
}

func canonicalProfileID(id string) bool {
	if len(id) != 36 || id == "00000000-0000-0000-0000-000000000000" {
		return false
	}
	for i, b := range []byte(id) {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if b != '-' {
				return false
			}
		} else if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f')) {
			return false
		}
	}
	return true
}
