package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	userv1 "voice.app/voice/user/v1"
	"voice/backend/search/internal/profileprojection"
)

// desiredProjectionGeneration is deliberately optional: the durable database
// route remains the source of serving truth when an operator has not requested
// a new rebuild.
func desiredProjectionGeneration() (uint64, bool, error) {
	raw := strings.TrimSpace(os.Getenv("SEARCH_USER_PROJECTION_DESIRED_GENERATION"))
	if raw == "" {
		return 0, false, nil
	}
	generation, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || generation == 0 {
		return 0, false, fmt.Errorf("SEARCH_USER_PROJECTION_DESIRED_GENERATION must be a positive integer")
	}
	return generation, true, nil
}

// requireProtectedProjectionAuthority prevents an activated durable route from
// silently falling back to the raw legacy hydrator during configuration drift.
func requireProtectedProjectionAuthority(desired, routePresent, protectedClient bool) error {
	if (desired || routePresent) && !protectedClient {
		return fmt.Errorf("activated User projection generation requires protected User authority")
	}
	return nil
}

// runDesiredProjectionGeneration never changes serving state until a complete
// snapshot/replay marks the target ready. A PostgreSQL session advisory lock
// allows one rebuilder; other replicas retry after a failed owner disappears.
func runDesiredProjectionGeneration(ctx context.Context, logger *slog.Logger, client userv1.UserServiceClient, pool *pgxpool.Pool, generation uint64) {
	for ctx.Err() == nil {
		lease, acquired, err := profileprojection.TryStartGeneration(ctx, pool, generation)
		if err == nil && acquired {
			target := &profileprojection.StoreAdapter{Pool: pool, Generation: generation}
			err = runUserProjectionBootstrap(ctx, client, target)
			if err == nil {
				var evidence profileprojection.ReadinessEvidence
				evidence, err = replayProjectionEvidence(ctx, client, pool, generation)
				if err == nil {
					err = profileprojection.VerifyGenerationEvidence(ctx, pool, generation, evidence)
				}
			}
			if err == nil {
				err = profileprojection.MarkGenerationReady(ctx, pool, generation)
			}
			if err == nil {
				_, err = profileprojection.PromoteGenerationWithVerification(ctx, pool, generation, func() error {
					evidence, replayErr := replayProjectionEvidence(ctx, client, pool, generation)
					if replayErr != nil {
						return replayErr
					}
					return profileprojection.VerifyGenerationEvidence(ctx, pool, generation, evidence)
				})
				if err != nil {
					_ = profileprojection.ReopenGeneration(ctx, pool, generation)
				}
			}
			profileprojection.FinishGenerationRebuild(ctx, lease)
			if err == nil {
				return
			}
		} else if err == nil {
			route, routeErr := profileprojection.LoadGenerationRoute(ctx, pool)
			if routeErr == nil && route.Active == generation {
				return
			}
			if routeErr == nil {
				_, err = profileprojection.PromoteGenerationWithVerification(ctx, pool, generation, func() error {
					evidence, replayErr := replayProjectionEvidence(ctx, client, pool, generation)
					if replayErr != nil {
						return replayErr
					}
					return profileprojection.VerifyGenerationEvidence(ctx, pool, generation, evidence)
				})
				if err == nil {
					return
				}
			} else {
				err = routeErr
			}
		}
		if err != nil && logger != nil {
			logger.Warn("User profile projection generation rebuild failed; retrying", slog.Any("error", err))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// replayProjectionEvidence independently reads the protected authority without
// mutating Search. Promotion compares this proof to the candidate's atomically
// persisted stream evidence.
func replayProjectionEvidence(ctx context.Context, client userv1.UserServiceClient, pool *pgxpool.Pool, generation uint64) (profileprojection.ReadinessEvidence, error) {
	high, cutoff, err := profileprojection.GenerationReplayBounds(ctx, pool, generation)
	if err != nil {
		return profileprojection.ReadinessEvidence{}, err
	}
	return replayProjectionEvidenceAt(ctx, client, generation, high, cutoff)
}

func replayProjectionEvidenceAt(ctx context.Context, client userv1.UserServiceClient, generation, highWatermark, cutoff uint64) (profileprojection.ReadinessEvidence, error) {
	collector, err := profileprojection.NewReplayEvidenceCollector(generation, highWatermark)
	if err != nil {
		return profileprojection.ReadinessEvidence{}, err
	}
	for cursor := ""; ; {
		page, err := client.ListSearchProfileSnapshot(ctx, &userv1.ListSearchProfileSnapshotRequest{HighWatermark: highWatermark, PageSize: 100, Cursor: cursor})
		if err != nil {
			return profileprojection.ReadinessEvidence{}, err
		}
		for _, event := range page.GetEvents() {
			bytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(event)
			if err != nil {
				return profileprojection.ReadinessEvidence{}, err
			}
			if err := collector.AddSnapshot(event.GetJournalOffset(), bytes); err != nil {
				return profileprojection.ReadinessEvidence{}, err
			}
		}
		if page.GetNextCursor() == "" {
			break
		}
		cursor = page.GetNextCursor()
	}
	for after := highWatermark; ; {
		page, err := client.ListSearchProfileJournal(ctx, &userv1.ListSearchProfileJournalRequest{AfterOffset: after, PageSize: 100})
		if err != nil {
			return profileprojection.ReadinessEvidence{}, err
		}
		for _, event := range page.GetEvents() {
			// The authority page may have advanced beyond persisted C. C+1 is not
			// an invalid candidate; route-lock promotion detects the fresh cutoff
			// and sends this target through catch-up/reverification.
			if event.GetJournalOffset() > cutoff {
				break
			}
			bytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(event)
			if err != nil {
				return profileprojection.ReadinessEvidence{}, err
			}
			if err := collector.AddJournal(event.GetJournalOffset(), bytes); err != nil {
				return profileprojection.ReadinessEvidence{}, err
			}
			after = event.GetJournalOffset()
		}
		if len(page.GetEvents()) < 100 {
			if collector.Evidence.Cutoff != cutoff {
				return profileprojection.ReadinessEvidence{}, fmt.Errorf("journal replay did not reach candidate cutoff")
			}
			return collector.Evidence, nil
		}
	}
}
