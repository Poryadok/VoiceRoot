//go:build linux

package nativeapp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"voice/backend/matchmaking/internal/grpcsvc"
	"voice/backend/matchmaking/internal/matcher"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/partysearch"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/rosterruntime"
	"voice/backend/matchmaking/internal/runtimeconfig"
	"voice/backend/matchmaking/internal/spacelifecycle"
	"voice/backend/matchmaking/internal/spaceprincipal"
	"voice/backend/matchmaking/internal/squad"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/matchmaking/internal/timeout"
	w "voice/backend/pkg/mmadmissionwire"
)

// preparedRuntime owns the real linked factories. Construction never starts a
// domain worker or registers a serving RPC. The sole early listener publishes
// public JWKS keys, resolving the documented upstream startup dependency.
type preparedRuntime struct {
	service   *grpcsvc.MatchmakingGRPC
	principal *spaceprincipal.Runtime
	provider  *squad.ProtectedProviderRuntime
	snapshot  *partysource.GRPCClient
	queue     *queue.RedisQueue
	events    *mmevents.JetStreamPublisher
	roster    *rosterruntime.Runtime
	projector *partysearch.Projector
	matcher   *matcher.Worker
	sweeper   *timeout.Sweeper
	metrics   *prometheus.Registry
	prepared  w.Hash
	gate      *forwardGate
	errors    chan error
	closeOnce sync.Once
	closers   []func()
}

func (r *preparedRuntime) Close() {
	if r == nil {
		return
	}
	r.closeOnce.Do(func() {
		for i := len(r.closers) - 1; i >= 0; i-- {
			r.closers[i]()
		}
	})
}

// Close admission synchronously with the observed failure, before waking the
// root loop. A queued error alone leaves a window for another forward caller.
func (r *preparedRuntime) fail() {
	if r.gate != nil {
		r.gate.beginClose()
	}
	select {
	case r.errors <- errUnavailable:
	default:
	}
}

func prepareFactories(ctx context.Context, bootstrap *closedBootstrap, logger *slog.Logger) (_ *preparedRuntime, err error) {
	if ctx == nil || ctx.Err() != nil || bootstrap == nil || bootstrap.pool == nil || checkCodeContracts(bootstrap.config) != nil {
		return nil, errUnavailable
	}
	if logger == nil {
		logger = slog.Default()
	}
	cfg := bootstrap.config
	startup, cancelStartup := context.WithTimeout(ctx, 45*time.Second)
	defer cancelStartup()
	getenv := func(key string) string { return cfg.values[key] }
	r := &preparedRuntime{metrics: prometheus.NewRegistry(), errors: make(chan error, 8)}
	stage := "timeout-schema"
	defer func() {
		if err != nil {
			logger.Error("native factory preparation refused", "stage", stage)
			r.Close()
		}
	}()
	columns, err := bootstrap.pool.Query(startup, `SELECT event_id,party_id,search_session_id,profile_id,terminal_generation,occurred_at,payload,delivered_at,lease_token,lease_until,next_attempt_at FROM matchmaking_native_timeout_outbox LIMIT 0`)
	if err != nil {
		return nil, errUnavailable
	}
	columns.Close()
	binding := localBinding(cfg.descriptor.Binding)
	matches := &store.MatchStore{Pool: bootstrap.pool, NativeAdmission: &binding}
	parties := &store.PartyStore{Pool: bootstrap.pool, NativeAdmission: &binding}
	sessions := &store.SessionStore{Pool: bootstrap.pool}
	games := &store.GameStore{Pool: bootstrap.pool}
	bans := &store.BanStore{Pool: bootstrap.pool}
	r.gate = newForwardGate(cfg.descriptor.Binding, bootstrap.stamp, readbackObserver{bootstrap.client})
	var closeProvider func() error
	stage = "protected-provider"
	r.provider, closeProvider, err = squad.LoadRequiredProtectedProviderRuntime(matches, getenv)
	if err != nil {
		return nil, errUnavailable
	}
	r.closers = append(r.closers, func() { _ = closeProvider() })
	var configured bool
	var closeSnapshot func() error
	stage = "protected-snapshot"
	r.snapshot, closeSnapshot, configured, err = squad.LoadProtectedPartySnapshotReader(getenv)
	if err != nil || !configured || r.snapshot == nil {
		return nil, errUnavailable
	}
	r.closers = append(r.closers, func() { _ = closeSnapshot() })
	stage = "local-principal-profiles"
	jwks, err := squad.LoadPrincipalJWKSListener(getenv)
	if err != nil || !jwks.Enabled {
		return nil, errUnavailable
	}
	principalConfig, err := nativePrincipalConfig(cfg)
	if err != nil {
		return nil, errUnavailable
	}
	options, err := nativeBrokerOptions(cfg)
	if err != nil {
		return nil, errUnavailable
	}
	stage = "configuration-contracts"
	r.prepared, err = checkConfigurationContracts(cfg)
	if err != nil {
		return nil, errUnavailable
	}
	rdb := redis.NewClient(&redis.Options{Addr: cfg.values["MATCHMAKING_REDIS_ADDR"], Password: cfg.values["MATCHMAKING_REDIS_PASSWORD"], DialTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second})
	r.closers = append(r.closers, func() { _ = rdb.Close() })
	r.queue = &queue.RedisQueue{Client: rdb, Prefix: "mm"}
	stage = "redis"
	if r.queue.Ping(startup) != nil {
		return nil, errUnavailable
	}
	stage = "jetstream-publisher"
	r.events, err = mmevents.NewJetStreamPublisher(cfg.values["NATS_URL"], options...)
	if err != nil {
		return nil, errUnavailable
	}
	r.events.Logger = logger
	r.closers = append(r.closers, func() { _ = r.events.Close() })
	if r.events.CheckNativeConnection(startup) != nil {
		return nil, errUnavailable
	}
	stage = "roster-prepare"
	r.roster, err = rosterruntime.PrepareNative(ctx, bootstrap.pool, cfg.values["NATS_URL"], r.events, r.metrics, logger, options...)
	if err != nil {
		return nil, errUnavailable
	}
	r.closers = append(r.closers, r.roster.Close)
	if r.roster.SetContractMismatchHandler(r.gate.beginClose) != nil {
		return nil, errUnavailable
	}
	// Public-key-only bootstrap is intentionally before remote JWKS/transport
	// readiness, but after local binding and every local profile is constructed.
	stage = "jwks-only-bootstrap"
	listener, err := net.Listen("tcp", jwks.ListenAddr)
	if err != nil {
		return nil, errUnavailable
	}
	server := &http.Server{Handler: jwks.Handler, TLSConfig: jwks.TLSConfig, ReadHeaderTimeout: 3 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 8192}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if serveErr := server.ServeTLS(listener, "", ""); serveErr != nil && serveErr != http.ErrServerClosed {
			r.fail()
		}
	}()
	r.closers = append(r.closers, func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
		_ = server.Close()
		<-done
	})
	var closePrincipal func()
	stage = "protected-principal"
	r.principal, closePrincipal, err = preparePrincipal(ctx, startup, principalConfig)
	if err != nil {
		return nil, errUnavailable
	}
	r.closers = append(r.closers, closePrincipal)
	stage = "upstream-transport-readiness"
	if r.snapshot.CheckTransport(startup) != nil || r.provider.CheckTransports(startup) != nil {
		return nil, errUnavailable
	}
	validator := &partysource.Revalidator{Snapshots: r.snapshot}
	r.provider.Worker.PartyValidator = validator
	r.provider.Worker.ForwardLifetime = r.gate
	r.service = grpcsvc.NewNativeRouteHandler(grpcsvc.MatchmakingGRPC{
		Games: games, Sessions: sessions, Matches: matches, Bans: bans,
		Parties: parties, Queue: r.queue, Events: r.events, Logger: logger,
		Squad:          r.provider.Worker,
		SpaceLifecycle: spacelifecycle.New(bootstrap.pool), PartySnapshots: r.snapshot,
		ResponseValidator: validator, ResponseLifetime: r.gate,
	})
	r.projector = &partysearch.Projector{Store: parties, Queue: r.queue, Validator: validator, ForwardLifetime: r.gate, Resetter: matches}
	r.matcher = &matcher.Worker{Queue: r.queue, Sessions: sessions, Matches: matches, Games: games, Bans: bans, Events: matcher.MMEventsAdapter{Pub: r.events}, Logger: logger, ProposalValidator: validator}
	r.sweeper = &timeout.Sweeper{Sessions: sessions, Matches: matches, Queue: r.queue, Events: r.events, Logger: logger, Timing: runtimeconfig.LoadSearchTiming()}
	return r, nil
}

// The finite startup budget must not become the runtime refresh lifetime. A
// successful constructor retains its process child until explicit Close; failed
// preparation cancels and joins construction before releasing its dependencies.
func preparePrincipal(parent, startup context.Context, cfg spaceprincipal.Config) (*spaceprincipal.Runtime, func(), error) {
	if parent == nil || startup == nil || parent.Err() != nil || startup.Err() != nil {
		return nil, nil, errUnavailable
	}
	lifetime, cancel := context.WithCancel(parent)
	type result struct {
		runtime *spaceprincipal.Runtime
		err     error
	}
	ready := make(chan result, 1)
	go func() { runtime, err := spaceprincipal.New(lifetime, cfg); ready <- result{runtime, err} }()
	select {
	case initialized := <-ready:
		if initialized.err != nil || initialized.runtime == nil {
			cancel()
			return nil, nil, errUnavailable
		}
		return initialized.runtime, func() { cancel(); _ = initialized.runtime.Close() }, nil
	case <-startup.Done():
		cancel()
		initialized := <-ready
		if initialized.runtime != nil {
			_ = initialized.runtime.Close()
		}
		return nil, nil, errUnavailable
	}
}

func nativePrincipalConfig(cfg nativeConfig) (spaceprincipal.Config, error) {
	var urls map[string]string
	if json.Unmarshal([]byte(cfg.values["S2S_JWKS_URLS_JSON"]), &urls) != nil || urls["gateway"] == "" || urls["space"] == "" {
		return spaceprincipal.Config{}, errUnavailable
	}
	result := spaceprincipal.Config{
		ListenAddr:  cfg.values["MATCHMAKING_SPACE_PRINCIPAL_GRPC_LISTEN"],
		TLSCertFile: cfg.values["MATCHMAKING_SPACE_PRINCIPAL_TLS_CERT_FILE"], TLSKeyFile: cfg.values["MATCHMAKING_SPACE_PRINCIPAL_TLS_KEY_FILE"], ClientCAFile: cfg.values["MATCHMAKING_SPACE_PRINCIPAL_CLIENT_CA_FILE"],
		JWKSCAFile: cfg.values["S2S_JWKS_CA_FILE"], JWKSURL: urls["space"], GatewayJWKSURL: urls["gateway"],
		ReplayAddr: cfg.values["MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_ADDR"], ReplayPassword: cfg.values["MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_PASSWORD"],
		RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second,
	}
	for key, target := range map[string]*time.Duration{"S2S_JWKS_REFRESH_AFTER": &result.RefreshAfter, "S2S_JWKS_HARD_EXPIRY": &result.HardExpiry, "S2S_UNKNOWN_KID_COOLDOWN": &result.UnknownKIDCooldown} {
		if raw := cfg.values[key]; raw != "" {
			duration, err := time.ParseDuration(raw)
			if err != nil || duration <= 0 {
				return spaceprincipal.Config{}, errUnavailable
			}
			*target = duration
		}
	}
	return result, nil
}
