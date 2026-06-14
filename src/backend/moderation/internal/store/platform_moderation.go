package store

import "github.com/jackc/pgx/v5/pgxpool"

// SanctionStore persists platform sanctions (Phase 14).
type SanctionStore struct {
	Pool *pgxpool.Pool
}

// AppealStore persists sanction appeals (Phase 14).
type AppealStore struct {
	Pool *pgxpool.Pool
}

// AuditLogStore persists moderator audit rows (Phase 14).
type AuditLogStore struct {
	Pool *pgxpool.Pool
}

// AutoModStore tracks spam-pattern mutes and automod counters (Phase 14).
type AutoModStore struct {
	Pool *pgxpool.Pool
}
