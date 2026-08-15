package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"voice/backend/pkg/runtimeconfig"
)

// NewPool opens a pgxpool with POSTGRES_MAX_CONNS from the environment.
func NewPool(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = runtimeconfig.PostgresMaxConnsFromEnv()
	return pgxpool.NewWithConfig(ctx, cfg)
}
