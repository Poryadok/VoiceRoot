package sourcecustody

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
)

type readyListener struct {
	net.Listener
	once  sync.Once
	ready chan struct{}
}

func (l *readyListener) Accept() (net.Conn, error) {
	l.once.Do(func() { close(l.ready) })
	return l.Listener.Accept()
}

func certificatePool(path string) (*x509.CertPool, error) {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 || len(raw) > 1<<20 {
		return nil, ErrUnavailable
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(raw) {
		return nil, ErrUnavailable
	}
	return pool, nil
}

// ServeClosed starts admission closed. An optional independently authenticated
// operator can submit a signed opening decision. Neither configuration nor a
// signature proves administrator, deployment, Auth or restore custody.
func ServeClosed(ctx context.Context, cfg ServerConfig, ready func(string)) (result error) {
	if ctx == nil || ctx.Err() != nil || !cfg.valid() || ready == nil {
		return ErrUnavailable
	}
	if cfg.Operator != nil {
		operator := *cfg.Operator
		cfg.Operator = &operator
	}
	cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return ErrUnavailable
	}
	clientRoots, err := certificatePool(cfg.TLSClientCAFile)
	if err != nil {
		return ErrUnavailable
	}
	jwksRoots, err := certificatePool(cfg.UserJWKSCAFile)
	if err != nil {
		return ErrUnavailable
	}
	redisRoots, err := certificatePool(cfg.RedisTLSCAFile)
	if err != nil {
		return ErrUnavailable
	}
	// Do not inherit libpq defaults that can alter host, authentication or TLS.
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(name), "PG") {
			return ErrUnavailable
		}
	}
	poolConfig, err := pgxpool.ParseConfig(cfg.ControlDatabaseURL)
	if err != nil {
		return ErrUnavailable
	}
	u, err := url.Parse(cfg.ControlDatabaseURL)
	if err != nil {
		return ErrUnavailable
	}
	pg := poolConfig.ConnConfig
	if pg.Host != u.Hostname() || len(pg.Fallbacks) != 0 || pg.TLSConfig == nil || pg.TLSConfig.InsecureSkipVerify || pg.TLSConfig.ServerName != u.Hostname() || pg.TLSConfig.RootCAs == nil || len(pg.RuntimeParams) != 0 {
		return ErrUnavailable
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return ErrUnavailable
	}
	defer pool.Close()
	var operatorMaterials *activationOperatorMaterials
	if cfg.Operator != nil {
		operatorMaterials, err = loadActivationOperatorMaterials(cfg.Operator)
		if err != nil || recoverBeforeStart(ctx, pool, cfg.Operator, cfg.Binding) != nil {
			return ErrUnavailable
		}
	}
	controller, err := Start(ctx, pool, cfg.Binding)
	if err != nil {
		return ErrUnavailable
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if controller.Shutdown(cleanup) != nil {
			result = ErrUnavailable
		}
	}()
	transport := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: jwksRoots}, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext}
	defer transport.CloseIdleConnections()
	httpClient := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect denied") }}
	fetch := func(fetchCtx context.Context, issuer string) ([]byte, error) {
		if issuer != "user" {
			return nil, ErrUnavailable
		}
		req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, cfg.UserJWKSURL, nil)
		if err != nil {
			return nil, ErrUnavailable
		}
		response, err := httpClient.Do(req)
		if err != nil {
			return nil, ErrUnavailable
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return nil, ErrUnavailable
		}
		doc, err := io.ReadAll(io.LimitReader(response.Body, 65537))
		if err != nil || len(doc) > 65536 {
			return nil, ErrUnavailable
		}
		keys, err := principal.ParseJWKS(doc)
		if err != nil || len(keys) != 2 {
			return nil, ErrUnavailable
		}
		for _, key := range keys {
			if key.N.BitLen() < 2048 {
				return nil, ErrUnavailable
			}
		}
		return doc, nil
	}
	if _, err = fetch(ctx, "user"); err != nil {
		return ErrUnavailable
	}
	resolver := principal.NewJWKSResolver(fetch)
	replay := redis.NewClient(&redis.Options{Addr: cfg.RedisAddress, Password: cfg.RedisPassword, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: redisRoots, ServerName: cfg.RedisTLSServerName}, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, MaxRetries: -1, ContextTimeoutEnabled: true})
	defer func() { _ = replay.Close() }()
	startup, cancel := context.WithTimeout(ctx, 2*time.Second)
	err = replay.Ping(startup).Err()
	cancel()
	if err != nil {
		return ErrUnavailable
	}
	guard := func(guardCtx context.Context, issuer, id string, expires time.Time) error {
		ttl := time.Until(expires)
		if issuer != "user" || id == "" || ttl <= 0 {
			return ErrUnavailable
		}
		ok, err := replay.SetNX(guardCtx, "sourcecustody:"+issuer+":"+id, "1", ttl+time.Second).Result()
		if err != nil || !ok {
			return ErrUnavailable
		}
		return nil
	}
	handler, err := NewStreamServer(controller, StreamConfig{KeyResolver: resolver.Resolve, ReplayGuard: guard, Heartbeat: cfg.Heartbeat, RequireQualified: true, RequireAttested: !cfg.testAllowSyntheticOpen})
	if err != nil {
		return ErrUnavailable
	}
	var operatorDone <-chan error
	if cfg.Operator != nil {
		op, startErr := startActivationOperator(ctx, cfg.Operator, operatorMaterials, controller)
		if startErr != nil {
			return ErrUnavailable
		}
		operatorDone = op.done
		defer func() {
			if op.close() != nil {
				result = ErrUnavailable
			}
		}()
	}
	listen := net.Listen
	if cfg.testListen != nil {
		listen = cfg.testListen
	}
	listener, err := listen("tcp", cfg.ListenAddress)
	if err != nil {
		return ErrUnavailable
	}
	defer func() { _ = listener.Close() }()
	observed := &readyListener{Listener: listener, ready: make(chan struct{})}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientRoots})), grpc.MaxRecvMsgSize(4096), grpc.MaxConcurrentStreams(64))
	userv1.RegisterUserSourceCustodyServiceServer(server, handler)
	served := make(chan error, 1)
	serveDone := make(chan struct{})
	go func() { defer close(serveDone); served <- server.Serve(observed) }()
	// Every exit after Serve starts drains handlers before controller/pool cleanup.
	defer func() {
		stopped := make(chan struct{})
		go func() { server.GracefulStop(); close(stopped) }()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			server.Stop()
			<-stopped
			result = ErrUnavailable
		}
		<-serveDone
	}()
	select {
	case <-observed.ready:
		if ctx.Err() == nil {
			ready(listener.Addr().String())
		}
	case <-ctx.Done():
	case <-served:
		return ErrUnavailable
	case <-operatorDone:
		return ErrUnavailable
	case <-controller.failure:
		return ErrUnavailable
	}
	select {
	case <-ctx.Done():
	case <-served:
		return ErrUnavailable
	case <-operatorDone:
		return ErrUnavailable
	case <-controller.failure:
		return ErrUnavailable
	}
	return nil
}
