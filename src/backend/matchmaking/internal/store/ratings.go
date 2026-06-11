package store

import (
	"context"
	"errors"
	"fmt"

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

func validateStars(stars int) error {
	if stars < 1 || stars > 5 {
		return ErrInvalidRatingStars
	}
	return nil
}

// InsertMatchRating stores a single peer rating.
func (s *RatingStore) InsertMatchRating(ctx context.Context, p InsertMatchRatingParams) error {
	if s == nil || s.Pool == nil {
		return errors.New("rating store unavailable")
	}
	if err := validateStars(p.Stars); err != nil {
		return err
	}
	if p.RaterProfileID == p.RatedProfileID {
		return errors.New("cannot rate self")
	}
	tag, err := s.Pool.Exec(ctx, `
		INSERT INTO match_ratings (match_id, rater_profile_id, rated_profile_id, score)
		VALUES ($1, $2, $3, $4)
	`, p.MatchID, p.RaterProfileID, p.RatedProfileID, p.Stars)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateMatchRating
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("insert match rating: no rows")
	}
	return nil
}

// UpsertPlayerRating increments the running average for a profile+game.
func (s *RatingStore) UpsertPlayerRating(ctx context.Context, profileID, gameID uuid.UUID, stars int) (PlayerRating, error) {
	if s == nil || s.Pool == nil {
		return PlayerRating{}, errors.New("rating store unavailable")
	}
	if err := validateStars(stars); err != nil {
		return PlayerRating{}, err
	}
	var pr PlayerRating
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO player_ratings (profile_id, game_id, average_rating, total_ratings_received)
		VALUES ($1, $2, $3, 1)
		ON CONFLICT (profile_id, game_id) DO UPDATE SET
			average_rating = (
				player_ratings.average_rating * player_ratings.total_ratings_received + EXCLUDED.average_rating
			) / (player_ratings.total_ratings_received + 1),
			total_ratings_received = player_ratings.total_ratings_received + 1,
			updated_at = now()
		RETURNING profile_id, game_id, average_rating, total_ratings_received
	`, profileID, gameID, float64(stars)).Scan(
		&pr.ProfileID, &pr.GameID, &pr.RatingValue, &pr.GamesPlayed,
	)
	return pr, err
}

// GetPlayerRating loads aggregate rating for a profile+game.
func (s *RatingStore) GetPlayerRating(ctx context.Context, profileID, gameID uuid.UUID) (PlayerRating, error) {
	if s == nil || s.Pool == nil {
		return PlayerRating{}, errors.New("rating store unavailable")
	}
	var pr PlayerRating
	err := s.Pool.QueryRow(ctx, `
		SELECT profile_id, game_id, average_rating, total_ratings_received
		FROM player_ratings WHERE profile_id = $1 AND game_id = $2
	`, profileID, gameID).Scan(&pr.ProfileID, &pr.GameID, &pr.RatingValue, &pr.GamesPlayed)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlayerRating{ProfileID: profileID, GameID: gameID}, nil
	}
	return pr, err
}
