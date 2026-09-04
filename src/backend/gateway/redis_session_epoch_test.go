package main

import (
	"context"
	"errors"
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

	floor := newRedisSessionEpochFloor(listener.Addr().String(), "")
	done := make(chan error, 1)
	go func() {
		_, minimumErr := floor.Minimum(context.Background(), "account-1")
		done <- minimumErr
	}()
	conn := acceptRedisConnection(t, listener)
	defer func() { _ = conn.Close() }()
	_, _ = conn.Read(make([]byte, 1024))
	_, _ = conn.Write([]byte("-ERR Redis unavailable\r\n"))

	err = <-done
	require.Error(t, err)
}

func TestRedisSessionEpochFloor_MinimumReturnsTimeoutWhenRedisStalls(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	floor := newRedisSessionEpochFloor(listener.Addr().String(), "")
	floor.timeout = 25 * time.Millisecond
	done := make(chan error, 1)
	go func() {
		_, minimumErr := floor.Minimum(context.Background(), "account-1")
		done <- minimumErr
	}()
	conn := acceptRedisConnection(t, listener)
	defer func() { _ = conn.Close() }()

	err = <-done
	if !isDeadlineOrTimeout(err) {
		t.Fatalf("stalled Redis error = %v, want deadline or timeout", err)
	}
}

func TestRedisSessionEpochFloor_MinimumHonorsCanceledRequestContextWhileRedisStalls(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	floor := newRedisSessionEpochFloor(listener.Addr().String(), "")
	floor.timeout = 25 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, minimumErr := floor.Minimum(ctx, "account-1")
		done <- minimumErr
	}()
	conn := acceptRedisConnection(t, listener)
	defer func() { _ = conn.Close() }()
	cancel()

	err = <-done
	require.ErrorIs(t, err, context.Canceled)
}

func acceptRedisConnection(t *testing.T, listener net.Listener) net.Conn {
	t.Helper()

	connections := make(chan net.Conn, 1)
	acceptErrors := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptErrors <- err
			return
		}
		connections <- conn
	}()

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case conn := <-connections:
		return conn
	case err := <-acceptErrors:
		t.Fatalf("accept Redis connection: %v", err)
	case <-timer.C:
		t.Fatal("timed out waiting for Redis client connection")
	}
	return nil
}

func isDeadlineOrTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var networkErr net.Error
	return errors.As(err, &networkErr) && networkErr.Timeout()
}
