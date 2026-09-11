package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	voicepostgres "voice/backend/pkg/postgres"
	"voice/backend/pkg/runtimeconfig"
	"voice/backend/voice/internal/roomlifecycle"
)

func openLifecycleDatabase(parent context.Context) (roomlifecycle.LifecycleStore, func(), bool, error) {
	closeDatabase := func() {}
	dsn := strings.TrimSpace(os.Getenv("VOICE_DATABASE_URL"))
	if dsn == "" {
		return nil, closeDatabase, false, nil
	}

	ctx, cancel := context.WithTimeout(parent, runtimeconfig.PostgresConnectTimeoutFromEnv())
	defer cancel()

	pool, err := voicepostgres.NewPool(ctx, dsn)
	if err != nil {
		return nil, closeDatabase, false, fmt.Errorf("parse VOICE_DATABASE_URL: %w", err)
	}
	closeDatabase = pool.Close
	if err := pool.Ping(ctx); err != nil {
		closeDatabase()
		return nil, func() {}, false, fmt.Errorf("ping voice lifecycle database: %w", err)
	}

	return roomlifecycle.NewPostgresLifecycleStore(pool), closeDatabase, true, nil
}
