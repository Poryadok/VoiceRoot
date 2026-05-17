package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// TTLs from docs/microservices/user-service.md (presence cache heartbeat).
const (
	PresenceSessionTTL = 5 * time.Minute
	presenceLastSeenTTL = 30 * 24 * time.Hour
)

const (
	hashFieldStatus       = "status"
	hashFieldStatusEnum   = "status_enum"
	hashFieldGameTitle    = "game_title"
	hashFieldCustomStatus = "custom_status"
	hashFieldCallInfo     = "call_info_json"
	hashFieldTS           = "ts_unix"
)

// PresenceStore persists ephemeral presence in Redis (session hash + last_seen string).
type PresenceStore struct {
	rdb *redis.Client
}

func NewPresenceStore(rdb *redis.Client) *PresenceStore {
	return &PresenceStore{rdb: rdb}
}

func presenceHashKey(profileID uuid.UUID) string {
	return fmt.Sprintf("voice:user:presence:%s", profileID.String())
}

func lastSeenRedisKey(profileID uuid.UUID) string {
	return fmt.Sprintf("voice:user:last_seen:%s", profileID.String())
}

// PresenceUpsert is one heartbeat / status update for a profile.
type PresenceUpsert struct {
	Status         string
	StatusEnum     int32
	GameTitle      string
	CustomStatus   string
	CallInfoJSON   string
	Now            time.Time
}

// PresenceSnapshot is Redis-backed presence for one profile (live session and/or last_seen).
type PresenceSnapshot struct {
	Live           bool
	Status         string
	StatusEnum     int32
	GameTitle      string
	CustomStatus   string
	CallInfoJSON   string
	LastActiveUnix int64
	LastSeenUnix   int64
}

func (s *PresenceStore) Upsert(ctx context.Context, profileID uuid.UUID, in PresenceUpsert) error {
	if s == nil || s.rdb == nil {
		return fmt.Errorf("presence store not configured")
	}
	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	ts := strconv.FormatInt(now.Unix(), 10)
	hk := presenceHashKey(profileID)
	lsk := lastSeenRedisKey(profileID)

	pipe := s.rdb.Pipeline()
	pipe.HSet(ctx, hk,
		hashFieldStatus, in.Status,
		hashFieldStatusEnum, strconv.FormatInt(int64(in.StatusEnum), 10),
		hashFieldGameTitle, in.GameTitle,
		hashFieldCustomStatus, in.CustomStatus,
		hashFieldCallInfo, in.CallInfoJSON,
		hashFieldTS, ts,
	)
	pipe.Expire(ctx, hk, PresenceSessionTTL)
	pipe.Set(ctx, lsk, ts, presenceLastSeenTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// Get returns live presence if the session key exists; otherwise offline data from last_seen.
func (s *PresenceStore) Get(ctx context.Context, profileID uuid.UUID) (*PresenceSnapshot, error) {
	if s == nil || s.rdb == nil {
		return nil, fmt.Errorf("presence store not configured")
	}
	hk := presenceHashKey(profileID)
	lsk := lastSeenRedisKey(profileID)

	pipe := s.rdb.Pipeline()
	cmdH := pipe.HGetAll(ctx, hk)
	cmdLS := pipe.Get(ctx, lsk)
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}

	h, err := cmdH.Result()
	if err != nil {
		return nil, err
	}
	lsStr, lsErr := cmdLS.Result()
	if lsErr != nil && lsErr != redis.Nil {
		return nil, lsErr
	}

	out := &PresenceSnapshot{}
	if len(h) > 0 {
		out.Live = true
		out.Status = h[hashFieldStatus]
		if v := h[hashFieldStatusEnum]; v != "" {
			if n, err := strconv.ParseInt(v, 10, 32); err == nil {
				out.StatusEnum = int32(n)
			}
		}
		out.GameTitle = h[hashFieldGameTitle]
		out.CustomStatus = h[hashFieldCustomStatus]
		out.CallInfoJSON = h[hashFieldCallInfo]
		if v := h[hashFieldTS]; v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				out.LastActiveUnix = n
				out.LastSeenUnix = n
			}
		}
		return out, nil
	}

	if lsStr != "" {
		if n, err := strconv.ParseInt(strings.TrimSpace(lsStr), 10, 64); err == nil {
			out.LastSeenUnix = n
		}
	}
	return out, nil
}

const maxBulkPresence = 256

// GetMany returns a snapshot per requested profile id (missing keys are omitted from the map).
func (s *PresenceStore) GetMany(ctx context.Context, profileIDs []uuid.UUID) (map[uuid.UUID]*PresenceSnapshot, error) {
	if s == nil || s.rdb == nil {
		return nil, fmt.Errorf("presence store not configured")
	}
	if len(profileIDs) == 0 {
		return map[uuid.UUID]*PresenceSnapshot{}, nil
	}
	if len(profileIDs) > maxBulkPresence {
		profileIDs = profileIDs[:maxBulkPresence]
	}

	seen := make(map[uuid.UUID]struct{}, len(profileIDs))
	dedup := make([]uuid.UUID, 0, len(profileIDs))
	for _, id := range profileIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		dedup = append(dedup, id)
	}

	pipe := s.rdb.Pipeline()
	type pair struct {
		id uuid.UUID
		h  *redis.MapStringStringCmd
		ls *redis.StringCmd
	}
	orders := make([]pair, 0, len(dedup))
	for _, id := range dedup {
		orders = append(orders, pair{
			id: id,
			h:  pipe.HGetAll(ctx, presenceHashKey(id)),
			ls: pipe.Get(ctx, lastSeenRedisKey(id)),
		})
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}

	out := make(map[uuid.UUID]*PresenceSnapshot, len(orders))
	for _, o := range orders {
		snap, err := snapshotFromHGetAndGet(o.h, o.ls)
		if err != nil {
			return nil, err
		}
		out[o.id] = snap
	}
	return out, nil
}

func snapshotFromHGetAndGet(cmdH *redis.MapStringStringCmd, cmdLS *redis.StringCmd) (*PresenceSnapshot, error) {
	h, err := cmdH.Result()
	if err != nil {
		return nil, err
	}
	lsStr, lsErr := cmdLS.Result()
	if lsErr != nil && lsErr != redis.Nil {
		return nil, lsErr
	}

	out := &PresenceSnapshot{}
	if len(h) > 0 {
		out.Live = true
		out.Status = h[hashFieldStatus]
		if v := h[hashFieldStatusEnum]; v != "" {
			if n, err := strconv.ParseInt(v, 10, 32); err == nil {
				out.StatusEnum = int32(n)
			}
		}
		out.GameTitle = h[hashFieldGameTitle]
		out.CustomStatus = h[hashFieldCustomStatus]
		out.CallInfoJSON = h[hashFieldCallInfo]
		if v := h[hashFieldTS]; v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				out.LastActiveUnix = n
				out.LastSeenUnix = n
			}
		}
		return out, nil
	}
	if lsStr != "" {
		if n, err := strconv.ParseInt(strings.TrimSpace(lsStr), 10, 64); err == nil {
			out.LastSeenUnix = n
		}
	}
	return out, nil
}
