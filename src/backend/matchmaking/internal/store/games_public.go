package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

var (
	ErrGamesCursorInvalid = errors.New("invalid games cursor")
	ErrGamesCursorStale   = errors.New("games_cursor_stale")
)

const publicGamesCursorPrefix = "gpop1"
const maxGamesCursorBytes = 128

// The entire order and page metadata come from one statement snapshot. The
// catalog is small; reading its ordered IDs avoids a retained snapshot/cache
// and prevents volatile score-keyset boundaries from skipping games.
const publicGamesQuery = `
	SELECT g.id, g.name, g.icon_url, g.external_id, g.config, g.status,
	       g.created_by, g.created_at, g.updated_at
	FROM games g
	LEFT JOIN (
		SELECT game_id, count(*) AS depth
		FROM search_sessions
		WHERE status = 'searching' AND space_id IS NULL
		  AND timeout_at IS NOT NULL AND timeout_at > CURRENT_TIMESTAMP
		GROUP BY game_id
	) q ON q.game_id = g.id
	WHERE g.status = 'active'
	ORDER BY COALESCE(q.depth, 0) DESC, g.created_at DESC, g.id DESC`

type gameQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

// ListPublic is deliberately separate from List: staff may request active games
// too, but must retain chronological moderation ordering and legacy cursors.
func (s *GameStore) ListPublic(ctx context.Context, p ListGamesParams) (ListGamesResult, error) {
	if len(p.Cursor) > maxGamesCursorBytes {
		return ListGamesResult{}, ErrGamesCursorInvalid
	}
	if strings.Contains(p.Cursor, "|") {
		if _, _, err := decodeCursor(p.Cursor); err != nil {
			return ListGamesResult{}, ErrGamesCursorInvalid
		}
		if s == nil || s.Pool == nil {
			return ListGamesResult{}, errors.New("game store unavailable")
		}
		p.Status = StatusActive
		return s.List(ctx, p)
	}
	if _, _, err := decodePublicGamesCursor(p.Cursor); err != nil {
		return ListGamesResult{}, err
	}
	if s == nil || s.Pool == nil {
		return ListGamesResult{}, errors.New("game store unavailable")
	}
	return listPublicGames(ctx, s.Pool, p)
}

func listPublicGames(ctx context.Context, q gameQuerier, p ListGamesParams) (ListGamesResult, error) {
	// Validate before accessing the database, including when this helper is used
	// with a transaction in integration tests.
	if _, _, err := decodePublicGamesCursor(p.Cursor); err != nil {
		return ListGamesResult{}, err
	}
	rows, err := q.Query(ctx, publicGamesQuery)
	if err != nil {
		return ListGamesResult{}, err
	}
	defer rows.Close()
	var games []Game
	for rows.Next() {
		game, err := scanGame(rows)
		if err != nil {
			return ListGamesResult{}, err
		}
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		return ListGamesResult{}, err
	}
	return pagePublicGames(games, p)
}

func publicGamesFingerprint(games []Game) string {
	h := sha256.New()
	_, _ = h.Write([]byte("voice:matchmaking:public-games:popularity:v1\x00"))
	for _, game := range games {
		_, _ = h.Write(game.ID[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func decodePublicGamesCursor(cursor string) (string, int, error) {
	if cursor == "" {
		return "", 0, nil
	}
	if len(cursor) > maxGamesCursorBytes {
		return "", 0, ErrGamesCursorInvalid
	}
	parts := strings.Split(cursor, ".")
	if len(parts) != 3 || parts[0] != publicGamesCursorPrefix || len(parts[1]) != sha256.Size*2 {
		return "", 0, ErrGamesCursorInvalid
	}
	hash, err := hex.DecodeString(parts[1])
	if err != nil || hex.EncodeToString(hash) != parts[1] {
		return "", 0, ErrGamesCursorInvalid
	}
	position, err := strconv.ParseInt(parts[2], 10, 32)
	if err != nil || position <= 0 || strconv.FormatInt(position, 10) != parts[2] {
		return "", 0, ErrGamesCursorInvalid
	}
	return parts[1], int(position), nil
}

func pagePublicGames(games []Game, p ListGamesParams) (ListGamesResult, error) {
	fingerprint, position, err := decodePublicGamesCursor(p.Cursor)
	if err != nil {
		return ListGamesResult{}, err
	}
	current := publicGamesFingerprint(games)
	if p.Cursor != "" {
		if fingerprint != current {
			return ListGamesResult{}, ErrGamesCursorStale
		}
		if position >= len(games) {
			return ListGamesResult{}, ErrGamesCursorInvalid
		}
	}
	limit := int(p.PageSize)
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	end := min(position+limit, len(games))
	result := ListGamesResult{Games: games[position:end]}
	if end < len(games) {
		result.NextCursor = publicGamesCursorPrefix + "." + current + "." + strconv.Itoa(end)
	}
	return result, nil
}
