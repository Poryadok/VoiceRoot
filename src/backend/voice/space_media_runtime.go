package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"log/slog"
	"sync/atomic"
	grpcsvc "voice/backend/voice/internal/grpcsvc"
	"voice/backend/voice/internal/spacelifecycle"
	"voice/backend/voice/internal/spacemedia"
	voicestore "voice/backend/voice/internal/store"
)

// startSpaceMediaRuntime is the production root's Space/P3 composition. Native
// roster facts are transactionally saved by writers, but their A3 relay remains
// dormant until its separate broker/consumer activation package is qualified.
// The existing participant/admission relay and restrictive P3 bindings remain.
func startSpaceMediaRuntime(runCtx context.Context, voiceSvc *grpcsvc.VoiceGRPC,
	lifecyclePool *pgxpool.Pool,
	sessionEpochChecker spacemedia.SessionEpochChecker, callStoreDurable bool,
	natsURL string, mediaLifecycle spacemedia.MediaLifecycle, logger *slog.Logger,
) func() error {
	callStore := voiceSvc.Calls
	spaceMediaAdmissions := spacemedia.NewPostgresAdmissionStore(lifecyclePool)
	spaceLifecycleController := voiceSvc.SpaceLifecycle
	voiceRoomAccessResolver := voiceSvc.VoiceRoomAccessResolver
	voiceRoomGrantResolver := voiceSvc.SpaceVoiceRoomGrants
	events := voiceSvc.Events
	closeRuntime := func() error { return nil }
	if service, ok := spaceLifecycleController.(*spacelifecycle.Service); ok {
		fence, ok := callStore.(interface {
			SetSpaceLifecycle(context.Context, string, int64, bool) error
		})
		gates, gatesOK := callStore.(voicestore.SpaceProjectionGates)
		if !ok || !gatesOK || !callStoreDurable {
			log.Fatal("P3 requires durable lifecycle-fenced Redis")
		}
		service.Effects.ProjectionFence = fence
		service.Effects.NativeDrain = &spacemedia.Coordinator{ProjectionGates: gates, Calls: callStore, Store: callStore, Admissions: spaceMediaAdmissions}
	}
	if natsURL != "" {
		if sessionEpochChecker == nil {
			log.Fatal("Space media admission requires the current Auth session-floor checker")
		}
		if !callStoreDurable {
			log.Fatal("Space media invalidation requires the durable Voice Redis call store")
		}
		if err := voiceSvc.SpaceMediaAdmissions.CheckSchema(runCtx); err != nil {
			log.Fatalf("Space media PostgreSQL admission schema: %v", err)
		}
		if voiceRoomAccessResolver == nil || voiceRoomGrantResolver == nil {
			log.Fatal("Space media invalidation requires Space and Role authority resolvers")
		}
		projectionGates, ok := callStore.(voicestore.SpaceProjectionGates)
		if !ok {
			log.Fatal("Space requires an incarnation-fenced projection store")
		}
		voiceSvc.SpaceProjectionGates = projectionGates
		spaceMediaServing := &atomic.Bool{}
		voiceSvc.SpaceMediaServing = spaceMediaServing
		spaceMediaCoordinator := &spacemedia.Coordinator{
			ProjectionGates: projectionGates, Store: callStore, Calls: callStore, Admissions: spaceMediaAdmissions,
			SessionEpochChecker: sessionEpochChecker,
			Access:              voiceRoomAccessResolver, Grants: voiceRoomGrantResolver, Media: mediaLifecycle,
			Every: spacemedia.ReconcileInterval,
			OnError: func(err error) {
				logger.Warn("Space media authority reconciliation failed", slog.String("error", err.Error()))
			},
			OnReadiness: spaceMediaServing.Store,
		}
		if service, ok := spaceLifecycleController.(*spacelifecycle.Service); ok {
			service.Effects.NativeDrain = spaceMediaCoordinator
			fence, ok := callStore.(interface {
				SetSpaceLifecycle(context.Context, string, int64, bool) error
			})
			if !ok {
				log.Fatal("P3 requires lifecycle-fenced Redis projection")
			}
			service.Effects.ProjectionFence = fence
			// Protected P3 has one independently wired exact-drain recovery loop.
		}
		voiceSvc.SpaceMediaRevoker = spaceMediaCoordinator
		invalidationConsumer, err := spacemedia.StartInvalidationConsumer(runCtx, natsURL, spaceMediaCoordinator, logger)
		if err != nil {
			log.Fatalf("Space media invalidation consumer startup: %v", err)
		}
		closeRuntime = invalidationConsumer.Close
		if err := spaceMediaCoordinator.ReconcileAll(runCtx); err != nil {
			log.Fatalf("Space media initial reconciliation: %v", err)
		}
		admissionPublisher, ok := events.(spacemedia.AdmissionOutboxPublisher)
		if !ok {
			log.Fatal("Space media durable event publisher is not configured")
		}
		admissionRelay := &spacemedia.AdmissionOutboxRelay{

			Store: spaceMediaAdmissions, Publisher: admissionPublisher,
			OnError: func(err error) {
				logger.Warn("Space media admission outbox delivery failed", slog.String("error", err.Error()))
			},
		}
		// Native A3 facts remain pending. Starting their roster relay requires a
		// separately qualified broker/consumer package, not ordinary NATS_URL.
		go func() {
			if err := admissionRelay.Run(runCtx); err != nil && runCtx.Err() == nil {
				logger.Error("Space media admission outbox relay stopped", slog.String("error", err.Error()))
			}
		}()
		go func() {
			if err := spaceMediaCoordinator.Run(runCtx); err != nil && runCtx.Err() == nil {
				logger.Error("Space media authority reconciler stopped", slog.String("error", err.Error()))
			}
		}()
	}
	return closeRuntime
}
