package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrOwnershipFrozen           = errors.New("ownership operation frozen")
	ErrOwnershipScopeUnavailable = errors.New("ownership scope unavailable")
)

// OwnershipFrozenError exposes the stable classification to adapters without
// coupling them to database error text.
func (*SpaceStore) OwnershipFrozenError() error { return ErrOwnershipFrozen }

func normalizedOwnershipSpaceIDs(spaceIDs []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(spaceIDs))
	result := make([]uuid.UUID, 0, len(spaceIDs))
	for _, id := range spaceIDs {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	slices.SortFunc(result, func(left, right uuid.UUID) int { return bytes.Compare(left[:], right[:]) })
	return result
}

func (s *SpaceStore) withOwnershipScope(ctx context.Context, spaceIDs []uuid.UUID, action func(*SpaceStore) error) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	if s.tx != nil {
		return action(s)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
	}
	defer func() {
		cleanupCtx, cancel := BoundedCleanupContext(ctx)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()
	if err := lockAndCheckOwnershipScope(ctx, tx, spaceIDs); err != nil {
		return err
	}
	scoped := *s
	scoped.tx = tx
	if err := action(&scoped); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func lockAndCheckOwnershipScope(ctx context.Context, tx pgx.Tx, spaceIDs []uuid.UUID) error {
	normalized := normalizedOwnershipSpaceIDs(spaceIDs)
	for _, spaceID := range normalized {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(spaceID)); err != nil {
			return fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
		}
	}
	var frozen bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ownership_journal
		WHERE space_id=ANY($1::uuid[]) AND state NOT IN ('completed','aborted'))`, normalized).Scan(&frozen); err != nil {
		return fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
	}
	if frozen {
		return ErrOwnershipFrozen
	}
	return nil
}

func withOwnershipScopeValue[T any](s *SpaceStore, ctx context.Context, spaceIDs []uuid.UUID, action func(*SpaceStore) (T, error)) (T, error) {
	var value T
	err := s.withOwnershipScope(ctx, spaceIDs, func(scoped *SpaceStore) error {
		var callErr error
		value, callErr = action(scoped)
		return callErr
	})
	return value, err
}

func withResolvedOwnershipScopeValue[T any](s *SpaceStore, ctx context.Context, resolve func(spaceStoreDB) (uuid.UUID, error), action func(*SpaceStore) (T, error), missing ...func() (T, error)) (T, error) {
	var zero T
	if s == nil || s.Pool == nil {
		return zero, errors.New("space store: pool not configured")
	}
	if s.tx != nil {
		return action(s)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return zero, fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
	}
	defer func() {
		cleanupCtx, cancel := BoundedCleanupContext(ctx)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()
	resolvedBefore, err := resolve(tx)
	if err != nil {
		return zero, fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
	}
	if resolvedBefore == uuid.Nil {
		if len(missing) == 0 {
			return zero, nil
		}
		return missing[0]()
	}
	if err := lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{resolvedBefore}); err != nil {
		return zero, err
	}
	resolvedAfter, err := resolve(tx)
	if err != nil {
		return zero, fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
	}
	if resolvedAfter != resolvedBefore {
		return zero, ErrOwnershipScopeUnavailable
	}
	scoped := *s
	scoped.tx = tx
	value, err := action(&scoped)
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, err
	}
	return value, nil
}

func (s *SpaceStore) withResolvedOwnershipScope(ctx context.Context, resolve func(spaceStoreDB) (uuid.UUID, error), action func(*SpaceStore) error, missing ...func() error) error {
	missingValue := func() (struct{}, error) {
		if len(missing) == 0 {
			return struct{}{}, nil
		}
		return struct{}{}, missing[0]()
	}
	_, err := withResolvedOwnershipScopeValue(s, ctx, resolve, func(scoped *SpaceStore) (struct{}, error) {
		return struct{}{}, action(scoped)
	}, missingValue)
	return err
}

func (s *SpaceStore) ensureOwnershipAvailable(ctx context.Context, spaceIDs ...uuid.UUID) error {
	return s.withOwnershipScope(ctx, spaceIDs, func(*SpaceStore) error { return nil })
}

// CheckOwnershipAvailable serializes adapter authorization before any
// external permission fallback. Store actions repeat the check in their scope.
func (s *SpaceStore) CheckOwnershipAvailable(ctx context.Context, spaceIDs ...uuid.UUID) error {
	return s.ensureOwnershipAvailable(ctx, spaceIDs...)
}

// lockOwnershipTransaction uses disjoint PostgreSQL advisory key spaces:
// operation two-int32 first, then space bigint. A future ordinary transaction
// may take the space lock alone but must never call this while holding it.
// The transaction and all reads/writes remain on one connection.
func lockOwnershipTransaction(ctx context.Context, tx pgx.Tx, operationID, spaceID uuid.UUID) error {
	material := append([]byte("voice.space.ownership.operation.v1\x00"), operationID[:]...)
	digest := sha256.Sum256(material)
	first := int32(binary.BigEndian.Uint32(digest[:4]))
	second := int32(binary.BigEndian.Uint32(digest[4:8]))
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, first, second); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(spaceID))
	return err
}
