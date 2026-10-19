package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"voice/backend/pkg/principal"
	"voice/backend/space/internal/creatorauthority"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/outboxdelivery"
	"voice/backend/space/internal/store"
)

type creatorReplayStore interface {
	ClaimCreatorPrincipalReplay(context.Context, string, string, time.Time) error
}

// Creator readiness belongs to its own public receiver dependencies, not the
// independent Role-only member-authority listener or its cleanup worker.
func creatorMembershipDependencies(replays creatorReplayStore, auth creatorauthority.Client, issuer *principal.Issuer) lifecycleprincipal.MembershipDependencies {
	if replays == nil || auth == nil || issuer == nil {
		return lifecycleprincipal.MembershipDependencies{}
	}
	checker := &creatorauthority.Checker{Client: auth, Issuer: issuer}
	return lifecycleprincipal.MembershipDependencies{
		CreatorEpoch: func(ctx context.Context, account string, epoch int64) error {
			err := checker.CheckEpoch(ctx, account, epoch)
			if errors.Is(err, creatorauthority.ErrUnavailable) {
				return lifecycleprincipal.ErrDependencyUnavailable
			}
			return err
		},
		CreatorReplay: func(ctx context.Context, issuer, jti string, expiry time.Time) error {
			err := replays.ClaimCreatorPrincipalReplay(ctx, issuer, jti, expiry)
			if errors.Is(err, store.ErrMemberAuthorityUnavailable) {
				return lifecycleprincipal.ErrDependencyUnavailable
			}
			return err
		},
	}
}

type creatorReplayCleanupStore interface {
	CleanupCreatorPrincipalReplays(context.Context, int) (int64, error)
}
type creatorReplayCleanup struct{ source creatorReplayCleanupStore }

func (d *creatorReplayCleanup) DispatchOnce(ctx context.Context) error {
	_, err := d.source.CleanupCreatorPrincipalReplays(ctx, 100)
	return err
}

func startCreatorReplayCleanup(ctx context.Context, source creatorReplayCleanupStore, logger *slog.Logger) func() {
	workerCtx, cancel := context.WithCancel(ctx)
	worker := outboxdelivery.NewWorker(&creatorReplayCleanup{source: source}, time.Second, func(error) { logger.Warn("creator replay retention retry pending") })
	done := make(chan struct{})
	go func() { defer close(done); _ = worker.Run(workerCtx) }()
	var once sync.Once
	return func() { once.Do(func() { cancel(); <-done }) }
}
