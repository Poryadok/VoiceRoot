package store

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"strings"

	"github.com/redis/go-redis/v9"
)

// LegacyRedisSnapshot is raw compatibility evidence. It does not prove that
// historical endpoints, writers or media realms have been retired.
type LegacyRedisSnapshot struct {
	Prefix  string
	Entries []LegacyRedisEntry
}

type LegacyRedisEntry struct {
	Key     string
	Kind    string
	Value   string
	Members []string
}

// Validate checks captured raw evidence only, without granting cleanup authority.
func (s LegacyRedisSnapshot) Validate() error { return validateLegacySnapshot(s) }

func (s *RedisCallStore) CaptureLegacyCutover(ctx context.Context) (LegacyRedisSnapshot, error) {
	if s == nil || s.client == nil || s.prefix == "" || strings.ContainsAny(s.prefix, "*?[]\\") {
		return LegacyRedisSnapshot{}, ErrInvalidState
	}
	result := LegacyRedisSnapshot{Prefix: s.prefix, Entries: []LegacyRedisEntry{}}
	seen := map[string]bool{}
	var cursor uint64
	for {
		keys, next, err := s.client.Scan(ctx, cursor, s.prefix+"*", 128).Result()
		if err != nil {
			return result, err
		}
		for _, key := range keys {
			if seen[key] {
				continue
			}
			seen[key] = true
			entry, err := readLegacyEntry(ctx, s.client, key)
			if err != nil {
				return result, err
			}
			result.Entries = append(result.Entries, entry)
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	sort.Slice(result.Entries, func(i, j int) bool { return result.Entries[i].Key < result.Entries[j].Key })
	// Preserve the raw inventory even when its interpretation is unavailable.
	// A missing call can mean TTL loss, never no historical issuance.
	return result, validateLegacySnapshot(result)
}

// CompareCleanupLegacyCutover is a projection-owner primitive for qualified
// maintenance only. Its caller must resolve protected owner work and fresh
// retirement evidence first. This method neither fences writers nor contacts
// any media provider. It deliberately preserves epoch/lifecycle denial records.
func (s *RedisCallStore) CompareCleanupLegacyCutover(ctx context.Context, captured LegacyRedisSnapshot) error {
	if s == nil || s.client == nil || captured.Prefix != s.prefix {
		return ErrInvalidState
	}
	if err := validateLegacySnapshot(captured); err != nil {
		return err
	}
	keys := make([]string, len(captured.Entries))
	for i, entry := range captured.Entries {
		keys[i] = entry.Key
	}
	if len(keys) == 0 {
		return nil
	}
	err := s.client.Watch(ctx, func(tx *redis.Tx) error {
		for _, expected := range captured.Entries {
			actual, err := readLegacyEntry(ctx, tx, expected.Key)
			if errors.Is(err, redis.Nil) && !retainedLegacyKey(strings.TrimPrefix(expected.Key, s.prefix)) {
				continue
			}
			if err != nil {
				return err
			}
			if actual.Kind != expected.Kind || actual.Value != expected.Value || !slices.Equal(actual.Members, expected.Members) {
				return ErrOperationConflict
			}
		}
		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			for _, entry := range captured.Entries {
				if !retainedLegacyKey(strings.TrimPrefix(entry.Key, s.prefix)) {
					pipe.Del(ctx, entry.Key)
				}
			}
			return nil
		})
		return err
	}, keys...)
	if errors.Is(err, redis.TxFailedErr) {
		return ErrOperationConflict
	}
	return err
}

type legacyRedisReader interface {
	Type(context.Context, string) *redis.StatusCmd
	Get(context.Context, string) *redis.StringCmd
	SMembers(context.Context, string) *redis.StringSliceCmd
}

func readLegacyEntry(ctx context.Context, r legacyRedisReader, key string) (LegacyRedisEntry, error) {
	e := LegacyRedisEntry{Key: key}
	kind, err := r.Type(ctx, key).Result()
	if err != nil {
		return e, err
	}
	e.Kind = kind
	switch kind {
	case "none":
		return e, redis.Nil
	case "string":
		e.Value, err = r.Get(ctx, key).Result()
	case "set":
		e.Members, err = r.SMembers(ctx, key).Result()
		sort.Strings(e.Members)
	default:
		return e, ErrInvalidState
	}
	return e, err
}

func retainedLegacyKey(key string) bool {
	return strings.HasPrefix(key, "voice_move_operation:") || strings.HasPrefix(key, "match_squad_epoch:") || strings.HasPrefix(key, "match_squad_terminal_epoch:") || strings.HasPrefix(key, "space_lifecycle:") || strings.HasPrefix(key, "space_media_floor:") || strings.HasPrefix(key, "space_media_reconciled:")
}

func validateLegacySnapshot(snapshot LegacyRedisSnapshot) error {
	if snapshot.Prefix == "" || strings.ContainsAny(snapshot.Prefix, "*?[]\\") {
		return ErrInvalidState
	}
	calls := map[string]Call{}
	seen := map[string]bool{}
	for _, e := range snapshot.Entries {
		key, ok := strings.CutPrefix(e.Key, snapshot.Prefix)
		if !ok || key == "" || seen[e.Key] {
			return ErrInvalidState
		}
		seen[e.Key] = true
		if e.Kind != "string" && e.Kind != "set" {
			return ErrInvalidState
		}
		if e.Kind == "string" && len(e.Members) != 0 || e.Kind == "set" && (e.Value != "" || !sort.StringsAreSorted(e.Members)) {
			return ErrInvalidState
		}
		if strings.HasPrefix(key, "call:") {
			var call Call
			if e.Kind != "string" || json.Unmarshal([]byte(e.Value), &call) != nil || call.RoomID == "" || key != "call:"+call.RoomID {
				return ErrInvalidState
			}
			calls[call.RoomID] = call
		}
	}
	for _, e := range snapshot.Entries {
		key := strings.TrimPrefix(e.Key, snapshot.Prefix)
		kind, id, ok := strings.Cut(key, ":")
		if !ok || id == "" {
			return ErrInvalidState
		}
		if retainedLegacyKey(key) && kind != "voice_move_operation" {
			if e.Kind != "string" || e.Value == "" {
				return ErrInvalidState
			}
			if kind == "space_lifecycle" {
				if _, err := decodeSpaceGate(e.Value); err != nil {
					return err
				}
			}
			continue
		}
		switch kind {
		case "call":
		case "session", "active_chat", "active_voice_room":
			call, exists := calls[e.Value]
			if e.Kind != "string" || !exists {
				return ErrInvalidState
			}
			if kind == "session" && !call.IsParticipant(id) || kind == "active_chat" && call.ChatID != id || kind == "active_voice_room" && call.VoiceRoomID != id {
				return ErrInvalidState
			}
		case "active_space_voice_rooms":
			if e.Kind != "set" || len(e.Members) == 0 {
				return ErrInvalidState
			}
			for _, room := range e.Members {
				if c, exists := calls[room]; !exists || c.SpaceID != id || !c.IsVoiceRoom() {
					return ErrInvalidState
				}
			}
		case "voice_move_operation":
			var ledger redisVoiceRoomMoveLedger
			if e.Kind != "string" || json.Unmarshal([]byte(e.Value), &ledger) != nil || ledger.Request.ActorProfileID == "" || ledger.Request.OperationID == "" || id != ledger.Request.ActorProfileID+":"+ledger.Request.OperationID || ledger.Result.Source.RoomID == "" || ledger.Result.Destination.RoomID == "" {
				return ErrInvalidState
			}
			source, sourceFound := calls[ledger.Result.Source.RoomID]
			destination, destinationFound := calls[ledger.Result.Destination.RoomID]
			if !sourceFound || !destinationFound || source.SpaceID != ledger.Request.SpaceID || destination.SpaceID != ledger.Request.SpaceID || source.VoiceRoomID != ledger.Request.FromVoiceRoomID || destination.VoiceRoomID != ledger.Request.ToVoiceRoomID {
				return ErrInvalidState
			}
		default:
			return ErrInvalidState
		}
	}
	return nil
}
