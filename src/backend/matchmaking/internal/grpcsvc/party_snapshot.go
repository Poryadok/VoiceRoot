package grpcsvc

import (
	"bytes"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PartySnapshotKind is the authoritative Voice classification of the
// initiating profile's pre-match party. It deliberately has no client-facing
// representation: only the future protected Voice RPC may supply it.
type PartySnapshotKind string

const (
	PartySnapshotKindSolo        PartySnapshotKind = "SOLO"
	PartySnapshotKindVoiceRoster PartySnapshotKind = "VOICE_ROSTER"
)

// PartySnapshot is the Matchmaking-owned representation of the frozen Voice
// response. Transport/protected-principal validation remains at the Voice RPC
// boundary; this type only verifies the response shape before persistence.
type PartySnapshot struct {
	ProtocolVersion  uint32
	Kind             PartySnapshotKind
	RoomID           string
	RosterVersion    uint64
	MemberProfileIDs []uuid.UUID
}

func validatePartySnapshot(snapshot PartySnapshot, initiatorProfileID uuid.UUID) error {
	if initiatorProfileID == uuid.Nil {
		return status.Error(codes.FailedPrecondition, "party snapshot initiator invalid")
	}
	if snapshot.ProtocolVersion != 1 {
		return status.Error(codes.FailedPrecondition, "party snapshot protocol version unsupported")
	}

	switch snapshot.Kind {
	case PartySnapshotKindSolo:
		if snapshot.RoomID != "" || snapshot.RosterVersion != 0 || len(snapshot.MemberProfileIDs) != 1 || snapshot.MemberProfileIDs[0] != initiatorProfileID {
			return status.Error(codes.FailedPrecondition, "invalid solo party snapshot")
		}
		return nil
	case PartySnapshotKindVoiceRoster:
		if snapshot.RoomID == "" || snapshot.RosterVersion == 0 || len(snapshot.MemberProfileIDs) == 0 {
			return status.Error(codes.FailedPrecondition, "invalid voice roster party snapshot")
		}
		roomID, err := uuid.Parse(snapshot.RoomID)
		if err != nil || roomID == uuid.Nil {
			return status.Error(codes.FailedPrecondition, "invalid voice roster room id")
		}
		seenInitiator := false
		for i, memberID := range snapshot.MemberProfileIDs {
			if memberID == uuid.Nil {
				return status.Error(codes.FailedPrecondition, "invalid voice roster member")
			}
			if i > 0 && bytes.Compare(snapshot.MemberProfileIDs[i-1][:], memberID[:]) >= 0 {
				return status.Error(codes.FailedPrecondition, "voice roster members not canonical")
			}
			if memberID == initiatorProfileID {
				seenInitiator = true
			}
		}
		if !seenInitiator {
			return status.Error(codes.FailedPrecondition, "party snapshot initiator absent")
		}
		return nil
	default:
		return status.Error(codes.FailedPrecondition, "unknown party snapshot kind")
	}
}
