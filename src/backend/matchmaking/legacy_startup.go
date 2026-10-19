package main

import (
	"context"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/matchmaking/internal/mmadmission"
	"voice/backend/pkg/postgres"
	"voice/backend/pkg/runtimeconfig"
)

func checkLegacyNativeDescriptor(stat func(string) (os.FileInfo, error)) error {
	if stat == nil {
		return mmadmission.ErrUnavailable
	}
	_, err := stat(mmadmission.InstallationDescriptorPath)
	if !os.IsNotExist(err) {
		return mmadmission.ErrUnavailable
	}
	return nil
}

func validateLegacyAdmission(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return mmadmission.ErrUnavailable
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return mmadmission.ErrUnavailable
	}
	defer tx.Rollback(ctx)
	if err := mmadmission.Acquire(ctx, tx, nil); err != nil {
		return mmadmission.ErrUnavailable
	}
	if err := tx.Commit(ctx); err != nil {
		return mmadmission.ErrUnavailable
	}
	return nil
}

// prepareLegacyDatabase runs before any bootstrap listener or worker. It is a
// negative compatibility guard, not authority to launch a native installation.
func prepareLegacyDatabase(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	if err := checkLegacyNativeDescriptor(os.Lstat); err != nil {
		return nil, err
	}
	if strings.TrimSpace(dbURL) == "" {
		return nil, nil
	}
	bounded, cancel := context.WithTimeout(ctx, runtimeconfig.PostgresConnectTimeoutFromEnv())
	defer cancel()
	pool, err := postgres.NewPool(bounded, dbURL)
	if err != nil {
		return nil, mmadmission.ErrUnavailable
	}
	if err := validateLegacyAdmission(bounded, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
