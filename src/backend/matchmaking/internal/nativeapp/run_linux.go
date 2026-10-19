//go:build linux

package nativeapp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/searchprincipal"
	"voice/backend/matchmaking/internal/spaceprincipal"
	w "voice/backend/pkg/mmadmissionwire"
	voiceprom "voice/backend/pkg/promhttp"
)

// Run is the fixed native executable root. It has no compatibility selector,
// optional database or anonymous provider/broker fallback.
func Run(ctx context.Context, logger *slog.Logger) error {
	if ctx == nil || ctx.Err() != nil {
		return errUnavailable
	}
	if logger == nil {
		logger = slog.Default()
	}
	cfg, err := resolveConfig(os.Getenv, readNativeDescriptor)
	if err != nil {
		logger.Error("native startup refused", "stage", "required-local-inputs")
		return errUnavailable
	}
	bootstrap, err := openClosedBootstrap(ctx, cfg)
	if err != nil {
		logger.Error("native startup refused", "stage", "closed-bootstrap")
		return errUnavailable
	}
	defer bootstrap.Close()
	lifetime, cancel := context.WithCancel(ctx)
	defer cancel()
	runtime, err := prepareFactories(lifetime, bootstrap, logger)
	if err != nil {
		logger.Error("native startup refused", "stage", "linked-factories")
		return errUnavailable
	}
	defer runtime.Close()
	control := &workloadControl{descriptor: cfg.descriptor, peer: bootstrap.client, gate: runtime.gate, prepared: runtime.prepared, launch: bootstrap.launch,
		unresolved: func(ctx context.Context) ([]w.ID, error) { return unresolvedProviderOperations(ctx, bootstrap.pool) }}
	workload, err := startWorkloadServer(lifetime, control)
	if err != nil {
		logger.Error("native startup refused", "stage", "private-workload-control")
		return errUnavailable
	}
	defer workload.Close()
	// ARMED is startup-only: the private prepared acknowledgement is available
	// above, but no public service or worker can advance until fresh OPEN.
	openCtx, cancelOpen := context.WithTimeout(lifetime, 45*time.Second)
	err = awaitOpen(openCtx, runtime.gate, runtime.errors)
	cancelOpen()
	if err != nil {
		runtime.gate.beginClose()
		return errUnavailable
	}
	servers, err := startNativeServing(lifetime, cfg, runtime, control)
	if err != nil {
		runtime.gate.beginClose()
		return errUnavailable
	}
	defer servers.Close()
	if runtime.roster.Start() != nil {
		runtime.gate.beginClose()
		return errUnavailable
	}
	workers := startNativeWorkers(lifetime, runtime, logger)
	defer workers.Close()
	control.opened.Store(true)
	select {
	case <-ctx.Done():
	case <-runtime.errors:
		err = errUnavailable
	}
	runtime.gate.beginClose()
	servers.stopReady()
	// Explicit release remains the only evidence that an active registration
	// completed. A timeout does not advertise a drained replica to its controller.
	drain, cancelDrain := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelDrain()
	if runtime.gate.closeAndDrain(drain) != nil {
		return errUnavailable
	}
	return err
}

func awaitOpen(ctx context.Context, gate *forwardGate, errors <-chan error) error {
	if ctx == nil || gate == nil {
		return errUnavailable
	}
	hash := sha256.Sum256([]byte("voice.mm.native.startup-open.v1"))
	for {
		select {
		case <-ctx.Done():
			return errUnavailable
		case <-errors:
			return errUnavailable
		default:
		}
		release, err := gate.EnterForward(ctx, uuid.New(), hash)
		if err == nil {
			release()
			select {
			case <-ctx.Done():
				return errUnavailable
			case <-errors:
				return errUnavailable
			default:
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return errUnavailable
		case <-errors:
			return errUnavailable
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Only StartSearch lacks an inner domain-owned lifetime. Response ACCEPT and
// provider attempts keep their narrower existing explicit registrations, while
// historical replay and terminal cleanup remain possible after closure.
func nativeForwardInterceptor(gate *forwardGate) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != matchmakingv1.MatchmakingService_StartSearch_FullMethodName {
			return handler(ctx, request)
		}
		req, ok := request.(*matchmakingv1.StartSearchRequest)
		if !ok || req == nil {
			return nil, status.Error(codes.InvalidArgument, "invalid search")
		}
		subject, ok := searchprincipal.SubjectFromContext(ctx, req)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "invalid search principal")
		}
		release, err := gate.EnterForward(ctx, uuid.New(), subject.SearchRequestHash)
		if err != nil || release == nil {
			return nil, status.Error(codes.Unavailable, "search admission unavailable")
		}
		defer release()
		return handler(ctx, request)
	}
}

type nativeServers struct {
	grpc      []*grpc.Server
	listeners []net.Listener
	http      *http.Server
	done      sync.WaitGroup
	once      sync.Once
	control   *workloadControl
}

func startNativeServing(ctx context.Context, cfg nativeConfig, runtime *preparedRuntime, control *workloadControl) (_ *nativeServers, err error) {
	if ctx == nil || ctx.Err() != nil || runtime == nil || runtime.service == nil || runtime.principal == nil || control == nil {
		return nil, errUnavailable
	}
	servers := &nativeServers{control: control}
	defer func() {
		if err != nil {
			servers.Close()
		}
	}()
	protectedOptions := runtime.principal.ServerOptions()
	protectedOptions = append(protectedOptions, grpc.ChainUnaryInterceptor(nativeForwardInterceptor(runtime.gate)))
	protected := grpc.NewServer(protectedOptions...)
	ordinary := grpc.NewServer(grpc.ChainUnaryInterceptor(spaceprincipal.NativeOrdinaryUnaryInterceptor()))
	servers.grpc = []*grpc.Server{protected, ordinary}
	for _, server := range servers.grpc {
		matchmakingv1.RegisterMatchmakingServiceServer(server, runtime.service)
	}
	protectedConfig, err := nativePrincipalConfig(cfg)
	if err != nil {
		return nil, errUnavailable
	}
	ordinaryAddress := cfg.values["MATCHMAKING_GRPC_LISTEN"]
	if ordinaryAddress == "" {
		ordinaryAddress = ":9090"
	}
	for i, address := range []string{protectedConfig.ListenAddr, ordinaryAddress} {
		listener, listenErr := net.Listen("tcp", address)
		if listenErr != nil {
			return nil, errUnavailable
		}
		servers.listeners = append(servers.listeners, listener)
		server := servers.grpc[i]
		servers.done.Add(1)
		go func() {
			defer servers.done.Done()
			if serveErr := server.Serve(listener); serveErr != nil {
				runtime.fail()
			}
		}()
	}
	address := cfg.values["LISTEN_ADDR"]
	if address == "" {
		address = ":8080"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(out http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			out.Header().Set("Allow", http.MethodGet)
			out.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		out.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(out).Encode(map[string]string{"service": "matchmaking", "status": "ok"})
	})
	mux.HandleFunc("/ready", func(out http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			out.Header().Set("Allow", http.MethodGet)
			out.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !control.opened.Load() {
			out.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		probe, cancel := context.WithTimeout(req.Context(), time.Second)
		defer cancel()
		release, probeErr := runtime.gate.EnterForward(probe, uuid.New(), sha256.Sum256([]byte("voice.mm.native.readiness.v1")))
		if probeErr != nil || release == nil {
			out.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		defer release()
		if checkNativeDatabase(probe, runtime.service.Matches.Pool, cfg.descriptor, runtime.gate.stamp) != nil || runtime.queue.Ping(probe) != nil || runtime.events.CheckNativeConnection(probe) != nil || runtime.snapshot.CheckTransport(probe) != nil || runtime.provider.CheckTransports(probe) != nil {
			out.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		out.WriteHeader(http.StatusOK)
		_, _ = out.Write([]byte("ready\n"))
	})
	servers.http = &http.Server{Handler: voiceprom.MountMetricsOnHealth(mux, runtime.metrics), ReadHeaderTimeout: 3 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 8192}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, errUnavailable
	}
	servers.listeners = append(servers.listeners, listener)
	servers.done.Add(1)
	go func() {
		defer servers.done.Done()
		if serveErr := servers.http.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			runtime.fail()
		}
	}()
	return servers, nil
}

func (s *nativeServers) stopReady() {
	if s != nil && s.control != nil {
		s.control.opened.Store(false)
	}
}

func (s *nativeServers) Close() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		s.stopReady()
		stopped := make(chan struct{})
		go func() {
			for _, server := range s.grpc {
				server.GracefulStop()
			}
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			for _, server := range s.grpc {
				server.Stop()
			}
			<-stopped
		}
		if s.http != nil {
			_ = s.http.Close()
		}
		for _, listener := range s.listeners {
			_ = listener.Close()
		}
		s.done.Wait()
	})
}
