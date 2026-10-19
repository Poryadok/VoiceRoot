package nativestartup

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	voicepostgres "voice/backend/pkg/postgres"
	"voice/backend/voice/internal/partycoverage"
)

// OpenDatabase has no optional/legacy success result. The caller supplies the
// admitted application credential once; no environment or admin DSN is read.
// The caller owns the successful pool and must still qualify actual factories
// and protected listeners before starting any serving or forward work.
func OpenDatabase(ctx context.Context, descriptor Descriptor, connection string) (*pgxpool.Pool, error) {
	if strings.TrimSpace(connection) == "" {
		return nil, ErrInvalidInstallation
	}
	body, err := json.Marshal(descriptor)
	if err != nil {
		return nil, ErrInvalidInstallation
	}
	descriptor, err = ParseDescriptor(bytes.NewReader(body))
	if err != nil {
		return nil, ErrInvalidInstallation
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pool, err := voicepostgres.NewPool(ctx, connection)
	if err != nil {
		return nil, ErrInvalidInstallation
	}
	if err := CheckDatabase(ctx, pool, descriptor); err != nil {
		pool.Close()
		return nil, ErrInvalidInstallation
	}
	return pool, nil
}

func CheckDatabase(ctx context.Context, pool *pgxpool.Pool, descriptor Descriptor) error {
	if pool == nil || descriptor.ExpectedDatabaseLoginRoleOID == 0 || descriptor.ExpectedDatabaseLoginRoleName == "" {
		return ErrInvalidInstallation
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ErrInvalidInstallation
	}
	defer tx.Rollback(ctx)
	var session, current string
	var sessionOID, backendOID uint32
	if err := tx.QueryRow(ctx, `SELECT session_user::text,current_user::text,
 (SELECT oid FROM pg_catalog.pg_roles WHERE rolname=session_user),
 (SELECT usesysid FROM pg_catalog.pg_stat_activity WHERE pid=pg_backend_pid())`).Scan(&session, &current, &sessionOID, &backendOID); err != nil ||
		session != descriptor.ExpectedDatabaseLoginRoleName || current != session || sessionOID != descriptor.ExpectedDatabaseLoginRoleOID || backendOID != sessionOID {
		return ErrInvalidInstallation
	}
	binding := descriptor.RuntimeBinding()
	if err := partycoverage.CheckStartup(ctx, tx, &binding); err != nil {
		return ErrInvalidInstallation
	}
	if err := tx.Commit(ctx); err != nil {
		return ErrInvalidInstallation
	}
	return nil
}
