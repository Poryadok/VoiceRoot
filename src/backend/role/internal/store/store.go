package store

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RoleStore persists roles and member assignments in role_db.
type RoleStore struct {
	Pool *pgxpool.Pool
}

// RoleRow is a roles table row.
type RoleRow struct {
	ID              uuid.UUID
	SpaceID         uuid.UUID
	Name            string
	PermissionsMask uint64
	Position        int32
	Managed         bool
}
