package store

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrPlatformBanDenied      = errors.New("party matchmaking unavailable")
	ErrPlatformBanUnavailable = errors.New("platform ban authority unavailable")
)

// PlatformBanDenial carries private local evidence for whole-party reset. Its
// accounts must never enter public errors, responses, events or log fields.
type PlatformBanDenial struct{ accounts []uuid.UUID }

func (d *PlatformBanDenial) Error() string        { return ErrPlatformBanDenied.Error() }
func (d *PlatformBanDenial) Is(target error) bool { return target == ErrPlatformBanDenied }
func (d *PlatformBanDenial) Accounts() []uuid.UUID {
	if d == nil {
		return nil
	}
	return append([]uuid.UUID(nil), d.accounts...)
}

// PlatformBanGuard retains the account locks until the caller completes its transaction.
type PlatformBanGuard struct {
	tx       pgx.Tx
	accounts []uuid.UUID
}

// AcquirePlatformBanGuard precedes all match, party and search-session locks.
func AcquirePlatformBanGuard(ctx context.Context, tx pgx.Tx, accounts []uuid.UUID) (*PlatformBanGuard, error) {
	normalized, err := acquirePlatformBanAccounts(ctx, tx, accounts, true)
	if err != nil {
		return nil, err
	}
	return &PlatformBanGuard{tx: tx, accounts: normalized}, nil
}

// Check uses a fresh statement after account locks have been acquired.
func (g *PlatformBanGuard) Check(ctx context.Context) error {
	if g == nil || g.tx == nil || len(g.accounts) == 0 {
		return ErrPlatformBanUnavailable
	}
	// This must be a separate READ COMMITTED statement after acquiring every
	// account lock. A CTE containing locks and this read shares a stale snapshot.
	rows, err := g.tx.Query(ctx, `SELECT DISTINCT account_id FROM mm_platform_bans
		WHERE account_id = ANY($1::uuid[]) AND revoked_at IS NULL
		AND (expires_at IS NULL OR expires_at > clock_timestamp())
		ORDER BY account_id`, g.accounts)
	if err != nil {
		return ErrPlatformBanUnavailable
	}
	defer rows.Close()
	var denied []uuid.UUID
	for rows.Next() {
		var account uuid.UUID
		if err := rows.Scan(&account); err != nil {
			return ErrPlatformBanUnavailable
		}
		denied = append(denied, account)
	}
	if rows.Err() != nil {
		return ErrPlatformBanUnavailable
	}
	if len(denied) > 0 {
		return &PlatformBanDenial{accounts: denied}
	}
	return nil
}

func acquirePlatformBanAccounts(ctx context.Context, tx pgx.Tx, accounts []uuid.UUID, shared bool) ([]uuid.UUID, error) {
	if tx == nil || len(accounts) == 0 {
		return nil, ErrPlatformBanUnavailable
	}
	// Sort actual advisory keys, not UUIDs: even a hash collision can only add
	// contention, and cannot reverse lock order between overlapping account sets.
	keys := make(map[int64]struct{}, len(accounts))
	unique := make(map[uuid.UUID]struct{}, len(accounts))
	for _, account := range accounts {
		if account == uuid.Nil {
			return nil, ErrPlatformBanUnavailable
		}
		unique[account] = struct{}{}
		material := append([]byte("voice.mm-platform-ban.account-lock.v1\x00"), account[:]...)
		digest := sha256.Sum256(material)
		keys[int64(binary.BigEndian.Uint64(digest[:8]))] = struct{}{}
	}
	var isolation string
	if err := tx.QueryRow(ctx, `SHOW transaction_isolation`).Scan(&isolation); err != nil || isolation != "read committed" {
		return nil, ErrPlatformBanUnavailable
	}
	ordered := make([]int64, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	query := `SELECT pg_advisory_xact_lock($1)`
	if shared {
		query = `SELECT pg_advisory_xact_lock_shared($1)`
	}
	for _, key := range ordered {
		if _, err := tx.Exec(ctx, query, key); err != nil {
			return nil, ErrPlatformBanUnavailable
		}
	}
	normalized := make([]uuid.UUID, 0, len(unique))
	for account := range unique {
		normalized = append(normalized, account)
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].String() < normalized[j].String() })
	return normalized, nil
}
