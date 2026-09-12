package roomlifecycle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"

	callsv1 "voice.app/voice/calls/v1"
)

var (
	ErrRedisMirrorUnavailable = errors.New("redis lifecycle mirror unavailable")
	ErrRedisMirrorDiverged    = errors.New("redis lifecycle mirror diverged")
	ErrRedisMirrorExpired     = errors.New("redis lifecycle replay expired")
)

type RedisMirrorDivergenceError struct {
	Class RedisDivergenceClass
}

func (err *RedisMirrorDivergenceError) Error() string {
	if err == nil {
		return ErrRedisMirrorDiverged.Error()
	}
	return fmt.Sprintf("%s: %s", ErrRedisMirrorDiverged, err.Class)
}

func (*RedisMirrorDivergenceError) Unwrap() error { return ErrRedisMirrorDiverged }

type RedisMirrorKey struct {
	Origin         string
	ActorProfileID uuid.UUID
	OperationID    uuid.UUID
}

type RedisMirrorBinding struct {
	Key         RedisMirrorKey
	Method      LifecycleMethod
	Fingerprint LifecycleDigest
}

type RedisMirrorReservation struct {
	Binding    RedisMirrorBinding
	OwnerToken LifecycleDigest
}

type RedisMirrorReceipt struct {
	Bytes       []byte
	Hash        LifecycleDigest
	ReplayUntil time.Time
}

type RedisMirrorState uint8

const (
	RedisMirrorAbsent RedisMirrorState = iota
	RedisMirrorPending
	RedisMirrorCompleted
)

type RedisMirrorObservation struct {
	State      RedisMirrorState
	Binding    RedisMirrorBinding
	OwnerToken LifecycleDigest
	Receipt    *RedisMirrorReceipt
}

func NewRedisMirrorReservation(origin string, operation LifecycleOperation) (RedisMirrorReservation, error) {
	if !validRedisMirrorOrigin(origin) || operation.ActorProfileID == uuid.Nil || operation.OperationID == uuid.Nil {
		return RedisMirrorReservation{}, ErrInvariant
	}
	if _, err := methodName(operation.Method); err != nil {
		return RedisMirrorReservation{}, ErrInvariant
	}
	return RedisMirrorReservation{
		Binding: RedisMirrorBinding{
			Key:    RedisMirrorKey{Origin: origin, ActorProfileID: operation.ActorProfileID, OperationID: operation.OperationID},
			Method: operation.Method, Fingerprint: operation.Fingerprint,
		},
		OwnerToken: operation.RedisOwnerToken,
	}, nil
}

func NewRedisMirrorReceipt(operation LifecycleOperation) (RedisMirrorReceipt, error) {
	if operation.Receipt == nil || len(operation.ReceiptBytes) == 0 || operation.ReceiptHash == nil || operation.ReplayUntil == nil {
		return RedisMirrorReceipt{}, ErrInvariant
	}
	deadline := operation.ReplayUntil.UTC()
	if deadline.IsZero() || deadline.UnixMilli() <= 0 || deadline.UnixNano()%int64(time.Millisecond) != 0 {
		return RedisMirrorReceipt{}, ErrInvariant
	}
	digest := LifecycleDigest(sha256.Sum256(operation.ReceiptBytes))
	if digest != *operation.ReceiptHash {
		return RedisMirrorReceipt{}, ErrInvariant
	}
	canonicalBytes, canonicalHash, err := EncodeLifecycleReceipt(*operation.Receipt)
	if err != nil || !bytes.Equal(canonicalBytes, operation.ReceiptBytes) || canonicalHash != digest {
		return RedisMirrorReceipt{}, ErrInvariant
	}
	receipt := RedisMirrorReceipt{Bytes: append([]byte(nil), operation.ReceiptBytes...), Hash: digest, ReplayUntil: deadline}
	if err := validateRedisMirrorReceipt(receipt); err != nil {
		return RedisMirrorReceipt{}, ErrInvariant
	}
	return receipt, nil
}

func validRedisMirrorOrigin(origin string) bool {
	return origin == "delegated_user" || origin == "system_disconnect"
}

func redisMirrorKey(key RedisMirrorKey) (string, error) {
	if !validRedisMirrorOrigin(key.Origin) || key.ActorProfileID == uuid.Nil || key.OperationID == uuid.Nil {
		return "", ErrInvariant
	}
	return fmt.Sprintf("voice:lifecycle:v2:op:%s:%s:%s", key.Origin, key.ActorProfileID, key.OperationID), nil
}

func redisMirrorDivergence(class RedisDivergenceClass) error {
	return &RedisMirrorDivergenceError{Class: class}
}

const redisMirrorLuaValidation = `
local function valid_hex(value)
  return value and string.len(value) == 64 and not string.find(value, '[^0-9a-f]')
end
local function valid_method(value)
  return value == 'join' or value == 'leave' or value == 'self_move' or value == 'moderator_move'
end
local function valid_deadline(value)
  if not value or not string.match(value, '^%d+$') or value == '0' then return false end
  if string.len(value) > 1 and string.sub(value, 1, 1) == '0' then return false end
  if string.len(value) > 19 then return false end
  if string.len(value) == 19 and value > '9223372036854775807' then return false end
  return true
end
local function redis_now_ms()
  local parts = redis.call('TIME')
  return tonumber(parts[1]) * 1000 + math.floor(tonumber(parts[2]) / 1000)
end
`

var redisMirrorInspectScript = redis.NewScript(redisMirrorLuaValidation + `
local count = redis.call('HLEN', KEYS[1])
if count == 0 then return {'absent'} end
local values = redis.call('HMGET', KEYS[1], 'schema_version', 'state', 'method', 'fingerprint', 'owner_token', 'receipt_bytes', 'receipt_hash', 'replay_until')
if not values[1] then
  if redis.call('HEXISTS', KEYS[1], 'state') == 1 and redis.call('HEXISTS', KEYS[1], 'owner') == 1 then return {'legacy_schema'} end
  return {'malformed'}
end
if values[1] ~= '2' or not values[2] or not valid_method(values[3]) or not valid_hex(values[4]) or not valid_hex(values[5]) then return {'malformed'} end
if values[2] == 'pending' then
  if count ~= 5 or values[6] or values[7] or values[8] or redis.call('PTTL', KEYS[1]) ~= -1 then return {'malformed'} end
  return {'pending', values[3], values[4], values[5]}
end
if values[2] ~= 'completed' or count ~= 8 or not values[6] or not valid_hex(values[7]) or not valid_deadline(values[8]) then return {'malformed'} end
local expires = redis.call('PEXPIRETIME', KEYS[1])
local ttl = redis.call('PTTL', KEYS[1])
local now = redis_now_ms()
if expires == -1 or ttl <= 0 then return {'malformed'} end
if tonumber(expires) ~= tonumber(values[8]) then return {'deadline_mismatch'} end
if now >= tonumber(values[8]) then return {'redis_expired'} end
return {'completed', values[3], values[4], values[5], values[6], values[7], values[8]}
`)

var redisMirrorAttachScript = redis.NewScript(redisMirrorLuaValidation + `
local count = redis.call('HLEN', KEYS[1])
if count == 0 then
  redis.call('HSET', KEYS[1], 'schema_version', '2', 'state', 'pending', 'method', ARGV[1], 'fingerprint', ARGV[2], 'owner_token', ARGV[3])
  return {'pending', ARGV[1], ARGV[2], ARGV[3]}
end
local values = redis.call('HMGET', KEYS[1], 'schema_version', 'state', 'method', 'fingerprint', 'owner_token', 'receipt_bytes', 'receipt_hash', 'replay_until')
if not values[1] then
  if redis.call('HEXISTS', KEYS[1], 'state') == 1 and redis.call('HEXISTS', KEYS[1], 'owner') == 1 then return {'legacy_schema'} end
  return {'malformed'}
end
if values[1] ~= '2' or not values[2] or not valid_method(values[3]) or not valid_hex(values[4]) or not valid_hex(values[5]) then return {'malformed'} end
if values[2] == 'pending' then
  if count ~= 5 or values[6] or values[7] or values[8] or redis.call('PTTL', KEYS[1]) ~= -1 then return {'malformed'} end
  if values[3] ~= ARGV[1] or values[4] ~= ARGV[2] then return {'binding_mismatch'} end
  if values[5] ~= ARGV[3] then return {'owner_mismatch'} end
  return {'pending', values[3], values[4], values[5]}
end
if values[2] ~= 'completed' or count ~= 8 or not values[6] or not valid_hex(values[7]) or not valid_deadline(values[8]) then return {'malformed'} end
local expires = redis.call('PEXPIRETIME', KEYS[1])
local ttl = redis.call('PTTL', KEYS[1])
local now = redis_now_ms()
if expires == -1 or ttl <= 0 then return {'malformed'} end
if tonumber(expires) ~= tonumber(values[8]) then return {'deadline_mismatch'} end
if now >= tonumber(values[8]) then return {'redis_expired'} end
if values[3] ~= ARGV[1] or values[4] ~= ARGV[2] then return {'binding_mismatch'} end
if values[5] ~= ARGV[3] then return {'owner_mismatch'} end
return {'completed', values[3], values[4], values[5], values[6], values[7], values[8]}
`)

var redisMirrorCompleteScript = redis.NewScript(redisMirrorLuaValidation + `
local count = redis.call('HLEN', KEYS[1])
if count == 0 then
  local now = redis_now_ms()
  if now >= tonumber(ARGV[6]) then return {'redis_expired'} end
  redis.call('HSET', KEYS[1], 'schema_version', '2', 'state', 'completed', 'method', ARGV[1], 'fingerprint', ARGV[2], 'owner_token', ARGV[3], 'receipt_bytes', ARGV[4], 'receipt_hash', ARGV[5], 'replay_until', ARGV[6])
  redis.call('PEXPIREAT', KEYS[1], ARGV[6])
  -- Finish any hash-table rehash before returning so a read-only replay leaves DUMP byte-identical.
  redis.call('HGETALL', KEYS[1])
  return {'completed', ARGV[1], ARGV[2], ARGV[3], ARGV[4], ARGV[5], ARGV[6]}
end
local values = redis.call('HMGET', KEYS[1], 'schema_version', 'state', 'method', 'fingerprint', 'owner_token', 'receipt_bytes', 'receipt_hash', 'replay_until')
if not values[1] then
  if redis.call('HEXISTS', KEYS[1], 'state') == 1 and redis.call('HEXISTS', KEYS[1], 'owner') == 1 then return {'legacy_schema'} end
  return {'malformed'}
end
if values[1] ~= '2' or not values[2] or not valid_method(values[3]) or not valid_hex(values[4]) or not valid_hex(values[5]) then return {'malformed'} end
if values[2] == 'pending' then
  local ttl = redis.call('PTTL', KEYS[1])
  local now = redis_now_ms()
  if count ~= 5 or values[6] or values[7] or values[8] or ttl ~= -1 then return {'malformed'} end
  if values[3] ~= ARGV[1] or values[4] ~= ARGV[2] then return {'binding_mismatch'} end
  if values[5] ~= ARGV[3] then return {'owner_mismatch'} end
  if now >= tonumber(ARGV[6]) then return {'redis_expired'} end
  redis.call('HSET', KEYS[1], 'state', 'completed', 'receipt_bytes', ARGV[4], 'receipt_hash', ARGV[5], 'replay_until', ARGV[6])
  redis.call('PEXPIREAT', KEYS[1], ARGV[6])
  return {'completed', values[3], values[4], values[5], ARGV[4], ARGV[5], ARGV[6]}
end
if values[2] ~= 'completed' or count ~= 8 or not values[6] or not valid_hex(values[7]) or not valid_deadline(values[8]) then return {'malformed'} end
local expires = redis.call('PEXPIRETIME', KEYS[1])
local ttl = redis.call('PTTL', KEYS[1])
local now = redis_now_ms()
if expires == -1 or ttl <= 0 then return {'malformed'} end
if tonumber(expires) ~= tonumber(values[8]) then return {'deadline_mismatch'} end
if now >= tonumber(values[8]) then return {'redis_expired'} end
if values[3] ~= ARGV[1] or values[4] ~= ARGV[2] then return {'binding_mismatch'} end
if values[5] ~= ARGV[3] then return {'owner_mismatch'} end
return {'completed', values[3], values[4], values[5], values[6], values[7], values[8]}
`)

func (l *RedisLedger) InspectMirror(ctx context.Context, key RedisMirrorKey) (RedisMirrorObservation, error) {
	redisKey, err := redisMirrorKey(key)
	if err != nil {
		return RedisMirrorObservation{}, err
	}
	if l == nil || l.client == nil {
		return RedisMirrorObservation{}, ErrRedisMirrorUnavailable
	}
	values, err := redisMirrorInspectScript.Run(ctx, l.client, []string{redisKey}).Slice()
	if err != nil {
		return RedisMirrorObservation{}, fmt.Errorf("%w: %v", ErrRedisMirrorUnavailable, err)
	}
	return parseRedisMirrorResult(key, values)
}

func (l *RedisLedger) AttachPending(ctx context.Context, reservation RedisMirrorReservation) (RedisMirrorObservation, error) {
	redisKey, method, fingerprint, owner, err := redisMirrorReservationArguments(reservation)
	if err != nil {
		return RedisMirrorObservation{}, err
	}
	if l == nil || l.client == nil {
		return RedisMirrorObservation{}, ErrRedisMirrorUnavailable
	}
	values, err := redisMirrorAttachScript.Run(ctx, l.client, []string{redisKey}, method, fingerprint, owner).Slice()
	if err != nil {
		return RedisMirrorObservation{}, fmt.Errorf("%w: %v", ErrRedisMirrorUnavailable, err)
	}
	observation, err := parseRedisMirrorResult(reservation.Binding.Key, values)
	if err != nil {
		return RedisMirrorObservation{}, err
	}
	if err := compareRedisMirrorReservation(observation, reservation); err != nil {
		return RedisMirrorObservation{}, err
	}
	return cloneRedisMirrorObservation(observation), nil
}

func (l *RedisLedger) CompleteMirror(ctx context.Context, reservation RedisMirrorReservation, receipt RedisMirrorReceipt) (RedisMirrorObservation, error) {
	redisKey, method, fingerprint, owner, err := redisMirrorReservationArguments(reservation)
	if err != nil {
		return RedisMirrorObservation{}, err
	}
	if err := validateRedisMirrorReceiptCommand(receipt); err != nil {
		return RedisMirrorObservation{}, err
	}
	receiptValid := validateRedisMirrorReceipt(receipt) == nil && redisMirrorReceiptMatchesReservation(receipt, reservation)
	if l == nil || l.client == nil {
		return RedisMirrorObservation{}, ErrRedisMirrorUnavailable
	}
	deadlineArgument := strconv.FormatInt(receipt.ReplayUntil.UnixMilli(), 10)
	if !receiptValid {
		// The existing-state branch still returns immutable evidence so a mutated
		// typed value is classified without allowing an absent/pending write.
		deadlineArgument = "0"
	}
	values, err := redisMirrorCompleteScript.Run(ctx, l.client, []string{redisKey}, method, fingerprint, owner,
		string(receipt.Bytes), hex.EncodeToString(receipt.Hash[:]), deadlineArgument).Slice()
	if err != nil {
		return RedisMirrorObservation{}, fmt.Errorf("%w: %v", ErrRedisMirrorUnavailable, err)
	}
	observation, err := parseRedisMirrorResult(reservation.Binding.Key, values)
	if err != nil {
		if errors.Is(err, ErrRedisMirrorExpired) {
			if !receiptValid {
				return RedisMirrorObservation{}, redisMirrorDivergence(RedisDivergenceReceiptMismatch)
			}
			return RedisMirrorObservation{}, fmt.Errorf("%w: redis clock reached replay deadline", ErrRedisMirrorUnavailable)
		}
		return RedisMirrorObservation{}, err
	}
	if err := compareRedisMirrorReservation(observation, reservation); err != nil {
		return RedisMirrorObservation{}, err
	}
	if observation.State != RedisMirrorCompleted || observation.Receipt == nil {
		return RedisMirrorObservation{}, redisMirrorDivergence(RedisDivergenceStateOrderMismatch)
	}
	if !bytes.Equal(observation.Receipt.Bytes, receipt.Bytes) || observation.Receipt.Hash != receipt.Hash {
		return RedisMirrorObservation{}, redisMirrorDivergence(RedisDivergenceReceiptMismatch)
	}
	if !observation.Receipt.ReplayUntil.Equal(receipt.ReplayUntil) {
		return RedisMirrorObservation{}, redisMirrorDivergence(RedisDivergenceDeadlineMismatch)
	}
	return cloneRedisMirrorObservation(observation), nil
}

func redisMirrorReservationArguments(reservation RedisMirrorReservation) (string, string, string, string, error) {
	key, err := redisMirrorKey(reservation.Binding.Key)
	if err != nil {
		return "", "", "", "", err
	}
	method, err := methodName(reservation.Binding.Method)
	if err != nil {
		return "", "", "", "", ErrInvariant
	}
	return key, method, hex.EncodeToString(reservation.Binding.Fingerprint[:]), hex.EncodeToString(reservation.OwnerToken[:]), nil
}

func compareRedisMirrorReservation(observation RedisMirrorObservation, reservation RedisMirrorReservation) error {
	if observation.State == RedisMirrorAbsent {
		return redisMirrorDivergence(RedisDivergenceStateOrderMismatch)
	}
	if observation.Binding.Method != reservation.Binding.Method || observation.Binding.Fingerprint != reservation.Binding.Fingerprint {
		return redisMirrorDivergence(RedisDivergenceBindingMismatch)
	}
	if observation.OwnerToken != reservation.OwnerToken {
		return redisMirrorDivergence(RedisDivergenceOwnerMismatch)
	}
	return nil
}

func parseRedisMirrorResult(key RedisMirrorKey, values []any) (RedisMirrorObservation, error) {
	code, fields, ok := redisScriptResult(values)
	if !ok {
		return RedisMirrorObservation{}, ErrRedisMirrorUnavailable
	}
	switch code {
	case "absent":
		if len(fields) != 0 {
			return RedisMirrorObservation{}, ErrRedisMirrorUnavailable
		}
		return RedisMirrorObservation{State: RedisMirrorAbsent}, nil
	case "pending":
		if len(fields) != 3 {
			return RedisMirrorObservation{}, ErrRedisMirrorUnavailable
		}
		binding, owner, err := parseRedisMirrorBinding(key, fields[0], fields[1], fields[2])
		if err != nil {
			return RedisMirrorObservation{}, redisMirrorDivergence(RedisDivergenceMalformed)
		}
		return RedisMirrorObservation{State: RedisMirrorPending, Binding: binding, OwnerToken: owner}, nil
	case "completed":
		if len(fields) != 6 {
			return RedisMirrorObservation{}, ErrRedisMirrorUnavailable
		}
		binding, owner, err := parseRedisMirrorBinding(key, fields[0], fields[1], fields[2])
		if err != nil {
			return RedisMirrorObservation{}, redisMirrorDivergence(RedisDivergenceMalformed)
		}
		hash, err := parseRedisMirrorDigest(fields[4])
		if err != nil {
			return RedisMirrorObservation{}, redisMirrorDivergence(RedisDivergenceMalformed)
		}
		deadlineMillis, err := strconv.ParseInt(fields[5], 10, 64)
		if err != nil || deadlineMillis <= 0 || strconv.FormatInt(deadlineMillis, 10) != fields[5] {
			return RedisMirrorObservation{}, redisMirrorDivergence(RedisDivergenceMalformed)
		}
		bytesValue := []byte(fields[3])
		decoded, decodeErr := decodeRedisMirrorReceiptBytes(bytesValue)
		if LifecycleDigest(sha256.Sum256(bytesValue)) != hash || decodeErr != nil {
			return RedisMirrorObservation{}, redisMirrorDivergence(RedisDivergenceMalformed)
		}
		if decoded.OperationID != key.OperationID || decoded.ActorProfileID != key.ActorProfileID || decoded.Method != binding.Method {
			return RedisMirrorObservation{}, redisMirrorDivergence(RedisDivergenceReceiptMismatch)
		}
		receipt := &RedisMirrorReceipt{Bytes: append([]byte(nil), bytesValue...), Hash: hash, ReplayUntil: time.UnixMilli(deadlineMillis).UTC()}
		return RedisMirrorObservation{State: RedisMirrorCompleted, Binding: binding, OwnerToken: owner, Receipt: receipt}, nil
	case "legacy_schema":
		return RedisMirrorObservation{}, redisMirrorDivergence(RedisDivergenceLegacySchema)
	case "malformed":
		return RedisMirrorObservation{}, redisMirrorDivergence(RedisDivergenceMalformed)
	case "binding_mismatch":
		return RedisMirrorObservation{}, redisMirrorDivergence(RedisDivergenceBindingMismatch)
	case "owner_mismatch":
		return RedisMirrorObservation{}, redisMirrorDivergence(RedisDivergenceOwnerMismatch)
	case "deadline_mismatch":
		return RedisMirrorObservation{}, redisMirrorDivergence(RedisDivergenceDeadlineMismatch)
	case "redis_expired":
		return RedisMirrorObservation{}, ErrRedisMirrorExpired
	default:
		return RedisMirrorObservation{}, ErrRedisMirrorUnavailable
	}
}

func parseRedisMirrorBinding(key RedisMirrorKey, methodText, fingerprintText, ownerText string) (RedisMirrorBinding, LifecycleDigest, error) {
	method, err := parseMethod(methodText)
	if err != nil {
		return RedisMirrorBinding{}, LifecycleDigest{}, err
	}
	fingerprint, err := parseRedisMirrorDigest(fingerprintText)
	if err != nil {
		return RedisMirrorBinding{}, LifecycleDigest{}, err
	}
	owner, err := parseRedisMirrorDigest(ownerText)
	if err != nil {
		return RedisMirrorBinding{}, LifecycleDigest{}, err
	}
	return RedisMirrorBinding{Key: key, Method: method, Fingerprint: fingerprint}, owner, nil
}

func parseRedisMirrorDigest(text string) (LifecycleDigest, error) {
	if len(text) != 64 {
		return LifecycleDigest{}, ErrInvariant
	}
	raw, err := hex.DecodeString(text)
	if err != nil || hex.EncodeToString(raw) != text {
		return LifecycleDigest{}, ErrInvariant
	}
	return digestFromBytes(raw)
}

func validateRedisMirrorReceipt(receipt RedisMirrorReceipt) error {
	if len(receipt.Bytes) == 0 || receipt.ReplayUntil.IsZero() || receipt.ReplayUntil.UnixMilli() <= 0 || receipt.ReplayUntil.UnixNano()%int64(time.Millisecond) != 0 {
		return ErrInvariant
	}
	if LifecycleDigest(sha256.Sum256(receipt.Bytes)) != receipt.Hash || validateRedisMirrorReceiptBytes(receipt.Bytes) != nil {
		return ErrInvariant
	}
	return nil
}

func validateRedisMirrorReceiptCommand(receipt RedisMirrorReceipt) error {
	if len(receipt.Bytes) == 0 || receipt.ReplayUntil.IsZero() || receipt.ReplayUntil.UnixMilli() <= 0 || receipt.ReplayUntil.UnixNano()%int64(time.Millisecond) != 0 {
		return ErrInvariant
	}
	return nil
}

func redisMirrorReceiptMatchesReservation(receipt RedisMirrorReceipt, reservation RedisMirrorReservation) bool {
	decoded, err := decodeRedisMirrorReceiptBytes(receipt.Bytes)
	return err == nil && decoded.OperationID == reservation.Binding.Key.OperationID &&
		decoded.ActorProfileID == reservation.Binding.Key.ActorProfileID && decoded.Method == reservation.Binding.Method
}

func validateRedisMirrorReceiptBytes(raw []byte) error {
	_, err := decodeRedisMirrorReceiptBytes(raw)
	return err
}

func decodeRedisMirrorReceiptBytes(raw []byte) (LifecycleReceipt, error) {
	var message callsv1.VoiceRoomLifecycleReceipt
	if err := proto.Unmarshal(raw, &message); err != nil {
		return LifecycleReceipt{}, err
	}
	canonical, err := proto.MarshalOptions{Deterministic: true}.Marshal(&message)
	if err != nil || !bytes.Equal(canonical, raw) {
		return LifecycleReceipt{}, ErrInvariant
	}
	operationID, err := uuid.Parse(message.GetOperationId())
	if err != nil || operationID == uuid.Nil {
		return LifecycleReceipt{}, ErrInvariant
	}
	actorID, err := uuid.Parse(message.GetActorProfileId())
	if err != nil || actorID == uuid.Nil {
		return LifecycleReceipt{}, ErrInvariant
	}
	subjectID, err := uuid.Parse(message.GetSubjectProfileId())
	if err != nil || subjectID == uuid.Nil || message.GetSpace() == nil {
		return LifecycleReceipt{}, ErrInvariant
	}
	spaceID, err := uuid.Parse(message.GetSpace().GetId())
	if err != nil || spaceID == uuid.Nil {
		return LifecycleReceipt{}, ErrInvariant
	}
	receipt := LifecycleReceipt{OperationID: operationID, ActorProfileID: actorID, SubjectProfileID: subjectID, SpaceID: spaceID,
		Method: LifecycleMethod(message.GetMethod()), Outcome: LifecycleOutcome(message.GetOutcome())}
	if receipt.SourceVoiceRoomID, err = parseOptionalReceiptUUID(message.SourceVoiceRoomId); err != nil {
		return LifecycleReceipt{}, err
	}
	if receipt.DestinationVoiceRoomID, err = parseOptionalReceiptUUID(message.DestinationVoiceRoomId); err != nil {
		return LifecycleReceipt{}, err
	}
	if receipt.RoomID, err = parseOptionalReceiptUUID(message.RoomId); err != nil {
		return LifecycleReceipt{}, err
	}
	if receipt.MediaEpoch, err = parseOptionalReceiptUUID(message.MediaEpoch); err != nil {
		return LifecycleReceipt{}, err
	}
	if receipt.SourceRosterVersion, err = parseOptionalReceiptInt64(message.SourceRosterVersion); err != nil {
		return LifecycleReceipt{}, err
	}
	if receipt.DestinationRosterVersion, err = parseOptionalReceiptInt64(message.DestinationRosterVersion); err != nil {
		return LifecycleReceipt{}, err
	}
	if receipt.SpaceAccessEpoch, err = parseOptionalReceiptInt64(message.SpaceAccessEpoch); err != nil {
		return LifecycleReceipt{}, err
	}
	if receipt.RolePolicyEpoch, err = parseOptionalReceiptInt64(message.RolePolicyEpoch); err != nil {
		return LifecycleReceipt{}, err
	}
	if len(message.AuthorizationDigest) > 0 {
		digest, digestErr := digestFromBytes(message.AuthorizationDigest)
		if digestErr != nil {
			return LifecycleReceipt{}, digestErr
		}
		receipt.AuthorizationDigest = &digest
	}
	if err := validateReceipt(receipt); err != nil {
		return LifecycleReceipt{}, err
	}
	return receipt, nil
}

func parseOptionalReceiptUUID(value *string) (*uuid.UUID, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := uuid.Parse(*value)
	if err != nil || parsed == uuid.Nil {
		return nil, ErrInvariant
	}
	return &parsed, nil
}

func parseOptionalReceiptInt64(value *uint64) (*int64, error) {
	if value == nil {
		return nil, nil
	}
	if *value > math.MaxInt64 {
		return nil, ErrInvariant
	}
	converted := int64(*value)
	return &converted, nil
}

func cloneRedisMirrorObservation(observation RedisMirrorObservation) RedisMirrorObservation {
	if observation.Receipt != nil {
		receipt := *observation.Receipt
		receipt.Bytes = append([]byte(nil), receipt.Bytes...)
		observation.Receipt = &receipt
	}
	return observation
}
