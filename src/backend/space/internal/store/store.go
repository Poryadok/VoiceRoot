package store

import "github.com/jackc/pgx/v5/pgxpool"

// SpaceStore persists spaces and memberships in space_db.
type SpaceStore struct {
	Pool *pgxpool.Pool
}
