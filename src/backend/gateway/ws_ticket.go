package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultWsTicketTTL = 60 * time.Second

type wsTicketRecord struct {
	UserID           string   `json:"user_id"`
	ProfileID        string   `json:"profile_id"`
	Roles            []string `json:"roles"`
	SubscriptionTier string   `json:"subscription_tier"`
	AccountType      string   `json:"account_type"`
	UpstreamToken    string   `json:"upstream_token"`
}

func (r wsTicketRecord) claims() tokenClaims {
	accountType := r.AccountType
	if accountType == "" {
		accountType = "regular"
	}
	return tokenClaims{
		UserID:           r.UserID,
		ProfileID:        r.ProfileID,
		Roles:            r.Roles,
		SubscriptionTier: r.SubscriptionTier,
		AccountType:      accountType,
	}
}

type wsTicketStore interface {
	Issue(ctx context.Context, record wsTicketRecord, ttl time.Duration) (string, error)
	Consume(ctx context.Context, ticket string) (wsTicketRecord, bool, error)
}

type memoryWsTicketStore struct {
	mu      sync.Mutex
	entries map[string]memoryWsTicketEntry
}

type memoryWsTicketEntry struct {
	record wsTicketRecord
	expiry time.Time
}

func newMemoryWsTicketStore() *memoryWsTicketStore {
	return &memoryWsTicketStore{entries: map[string]memoryWsTicketEntry{}}
}

func (s *memoryWsTicketStore) Issue(_ context.Context, record wsTicketRecord, ttl time.Duration) (string, error) {
	ticket, err := newWsTicketValue()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpired()
	s.entries[ticket] = memoryWsTicketEntry{record: record, expiry: time.Now().Add(ttl)}
	return ticket, nil
}

func (s *memoryWsTicketStore) Consume(_ context.Context, ticket string) (wsTicketRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[ticket]
	if !ok {
		return wsTicketRecord{}, false, nil
	}
	delete(s.entries, ticket)
	if time.Now().After(entry.expiry) {
		return wsTicketRecord{}, false, nil
	}
	return entry.record, true, nil
}

func (s *memoryWsTicketStore) pruneExpired() {
	now := time.Now()
	for key, entry := range s.entries {
		if now.After(entry.expiry) {
			delete(s.entries, key)
		}
	}
}

type redisWsTicketStore struct {
	client *redis.Client
	prefix string
}

func newRedisWsTicketStore(addr, password, prefix string) *redisWsTicketStore {
	if prefix == "" {
		prefix = "ws:ticket:"
	}
	return &redisWsTicketStore{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
		}),
		prefix: prefix,
	}
}

func (s *redisWsTicketStore) Issue(ctx context.Context, record wsTicketRecord, ttl time.Duration) (string, error) {
	ticket, err := newWsTicketValue()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	if err := s.client.Set(ctx, s.prefix+ticket, payload, ttl).Err(); err != nil {
		return "", err
	}
	return ticket, nil
}

func (s *redisWsTicketStore) Consume(ctx context.Context, ticket string) (wsTicketRecord, bool, error) {
	raw, err := s.client.GetDel(ctx, s.prefix+ticket).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return wsTicketRecord{}, false, nil
		}
		return wsTicketRecord{}, false, err
	}
	var record wsTicketRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return wsTicketRecord{}, false, err
	}
	return record, true, nil
}

func newWsTicketValue() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func wsTicketTTLFromEnv() time.Duration {
	ttl := defaultWsTicketTTL
	if raw := strings.TrimSpace(os.Getenv("GATEWAY_WS_TICKET_TTL_SECONDS")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			ttl = time.Duration(seconds) * time.Second
		}
	}
	return ttl
}
