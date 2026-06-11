package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"voice/backend/matchmaking/internal/config"
)

var ErrGameNotFound = errors.New("game not found")

const (
	StatusActive   = "active"
	StatusArchived = "archived"
)

type Game struct {
	ID         uuid.UUID
	Name       string
	IconURL    *string
	ExternalID *string
	Config     config.GameConfig
	ConfigRaw  string
	Status     string
	CreatedBy  *uuid.UUID
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type GameStore struct {
	Pool *pgxpool.Pool
}

type ListGamesParams struct {
	Cursor   string
	PageSize int32
	Status   string
}

type ListGamesResult struct {
	Games      []Game
	NextCursor string
}

func (s *GameStore) Create(ctx context.Context, name string, cfg config.GameConfig, createdBy uuid.UUID) (Game, error) {
	cfgRaw := config.MustMarshal(cfg)
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO games (name, config, status, created_by)
		VALUES ($1, $2::jsonb, $3, $4)
		RETURNING id, name, icon_url, external_id, config, status, created_by, created_at, updated_at
	`, strings.TrimSpace(name), cfgRaw, StatusActive, createdBy)
	return scanGame(row)
}

func (s *GameStore) Get(ctx context.Context, id uuid.UUID) (Game, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, name, icon_url, external_id, config, status, created_by, created_at, updated_at
		FROM games WHERE id = $1
	`, id)
	g, err := scanGame(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Game{}, ErrGameNotFound
	}
	return g, err
}

func (s *GameStore) List(ctx context.Context, p ListGamesParams) (ListGamesResult, error) {
	status := p.Status
	if status == "" {
		status = StatusActive
	}
	limit := int(p.PageSize)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	cursorTime, cursorID, err := decodeCursor(p.Cursor)
	if err != nil {
		return ListGamesResult{}, err
	}

	args := []any{status, limit + 1}
	query := `
		SELECT id, name, icon_url, external_id, config, status, created_by, created_at, updated_at
		FROM games
		WHERE status = $1
	`
	if cursorTime != nil && cursorID != nil {
		query += ` AND (created_at, id) < ($3, $4)`
		args = append(args, *cursorTime, *cursorID)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $2`

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return ListGamesResult{}, err
	}
	defer rows.Close()

	var games []Game
	for rows.Next() {
		g, err := scanGame(rows)
		if err != nil {
			return ListGamesResult{}, err
		}
		games = append(games, g)
	}
	if err := rows.Err(); err != nil {
		return ListGamesResult{}, err
	}

	var nextCursor string
	if len(games) > limit {
		last := games[limit-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
		games = games[:limit]
	}
	return ListGamesResult{Games: games, NextCursor: nextCursor}, nil
}

func (s *GameStore) Search(ctx context.Context, query string, limit int) ([]Game, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, name, icon_url, external_id, config, status, created_by, created_at, updated_at
		FROM games
		WHERE status = $1 AND name ILIKE '%' || $2 || '%'
		ORDER BY name ASC
		LIMIT $3
	`, StatusActive, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var games []Game
	for rows.Next() {
		g, err := scanGame(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

type UpdateGameParams struct {
	Name       *string
	Config     *config.GameConfig
	Status     *string
}

func (s *GameStore) Update(ctx context.Context, id uuid.UUID, p UpdateGameParams) (Game, error) {
	sets := []string{"updated_at = now()"}
	args := []any{id}
	argN := 2
	if p.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argN))
		args = append(args, strings.TrimSpace(*p.Name))
		argN++
	}
	if p.Config != nil {
		sets = append(sets, fmt.Sprintf("config = $%d::jsonb", argN))
		args = append(args, config.MustMarshal(*p.Config))
		argN++
	}
	if p.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", argN))
		args = append(args, *p.Status)
	}
	query := fmt.Sprintf(`
		UPDATE games SET %s
		WHERE id = $1
		RETURNING id, name, icon_url, external_id, config, status, created_by, created_at, updated_at
	`, strings.Join(sets, ", "))
	row := s.Pool.QueryRow(ctx, query, args...)
	g, err := scanGame(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Game{}, ErrGameNotFound
	}
	return g, err
}

type scannable interface {
	Scan(dest ...any) error
}

func scanGame(row scannable) (Game, error) {
	var g Game
	var cfgBytes []byte
	var createdBy *uuid.UUID
	if err := row.Scan(&g.ID, &g.Name, &g.IconURL, &g.ExternalID, &cfgBytes, &g.Status, &createdBy, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return Game{}, err
	}
	g.CreatedBy = createdBy
	g.ConfigRaw = string(cfgBytes)
	if err := json.Unmarshal(cfgBytes, &g.Config); err != nil {
		return Game{}, fmt.Errorf("decode config: %w", err)
	}
	return g, nil
}

func encodeCursor(t time.Time, id uuid.UUID) string {
	return fmt.Sprintf("%s|%s", t.UTC().Format(time.RFC3339Nano), id.String())
}

func decodeCursor(cursor string) (*time.Time, *uuid.UUID, error) {
	if strings.TrimSpace(cursor) == "" {
		return nil, nil, nil
	}
	parts := strings.SplitN(cursor, "|", 2)
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("invalid cursor")
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cursor time")
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cursor id")
	}
	return &t, &id, nil
}
