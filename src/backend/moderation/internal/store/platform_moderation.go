package store

import "github.com/jackc/pgx/v5/pgxpool"

// SanctionStore persists platform sanctions (moderation (docs/features/reports.md)).
type SanctionStore struct {
	Pool *pgxpool.Pool
}

// AppealStore persists sanction appeals (moderation (docs/features/reports.md)).
type AppealStore struct {
	Pool *pgxpool.Pool
}

// AuditLogStore persists moderator audit rows (moderation (docs/features/reports.md)).
type AuditLogStore struct {
	Pool *pgxpool.Pool
}

// AutoModStore tracks spam-pattern mutes and automod counters (moderation (docs/features/reports.md)).
type AutoModStore struct {
	Pool *pgxpool.Pool
}
