package partysnapshot

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"github.com/google/uuid"
	"sort"
	"unicode/utf8"
	"voice/backend/pkg/partyauthority"
)

// buildSnapshot exports only the final validated cut. Native journal witnesses
// remain private to Voice, while the opaque digest binds their exact identity.
func buildSnapshot(subject Subject, c Candidate) (Snapshot, error) {
	if subject.SearchOperationID == uuid.Nil || subject.AccountID == uuid.Nil || subject.ProfileID == uuid.Nil || subject.SessionEpoch <= 0 {
		return Snapshot{}, ErrUnavailable
	}
	members, err := eligible(c, subject)
	if err != nil {
		return Snapshot{}, err
	}
	kind, source := byte(2), byte(1)
	result := Snapshot{ProtocolVersion: 1, Kind: "VOICE_ROSTER", RoomID: c.RoomID, RosterVersion: c.RosterVersion, MemberAuthorityVersion: partyauthority.Version}
	if c.Disposition == Absent {
		kind, source = 1, 0
		result.Kind = "SOLO"
		members = []Member{{AccountID: subject.AccountID, ProfileID: subject.ProfileID, SessionEpoch: subject.SessionEpoch}}
	} else if c.SourceKind == "SPACE" {
		source = 2
	}
	members = append([]Member(nil), members...)
	sort.Slice(members, func(i, j int) bool { return bytes.Compare(members[i].ProfileID[:], members[j].ProfileID[:]) < 0 })
	for _, m := range members {
		if !utf8.ValidString(m.Generation) || uint64(len(m.Generation)) > uint64(^uint32(0)) {
			return Snapshot{}, ErrUnavailable
		}
		result.MemberProfileIDs = append(result.MemberProfileIDs, m.ProfileID)
		result.MemberAuthorities = append(result.MemberAuthorities, partyauthority.Member{ProfileID: m.ProfileID, AccountID: m.AccountID, SessionEpoch: m.SessionEpoch})
	}
	if _, _, err := partyauthority.Encode(result.MemberAuthorities); err != nil {
		return Snapshot{}, ErrUnavailable
	}
	raw := []byte("voice.mm-party.member-authority.v1\x00")
	raw = binary.BigEndian.AppendUint32(raw, 1)
	raw = append(raw, subject.SearchOperationID[:]...)
	raw = append(raw, subject.AccountID[:]...)
	raw = append(raw, subject.ProfileID[:]...)
	raw = binary.BigEndian.AppendUint64(raw, uint64(subject.SessionEpoch))
	raw = append(raw, kind, source)
	raw = append(raw, c.RoomID[:]...)
	raw = append(raw, c.Incarnation[:]...)
	raw = binary.BigEndian.AppendUint64(raw, c.RosterVersion)
	raw = append(raw, c.Coverage.CoverageID[:]...)
	raw = append(raw, c.Coverage.AuthorityIncarnation[:]...)
	raw = binary.BigEndian.AppendUint64(raw, c.Coverage.CoverageEpoch)
	raw = binary.BigEndian.AppendUint64(raw, c.Coverage.HeadRevision)
	raw = append(raw, c.Coverage.ReceiptHash[:]...)
	raw = binary.BigEndian.AppendUint32(raw, uint32(len(members)))
	for _, m := range members {
		raw = append(raw, m.ProfileID[:]...)
		raw = append(raw, m.AccountID[:]...)
		raw = binary.BigEndian.AppendUint64(raw, uint64(m.SessionEpoch))
		raw = append(raw, m.OperationID[:]...)
		raw = binary.BigEndian.AppendUint32(raw, uint32(len(m.Generation)))
		raw = append(raw, []byte(m.Generation)...)
	}
	result.SourceAuthorityHash = sha256.Sum256(raw)
	return result, nil
}
