package partysnapshot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/voice/internal/ordinarycall"
	"voice/backend/voice/internal/partycoverage"
	"voice/backend/voice/internal/spacemedia"
)

// NativeSource is composed only with the deployment's trusted installation
// binding. This reader neither issues coverage nor qualifies its own factories.
type NativeSource struct{ Binding partycoverage.RuntimeBinding }

func (s *NativeSource) ReadNativeCut(ctx context.Context, tx pgx.Tx, subject Subject) (Candidate, error) {
	if s == nil || tx == nil || subject.SearchOperationID == uuid.Nil || subject.AccountID == uuid.Nil || subject.ProfileID == uuid.Nil || subject.SessionEpoch <= 0 {
		return Candidate{}, ErrUnavailable
	}
	coverage, err := partycoverage.ReadCoverage(ctx, tx, s.Binding)
	if err != nil {
		return Candidate{}, ErrUnavailable
	}
	ordinary, err := ordinarycall.ReadPartyCut(ctx, tx, subject.AccountID, subject.ProfileID)
	if err != nil {
		return Candidate{}, ErrUnavailable
	}
	space, err := spacemedia.ReadPartyCut(ctx, tx, subject.AccountID, subject.ProfileID)
	if err != nil {
		return Candidate{}, ErrUnavailable
	}
	cut := CoverageCut{CoverageID: coverage.CoverageID, AuthorityIncarnation: coverage.AuthorityIncarnation, CoverageEpoch: coverage.CoverageEpoch, HeadRevision: coverage.HeadRevision, ReceiptHash: coverage.ReceiptHash}
	// First derive the exact native members used to select all related accounts.
	native, err := composeNativeCandidate(cut, ordinary, space, foreignCut{Witness: []byte(`{}`)})
	if err != nil {
		return Candidate{}, ErrUnavailable
	}
	foreign, err := readForeignCut(ctx, tx, subject, native)
	if err != nil {
		return Candidate{}, ErrUnavailable
	}
	return composeNativeCandidate(cut, ordinary, space, foreign)
}

func composeNativeCandidate(coverage CoverageCut, ordinary ordinarycall.NativePartyCut, space spacemedia.NativePartyCut, foreign foreignCut) (Candidate, error) {
	if coverage.CoverageID == uuid.Nil || coverage.AuthorityIncarnation == uuid.Nil || coverage.CoverageEpoch == 0 || coverage.HeadRevision == 0 || coverage.ReceiptHash == [32]byte{} || len(ordinary.Witness) == 0 || len(space.Witness) == 0 || len(foreign.Witness) == 0 {
		return Candidate{}, ErrUnavailable
	}
	out := Candidate{Coverage: coverage, Disposition: Absent}
	switch {
	case foreign.Unresolved || (ordinary.Room != nil && space.Head != nil):
		out.Disposition = Unresolved
	case foreign.Purpose != "":
		if ordinary.Room != nil || space.Head != nil || foreign.RoomID == uuid.Nil || (foreign.Purpose != "GAME" && foreign.Purpose != "MATCH_SQUAD") {
			out.Disposition = Unresolved
		} else {
			out.Disposition, out.SourceKind, out.Purpose, out.RoomState, out.RoomID = Present, "FOREIGN", foreign.Purpose, "ACTIVE", foreign.RoomID
		}
	case ordinary.Room != nil:
		r := ordinary.Room
		out.Disposition, out.SourceKind, out.Purpose, out.RoomState = Present, "ORDINARY", "ORDINARY", string(r.Stage)
		out.RoomID, out.Incarnation, out.RosterVersion = r.Intent.RoomID, r.Intent.OperationID, r.RosterVersion
		for _, m := range r.Members {
			var state MemberState
			switch m.State {
			case ordinarycall.MemberPending, ordinarycall.MemberReserved:
				state = Joining
			case ordinarycall.MemberJoined:
				state = Joined
			case ordinarycall.MemberLeaving:
				state = Leaving
			case ordinarycall.MemberLeft, ordinarycall.MemberAborted:
				state = Left
			default:
				return Candidate{}, ErrUnavailable
			}
			out.Members = append(out.Members, Member{AccountID: m.AccountID, ProfileID: m.ProfileID, OperationID: m.FenceOperationID, Generation: m.Generation.String(), SessionEpoch: m.SessionEpoch, State: state})
		}
	case space.Head != nil:
		h := space.Head
		out.Disposition, out.SourceKind, out.Purpose, out.RoomState = Present, "SPACE", "ORDINARY", h.State
		if h.State == "OPEN" {
			out.RoomState = "ACTIVE"
		}
		out.RoomID, out.Incarnation, out.RosterVersion = h.RoomID, h.CreatorOperationID, h.RosterVersion
		for _, m := range space.Members {
			var state MemberState
			switch m.State {
			case "PREPARED":
				state = Joining
			case "ACTIVE":
				state = Joined
			case "REVOKING":
				state = Leaving
			case "ENDED":
				state = Left
			default:
				return Candidate{}, ErrUnavailable
			}
			out.Members = append(out.Members, Member{AccountID: m.AccountID, ProfileID: m.ProfileID, OperationID: m.OperationID, Generation: m.Generation, SessionEpoch: m.SessionEpoch, State: state})
		}
	}
	sort.Slice(out.Members, func(i, j int) bool {
		if out.Members[i].ProfileID != out.Members[j].ProfileID {
			return bytes.Compare(out.Members[i].ProfileID[:], out.Members[j].ProfileID[:]) < 0
		}
		return bytes.Compare(out.Members[i].OperationID[:], out.Members[j].OperationID[:]) < 0
	})
	body, err := json.Marshal(struct {
		Protocol        uint32
		Derived         Candidate
		Ordinary, Space []byte
		Foreign         foreignCut
	}{1, out, ordinary.Witness, space.Witness, foreign})
	if err != nil {
		return Candidate{}, ErrUnavailable
	}
	out.Fingerprint = sha256.Sum256(append([]byte("voice.mm-party.native-cut.v1\x00"), body...))
	return out, nil
}
