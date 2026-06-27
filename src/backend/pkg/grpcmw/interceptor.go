package grpcmw

import (
	"context"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/correlation"
)

// ServerOption configures grpcmw.ServerOptions.
type ServerOption func(*serverConfig)

type serverConfig struct {
	registry *prometheus.Registry
}

// WithRegistry registers grpc_server_* metrics on reg instead of the default registry.
func WithRegistry(reg *prometheus.Registry) ServerOption {
	return func(c *serverConfig) {
		c.registry = reg
	}
}

// ServerOptions returns grpc.ServerOption with recovery, Prometheus metrics, and access logging.
// Interceptor order: Recovery → Metrics → AccessLog (metrics wrap full handler duration).
func ServerOptions(logger *slog.Logger, opts ...ServerOption) []grpc.ServerOption {
	cfg := serverConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	var reg = prometheus.DefaultRegisterer
	if cfg.registry != nil {
		reg = cfg.registry
	}
	metrics := newMetricsCollector(reg)
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			UnaryRecovery(logger),
			UnaryMetrics(metrics),
			UnaryAccessLog(logger),
		),
	}
}

// UnaryAccessLog logs one structured line per unary gRPC call.
func UnaryAccessLog(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		if logger == nil {
			return resp, err
		}
		code := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code()
			} else {
				code = codes.Unknown
			}
		}
		attrs := []slog.Attr{
			slog.String("event", "grpc_call"),
			slog.String("grpc_method", info.FullMethod),
			slog.String("grpc_code", code.String()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("request_id", correlation.FromGRPC(ctx)),
		}
		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
		}
		level := slog.LevelInfo
		if err != nil && code != codes.OK {
			level = slog.LevelWarn
		}
		logger.LogAttrs(ctx, level, "grpc request", attrs...)
		return resp, err
	}
}

// UnaryRecovery logs panics and returns Internal error to the client.
func UnaryRecovery(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				if logger != nil {
					logger.LogAttrs(ctx, slog.LevelError, "grpc panic",
						slog.String("event", "grpc_call"),
						slog.String("grpc_method", info.FullMethod),
						slog.String("request_id", correlation.FromGRPC(ctx)),
						slog.Any("panic", r),
					)
				}
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}
