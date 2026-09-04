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

func TestRedisSessionEpochFloorMinimumReadsOnlyCanonicalPositiveInt64(t *testing.T) {
	mr := miniredis.RunT(t)
	floor := newRedisSessionEpochFloor(mr.Addr(), "")
	require.Equal(t, 2*time.Second, floor.timeout, "each floor read is bounded")

	const accountID = "account-1"
	const key = "auth:session:min_epoch:account-1"
	_, err := floor.Minimum(context.Background(), accountID)
	require.Error(t, err, "a missing Auth floor must fail closed, never default to epoch one")

	for _, tc := range []struct {
		name    string
		value   string
		want    int64
		wantErr bool
	}{
		{name: "above JavaScript integer precision", value: "9007199254740993", want: 9007199254740993},
		{name: "maximum int64", value: "9223372036854775807", want: 9223372036854775807},
		{name: "zero", value: "0", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
		{name: "text", value: "seven", wantErr: true},
		{name: "fraction", value: "7.5", wantErr: true},
		{name: "overflow", value: "9223372036854775808", wantErr: true},
	} {
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

func TestRedisSessionEpochFloorMinimumFailsClosedOnRedisError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	floor := newRedisSessionEpochFloor(listener.Addr().String(), "")
	done := make(chan error, 1)
	go func() {
		_, minimumErr := floor.Minimum(context.Background(), "account-1")
		done <- minimumErr
	}()
	conn := acceptSessionEpochRedisConnection(t, listener)
	defer func() { _ = conn.Close() }()
	_, _ = conn.Read(make([]byte, 1024))
	_, _ = conn.Write([]byte("-ERR Redis unavailable\r\n"))

	require.Error(t, <-done)
}

func TestRedisSessionEpochFloorMinimumHonorsBoundedTimeout(t *testing.T) {
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
	conn := acceptSessionEpochRedisConnection(t, listener)
	defer func() { _ = conn.Close() }()

	watchdog := time.NewTimer(500 * time.Millisecond)
	defer watchdog.Stop()
	select {
	case err = <-done:
	case <-watchdog.C:
		t.Fatal("stalled Redis request did not honor its bounded timeout")
	}
	require.True(t, isSessionEpochDeadlineOrTimeout(err), "got %v", err)
}

func TestRedisSessionEpochFloorMinimumHonorsCanceledContext(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	floor := newRedisSessionEpochFloor(listener.Addr().String(), "")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() {
		_, minimumErr := floor.Minimum(ctx, "account-1")
		done <- minimumErr
	}()
	conn := acceptSessionEpochRedisConnection(t, listener)
	defer func() { _ = conn.Close() }()
	cancel()

	require.ErrorIs(t, <-done, context.Canceled)
}

func acceptSessionEpochRedisConnection(t *testing.T, listener net.Listener) net.Conn {
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

func isSessionEpochDeadlineOrTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var networkErr net.Error
	return errors.As(err, &networkErr) && networkErr.Timeout()
}
