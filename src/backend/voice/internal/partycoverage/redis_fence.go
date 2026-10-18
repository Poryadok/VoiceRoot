package partycoverage

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strings"
)

// RetireRedisUser is a resource-side fence, not a scan or TTL attestation.
func RetireRedisUser(ctx context.Context, admin *redis.Client, user string) error {
	if admin == nil || user == "" || user == "default" || len(user) > 128 || strings.IndexFunc(user, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-')
	}) >= 0 {
		return ErrUnavailable
	}
	me, err := admin.Do(ctx, "ACL", "WHOAMI").Text()
	if err != nil || me == user {
		return ErrUnavailable
	}
	// An established old client with ACL/CONFIG authority could undo its own
	// retirement before CLIENT KILL. Shared administrator credentials are unsafe.
	for _, command := range [][]interface{}{{"ACL", "SETUSER", user, "on"}, {"CONFIG", "SET", "aclfile", "/not-executed"}, {"MODULE", "LOAD", "/not-executed"}} {
		args := []interface{}{"ACL", "DRYRUN", user}
		args = append(args, command...)
		result, err := admin.Do(ctx, args...).Text()
		if err != nil || result == "OK" {
			return ErrUnavailable
		}
	}
	if _, err = admin.Do(ctx, "ACL", "SETUSER", user, "off").Result(); err != nil {
		return err
	}
	// Persistence must succeed before certification; otherwise a Redis restart
	// could restore access even though this process observed an in-memory fence.
	if err = admin.Do(ctx, "ACL", "SAVE").Err(); err != nil {
		return fmt.Errorf("persist predecessor Redis retirement: %w", err)
	}
	if err = admin.Do(ctx, "CLIENT", "KILL", "USER", user).Err(); err != nil {
		return err
	}
	values, err := admin.Do(ctx, "ACL", "GETUSER", user).Result()
	if err != nil {
		return err
	}
	off := false
	var flags interface{}
	switch v := values.(type) {
	case map[interface{}]interface{}:
		flags = v["flags"]
	case map[string]interface{}:
		flags = v["flags"]
	case []interface{}:
		for i := 0; i+1 < len(v); i += 2 {
			if v[i] == "flags" {
				flags = v[i+1]
			}
		}
	default:
		return ErrUnavailable
	}
	items, ok := flags.([]interface{})
	if !ok {
		return ErrUnavailable
	}
	for _, f := range items {
		if f == "off" {
			off = true
		}
	}
	if !off {
		return ErrUnavailable
	}
	clients, err := admin.ClientList(ctx).Result()
	if err != nil {
		return err
	}
	for _, field := range strings.Fields(clients) {
		if field == "user="+user {
			return ErrUnavailable
		}
	}
	return nil
}
