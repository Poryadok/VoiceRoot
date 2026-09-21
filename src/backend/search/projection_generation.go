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
				err = profileprojection.MarkGenerationReady(ctx, pool, generation)
			}
			if err == nil {
				_, err = profileprojection.PromoteGeneration(ctx, pool, generation)
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
				_, err = profileprojection.PromoteGeneration(ctx, pool, generation)
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
