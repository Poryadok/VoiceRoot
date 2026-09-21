package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const searchNormalizationVersion = 1

// SearchKeyBackfillResult is the durable progress after one bounded backfill pass.
type SearchKeyBackfillResult struct {
	Scanned, Updated, SkippedConcurrent, Collisions int64
	Done                                            bool
}

// BackfillSearchKeys applies the current normalization version to at most batchSize
// profiles. Every value is produced in Go; PostgreSQL is used only for storage and CAS.
func (s *ProfileStore) BackfillSearchKeys(ctx context.Context, batchSize int) (SearchKeyBackfillResult, error) {
	if batchSize <= 0 {
		return SearchKeyBackfillResult{}, fmt.Errorf("batch size must be positive")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SearchKeyBackfillResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `INSERT INTO profile_search_key_backfill_checkpoints(normalization_version)
		VALUES ($1) ON CONFLICT (normalization_version) DO NOTHING`, searchNormalizationVersion); err != nil {
		return SearchKeyBackfillResult{}, err
	}
	var cursor *uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT last_profile_id FROM profile_search_key_backfill_checkpoints
		WHERE normalization_version=$1 FOR UPDATE`, searchNormalizationVersion).Scan(&cursor); err != nil {
		return SearchKeyBackfillResult{}, err
	}
	rows, err := tx.Query(ctx, `SELECT id, username, display_name, updated_at FROM profiles
		WHERE ($1::uuid IS NULL OR id > $1) ORDER BY id LIMIT $2`, cursor, batchSize)
	if err != nil {
		return SearchKeyBackfillResult{}, err
	}
	defer rows.Close()
	type rawCandidate struct {
		id                uuid.UUID
		username, display string
		updatedAt         time.Time
	}
	var candidates []rawCandidate
	for rows.Next() {
		var row rawCandidate
		if err = rows.Scan(&row.id, &row.username, &row.display, &row.updatedAt); err != nil {
			return SearchKeyBackfillResult{}, err
		}
		candidates = append(candidates, row)
	}
	if err = rows.Err(); err != nil {
		return SearchKeyBackfillResult{}, err
	}
	result := SearchKeyBackfillResult{Scanned: int64(len(candidates))}
	var last *uuid.UUID
	for _, row := range candidates {
		usernameKey, displayKey := NormalizeUsernameKey(row.username), NormalizeUsernameKey(row.display)
		command, e := tx.Exec(ctx, `UPDATE profiles SET username_search_key=$1, display_name_search_key=$2,
			search_normalization_version=$3 WHERE id=$4 AND updated_at=$5`, usernameKey, displayKey, searchNormalizationVersion, row.id, row.updatedAt)
		if e != nil {
			return SearchKeyBackfillResult{}, e
		}
		if command.RowsAffected() == 0 {
			result.SkippedConcurrent++
			id := row.id
			last = &id
			continue
		}
		result.Updated++
		for _, kind := range []struct{ name, key, column string }{{"username", usernameKey, "username_search_key"}, {"display_name", displayKey, "display_name_search_key"}} {
			var other uuid.UUID
			e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM profiles WHERE %s=$1 AND search_normalization_version=$2 AND id<>$3 ORDER BY id LIMIT 1`, kind.column), kind.key, searchNormalizationVersion, row.id).Scan(&other)
			if e == nil {
				_, e = tx.Exec(ctx, `INSERT INTO profile_search_key_collisions(normalization_version,key_kind,search_key,profile_id,conflicting_profile_id)
					VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, searchNormalizationVersion, kind.name, kind.key, row.id, other)
				if e != nil {
					return SearchKeyBackfillResult{}, e
				}
				result.Collisions++
			} else if e != pgx.ErrNoRows {
				return SearchKeyBackfillResult{}, e
			}
		}
		id := row.id
		last = &id
	}
	var done bool
	if len(candidates) == 0 {
		var pending bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM profiles WHERE search_normalization_version IS DISTINCT FROM $1)`, searchNormalizationVersion).Scan(&pending); err != nil {
			return SearchKeyBackfillResult{}, err
		}
		done = !pending
		if !done {
			last = nil
		}
	}
	_, err = tx.Exec(ctx, `UPDATE profile_search_key_backfill_checkpoints SET last_profile_id=$2,
		scanned_count=scanned_count+$3, updated_count=updated_count+$4, skipped_concurrent_count=skipped_concurrent_count+$5,
		collision_count=collision_count+$6, completed_at=CASE WHEN $7 THEN now() ELSE NULL END, updated_at=now()
		WHERE normalization_version=$1`, searchNormalizationVersion, last, result.Scanned, result.Updated, result.SkippedConcurrent, result.Collisions, done)
	if err != nil {
		return SearchKeyBackfillResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SearchKeyBackfillResult{}, err
	}
	result.Done = done
	return result, nil
}
