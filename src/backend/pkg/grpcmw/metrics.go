package grpcmw

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DefaultHistogramBuckets match Gateway HTTP latency buckets (seconds).
var DefaultHistogramBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type metricsCollector struct {
	handled   *prometheus.CounterVec
	handling  *prometheus.HistogramVec
}

func newMetricsCollector(reg prometheus.Registerer) *metricsCollector {
	c := &metricsCollector{
		handled: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "grpc_server_handled_total",
			Help: "Total number of RPCs completed on the server, regardless of success or failure.",
		}, []string{"grpc_service", "grpc_method", "grpc_code"}),
		handling: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "grpc_server_handling_seconds",
			Help:    "Histogram of response latency (seconds) of gRPC that had been application-level handled by the server.",
			Buckets: DefaultHistogramBuckets,
		}, []string{"grpc_service", "grpc_method"}),
	}
	reg.MustRegister(c.handled, c.handling)
	return c
}

func splitFullMethod(fullMethod string) (service, method string) {
	fullMethod = strings.TrimPrefix(fullMethod, "/")
	if i := strings.LastIndex(fullMethod, "/"); i >= 0 {
		return fullMethod[:i], fullMethod[i+1:]
	}
	return "unknown", fullMethod
}

// UnaryMetricsForRegistry registers grpc_server_* on reg and returns the interceptor.
// Use when building a custom grpc.ChainUnaryInterceptor (e.g. bot service rate limits).
func UnaryMetricsForRegistry(reg prometheus.Registerer) grpc.UnaryServerInterceptor {
	return UnaryMetrics(newMetricsCollector(reg))
}

// UnaryMetrics records grpc_server_* Prometheus metrics per unary RPC.
func UnaryMetrics(collector *metricsCollector) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		if collector == nil {
			return resp, err
		}
		svc, method := splitFullMethod(info.FullMethod)
		code := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code()
			} else {
				code = codes.Unknown
			}
		}
		collector.handled.WithLabelValues(svc, method, code.String()).Inc()
		collector.handling.WithLabelValues(svc, method).Observe(time.Since(start).Seconds())
		return resp, err
	}
}
