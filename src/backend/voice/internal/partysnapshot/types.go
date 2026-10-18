// Package partysnapshot derives A3 parties from native Voice authority.
package partysnapshot

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"voice/backend/pkg/partyauthority"
)

var (
	ErrUnavailable   = errors.New("Voice party authority unavailable")
	ErrStaleSubject  = errors.New("Voice party subject epoch is stale")
	ErrIneligible    = errors.New("profile is not eligible for a Voice party")
	ErrSourceChanged = errors.New("native Voice source changed")
)

type Subject struct {
	SearchOperationID uuid.UUID
	AccountID         uuid.UUID
	ProfileID         uuid.UUID
	SessionEpoch      int64
}

type Disposition string

const (
	Absent     Disposition = "ABSENT"
	Present    Disposition = "PRESENT"
	Unresolved Disposition = "UNRESOLVED"
)

type MemberState string

const (
	Joined       MemberState = "JOINED"
	Reconnecting MemberState = "RECONNECTING"
	Joining      MemberState = "JOINING"
	Leaving      MemberState = "LEAVING"
	Left         MemberState = "LEFT"
)

// Member keeps native admission evidence; no epoch is inferred from Auth floors.
type Member struct {
	AccountID   uuid.UUID
	ProfileID   uuid.UUID
	OperationID uuid.UUID
	// Space generations are opaque native TEXT; ordinary UUIDs retain their
	// canonical string representation. Never synthesize a replacement ID.
	Generation         string
	SessionEpoch       int64
	State              MemberState
	ReconnectStartedAt time.Time
	ReconnectDeadline  time.Time
}

// Candidate comes from one caller-owned READ ONLY REPEATABLE READ transaction.
// Fingerprint includes native room/head, mapping/fence and unresolved-source
// witnesses, not just this eligible roster. DatabaseTime is sampled after reads.
type Candidate struct {
	Coverage      CoverageCut
	Disposition   Disposition
	SourceKind    string
	RoomID        uuid.UUID
	Incarnation   uuid.UUID
	Purpose       string
	RoomState     string
	RosterVersion uint64
	Members       []Member
	Fingerprint   [32]byte
	DatabaseTime  time.Time
}

// CoverageCut identifies an owning durable cold-cut certificate verified in the
// same native transaction. It is never an operator-ready boolean or Redis hint.
// Writer/schema/catalog/fence/drain bindings are verified by ReadCoverage.
type CoverageCut struct {
	CoverageID           uuid.UUID
	AuthorityIncarnation uuid.UUID
	CoverageEpoch        uint64
	HeadRevision         uint64
	ReceiptHash          [32]byte
}

// Reader closes its short transaction before returning; it performs no Auth IO.
type Reader interface {
	Read(context.Context, Subject) (Candidate, error)
}

type FloorReader interface {
	CurrentFloor(context.Context, string) (int64, error)
}

type LossTarget struct {
	SourceKind  string
	RoomID      uuid.UUID
	Incarnation uuid.UUID
	Member      Member
	Reason      string
}

// Normalizer uses generation-bound native mutations, including their roster
// counter/outbox and exact cleanup. An observed unknown epoch is never a loss.
type Normalizer interface {
	ApplyEligibilityLoss(context.Context, LossTarget) error
}

type Snapshot struct {
	MemberAuthorityVersion uint32
	MemberAuthorities      []partyauthority.Member
	SourceAuthorityHash    [32]byte
	ProtocolVersion        uint32
	Kind                   string
	RoomID                 uuid.UUID
	RosterVersion          uint64
	MemberProfileIDs       []uuid.UUID
}

type Engine struct {
	Reader     Reader
	Floors     FloorReader
	Normalizer Normalizer
	Clock      func() time.Time
}
