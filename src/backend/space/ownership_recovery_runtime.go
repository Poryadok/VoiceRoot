package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"voice/backend/pkg/runtimeconfig"
	grpcsvc "voice/backend/space/internal/grpcsvc"
	"voice/backend/space/internal/ownershiprecovery"
	"voice/backend/space/internal/store"
)

const ownershipRecoveryBatchSize = 100

type ownershipRecoveryRuntime struct {
	cancel context.CancelFunc
	wait   sync.WaitGroup
	stop   sync.Once
}

func startOwnershipRecoveryRuntime(parent context.Context, spaceStore *store.SpaceStore, service *grpcsvc.SpaceGRPC, logger *slog.Logger) *ownershipRecoveryRuntime {
	if parent == nil || spaceStore == nil || service == nil || service.OwnershipAuth == nil || service.OwnershipRoles == nil || service.PrincipalIssuer == nil {
		return nil
	}
	interval := runtimeconfig.DurationFromEnv("SPACE_OWNERSHIP_RECOVERY_INTERVAL", time.Second)
	if interval <= 0 {
		return nil
	}
	worker := ownershiprecovery.Worker{Coordinator: grpcsvc.NewOwnershipRecoveryCoordinator(service), Scanner: spaceStore, BatchSize: ownershipRecoveryBatchSize}
	ctx, cancel := context.WithCancel(parent)
	runtime := &ownershipRecoveryRuntime{cancel: cancel}
	runtime.wait.Add(1)
	go func() {
		defer runtime.wait.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if err := worker.RunOnce(ctx); err != nil && ctx.Err() == nil && logger != nil {
				logger.Error("ownership recovery pass", slog.String("error", err.Error()))
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return runtime
}

func (r *ownershipRecoveryRuntime) Stop() {
	if r != nil {
		r.stop.Do(func() { r.cancel(); r.wait.Wait() })
	}
}
