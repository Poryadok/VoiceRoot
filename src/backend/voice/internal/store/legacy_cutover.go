package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/redis/go-redis/v9"
)

// LegacyRedisSnapshot is raw compatibility evidence. It does not prove that
// historical endpoints, writers or media realms have been retired.
type LegacyRedisSnapshot struct {
	Prefix              string
	ObservedAtUnixMilli int64
	Diagnostic          string
	Entries             []LegacyRedisEntry
}

type LegacyRedisEntry struct {
	Key                string
	Kind               string
	Value              string
	Members            []string
	RawKey             []byte
	Dump               []byte
	RawValue           []byte
	Expiry             string
	ExpiresAtUnixMilli int64
}

var ErrLegacyPartitionRequired = errors.New("protected legacy ownership partition and projection step required")

// Validate checks captured raw evidence only, without granting cleanup authority.
func (s LegacyRedisSnapshot) Validate() error { return validateLegacySnapshot(s) }

func (s *RedisCallStore) CaptureLegacyCutover(ctx context.Context) (LegacyRedisSnapshot, error) {
	if s == nil || s.client == nil || s.prefix == "" || strings.ContainsAny(s.prefix, "*?[]\\") {
		return LegacyRedisSnapshot{}, ErrInvalidState
	}
	result := LegacyRedisSnapshot{Prefix: s.prefix, Entries: []LegacyRedisEntry{}}
	raw, err := s.client.Eval(ctx, legacyAtomicCapture, nil, s.prefix, 512, 2*1024*1024, 128, 100).Slice()
	if err != nil {
		return result, err
	}
	if len(raw) != 3 {
		return result, ErrInvalidState
	}
	result.ObservedAtUnixMilli, err = legacyInteger(raw[0])
	if err != nil {
		return result, err
	}
	result.Diagnostic, _ = raw[1].(string)
	entries, ok := raw[2].([]interface{})
	if !ok {
		return result, ErrInvalidState
	}
	for _, item := range entries {
		fields, ok := item.([]interface{})
		if !ok || len(fields) != 7 {
			return result, ErrInvalidState
		}
		key, ok := fields[0].(string)
		if !ok {
			return result, ErrInvalidState
		}
		e := LegacyRedisEntry{Key: key, RawKey: []byte(key)}
		e.Kind, ok = fields[1].(string)
		if !ok {
			return result, ErrInvalidState
		}
		dump, ok := fields[2].(string)
		if !ok {
			return result, ErrInvalidState
		}
		e.Dump = []byte(dump)
		e.Expiry, ok = fields[3].(string)
		if !ok {
			return result, ErrInvalidState
		}
		e.ExpiresAtUnixMilli, err = legacyInteger(fields[4])
		if err != nil {
			return result, err
		}
		e.Value, ok = fields[5].(string)
		if !ok {
			return result, ErrInvalidState
		}
		e.RawValue = []byte(e.Value)
		members, ok := fields[6].([]interface{})
		if !ok {
			return result, ErrInvalidState
		}
		for _, member := range members {
			value, ok := member.(string)
			if !ok {
				return result, ErrInvalidState
			}
			e.Members = append(e.Members, value)
		}
		result.Entries = append(result.Entries, e)
	}
	if result.Diagnostic != "" {
		return result, ErrInvalidState
	}
	if err := validateLegacySnapshot(result); err != nil {
		result.Diagnostic = "UNCLASSIFIED_SOURCE_EVIDENCE"
		return result, err
	}
	return result, nil
}

// CompareCleanupLegacyCutover is a projection-owner primitive for qualified
// maintenance only. Its caller must resolve protected owner work and fresh
// retirement evidence first. This method neither fences writers nor contacts
// any media provider. It deliberately preserves epoch/lifecycle denial records.
func (s *RedisCallStore) CompareCleanupLegacyCutover(context.Context, LegacyRedisSnapshot) error {
	// Deliberately fail closed for the superseded draft API. A full snapshot
	// covers several semantic owners; matching raw bytes grants no deletion.
	return ErrLegacyPartitionRequired
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
	if snapshot.Prefix == "" || snapshot.Diagnostic != "" || strings.ContainsAny(snapshot.Prefix, "*?[]\\") {
		return ErrInvalidState
	}
	calls := map[string]Call{}
	seen := map[string]bool{}
	for _, e := range snapshot.Entries {
		if !utf8.Valid(e.RawKey) || !utf8.Valid(e.RawValue) || e.Key != string(e.RawKey) || e.Value != string(e.RawValue) || e.Expiry != "PERSISTENT" && e.Expiry != "DEADLINE" && e.Expiry != "ABSENT_AT_CUT" {
			return ErrInvalidState
		}
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
			if e.Kind != "string" || strictLegacyJSON(e.Value, &call) != nil || call.RoomID == "" || key != "call:"+call.RoomID {
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
			if e.Kind != "string" || strictLegacyJSON(e.Value, &ledger) != nil || ledger.Request.ActorProfileID == "" || ledger.Request.OperationID == "" || id != ledger.Request.ActorProfileID+":"+ledger.Request.OperationID || ledger.Result.Source.RoomID == "" || ledger.Result.Destination.RoomID == "" {
				return ErrInvalidState
			}
			if ledger.Request.SpaceID == "" || ledger.Request.ParticipantProfileID == "" || ledger.Result.Source.RoomID == ledger.Result.Destination.RoomID || ledger.Result.Source.SpaceID != ledger.Request.SpaceID || ledger.Result.Destination.SpaceID != ledger.Request.SpaceID || ledger.Result.Source.VoiceRoomID != ledger.Request.FromVoiceRoomID || ledger.Result.Destination.VoiceRoomID != ledger.Request.ToVoiceRoomID || ledger.Result.Destination.RoomID != ledger.Request.DestinationRoomID {
				return ErrInvalidState
			}
		default:
			return ErrInvalidState
		}
	}
	return nil
}

func strictLegacyJSON(raw string, target any) error {
	if !json.Valid([]byte(raw)) {
		return ErrInvalidState
	}
	d := json.NewDecoder(bytes.NewBufferString(raw))
	d.DisallowUnknownFields()
	return d.Decode(target)
}

func legacyInteger(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, ErrInvalidState
	}
}

// One bounded read-only Redis execution captures all pages, types, exact raw
// values and absolute deadlines under one atomic observation. LIMIT is never a
// qualified partial capture. Unsupported types retain DUMP diagnostics.
const legacyAtomicCapture = `
local prefix,maxkeys,maxbytes,maxpages,maxms=ARGV[1],tonumber(ARGV[2]),tonumber(ARGV[3]),tonumber(ARGV[4]),tonumber(ARGV[5])
local clock=redis.call('TIME');local now=tonumber(clock[1])*1000+math.floor(tonumber(clock[2])/1000)
local cursor='0';local keys={};local seen={};local pages=0;local bytes=0;local entries={};local diagnostic=''
repeat
 pages=pages+1;if pages>maxpages then return {now,'CAPTURE_PAGE_LIMIT',entries} end
 local page=redis.call('SCAN',cursor,'MATCH',prefix..'*','COUNT',128);cursor=page[1]
 for _,key in ipairs(page[2]) do if not seen[key] then seen[key]=true;table.insert(keys,key);if #keys>maxkeys then return {now,'CAPTURE_KEY_LIMIT',entries} end end end
until cursor=='0'
table.sort(keys)
for _,key in ipairs(keys) do
 local current=redis.call('TIME');if tonumber(current[1])*1000+math.floor(tonumber(current[2])/1000)-now>maxms then return {now,'CAPTURE_TIME_LIMIT',entries} end
 local kind=redis.call('TYPE',key)['ok'];local value='';local members={};local dump='';local expiry='PERSISTENT';local deadline=0
 local ttl=redis.call('PTTL',key)
 if ttl==-2 then kind='none';expiry='ABSENT_AT_CUT'
 else
  if kind=='string' and redis.call('STRLEN',key)>maxbytes then return {now,'CAPTURE_BYTE_LIMIT',entries} end
  if kind=='set' and redis.call('SCARD',key)>maxkeys then return {now,'CAPTURE_MEMBER_LIMIT',entries} end
  dump=redis.call('DUMP',key)
  if not dump then return {now,'CAPTURE_CHANGED',entries} end
  if ttl>=0 then expiry='DEADLINE';deadline=redis.call('PEXPIRETIME',key) end
  if kind=='string' then value=redis.call('GET',key)
  elseif kind=='set' then members=redis.call('SMEMBERS',key);table.sort(members)
  else diagnostic='UNSUPPORTED_REDIS_TYPE' end
 end
 bytes=bytes+#key+#dump+#value;for _,member in ipairs(members) do bytes=bytes+#member end
 table.insert(entries,{key,kind,dump,expiry,deadline,value,members})
 if bytes>maxbytes then return {now,'CAPTURE_BYTE_LIMIT',entries} end
end
return {now,diagnostic,entries}
`
