package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	grpcsvc "voice/backend/analytics/internal/grpcsvc"
	"voice/backend/analytics/internal/adapters"
	"voice/backend/analytics/internal/buffer"
	"voice/backend/analytics/internal/consumer"
	"voice/backend/analytics/internal/metrics"
	"voice/backend/analytics/internal/store"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	"voice/backend/pkg/runtimeconfig"

	analyticsv1 "voice.app/voice/analytics/v1"
)

const serviceName = "analytics"

func main() {
	logger := httpserver.NewLogger(serviceName)
	httpAddr := envOr("LISTEN_ADDR", ":8080")
	grpcAddr := envOr("ANALYTICS_GRPC_LISTEN", ":9090")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var chStore *store.CHStore
	var acc *buffer.Accumulator
	hashKey := strings.TrimSpace(os.Getenv("ANALYTICS_ID_HASH_KEY"))
	if hashKey == "" {
		hashKey = "compose-dev-analytics-hash-key"
		logger.Warn("ANALYTICS_ID_HASH_KEY not set; using dev default")
	}

	if dsn := strings.TrimSpace(os.Getenv("CLICKHOUSE_DSN")); dsn != "" {
		var err error
		chStore, err = store.Open(ctx, dsn)
		if err != nil {
			log.Fatalf("clickhouse: %v", err)
		}
		defer func() { _ = chStore.Close() }()
	} else {
		logger.Warn("CLICKHOUSE_DSN not set; ingest will buffer but not persist")
	}

	flusher := func(flushCtx context.Context, rows []store.EventRow) error {
		if chStore == nil {
			return nil
		}
		start := time.Now()
		err := chStore.InsertBatch(flushCtx, rows)
		metrics.InsertLatency.Observe(time.Since(start).Seconds())
		if err != nil {
			metrics.FlushErrors.Inc()
			return err
		}
		metrics.FlushBatches.Inc()
		return nil
	}
	maxEvents := envInt("ANALYTICS_BATCH_MAX_EVENTS", 1000)
	flushEvery := envDuration("ANALYTICS_BATCH_FLUSH_INTERVAL", 5*time.Second)
	acc = buffer.New(maxEvents, flushEvery, flusher, logger)
	acc.Start(ctx)
	defer acc.Stop()

	var grpcSrv *grpc.Server
	if chStore != nil || acc != nil {
		ingest := &grpcsvc.IngestGRPC{Buffer: acc}
		query := &grpcsvc.QueryGRPC{Store: chStore}
		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger)...)
		analyticsv1.RegisterAnalyticsIngestServiceServer(grpcSrv, ingest)
		analyticsv1.RegisterAnalyticsQueryServiceServer(grpcSrv, query)
		go func() {
			logger.Info("analytics grpc listening", slog.String("addr", grpcAddr))
			if err := grpcSrv.Serve(lis); err != nil {
				log.Fatalf("grpc serve: %v", err)
			}
		}()
	}

	if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
		runner := &consumer.Runner{
			Mapper: adapters.Mapper{HashKey: hashKey},
			Buffer: acc,
			Logger: logger,
		}
		instanceID := strings.TrimSpace(os.Getenv("HOSTNAME"))
		if instanceID == "" {
			instanceID = "local"
		}
		go func() {
			if err := runner.Start(ctx, natsURL, instanceID); err != nil && ctx.Err() == nil {
				logger.Error("analytics consumer stopped", slog.Any("error", err))
			}
		}()
	}

	mux := healthHandler(serviceName)
	if sm, ok := mux.(*http.ServeMux); ok {
		sm.Handle("/metrics", promhttp.Handler())
	}

	server := &http.Server{
		Addr:    httpAddr,
		Handler: httpserver.Wrap(mux, logger),
	}
	httpserver.ApplyHTTPServerTimeouts(server)
	errCh := make(chan error, 1)
	logger.Info("listening", slog.String("addr", httpAddr))
	go func() {
		errCh <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-stop:
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
		defer shutdownCancel()
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatal(err)
		}
	}
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
