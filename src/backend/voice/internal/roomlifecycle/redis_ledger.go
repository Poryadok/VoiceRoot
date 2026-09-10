package roomlifecycle

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var redisBeginScript = redis.NewScript(`
local function valid_hex(value, length)
  return string.len(value) == length and not string.find(value, '[^0-9a-f]')
end
local function valid_fingerprint(value)
  return string.sub(value, 1, 7) == 'sha256:' and valid_hex(string.sub(value, 8), 64)
end
local function valid_uuid(value)
  if string.len(value) ~= 36 or value == '00000000-0000-0000-0000-000000000000' then return false end
  if string.sub(value, 9, 9) ~= '-' or string.sub(value, 14, 14) ~= '-' or string.sub(value, 19, 19) ~= '-' or string.sub(value, 24, 24) ~= '-' then return false end
  local compact = string.gsub(value, '-', '')
  return valid_hex(compact, 32)
end
local exists = redis.call('EXISTS', KEYS[1])
if exists == 0 then
  redis.call('HSET', KEYS[1],
    'state', 'pending',
    'method', ARGV[1],
    'fingerprint', ARGV[2],
    'owner', ARGV[3])
  return {'new'}
end

local state = redis.call('HGET', KEYS[1], 'state')
local method = redis.call('HGET', KEYS[1], 'method')
local fingerprint = redis.call('HGET', KEYS[1], 'fingerprint')
local owner = redis.call('HGET', KEYS[1], 'owner')
if not state or not method or not fingerprint or not owner or method ~= 'JOIN' or not valid_fingerprint(fingerprint) or not valid_hex(owner, 64) then
  return {'corrupt'}
end
if method ~= ARGV[1] or fingerprint ~= ARGV[2] then
  return {'binding_conflict'}
end
if state == 'pending' then
  if redis.call('HLEN', KEYS[1]) ~= 4 or redis.call('PTTL', KEYS[1]) ~= -1 then
    return {'corrupt'}
  end
  return {'pending'}
end
if state == 'completed' then
  local outcome = redis.call('HGET', KEYS[1], 'receipt_outcome')
  local session_id = redis.call('HGET', KEYS[1], 'receipt_session_id')
  if not outcome or not session_id or outcome ~= 'JOIN_SUCCEEDED' or not valid_uuid(session_id) or redis.call('HLEN', KEYS[1]) ~= 6 or redis.call('PTTL', KEYS[1]) <= 0 then
    return {'corrupt'}
  end
  return {'completed', outcome, session_id}
end
return {'corrupt'}
`)

var redisCompleteScript = redis.NewScript(`
local function valid_hex(value, length)
  return string.len(value) == length and not string.find(value, '[^0-9a-f]')
end
local function valid_fingerprint(value)
  return string.sub(value, 1, 7) == 'sha256:' and valid_hex(string.sub(value, 8), 64)
end
local function valid_uuid(value)
  if string.len(value) ~= 36 or value == '00000000-0000-0000-0000-000000000000' then return false end
  if string.sub(value, 9, 9) ~= '-' or string.sub(value, 14, 14) ~= '-' or string.sub(value, 19, 19) ~= '-' or string.sub(value, 24, 24) ~= '-' then return false end
  local compact = string.gsub(value, '-', '')
  return valid_hex(compact, 32)
end
if redis.call('EXISTS', KEYS[1]) == 0 then
  return {'fence_mismatch'}
end
local state = redis.call('HGET', KEYS[1], 'state')
local method = redis.call('HGET', KEYS[1], 'method')
local fingerprint = redis.call('HGET', KEYS[1], 'fingerprint')
local owner = redis.call('HGET', KEYS[1], 'owner')
if not state or not method or not fingerprint or not owner or method ~= 'JOIN' or not valid_fingerprint(fingerprint) or not valid_hex(owner, 64) then
  return {'corrupt'}
end
if method ~= ARGV[1] or fingerprint ~= ARGV[2] or owner ~= ARGV[3] then
  return {'fence_mismatch'}
end
if state == 'pending' then
  if redis.call('HLEN', KEYS[1]) ~= 4 or redis.call('PTTL', KEYS[1]) ~= -1 then
    return {'corrupt'}
  end
  redis.call('HSET', KEYS[1],
    'state', 'completed',
    'receipt_outcome', ARGV[4],
    'receipt_session_id', ARGV[5])
  redis.call('PEXPIRE', KEYS[1], ARGV[6])
  return {'completed'}
end
if state == 'completed' then
  local outcome = redis.call('HGET', KEYS[1], 'receipt_outcome')
  local session_id = redis.call('HGET', KEYS[1], 'receipt_session_id')
  if not outcome or not session_id or outcome ~= 'JOIN_SUCCEEDED' or not valid_uuid(session_id) or redis.call('HLEN', KEYS[1]) ~= 6 or redis.call('PTTL', KEYS[1]) <= 0 then
    return {'corrupt'}
  end
  if outcome == ARGV[4] and session_id == ARGV[5] then
    return {'completed'}
  end
  return {'receipt_conflict'}
end
return {'corrupt'}
`)

type RedisLedger struct {
	client redis.UniversalClient
}

func NewRedisLedger(client redis.UniversalClient) *RedisLedger {
	return &RedisLedger{client: client}
}

func (l *RedisLedger) Begin(ctx context.Context, binding Binding) (Reservation, *Receipt, error) {
	if l == nil || l.client == nil {
		return Reservation{}, nil, status.Error(codes.Unavailable, "Redis operation ledger unavailable")
	}
	if !validBinding(binding) {
		return Reservation{}, nil, status.Error(codes.FailedPrecondition, "valid operation binding required")
	}
	ownerToken, err := newOwnerToken()
	if err != nil {
		return Reservation{}, nil, status.Error(codes.Unavailable, "operation reservation unavailable")
	}
	values, err := redisBeginScript.Run(ctx, l.client, []string{redisLedgerKey(binding)}, string(binding.method), binding.fingerprint, ownerToken).Slice()
	if err != nil {
		return Reservation{}, nil, status.Error(codes.Unavailable, "Redis operation ledger unavailable")
	}
	code, fields, ok := redisScriptResult(values)
	if !ok {
		return Reservation{}, nil, status.Error(codes.Unavailable, "Redis operation ledger returned invalid state")
	}
	switch code {
	case "new":
		if len(fields) != 0 {
			break
		}
		return reservationFor(binding, ownerToken), nil, nil
	case "pending":
		if len(fields) != 0 {
			break
		}
		return Reservation{}, nil, status.Error(codes.FailedPrecondition, "operation is pending reconciliation")
	case "binding_conflict":
		if len(fields) != 0 {
			break
		}
		return Reservation{}, nil, status.Error(codes.AlreadyExists, "operation id is bound to another request")
	case "completed":
		if len(fields) != 2 {
			break
		}
		return Reservation{}, &Receipt{Outcome: OperationOutcome(fields[0]), VoiceSessionID: fields[1]}, nil
	case "corrupt":
		if len(fields) != 0 {
			break
		}
		return Reservation{}, nil, status.Error(codes.Unavailable, "Redis operation ledger state is invalid")
	}
	return Reservation{}, nil, status.Error(codes.Unavailable, "Redis operation ledger returned invalid state")
}

func (l *RedisLedger) Complete(ctx context.Context, reservation Reservation, receipt Receipt) error {
	if l == nil || l.client == nil {
		return status.Error(codes.Unavailable, "Redis operation ledger unavailable")
	}
	if !validReservation(reservation) {
		return status.Error(codes.FailedPrecondition, "valid operation reservation required")
	}
	if !validReceipt(receipt) {
		return status.Error(codes.InvalidArgument, "valid canonical operation receipt required")
	}
	values, err := redisCompleteScript.Run(ctx, l.client, []string{redisReservationKey(reservation)},
		string(reservation.method), reservation.fingerprint, reservation.ownerToken,
		string(receipt.Outcome), receipt.VoiceSessionID, receiptRetention.Milliseconds()).Slice()
	if err != nil {
		return status.Error(codes.Unavailable, "Redis operation ledger unavailable")
	}
	code, fields, ok := redisScriptResult(values)
	if !ok || len(fields) != 0 {
		return status.Error(codes.Unavailable, "Redis operation ledger returned invalid state")
	}
	switch code {
	case "completed":
		return nil
	case "fence_mismatch":
		return status.Error(codes.FailedPrecondition, "operation reservation fence mismatch")
	case "receipt_conflict":
		return status.Error(codes.AlreadyExists, "operation already completed with another result")
	case "corrupt":
		return status.Error(codes.Unavailable, "Redis operation ledger state is invalid")
	default:
		return status.Error(codes.Unavailable, "Redis operation ledger returned invalid state")
	}
}

func redisLedgerKey(binding Binding) string {
	return fmt.Sprintf("voice:lifecycle:op:%s:%s", binding.actorProfileID.String(), binding.operationID.String())
}

func redisReservationKey(reservation Reservation) string {
	return fmt.Sprintf("voice:lifecycle:op:%s:%s", reservation.actorProfileID.String(), reservation.operationID.String())
}

func redisScriptResult(values []any) (string, []string, bool) {
	if len(values) == 0 {
		return "", nil, false
	}
	result := make([]string, len(values))
	for index, value := range values {
		text, ok := value.(string)
		if !ok {
			return "", nil, false
		}
		result[index] = text
	}
	return result[0], result[1:], true
}
