package main

import (
	"bufio"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

func TestRedisTokenBlacklist_IsRevoked(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	bl := newRedisTokenBlacklist(mr.Addr(), "", "jwt:blacklist:")

	ok, err := bl.IsRevoked(context.Background(), "")
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = bl.IsRevoked(context.Background(), "missing-jti")
	require.NoError(t, err)
	require.False(t, ok)

	require.NoError(t, mr.Set("jwt:blacklist:revoked-jti", "1"))
	ok, err = bl.IsRevoked(context.Background(), "revoked-jti")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestRedisSlidingWindowLimiter_Allow(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	now := time.Unix(1700, 0)
	limiter := newRedisSlidingWindowLimiter(mr.Addr(), "", map[string]rateLimitRule{
		"MessagesSend": {Limit: 3, Window: 5 * time.Second},
	})
	limiter.now = func() time.Time { return now }

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		ok, err := limiter.Allow(ctx, "203.0.113.1", "MessagesSend")
		require.NoError(t, err)
		require.True(t, ok, "request %d", i+1)
	}
	ok, err := limiter.Allow(ctx, "203.0.113.1", "MessagesSend")
	require.NoError(t, err)
	require.False(t, ok)

	limiter.now = func() time.Time { return now.Add(6 * time.Second) }
	ok, err = limiter.Allow(ctx, "203.0.113.1", "MessagesSend")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestReadRedisInteger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		reply   string
		want    int64
		wantErr bool
	}{
		{"integer", ":42\r\n", 42, false},
		{"ok", "+OK\r\n", 1, false},
		{"error", "-ERR unknown\r\n", 0, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reader := bufio.NewReader(strings.NewReader(tc.reply))
			got, err := readRedisInteger(reader)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
