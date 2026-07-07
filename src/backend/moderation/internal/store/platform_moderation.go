package store

import "github.com/jackc/pgx/v5/pgxpool"

// SanctionStore persists platform sanctions (app stack4).
type SanctionStore struct {
	Pool *pgxpool.Pool
}

// AppealStore persists sanction appeals (app stack4).
type AppealStore struct {
	Pool *pgxpool.Pool
}

// AuditLogStore persists moderator audit rows (app stack4).
type AuditLogStore struct {
	Pool *pgxpool.Pool
}

// AutoModStore tracks spam-pattern mutes and automod counters (app stack4).
type AutoModStore struct {
	Pool *pgxpool.Pool
}
