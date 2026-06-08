package grpcmw

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/correlation"
)

func TestUnaryAccessLog_RequestIDAndMethod(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	interceptor := UnaryAccessLog(logger)
	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(correlation.GRPCMetadataKey, "req-42"))
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, handler)
	require.NoError(t, err)
	require.True(t, called)

	line := strings.TrimSpace(buf.String())
	require.NotEmpty(t, line)
	var rec map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &rec))
	require.Equal(t, "grpc_call", rec["event"])
	require.Equal(t, "/test.Service/Method", rec["grpc_method"])
	require.Equal(t, "OK", rec["grpc_code"])
	require.Equal(t, "req-42", rec["request_id"])
}

func TestUnaryRecovery_PanicReturnsInternal(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	interceptor := UnaryRecovery(logger)
	handler := func(context.Context, any) (any, error) {
		panic("boom")
	}
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Panic"}, handler)
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
	require.Contains(t, buf.String(), "grpc panic")
}

