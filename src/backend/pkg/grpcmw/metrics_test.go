package grpcmw

import (
	"context"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUnaryMetrics_RecordsHandledAndHandling(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := newMetricsCollector(reg)
	interceptor := UnaryMetrics(metrics)
	handler := func(context.Context, any) (any, error) {
		return "ok", nil
	}
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/voice.messaging.v1.MessagingService/SendMessage"}, handler)
	require.NoError(t, err)

	families, err := reg.Gather()
	require.NoError(t, err)

	var handled *dto.MetricFamily
	var handling *dto.MetricFamily
	for _, f := range families {
		switch f.GetName() {
		case "grpc_server_handled_total":
			handled = f
		case "grpc_server_handling_seconds":
			handling = f
		}
	}
	require.NotNil(t, handled)
	require.NotNil(t, handling)

	labels := handled.Metric[0].Label
	labelMap := map[string]string{}
	for _, l := range labels {
		labelMap[l.GetName()] = l.GetValue()
	}
	require.Equal(t, "voice.messaging.v1.MessagingService", labelMap["grpc_service"])
	require.Equal(t, "SendMessage", labelMap["grpc_method"])
	require.Equal(t, codes.OK.String(), labelMap["grpc_code"])
	require.Equal(t, float64(1), handled.Metric[0].GetCounter().GetValue())
	require.Len(t, handling.Metric, 1)
}

func TestUnaryMetrics_ErrorCode(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := newMetricsCollector(reg)
	interceptor := UnaryMetrics(metrics)
	handler := func(context.Context, any) (any, error) {
		return nil, status.Error(codes.NotFound, "missing")
	}
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Fail"}, handler)
	require.Error(t, err)

	families, err := reg.Gather()
	require.NoError(t, err)
	for _, f := range families {
		if f.GetName() != "grpc_server_handled_total" {
			continue
		}
		for _, m := range f.Metric {
			for _, l := range m.Label {
				if l.GetName() == "grpc_code" {
					require.Equal(t, codes.NotFound.String(), l.GetValue())
				}
			}
		}
	}
}

func TestServerOptions_WithRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	opts := ServerOptions(nil, WithRegistry(reg))
	require.Len(t, opts, 1)
	// Chain is wired; full integration covered by per-interceptor tests.
	require.NotNil(t, opts[0])
}

func TestMultipleServersShareMetricsRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	first := UnaryMetricsForRegistry(reg)
	// A protected listener shares the service's metrics endpoint with the ordinary listener.
	ServerOptions(nil, WithRegistry(reg))
	second := UnaryMetricsForRegistry(reg)
	handler := func(context.Context, any) (any, error) { return nil, nil }
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Call"}
	_, err := first(context.Background(), nil, info, handler)
	require.NoError(t, err)
	_, err = second(context.Background(), nil, info, handler)
	require.NoError(t, err)
	families, err := reg.Gather()
	require.NoError(t, err)
	require.Len(t, families, 2)
	for _, family := range families {
		require.Len(t, family.Metric, 1)
		if family.GetName() == "grpc_server_handled_total" {
			require.Equal(t, float64(2), family.Metric[0].GetCounter().GetValue())
		} else {
			require.Equal(t, uint64(2), family.Metric[0].GetHistogram().GetSampleCount())
		}
	}
}

func TestSplitFullMethod(t *testing.T) {
	svc, method := splitFullMethod("/voice.chat.v1.ChatService/ListChats")
	require.Equal(t, "voice.chat.v1.ChatService", svc)
	require.Equal(t, "ListChats", method)
}

func TestServerOptions_MetricNames(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := newMetricsCollector(reg)
	interceptor := UnaryMetrics(metrics)
	handler := func(context.Context, any) (any, error) { return nil, nil }
	_, _ = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Call"}, handler)

	families, err := reg.Gather()
	require.NoError(t, err)
	names := make([]string, 0, len(families))
	for _, f := range families {
		names = append(names, f.GetName())
	}
	require.Contains(t, strings.Join(names, ","), "grpc_server_handled_total")
	require.Contains(t, strings.Join(names, ","), "grpc_server_handling_seconds")
}
