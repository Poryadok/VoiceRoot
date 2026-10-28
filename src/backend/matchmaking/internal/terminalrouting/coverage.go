package terminalrouting

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrCoverageIncomplete = errors.New("terminal routing coverage incomplete")

// Coverage is a local verified-fact input, not a producer transport record.
// Completeness must come from User's protected attestation, never enumeration by
// the consumer. The raw roster and its digest are erased at their bound.
type Coverage struct {
	Schedule Schedule
	Revision uint64
	Complete bool
	Profiles []uuid.UUID
	SHA256   [sha256.Size]byte
}

func (c Coverage) Digest() ([sha256.Size]byte, error) {
	if !validSchedule(c.Schedule) || c.Revision == 0 {
		return [sha256.Size]byte{}, ErrAuthorityInvalid
	}
	var b bytes.Buffer
	b.WriteString("voice.account-lifecycle.user-coverage.v1\x00")
	_ = binary.Write(&b, binary.BigEndian, uint32(1))
	writeText := func(s string) {
		_ = binary.Write(&b, binary.BigEndian, uint32(len(s)))
		b.WriteString(s)
	}
	writeTime := func(t time.Time) {
		_ = binary.Write(&b, binary.BigEndian, t.Unix())
		_ = binary.Write(&b, binary.BigEndian, uint32(t.Nanosecond()))
	}
	writeText(c.Schedule.FenceID)
	writeText(c.Schedule.CycleID)
	_ = binary.Write(&b, binary.BigEndian, c.Schedule.Revision)
	writeTime(c.Schedule.ScheduledAt)
	writeTime(c.Schedule.PurgeAt)
	_ = binary.Write(&b, binary.BigEndian, c.Revision)
	if c.Complete {
		b.WriteByte(1)
	} else {
		b.WriteByte(0)
	}
	_ = binary.Write(&b, binary.BigEndian, uint64(len(c.Profiles)))
	var previous string
	for _, profile := range c.Profiles {
		id := profile.String()
		if profile == uuid.Nil || (previous != "" && previous >= id) {
			return [sha256.Size]byte{}, ErrAuthorityInvalid
		}
		writeText(id)
		previous = id
	}
	return sha256.Sum256(b.Bytes()), nil
}

func (c Coverage) Validate() error {
	if !c.Complete {
		return ErrCoverageIncomplete
	}
	digest, err := c.Digest()
	if err != nil {
		return err
	}
	if digest != c.SHA256 {
		return ErrAuthorityConflict
	}
	return nil
}
