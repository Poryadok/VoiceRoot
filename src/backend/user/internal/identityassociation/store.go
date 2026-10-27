package identityassociation

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

var ErrUnknownProfile = errors.New("user profile association unknown")

type Origin string

const (
	OriginBirth           Origin = "BIRTH"
	OriginRetainedNow     Origin = "RETAINED_NOW"
	OriginVerifiedHistory Origin = "VERIFIED_HISTORY"
)

type Fact struct {
	AssociationID                    uuid.UUID
	Revision, CaptureRevision        uint64
	Origin                           Origin
	CapturedAt                       time.Time
	ReceiptID                        uuid.UUID
	ReceiptDigest                    [32]byte
	ParentHandle                     string
	ParentRevision                   uint64
	ParentState                      ParentState
	ProfileRevision                  uint64
	Deleted, Frozen, AccountInactive bool
	Terminal                         *TerminalEvidence
}

// TerminalEvidence is a User-owned irreversible fact receipt. Auth contributes
// its existing positive parent proof; it is not required to mint these IDs.
type TerminalEvidence struct {
	OperationID   uuid.UUID
	Revision      uint64
	ReceiptID     uuid.UUID
	ReceiptDigest [32]byte
	ObservedAt    time.Time
}

func (Fact) String() string               { return "User canonical association fact [redacted]" }
func (Fact) GoString() string             { return "User canonical association fact [redacted]" }
func (TerminalEvidence) String() string   { return "User canonical terminal evidence [redacted]" }
func (TerminalEvidence) GoString() string { return "User canonical terminal evidence [redacted]" }

// Store is configured with the complete User-only keyset and pinned source
// generation. It neither provisions keys nor invents a generation on recovery.
type Store struct {
	pool       *pgxpool.Pool
	keys       *Keyset
	generation uuid.UUID
}

func NewStore(pool *pgxpool.Pool, keys *Keyset, generation uuid.UUID) (*Store, error) {
	if pool == nil || keys == nil || generation == uuid.Nil {
		return nil, ErrIncomplete
	}
	return &Store{pool: pool, keys: keys, generation: generation}, nil
}

// OwnsPool checks local transaction composition, not origin or recovery authority.
func (s *Store) OwnsPool(pool *pgxpool.Pool) bool { return s != nil && s.pool == pool }

// PrepareBirthSelectors keeps User keys local and runs before acquiring I.
func (s *Store) PrepareBirthSelectors(profile, account uuid.UUID) (PreparedSelectors, error) {
	if s == nil || s.keys == nil {
		return PreparedSelectors{}, ErrIncomplete
	}
	return s.keys.PrepareSelectors(profile, account)
}
func (s *Store) Begin(ctx context.Context) (*IdentityTx, error) {
	if s == nil {
		return nil, ErrIncomplete
	}
	return BeginIdentityTx(ctx, s.pool, &ExpectedFence{Generation: s.generation, KeysetRevision: s.keys.snapshot.Revision})
}
