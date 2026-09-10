package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrSpaceFrozen      = errors.New("role space ownership transfer pending")
	ErrSpaceRetired     = errors.New("role space retired")
	ErrScopeUnavailable = errors.New("role transaction scope unavailable")
)

// The two-integer operation namespace is disjoint from the legacy one-bigint
// advisory space locks. All new transitions acquire operation before space.
const ownershipOperationLockNamespace int32 = 0x524f5032

type scopeExecutor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// db never falls back to Pool: scoped helpers must retain their transaction.
func (s *RoleStore) db() scopeExecutor { return s.tx }

func (s *RoleStore) withLockedTransaction(ctx context.Context, operationID uuid.UUID, spaceIDs []uuid.UUID, fn func(*RoleStore) error) error {
	if s == nil || fn == nil || len(spaceIDs) == 0 {
		return fmt.Errorf("%w: invalid transaction scope", ErrScopeUnavailable)
	}
	spaces := make(map[uuid.UUID]struct{}, len(spaceIDs))
	for _, id := range spaceIDs {
		if id == uuid.Nil {
			return fmt.Errorf("%w: missing space", ErrScopeUnavailable)
		}
		spaces[id] = struct{}{}
	}
	if s.tx != nil {
		if operationID != uuid.Nil && operationID != s.scopeOperation {
			return fmt.Errorf("%w: nested operation lock", ErrScopeUnavailable)
		}
		for id := range spaces {
			if _, ok := s.scopeSpaces[id]; !ok {
				return fmt.Errorf("%w: nested space expansion", ErrScopeUnavailable)
			}
		}
		return fn(s)
	}
	if s.Pool == nil {
		return fmt.Errorf("%w: pool not configured", ErrScopeUnavailable)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%w: begin: %w", ErrScopeUnavailable, err)
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()
	if operationID != uuid.Nil {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1::integer, hashtext($2))", ownershipOperationLockNamespace, operationID.String()); err != nil {
			return fmt.Errorf("%w: operation lock: %w", ErrScopeUnavailable, err)
		}
	}
	ordered := make([]string, 0, len(spaces))
	for id := range spaces {
		ordered = append(ordered, id.String())
	}
	sort.Strings(ordered)
	for _, id := range ordered {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", id); err != nil {
			return fmt.Errorf("%w: space lock: %w", ErrScopeUnavailable, err)
		}
	}
	scoped := &RoleStore{Pool: s.Pool, tx: tx, scopeSpaces: spaces, scopeOperation: operationID}
	if err := fn(scoped); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%w: commit: %w", ErrScopeUnavailable, err)
	}
	return nil
}
