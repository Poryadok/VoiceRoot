package main

import (
	"context"
	"log/slog"
	"reflect"
	"sync"
	"time"

	"github.com/google/uuid"

	"voice/backend/pkg/runtimeconfig"
	"voice/backend/space/internal/outboxdelivery"
	"voice/backend/space/internal/store"
)

const ownershipOutboxCleanupBatch = 100

type ownershipOutboxRuntimeConfig struct {
	dispatchInterval time.Duration
	cleanupInterval  time.Duration
}

func ownershipOutboxRuntimeConfigFromEnv() ownershipOutboxRuntimeConfig {
	return ownershipOutboxRuntimeConfig{
		dispatchInterval: runtimeconfig.DurationFromEnv("SPACE_OWNERSHIP_OUTBOX_DISPATCH_INTERVAL", time.Second),
		cleanupInterval:  runtimeconfig.DurationFromEnv("SPACE_OWNERSHIP_OUTBOX_CLEANUP_INTERVAL", time.Hour),
	}
}

type ownershipOutboxRuntimeStore interface {
	ClaimReadyOwnershipOutbox(context.Context, int) ([]store.ClaimedOwnershipOutboxEvent, error)
	MarkOwnershipOutboxFailed(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	MarkOwnershipOutboxDelivered(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	CountAlertingOwnershipOutbox(context.Context) (int64, error)
	CleanupDeliveredOwnershipOutbox(context.Context, int) (int64, error)
}

type ownershipOutboxAlertGauge interface {
	Set(float64)
}

type ownershipOutboxRunLoop func(context.Context, time.Duration, func(context.Context) error, func(error))

type ownershipOutboxRuntimeDependencies struct {
	store      ownershipOutboxRuntimeStore
	transport  outboxdelivery.Transport
	alertGauge ownershipOutboxAlertGauge
	logger     *slog.Logger
	runLoop    ownershipOutboxRunLoop
}

type ownershipOutboxRuntime struct {
	cancel context.CancelFunc
	wait   sync.WaitGroup
	stop   sync.Once
}

func startOwnershipOutboxRuntime(parent context.Context, config ownershipOutboxRuntimeConfig, dependencies ownershipOutboxRuntimeDependencies) *ownershipOutboxRuntime {
	if parent == nil || ownershipOutboxDependencyMissing(dependencies.store) ||
		ownershipOutboxDependencyMissing(dependencies.transport) || ownershipOutboxDependencyMissing(dependencies.alertGauge) {
		return nil
	}
	runLoop := dependencies.runLoop
	if runLoop == nil {
		runLoop = runOwnershipOutboxLoop
	}

	adapter := outboxdelivery.NewStoreAdapter(dependencies.store)
	coordinator := outboxdelivery.NewCoordinator(adapter, dependencies.transport)
	monitor := outboxdelivery.NewAlertMonitor(dependencies.store, dependencies.alertGauge, coordinator)
	ctx, cancel := context.WithCancel(parent)
	runtime := &ownershipOutboxRuntime{cancel: cancel}

	if err := monitor.Refresh(ctx); err != nil {
		logOwnershipOutboxRuntimeError(dependencies.logger, "ownership outbox alert refresh", err)
	}
	runtime.startLoop(ctx, runLoop, config.dispatchInterval, monitor.DispatchOnce,
		func(err error) {
			logOwnershipOutboxRuntimeError(dependencies.logger, "ownership outbox dispatcher", err)
		})
	runtime.startLoop(ctx, runLoop, config.cleanupInterval,
		func(ctx context.Context) error {
			_, err := dependencies.store.CleanupDeliveredOwnershipOutbox(ctx, ownershipOutboxCleanupBatch)
			return err
		},
		func(err error) { logOwnershipOutboxRuntimeError(dependencies.logger, "ownership outbox cleanup", err) })
	return runtime
}

func ownershipOutboxDependencyMissing(dependency any) bool {
	if dependency == nil {
		return true
	}
	value := reflect.ValueOf(dependency)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (r *ownershipOutboxRuntime) startLoop(ctx context.Context, runLoop ownershipOutboxRunLoop, interval time.Duration, task func(context.Context) error, onError func(error)) {
	r.wait.Add(1)
	go func() {
		defer r.wait.Done()
		runLoop(ctx, interval, task, onError)
	}()
}

func (r *ownershipOutboxRuntime) Stop() {
	if r == nil {
		return
	}
	r.stop.Do(func() {
		r.cancel()
		r.wait.Wait()
	})
}

type ownershipOutboxTask func(context.Context) error

func (task ownershipOutboxTask) DispatchOnce(ctx context.Context) error {
	return task(ctx)
}

func runOwnershipOutboxLoop(ctx context.Context, interval time.Duration, task func(context.Context) error, onError func(error)) {
	worker := outboxdelivery.NewWorker(ownershipOutboxTask(task), interval, onError)
	if err := worker.Run(ctx); err != nil && ctx.Err() == nil && onError != nil {
		onError(err)
	}
}

func logOwnershipOutboxRuntimeError(logger *slog.Logger, message string, err error) {
	if logger != nil && err != nil {
		logger.Error(message, slog.String("error", err.Error()))
	}
}
