package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

func TestRedisSessionEpochFloor_Minimum(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	floor := newRedisSessionEpochFloor(mr.Addr(), "")
	require.Equal(t, 2*time.Second, floor.timeout, "each Redis request has a bounded timeout")

	const accountID = "account-1"
	const key = "auth:session:min_epoch:account-1"

	_, err := floor.Minimum(context.Background(), accountID)
	require.Error(t, err, "a missing floor must fail closed")

	tests := []struct {
		name    string
		value   string
		want    int64
		wantErr bool
	}{
		{name: "positive exact BIGINT", value: "9223372036854775807", want: 9223372036854775807},
		{name: "zero", value: "0", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
		{name: "text", value: "seven", wantErr: true},
		{name: "fraction", value: "7.5", wantErr: true},
		{name: "overflow", value: "9223372036854775808", wantErr: true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, mr.Set(key, tc.value))

			got, err := floor.Minimum(context.Background(), accountID)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestRedisSessionEpochFloor_MinimumFailsClosedOnRedisError(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	accepted := make(chan struct{})
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		close(accepted)
		_, _ = conn.Read(make([]byte, 1024))
		_, _ = conn.Write([]byte("-ERR Redis unavailable\r\n"))
	}()

	floor := newRedisSessionEpochFloor(listener.Addr().String(), "")
	_, err = floor.Minimum(context.Background(), "account-1")
	require.Error(t, err)
	<-accepted
}

func TestRedisSessionEpochFloor_MinimumHonorsCanceledRequestContext(t *testing.T) {
	t.Parallel()

	floor := newRedisSessionEpochFloor("127.0.0.1:1", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := floor.Minimum(ctx, "account-1")
	require.ErrorIs(t, err, context.Canceled)
}
