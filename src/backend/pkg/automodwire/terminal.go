package automodwire

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"

	"github.com/google/uuid"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

const MaxTerminalSubjects = 50000
const terminalDomain = "voice.automod.terminal-union.v1\x00"

// TerminalDigest hashes a sorted complete UUID set with explicit framing. It
// does not certify independently recovered authority; the protected Auth owner
// establishes that prerequisite before it may issue this view.
func TerminalDigest(subjects []string) ([32]byte, error) {
	if len(subjects) > MaxTerminalSubjects {
		return [32]byte{}, ErrCore
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(terminalDomain))
	var count [8]byte
	binary.BigEndian.PutUint64(count[:], uint64(len(subjects)))
	_, _ = hash.Write(count[:])
	previous := ""
	for _, raw := range subjects {
		if !canonicalUUID(raw) || raw <= previous {
			return [32]byte{}, ErrCore
		}
		id := uuid.MustParse(raw)
		_, _ = hash.Write(id[:])
		previous = raw
	}
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result, nil
}

func ValidateTerminalSnapshot(snapshot *automodv1.ListTerminalModerationSubjectsResponse) error {
	if snapshot == nil || RejectUnknown(snapshot) != nil || !snapshot.GetComplete() {
		return ErrCore
	}
	seal := snapshot.GetSeal()
	if seal == nil || seal.GetProtocolVersion() != 1 || !seal.GetComplete() || !canonicalUUID(seal.GetAuthorityGeneration()) || !canonicalUUID(seal.GetSnapshotId()) || seal.GetCut() == nil || seal.GetCut().CheckValid() != nil || seal.GetCut().GetNanos()%1000 != 0 || seal.GetItemCount() != uint64(len(snapshot.GetSubjectIds())) || seal.GetTerminalHighWater() > seal.GetItemCount() {
		return ErrCore
	}
	digest, err := TerminalDigest(snapshot.GetSubjectIds())
	if err != nil || !bytes.Equal(digest[:], seal.GetCanonicalSha256()) {
		return ErrCore
	}
	return nil
}
