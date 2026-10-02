package main

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"voice/backend/role/internal/store"
)

func startRoleDeletionRetention(st *store.RoleStore, logger *slog.Logger) (func(), error) {
	startup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	var ready bool
	err := st.Pool.QueryRow(startup, `SELECT to_regclass('role_space_deletion_fences') IS NOT NULL AND to_regclass('role_space_deletion_fence_receipts') IS NOT NULL`).Scan(&ready)
	cancel()
	if err != nil {
		return nil, err
	}
	if !ready {
		return nil, errors.New("Role Space lifecycle schema is missing")
	}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			attempt, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := st.CleanupSpaceDeletionFenceEvidence(attempt)
			cancel()
			if err != nil && ctx.Err() == nil {
				logger.Error("Role lifecycle retention pass failed")
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func() { stop(); <-done }, nil
}
