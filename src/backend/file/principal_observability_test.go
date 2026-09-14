package main

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/principalgrpc"
)

func TestProtectedDenialMetricsDoNotLogUnverifiedHeaders(t *testing.T) {
	var logs bytes.Buffer
	registry := prometheus.NewRegistry()
	_, observation := fileObservability(slog.New(slog.NewJSONHandler(&logs, nil)), registry)
	server := grpc.NewServer(append(observation, grpc.ChainUnaryInterceptor(principalgrpc.StrictUnaryInterceptor(nil)))...)
	filev1.RegisterFileServiceServer(server, &filev1.UnimplementedFileServiceServer{})
	listener := bufconn.Listen(1 << 20)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	conn, err := grpc.NewClient("passthrough:///protected-observation", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer private-token", "x-request-id", "private-profile-id"))
	_, err = filev1.NewFileServiceClient(conn).ValidateStoryMedia(ctx, &filev1.ValidateStoryMediaRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.NotContains(t, logs.String(), "private-token")
	require.NotContains(t, logs.String(), "private-profile-id")
	metrics, err := registry.Gather()
	require.NoError(t, err)
	counted := false
	for _, family := range metrics {
		if family.GetName() != "grpc_server_handled_total" {
			continue
		}
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				if label.GetName() == "grpc_code" && label.GetValue() == "Unauthenticated" && metric.GetCounter().GetValue() == 1 {
					counted = true
				}
			}
		}
	}
	require.True(t, counted, "protected denial must retain bounded outcome telemetry")
}
