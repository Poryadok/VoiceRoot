package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	voicepostgres "voice/backend/pkg/postgres"
	"voice/backend/pkg/runtimeconfig"
	"voice/backend/voice/internal/nativestartup"
	"voice/backend/voice/internal/partycoverage"
	"voice/backend/voice/internal/roomlifecycle"
)

func openLifecycleDatabase(parent context.Context) (roomlifecycle.LifecycleStore, *pgxpool.Pool, func(), bool, error) {
	closeDatabase := func() {}
	// This root is the admitted pre-cut legacy executable. A native installation
	// must use its separate mandatory bootstrap, never this optional DB path.
	if _, err := os.Lstat(nativestartup.DescriptorPath); err == nil || !os.IsNotExist(err) {
		return nil, nil, closeDatabase, false, fmt.Errorf("native installation requires the native serving executable")
	}
	dsn := strings.TrimSpace(os.Getenv("VOICE_DATABASE_URL"))
	if dsn == "" {
		return nil, nil, closeDatabase, false, nil
	}

	ctx, cancel := context.WithTimeout(parent, runtimeconfig.PostgresConnectTimeoutFromEnv())
	defer cancel()

	pool, err := voicepostgres.NewPool(ctx, dsn)
	if err != nil {
		return nil, nil, closeDatabase, false, fmt.Errorf("parse VOICE_DATABASE_URL: %w", err)
	}
	closeDatabase = pool.Close
	if err := pool.Ping(ctx); err != nil {
		closeDatabase()
		return nil, nil, func() {}, false, fmt.Errorf("ping voice lifecycle database: %w", err)
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		closeDatabase()
		return nil, nil, func() {}, false, fmt.Errorf("voice coverage startup unavailable")
	}
	if err := partycoverage.CheckStartup(ctx, tx, nil); err != nil {
		_ = tx.Rollback(ctx)
		closeDatabase()
		return nil, nil, func() {}, false, fmt.Errorf("voice coverage startup unavailable")
	}
	if err := tx.Commit(ctx); err != nil {
		closeDatabase()
		return nil, nil, func() {}, false, fmt.Errorf("voice coverage startup unavailable")
	}

	return roomlifecycle.NewPostgresLifecycleStore(pool), pool, closeDatabase, true, nil
}
