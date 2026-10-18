// Package partysource validates persisted Voice party captures with fresh authority.
package partysource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
	"math"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/partyauthority"
)

var ErrUnknown = errors.New("party snapshot authority unavailable")
var ErrIneligible = errors.New("party snapshot subject definitively ineligible")

type Subject struct {
	SearchOperationID, AccountID, ProfileID uuid.UUID
	SessionEpoch                            int64
}
type Snapshot struct {
	ProtocolVersion        uint32
	Kind                   string
	RoomID                 *uuid.UUID
	RosterVersion          uint64
	MemberProfileIDs       []uuid.UUID
	MemberAuthorityVersion uint32
	MemberAuthorities      []partyauthority.Member
	SourceAuthorityHash    [32]byte
}
type SnapshotReader interface {
	Snapshot(context.Context, Subject) (Snapshot, error)
}
type Validation uint8

const (
	ValidationUnknown Validation = iota
	ValidationCurrent
	ValidationChanged
)

type Revalidator struct{ Snapshots SnapshotReader }

func (r *Revalidator) RevalidateCapturedParty(ctx context.Context, p store.SourceParty) (Validation, error) {
	if r == nil || r.Snapshots == nil || p.ID == uuid.Nil || p.SearchOperationID == uuid.Nil || p.SearchRequestHash == [32]byte{} || p.InitiatorAccountID == uuid.Nil || p.LeaderProfileID == uuid.Nil || p.InitiatorSessionEpoch <= 0 || p.DisbandedAt != nil {
		return ValidationUnknown, ErrUnknown
	}
	subject := Subject{p.SearchOperationID, p.InitiatorAccountID, p.LeaderProfileID, p.InitiatorSessionEpoch}
	captured := Snapshot{ProtocolVersion: p.ProtocolVersion, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion, MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}
	hash, err := validateSnapshot(captured, p.LeaderProfileID)
	if err != nil || hash != p.MemberManifestHash {
		return ValidationUnknown, ErrUnknown
	}
	authorityHash, err := validateMemberAuthority(captured, subject)
	if err != nil || authorityHash != p.MemberAuthorityHash {
		return ValidationUnknown, ErrUnknown
	}
	fresh, err := r.Snapshots.Snapshot(ctx, subject)
	if errors.Is(err, ErrIneligible) {
		return ValidationChanged, nil
	}
	if err != nil {
		return ValidationUnknown, errors.Join(ErrUnknown, err)
	}
	freshHash, err := validateSnapshot(fresh, p.LeaderProfileID)
	if err != nil {
		return ValidationUnknown, ErrUnknown
	}
	freshAuthorityHash, err := validateMemberAuthority(fresh, subject)
	if err != nil {
		return ValidationUnknown, ErrUnknown
	}
	if fresh.ProtocolVersion != p.ProtocolVersion || fresh.Kind != p.SourceKind || fresh.RosterVersion != p.SourceRosterVersion || !sameRoom(fresh.RoomID, p.SourceRoomID) || freshHash != p.MemberManifestHash || fresh.MemberAuthorityVersion != p.MemberAuthorityVersion || freshAuthorityHash != p.MemberAuthorityHash || fresh.SourceAuthorityHash != p.SourceAuthorityHash {
		return ValidationChanged, nil
	}
	return ValidationCurrent, nil
}

func validateMemberAuthority(s Snapshot, subject Subject) ([32]byte, error) {
	if subject.SearchOperationID == uuid.Nil || subject.AccountID == uuid.Nil || subject.ProfileID == uuid.Nil || subject.SessionEpoch <= 0 || s.MemberAuthorityVersion != partyauthority.Version || s.SourceAuthorityHash == [32]byte{} || len(s.MemberAuthorities) != len(s.MemberProfileIDs) {
		return [32]byte{}, ErrUnknown
	}
	_, hash, err := partyauthority.Encode(s.MemberAuthorities)
	if err != nil {
		return [32]byte{}, ErrUnknown
	}
	found := false
	for i, member := range s.MemberAuthorities {
		if member.ProfileID != s.MemberProfileIDs[i] {
			return [32]byte{}, ErrUnknown
		}
		if member.ProfileID == subject.ProfileID {
			found = true
			if member.AccountID != subject.AccountID || (s.Kind == "SOLO" && member.SessionEpoch != subject.SessionEpoch) {
				return [32]byte{}, ErrUnknown
			}
		}
	}
	if !found {
		return [32]byte{}, ErrUnknown
	}
	return hash, nil
}

func validateSnapshot(s Snapshot, initiator uuid.UUID) ([32]byte, error) {
	if s.ProtocolVersion != 1 || initiator == uuid.Nil || len(s.MemberProfileIDs) == 0 {
		return [32]byte{}, ErrUnknown
	}
	if s.Kind == "SOLO" {
		if s.RoomID != nil || s.RosterVersion != 0 || len(s.MemberProfileIDs) != 1 {
			return [32]byte{}, ErrUnknown
		}
	} else if s.Kind != "VOICE_ROSTER" || s.RoomID == nil || *s.RoomID == uuid.Nil || s.RosterVersion == 0 || s.RosterVersion > math.MaxInt64 {
		return [32]byte{}, ErrUnknown
	}
	raw := make([]byte, 0, len(s.MemberProfileIDs)*16)
	found := false
	for i, id := range s.MemberProfileIDs {
		if id == uuid.Nil || (i > 0 && bytes.Compare(s.MemberProfileIDs[i-1][:], id[:]) >= 0) {
			return [32]byte{}, ErrUnknown
		}
		found = found || id == initiator
		raw = append(raw, id[:]...)
	}
	if !found {
		return [32]byte{}, ErrUnknown
	}
	return sha256.Sum256(raw), nil
}
func sameRoom(a, b *uuid.UUID) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
