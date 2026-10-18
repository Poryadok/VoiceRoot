package queue

import (
	"bytes"
	"context"
	"fmt"
	"github.com/google/uuid"
	"sort"
	"strconv"
	"time"
)

// RecoverParty projects one complete persisted party without partial locks or
// queue membership. It is a projection, never the authority for search status.
func (q *RedisQueue) RecoverParty(ctx context.Context, sessions []SearchRecovery) error {
	if q == nil || q.Client == nil || len(sessions) == 0 || len(sessions) > 256 {
		return ErrQueueUnavailable
	}
	sessions = append([]SearchRecovery(nil), sessions...)
	sort.Slice(sessions, func(i, j int) bool { return bytes.Compare(sessions[i].ProfileID[:], sessions[j].ProfileID[:]) < 0 })
	first := sessions[0]
	if first.GameID == uuid.Nil || first.Mode == "" || first.Region == "" || (first.SpaceID != nil && *first.SpaceID == uuid.Nil) {
		return ErrQueueUnavailable
	}
	keys := []string{q.scopedQueueKey(first.SpaceID, first.GameID, first.Mode, first.Region), q.generationKey()}
	args := []any{strconv.FormatInt(int64(defaultLockTTL/time.Second), 10)}
	seen := map[uuid.UUID]bool{}
	for i, s := range sessions {
		if s.SessionID == uuid.Nil || s.ProfileID == uuid.Nil || s.Generation < 0 || s.CreatedAt.IsZero() || seen[s.SessionID] || (i > 0 && s.ProfileID == sessions[i-1].ProfileID) || s.GameID != first.GameID || s.Mode != first.Mode || s.Region != first.Region || !sameSpace(s.SpaceID, first.SpaceID) {
			return ErrQueueUnavailable
		}
		seen[s.SessionID] = true
		keys = append(keys, q.lockKey(s.ProfileID))
		args = append(args, s.SessionID.String(), strconv.FormatInt(s.Generation, 10), strconv.FormatFloat(float64(s.CreatedAt.UTC().UnixNano()), 'f', -1, 64))
	}
	const script = generationLua + `
if not projection_types_ok() then return -2 end
if not redis.acl_check_cmd then return -2 end
for i=3,#KEYS do
 local base=2+(i-3)*3
 local session=ARGV[base]
 if not can_advance(session,ARGV[base+1],true) then return 0 end
 local owner=redis.call('GET',KEYS[i])
 if owner and owner~=session then return -1 end
 if not tonumber(ARGV[base+2]) then return -2 end
 if not redis.acl_check_cmd('SET',KEYS[i],session,'EX',ARGV[1]) or
    not redis.acl_check_cmd('HSET',KEYS[2],session,ARGV[base+1]) or
    not redis.acl_check_cmd('ZADD',KEYS[1],ARGV[base+2],session) then return -2 end
end
for i=3,#KEYS do
 local base=2+(i-3)*3
 redis.call('SET',KEYS[i],ARGV[base],'EX',ARGV[1])
 redis.call('HSET',KEYS[2],ARGV[base],ARGV[base+1])
 redis.call('ZADD',KEYS[1],ARGV[base+2],ARGV[base])
end
return 1
`
	n, err := q.Client.Eval(ctx, script, keys, args...).Int()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	switch n {
	case 1:
		return nil
	case 0:
		return ErrQueueGeneration
	case -1:
		return ErrLockHeld
	default:
		return ErrQueueUnavailable
	}
}

func sameSpace(a, b *uuid.UUID) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
