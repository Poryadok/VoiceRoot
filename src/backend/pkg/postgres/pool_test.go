package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/pkg/postgres"
)

func TestNewPoolConfiguredConnectionLimit(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		want        int32
	}{
		{"default", "", 10},
		{"override", "23", 23},
		{"trimmed", " 7 ", 7},
		{"zero", "0", 10},
		{"negative", "-1", 10},
		{"overflow", "2147483648", 10},
		{"malformed", "many", 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("POSTGRES_MAX_CONNS", tc.value)
			// NewPool is lazy: construction proves the configured bound without
			// opening a network connection or requiring a PostgreSQL fixture.
			pool, err := postgres.NewPool(context.Background(), "postgres://voice:fixture@127.0.0.1:1/voice?pool_max_conns=41&pool_min_conns=0")
			require.NoError(t, err)
			t.Cleanup(pool.Close)
			require.Equal(t, tc.want, pool.Config().MaxConns)
			require.Equal(t, "voice", pool.Config().ConnConfig.Database)
		})
	}
}

func TestNewPoolInvalidConnectionString(t *testing.T) {
	pool, err := postgres.NewPool(context.Background(), "postgres://host:bad-port/db")
	require.Error(t, err)
	require.Nil(t, pool)
}
