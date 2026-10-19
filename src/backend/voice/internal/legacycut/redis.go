package legacycut

import (
	"context"
	"strings"

	"github.com/redis/go-redis/v9"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/store"
)

// RedisEndpoints is configured with scoped maintenance clients by the trusted
// installer. This registry is routing only: every operation compares the live
// Redis instance identity and DB with the protected inventory's exact binding.
type RedisEndpoints struct{ Clients map[string]*redis.Client }

func (r RedisEndpoints) resolve(ctx context.Context, e Endpoint) (*store.RedisCallStore, error) {
	client := r.Clients[e.ID]
	if client == nil {
		return nil, retirementproof.ErrUnavailable
	}
	if client.Options().DB != e.Database {
		return nil, retirementproof.ErrConflict
	}
	info, err := client.Info(ctx, "server").Result()
	if err != nil {
		return nil, err
	}
	identity, err := RedisInstanceIdentity(info)
	if err != nil {
		return nil, err
	}
	if identity != e.ResourceIdentity {
		return nil, retirementproof.ErrConflict
	}
	return store.NewRedisCallStore(client, e.Prefix), nil
}

// RedisInstanceIdentity binds the actual process incarnation, not its address,
// role name or a namespace label. A restart requires fresh cut reconciliation.
func RedisInstanceIdentity(serverInfo string) (retirementproof.Digest, error) {
	var runID string
	for _, line := range strings.Split(serverInfo, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && key == "run_id" {
			if runID != "" {
				return retirementproof.Digest{}, retirementproof.ErrConflict
			}
			runID = value
		}
	}
	if len(runID) != 40 {
		return retirementproof.Digest{}, retirementproof.ErrUnavailable
	}
	for _, c := range runID {
		if c < '0' || c > '9' {
			if c < 'a' || c > 'f' {
				return retirementproof.Digest{}, retirementproof.ErrUnavailable
			}
		}
	}
	return retirementproof.Hash("voice.legacy.redis-instance.v1", []byte(runID)), nil
}

func (r RedisEndpoints) Capture(ctx context.Context, e Endpoint) (store.LegacyRedisSnapshot, error) {
	s, err := r.resolve(ctx, e)
	if err != nil {
		return store.LegacyRedisSnapshot{}, err
	}
	return s.CaptureLegacyCutover(ctx)
}

func (r RedisEndpoints) CompareCleanup(ctx context.Context, e Endpoint, snapshot store.LegacyRedisSnapshot) error {
	s, err := r.resolve(ctx, e)
	if err != nil {
		return err
	}
	return s.CompareCleanupLegacyCutover(ctx, snapshot)
}
