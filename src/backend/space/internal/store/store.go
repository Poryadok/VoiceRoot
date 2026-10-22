package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type spaceStoreDB interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// SpaceStore persists spaces and memberships in space_db.
type SpaceStore struct {
	Pool *pgxpool.Pool

	// AccountCaptures is the purpose-specific protected User owner transport.
	// Ordinary admission/current-pair clients cannot satisfy this dependency.
	AccountCaptures         SpaceAccountCaptureClient
	communityAccountCapture *communityAccountCapture
	tx                      pgx.Tx

	// legacyOwnershipLease is restricted to the disabled private v1 handler,
	// which already holds the same Space advisory key as a session lock.
	legacyOwnershipLease bool

	// voiceRoomAccessQuery is a test seam for the single exact resolver query.
	voiceRoomAccessQuery voiceRoomAccessQuerier
}

// LegacyOwnershipLeaseStoreForTest returns an isolated view for the disabled
// private v1 ownership saga after its session lease has been acquired. Direct
// callers still use the ordinary R20 transaction scope.
func (s *SpaceStore) LegacyOwnershipLeaseStoreForTest() *SpaceStore {
	if s == nil {
		return nil
	}
	legacy := *s
	legacy.legacyOwnershipLease = true
	return &legacy
}

func (s *SpaceStore) db() spaceStoreDB {
	if s != nil && s.tx != nil {
		return s.tx
	}
	if s == nil {
		return nil
	}
	return s.Pool
}
