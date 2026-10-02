package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	notificationv1 "voice.app/voice/notification/v1"
	rolev1 "voice.app/voice/role/v1"
	_ "voice.app/voice/search/v1"
	_ "voice.app/voice/subscription/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/space/internal/grpcsvc"
	"voice/backend/space/internal/lifecyclecoord"
	"voice/backend/space/internal/spacecore"
	"voice/backend/space/internal/spaceevents"
	"voice/backend/space/internal/store"
	"voice/backend/space/internal/tombstone"
)

type lifecycleRuntime struct {
	cancel      context.CancelFunc
	wait        sync.WaitGroup
	stop        sync.Once
	connections []*grpc.ClientConn
}
type localLifecycleFinalizer struct {
	store  *store.SpaceStore
	hasher store.AccountHasher
}

func (f localLifecycleFinalizer) Finalize(ctx context.Context, id uuid.UUID) error {
	_, err := f.store.FinalizeLifecyclePurge(ctx, id, f.hasher)
	return err
}

type lifecycleRecoveryScanner interface {
	ListDueLifecycleAttempts(context.Context, int) ([]store.LifecycleAttempt, error)
	DeferLifecycleAttempt(context.Context, uuid.UUID) error
}
type lifecycleRecoveryCoordinator interface {
	Recover(context.Context, uuid.UUID, lifecyclecoord.PurgeFinalizer) error
}
type lifecycleRecoveryWorker struct {
	scanner      lifecycleRecoveryScanner
	coordinator  lifecycleRecoveryCoordinator
	finalizer    lifecyclecoord.PurgeFinalizer
	recoverProof func(context.Context, uuid.UUID) error
	logger       *slog.Logger
}

func (w lifecycleRecoveryWorker) RunOnce(ctx context.Context) error {
	attempts, err := w.scanner.ListDueLifecycleAttempts(ctx, 100)
	if err != nil {
		return err
	}
	var failures []error
	for _, attempt := range attempts {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt.Stalled && w.logger != nil {
			w.logger.Error("Space lifecycle stalled", slog.String("space_id", attempt.SpaceID.String()), slog.String("phase", attempt.Phase), slog.Uint64("generation", attempt.Generation))
		}
		// Each owner RPC has its own deadline. A large immutable manifest can take
		// many pages; a fixed outer deadline would repeatedly starve its last page.
		call, cancel := context.WithCancel(ctx)
		var err error
		if attempt.Phase == "SCHEDULE_PENDING" {
			err = w.recoverProof(call, attempt.SpaceID)
		}
		if err == nil {
			err = w.coordinator.Recover(call, attempt.SpaceID, w.finalizer)
		}
		cancel()
		if err != nil {
			failures = append(failures, errors.Join(err, w.scanner.DeferLifecycleAttempt(ctx, attempt.SpaceID)))
		}
	}
	return errors.Join(failures...)
}

func newLifecycleRuntime(parent context.Context, c lifecycleRuntimeConfig, spaceStore *store.SpaceStore, service *grpcsvc.SpaceGRPC, publisher *spaceevents.JetStreamPublisher, logger *slog.Logger) (*lifecycleRuntime, error) {
	if parent == nil || spaceStore == nil || spaceStore.Pool == nil || service == nil || service.PrincipalIssuer == nil || service.OwnershipAuth == nil || publisher == nil || len(c.Owners) != 10 {
		return nil, errors.New("Space lifecycle requires database, Auth, issuer, JetStream and all ten owners")
	}
	// Validate the deployed migration before exposing a destructive public route.
	var ready bool
	if err := spaceStore.Pool.QueryRow(parent, `SELECT to_regprocedure('space_terminal_purge_ready(uuid)') IS NOT NULL AND EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='space_lifecycle_aggregates' AND column_name='next_attempt_at') AND to_regclass('space_lifecycle_completions') IS NOT NULL`).Scan(&ready); err != nil || !ready {
		return nil, errors.New("Space lifecycle migrations 000022 and 000023 are required")
	}
	hasher, err := tombstone.LoadDevelopmentKey(c.DevTombstoneKeyFile)
	if err != nil {
		return nil, err
	}
	rootsPEM, err := os.ReadFile(c.CAFile)
	if err != nil {
		return nil, errors.New("Space lifecycle CA unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(rootsPEM) {
		return nil, errors.New("Space lifecycle CA contains no certificates")
	}
	runtime := &lifecycleRuntime{}
	failed := func(err error) (*lifecycleRuntime, error) { runtime.Stop(); return nil, err }
	clients := map[commonv1.ParticipantId]*grpc.ClientConn{}
	fences := map[commonv1.ParticipantId]lifecyclecoord.FenceParticipant{}
	purges := map[commonv1.ParticipantId]lifecyclecoord.PurgeParticipant{}
	for _, owner := range c.Owners {
		certFile, keyFile := c.CertFile, c.KeyFile
		if owner.ID == commonv1.ParticipantId_PARTICIPANT_ID_ROLE {
			certFile, keyFile = c.RoleCertFile, c.RoleKeyFile
		}
		certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return failed(fmt.Errorf("Space lifecycle %s client certificate unavailable", owner.Audience))
		}
		connection, err := grpc.NewClient(grpcclient.DialTarget(owner.Address), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: owner.ServerName, Certificates: []tls.Certificate{certificate}})))
		if err != nil {
			return failed(fmt.Errorf("Space lifecycle %s transport: %w", owner.Audience, err))
		}
		runtime.connections = append(runtime.connections, connection)
		clients[owner.ID] = connection
		participant, err := lifecyclecoord.NewGRPCParticipant(owner.ID, service.PrincipalIssuer, connection)
		if err != nil {
			return failed(fmt.Errorf("Space lifecycle %s contract: %w", owner.Audience, err))
		}
		fences[owner.ID] = participant
		if owner.ID != commonv1.ParticipantId_PARTICIPANT_ID_ROLE {
			purges[owner.ID] = participant
		}
	}
	owners := &grpcsvc.LifecycleManifestOwners{Issuer: service.PrincipalIssuer, Chat: chatv1.NewChatServiceClient(clients[commonv1.ParticipantId_PARTICIPANT_ID_CHAT]), Messaging: messagingv1.NewMessagingServiceClient(clients[commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING]), File: filev1.NewFileServiceClient(clients[commonv1.ParticipantId_PARTICIPANT_ID_FILE]), Notification: notificationv1.NewNotificationServiceClient(clients[commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION])}
	coordinator := lifecyclecoord.New(lifecyclecoord.Dependencies{Store: spaceStore, Participants: fences, PurgeParticipants: purges, RoleRetirement: &lifecyclecoord.GRPCRoleRetirementParticipant{Issuer: service.PrincipalIssuer, Client: rolev1.NewRoleServiceClient(clients[commonv1.ParticipantId_PARTICIPANT_ID_ROLE])}, Manifests: owners, FileProducer: owners})
	service.DeletionLifecycle = coordinator
	worker := lifecycleRecoveryWorker{scanner: spaceStore, coordinator: coordinator, finalizer: localLifecycleFinalizer{store: spaceStore, hasher: hasher}, recoverProof: service.RecoverLifecycleProof, logger: logger}
	ctx, cancel := context.WithCancel(parent)
	runtime.cancel = cancel
	runtime.wait.Add(1)
	go func() {
		defer runtime.wait.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			if err := spaceStore.CleanupLifecycleEvidence(ctx); err != nil && ctx.Err() == nil && logger != nil {
				logger.Error("Space lifecycle retention pass failed")
			}
			if err := worker.RunOnce(ctx); err != nil && ctx.Err() == nil && logger != nil {
				logger.Error("Space lifecycle recovery pass failed")
			}
			if ctx.Err() == nil {
				if err := dispatchLifecycleEvents(ctx, spaceStore, publisher); err != nil && logger != nil {
					logger.Error("Space lifecycle outbox pass failed")
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return runtime, nil
}
func (r *lifecycleRuntime) Stop() {
	if r == nil {
		return
	}
	r.stop.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
		r.wait.Wait()
		for _, connection := range r.connections {
			_ = connection.Close()
		}
	})
}

func dispatchLifecycleEvents(ctx context.Context, spaceStore *store.SpaceStore, publisher *spaceevents.JetStreamPublisher) error {
	return spaceStore.ReadReadyLifecycleOutbox(ctx, 100, func(record spacecore.LifecycleOutboxRecord) error {
		var decidedAt time.Time
		if record.EventType == "space.deleted" {
			var err error
			decidedAt, err = spaceStore.LifecycleEventPurgeDecision(ctx, record)
			if err != nil {
				return err
			}
		}
		call, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := publisher.PublishLifecycle(call, record, decidedAt); err != nil {
			return err
		}
		return spaceStore.MarkLifecycleEventDelivered(ctx, record)
	})
}
