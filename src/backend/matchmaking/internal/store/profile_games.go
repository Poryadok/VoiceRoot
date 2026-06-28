package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrProfileGameEntryNotFound = errors.New("profile game entry not found")

type ProfileGameEntry struct {
	ProfileID uuid.UUID
	GameID    uuid.UUID
	Region    string
	Role      *string
	Rank      *string
	UpdatedAt time.Time
}

type UpsertProfileGameParams struct {
	ProfileID uuid.UUID
	GameID    uuid.UUID
	Region    string
	Role      *string
	Rank      *string
}

type ProfileGamesStore struct {
	Pool *pgxpool.Pool
}

func (s *ProfileGamesStore) ListByProfile(ctx context.Context, profileID uuid.UUID) ([]ProfileGameEntry, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT profile_id, game_id, region, role, rank, updated_at
		FROM profile_game_entries
		WHERE profile_id = $1
		ORDER BY updated_at DESC
	`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ProfileGameEntry
	for rows.Next() {
		e, err := scanProfileGameEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *ProfileGamesStore) Upsert(ctx context.Context, p UpsertProfileGameParams) (ProfileGameEntry, error) {
	role := trimOptional(p.Role)
	rank := trimOptional(p.Rank)
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO profile_game_entries (profile_id, game_id, region, role, rank, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (profile_id, game_id) DO UPDATE SET
			region = EXCLUDED.region,
			role = EXCLUDED.role,
			rank = EXCLUDED.rank,
			updated_at = now()
		RETURNING profile_id, game_id, region, role, rank, updated_at
	`, p.ProfileID, p.GameID, strings.TrimSpace(p.Region), role, rank)
	return scanProfileGameEntry(row)
}

func (s *ProfileGamesStore) Delete(ctx context.Context, profileID, gameID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM profile_game_entries WHERE profile_id = $1 AND game_id = $2
	`, profileID, gameID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrProfileGameEntryNotFound
	}
	return nil
}

func scanProfileGameEntry(row scannable) (ProfileGameEntry, error) {
	var e ProfileGameEntry
	if err := row.Scan(&e.ProfileID, &e.GameID, &e.Region, &e.Role, &e.Rank, &e.UpdatedAt); err != nil {
		return ProfileGameEntry{}, err
	}
	return e, nil
}

func trimOptional(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

// IsForeignKeyViolation reports FK errors from profile_game_entries.game_id.
func IsForeignKeyViolation(err error) bool {
	return !errors.Is(err, pgx.ErrNoRows) && strings.Contains(err.Error(), "foreign key")
}
