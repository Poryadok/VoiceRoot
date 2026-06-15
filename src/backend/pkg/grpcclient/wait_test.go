package grpcclient_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"voice/backend/pkg/grpcclient"
)

func TestDialTimeoutFromEnv_default(t *testing.T) {
	t.Setenv("GRPC_DIAL_TIMEOUT", "")
	require.Equal(t, 15*time.Second, grpcclient.DialTimeoutFromEnv())
}

func TestDialTimeoutFromEnv_custom(t *testing.T) {
	t.Setenv("GRPC_DIAL_TIMEOUT", "20s")
	require.Equal(t, 20*time.Second, grpcclient.DialTimeoutFromEnv())
}
