package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDuplicateMatchRating = errors.New("duplicate match rating")
	ErrInvalidRatingStars   = errors.New("invalid rating stars")
)

// PlayerRating is an aggregate row in player_ratings.
type PlayerRating struct {
	ProfileID   uuid.UUID
	GameID      uuid.UUID
	RatingValue float64
	GamesPlayed int32
}

// InsertMatchRatingParams is one peer rating submission.
type InsertMatchRatingParams struct {
	MatchID        uuid.UUID
	RaterProfileID uuid.UUID
	RatedProfileID uuid.UUID
	Stars          int
}

// RatingStore persists match and player ratings.
type RatingStore struct {
	Pool *pgxpool.Pool
}

const completedMatchesForRating = `
	SELECT COUNT(DISTINCT m.id)::int
	FROM matches m
	WHERE m.game_id = $2
	  AND m.status = 'completed'
	  AND EXISTS (
		SELECT 1
		FROM jsonb_array_elements(m.participants) AS participant
		WHERE participant->>'profile_id' = $1::text
	  )
`

func validateStars(stars int) error {
	if stars < 1 || stars > 5 {
		return ErrInvalidRatingStars
	}
	return nil
}

// RecordMatchRating stores the raw vote and rebuilds its per-game aggregate in
// one transaction. The aggregate row lock serializes votes for one profile and
// game; rebuilding from match_ratings also repairs a prior partial write.
func (s *RatingStore) RecordMatchRating(ctx context.Context, p InsertMatchRatingParams) error {
	if s == nil || s.Pool == nil {
		return errors.New("rating store unavailable")
	}
	if err := validateStars(p.Stars); err != nil {
		return err
	}
	if p.RaterProfileID == p.RatedProfileID {
		return errors.New("cannot rate self")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var gameID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT game_id FROM matches WHERE id = $1`, p.MatchID).Scan(&gameID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO player_ratings (profile_id, game_id)
		VALUES ($1, $2)
		ON CONFLICT (profile_id, game_id) DO NOTHING
	`, p.RatedProfileID, gameID); err != nil {
		return err
	}
	var lockedProfileID uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT profile_id FROM player_ratings
		WHERE profile_id = $1 AND game_id = $2
		FOR UPDATE
	`, p.RatedProfileID, gameID).Scan(&lockedProfileID); err != nil {
		return err
	}
	var insertedMatchID uuid.UUID
	insertErr := tx.QueryRow(ctx, `
		INSERT INTO match_ratings (match_id, rater_profile_id, rated_profile_id, score)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (match_id, rater_profile_id, rated_profile_id) DO NOTHING
		RETURNING match_id
	`, p.MatchID, p.RaterProfileID, p.RatedProfileID, p.Stars).Scan(&insertedMatchID)
	if insertErr != nil && !errors.Is(insertErr, pgx.ErrNoRows) {
		return insertErr
	}
	_, err = tx.Exec(ctx, `
		UPDATE player_ratings AS pr
		SET average_rating = COALESCE((
				SELECT AVG(mr.score)::double precision
				FROM match_ratings mr
				JOIN matches m ON m.id = mr.match_id
				WHERE mr.rated_profile_id = pr.profile_id AND m.game_id = pr.game_id
			), 0),
			total_ratings_received = (
				SELECT COUNT(*)::int
				FROM match_ratings mr
				JOIN matches m ON m.id = mr.match_id
				WHERE mr.rated_profile_id = pr.profile_id AND m.game_id = pr.game_id
			),
			updated_at = now()
		WHERE pr.profile_id = $1 AND pr.game_id = $2
	`, p.RatedProfileID, gameID)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if errors.Is(insertErr, pgx.ErrNoRows) {
		return ErrDuplicateMatchRating
	}
	return nil
}

// GetPlayerRating loads the rating aggregate and the number of completed
// matches containing this profile for the requested game.
func (s *RatingStore) GetPlayerRating(ctx context.Context, profileID, gameID uuid.UUID) (PlayerRating, error) {
	if s == nil || s.Pool == nil {
		return PlayerRating{}, errors.New("rating store unavailable")
	}
	var pr PlayerRating
	err := s.Pool.QueryRow(ctx, `
		SELECT $1::uuid, $2::uuid, COALESCE(pr.average_rating, 0), (`+completedMatchesForRating+`)
		FROM (SELECT $1::uuid AS profile_id, $2::uuid AS game_id) requested
		LEFT JOIN player_ratings pr
		  ON pr.profile_id = requested.profile_id AND pr.game_id = requested.game_id
	`, profileID, gameID).Scan(
		&pr.ProfileID, &pr.GameID, &pr.RatingValue, &pr.GamesPlayed,
	)
	return pr, err
}
