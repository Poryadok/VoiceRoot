package queue

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"voice/backend/matchmaking/internal/terminalrouting"
)

var ErrTerminalRoutingExpired = errors.New("terminal routing deadline reached")

func (q *RedisQueue) terminalGenerationKey(sessionID uuid.UUID) string {
	return fmt.Sprintf("%s:queue:terminal-generation:%s", q.prefix(), sessionID)
}

// RecoverTerminalRelease projects an already bound local lifecycle fact. This
// does not authenticate a producer or enable the A4 capability for legacy queue
// writers. The caller must bind the release to the exact verified cycle first.
// The raw generation gets its original absolute deadline, never a fresh TTL.
func (q *RedisQueue) RecoverTerminalRelease(ctx context.Context, sess SearchRecovery, life terminalrouting.Lifecycle) error {
	if q == nil || q.Client == nil {
		return ErrQueueUnavailable
	}
	if _, err := terminalrouting.New(life.Schedule); err != nil || sess.SessionID == uuid.Nil || sess.ProfileID == uuid.Nil || sess.Generation < 0 {
		return terminalrouting.ErrAuthorityInvalid
	}
	// TIME and the write run atomically on Redis. A delayed command cannot reset
	// the bound. A raw per-session key can expire independently of unrelated
	// generations in the legacy shared hash.
	const script = `
	local clock = redis.call('TIME')
	local now = tonumber(clock[1]) * 1000 + math.floor(tonumber(clock[2]) / 1000)
	local deadline = tonumber(ARGV[3])
	local function removeRaw()
	  redis.call('HDEL', KEYS[2], ARGV[1])
	  redis.call('ZREM', KEYS[1], ARGV[1])
	  if redis.call('GET', KEYS[3]) == ARGV[1] then redis.call('DEL', KEYS[3]) end
	end
	local legacy = redis.call('HGET', KEYS[2], ARGV[1])
	local bounded = redis.call('GET', KEYS[4])
	local function valid(value)
	  if not value then return true end
	  if value ~= '0' and not string.match(value, '^[1-9][0-9]*$') then return false end
	  return #value < 19 or (#value == 19 and value <= '9223372036854775807')
	end
	if not valid(legacy) or not valid(bounded) then return redis.error_reply('invalid terminal generation') end
	local function newer(value)
	  return value and (#value > #ARGV[2] or (#value == #ARGV[2] and value > ARGV[2]))
	end
	if newer(legacy) or newer(bounded) then return 0 end
	if now >= deadline or ARGV[4] == '1' then
	  removeRaw()
	  redis.call('DEL', KEYS[4])
	  return 2
	end
	removeRaw()
	redis.call('SET', KEYS[4], ARGV[2], 'PXAT', ARGV[3])
	return 1
	`
	erased := "0"
	if life.RoutingErased {
		erased = "1"
	}
	n, err := q.Client.Eval(ctx, script, []string{
		q.scopedQueueKey(sess.SpaceID, sess.GameID, sess.Mode, sess.Region), q.generationKey(),
		q.lockKey(sess.ProfileID), q.terminalGenerationKey(sess.SessionID),
	}, sess.SessionID.String(), strconv.FormatInt(sess.Generation, 10),
		strconv.FormatInt(life.Schedule.PurgeAt.UnixMilli(), 10), erased).Int()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	switch n {
	case 0:
		return ErrQueueGeneration
	case 2:
		return ErrTerminalRoutingExpired
	default:
		return nil
	}
}
