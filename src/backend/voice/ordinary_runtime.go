package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	voicepostgres "voice/backend/pkg/postgres"
	"voice/backend/voice/internal/grpcsvc"
	"voice/backend/voice/internal/livekit"
	"voice/backend/voice/internal/ordinarycall"
	voicestore "voice/backend/voice/internal/store"
)

type ordinaryRuntime struct {
	Coordinator *ordinarycall.Coordinator
	Store       *ordinarycall.PostgresStore
	pool        *pgxpool.Pool
}

func openOrdinaryRuntime(ctx context.Context, calls voicestore.CallStore) (*ordinaryRuntime, error) {
	enabled := strings.TrimSpace(os.Getenv("VOICE_ORDINARY_SDK_ENABLED"))
	if enabled != "" && enabled != "true" && enabled != "false" {
		return nil, fmt.Errorf("VOICE_ORDINARY_SDK_ENABLED must be true or false")
	}
	dsn := strings.TrimSpace(os.Getenv("VOICE_DATABASE_URL"))
	if enabled != "true" {
		// Rolling back configuration after native writes must not route its Redis
		// projections back into tuple-only legacy mutations.
		if dsn == "" {
			return nil, nil
		}
		pool, err := voicepostgres.NewPool(ctx, dsn)
		if err != nil {
			return nil, err
		}
		defer pool.Close()
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass('voice_ordinary_rooms') IS NOT NULL`).Scan(&exists); err != nil {
			return nil, err
		}
		if exists {
			var any bool
			if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_ordinary_rooms)`).Scan(&any); err != nil {
				return nil, err
			}
			if any {
				return nil, fmt.Errorf("native ordinary journal exists; disabling its authority requires an explicit retirement protocol")
			}
		}
		return nil, nil
	}
	if dsn == "" || strings.TrimSpace(os.Getenv("VOICE_ORDINARY_LEGACY_DRAINED")) != "true" {
		return nil, fmt.Errorf("ordinary SDK requires VOICE_DATABASE_URL and verified legacy drain attestation")
	}
	for _, key := range []string{"LIVEKIT_URL", "LIVEKIT_API_KEY", "LIVEKIT_API_SECRET", "NATS_URL", "USER_GRPC_ADDR", "CHAT_GRPC_ADDR"} {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			return nil, fmt.Errorf("ordinary SDK requires %s", key)
		}
	}
	redisCalls, ok := calls.(*voicestore.RedisCallStore)
	if !ok {
		return nil, fmt.Errorf("ordinary SDK requires durable Redis projection")
	}
	pool, err := voicepostgres.NewPool(ctx, dsn)
	if err != nil {
		return nil, err
	}
	journal := ordinarycall.NewPostgresStore(pool, grpcsvc.EncodeOrdinaryEvent)
	fail := func(err error) (*ordinaryRuntime, error) { pool.Close(); return nil, err }
	if err := journal.CheckSchema(ctx); err != nil {
		return fail(err)
	}
	if err := redisCalls.ScanOrdinaryCutover(ctx, func(ctx context.Context, c voicestore.Call) error {
		id, e := uuid.Parse(c.RoomID)
		if e != nil || id == uuid.Nil || c.OrdinaryOperationID == "" {
			return fmt.Errorf("legacy ordinary call remains; controlled cutover required")
		}
		r, e := journal.Load(ctx, id)
		if e != nil {
			return e
		}
		if r.Intent.OperationID.String() != c.OrdinaryOperationID {
			return ordinarycall.ErrConflict
		}
		return nil
	}); err != nil {
		return fail(err)
	}
	media := livekit.NewSDKRoomLifecycle(strings.TrimSpace(os.Getenv("LIVEKIT_URL")), strings.TrimSpace(os.Getenv("LIVEKIT_API_KEY")), strings.TrimSpace(os.Getenv("LIVEKIT_API_SECRET")))
	coordinator := &ordinarycall.Coordinator{Journal: journal, Projection: grpcsvc.NewOrdinaryProjection(redisCalls, journal), Media: media, EffectTimeout: 5 * time.Second, Now: time.Now}
	return &ordinaryRuntime{Coordinator: coordinator, Store: journal, pool: pool}, nil
}
