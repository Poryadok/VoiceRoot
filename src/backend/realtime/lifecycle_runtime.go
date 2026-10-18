package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	realtimev1 "voice.app/voice/realtime/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/realtimeprincipal"
	"voice/backend/realtime/internal/lifecycle"
)

type nativeChatLifecycleResolver struct {
	client chatv1.ChatRealtimeLifecycleServiceClient
	issuer *principal.Issuer
}

func (r nativeChatLifecycleResolver) Resolve(ctx context.Context, chat uuid.UUID) (*chatv1.ResolveChatLifecycleResponse, error) {
	req := &chatv1.ResolveChatLifecycleRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ChatId: chat.String()}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, err
	}
	token, err := r.issuer.IssueService(principal.ServiceInput{Audience: "chat", RPC: chatv1.ChatRealtimeLifecycleService_ResolveChatLifecycle_FullMethodName, RequestID: req.OperationId, RequestHash: hash})
	if err != nil {
		return nil, err
	}
	signed := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", req.OperationId))
	response, err := r.client.ResolveChatLifecycle(signed, req)
	if err != nil {
		return nil, err
	}
	if response == nil || len(response.ProtoReflect().GetUnknown()) != 0 || response.GetProtocolVersion() != 1 || response.GetSchemaVersion() != 1 || response.GetChatId() != chat.String() {
		return nil, lifecycle.ErrDenied
	}
	if response.GetSpaceId() != "" {
		space, e := uuid.Parse(response.GetSpaceId())
		if e != nil || space == uuid.Nil || space.String() != response.GetSpaceId() {
			return nil, lifecycle.ErrDenied
		}
	}
	if response.GetGeneration() > 0 {
		op, e := uuid.Parse(response.GetDeletionOperationId())
		if e != nil || op == uuid.Nil || op.String() != response.GetDeletionOperationId() {
			return nil, lifecycle.ErrDenied
		}
	}
	if response.GetSpaceId() == "" && (response.GetGeneration() != 0 || response.GetState() != chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_LIVE) {
		return nil, lifecycle.ErrDenied
	}
	return response, nil
}

type chatLifecycleResolver interface {
	Resolve(context.Context, uuid.UUID) (*chatv1.ResolveChatLifecycleResponse, error)
}
type deliveryLifecycle struct {
	store    *lifecycle.Store
	executor *lifecycle.Executor
	resolver chatLifecycleResolver
	serving  *atomic.Bool
}

func (d *deliveryLifecycle) Run(ctx context.Context, chat uuid.UUID, fanout func(*lifecycle.Scope)) error {
	if d.serving != nil && !d.serving.Load() {
		return lifecycle.ErrDenied
	}
	// Resolver is structural authority, not a recipient grant. It runs outside
	// Realtime transactions; monotonic hydration cannot overwrite imported denies.
	response, err := d.resolver.Resolve(ctx, chat)
	if err != nil {
		return err
	}
	var space uuid.UUID
	if response.GetSpaceId() != "" {
		space, err = uuid.Parse(response.GetSpaceId())
		if err != nil {
			return lifecycle.ErrDenied
		}
	}
	state := ""
	switch response.GetState() {
	case chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_LIVE:
		state = "LIVE"
	case chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_FROZEN:
		state = "FROZEN"
	case chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_PURGE_DECIDED:
		state = "PURGE_DECIDED"
	case chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_PURGED:
		state = "PURGED"
	default:
		return lifecycle.ErrDenied
	}
	if err = d.store.Hydrate(ctx, chat, space, response.GetGeneration(), state); err != nil {
		return err
	}
	return d.executor.Run(ctx, chat, fanout)
}

type realtimeLifecycleRuntime struct {
	pool        *pgxpool.Pool
	server      *grpc.Server
	transport   *realtimeprincipal.Runtime
	chat        *grpc.ClientConn
	executor    *lifecycle.Executor
	store       *lifecycle.Store
	incarnation uuid.UUID
	stop        context.CancelFunc
	done        chan struct{}
	once        sync.Once
	serving     atomic.Bool
}

func (r *realtimeLifecycleRuntime) Ready(ctx context.Context) error {
	if !r.serving.Load() {
		return errors.New("Realtime lifecycle listener unavailable")
	}
	return r.pool.Ping(ctx)
}
func (r *realtimeLifecycleRuntime) Close(ctx context.Context) {
	r.once.Do(func() {
		r.serving.Store(false)
		if r.executor != nil {
			r.executor.TryClose()
		}
		if r.stop != nil {
			r.stop()
			<-r.done
		}
		if r.server != nil {
			r.server.GracefulStop()
		}
		if r.executor != nil {
			_ = r.store.StartClosing(ctx, r.incarnation)
			ticker := time.NewTicker(20 * time.Millisecond)
			defer ticker.Stop()
			for {
				_ = r.executor.RetrySettlements(ctx)
				if r.executor.TryClose() {
					_ = r.store.CloseIncarnation(ctx, r.incarnation)
					break
				}
				select {
				case <-ctx.Done():
					goto stopped
				case <-ticker.C:
				}
			}
		}
	stopped:
		if r.transport != nil {
			_ = r.transport.Close()
		}
		if r.chat != nil {
			_ = r.chat.Close()
		}
		if r.pool != nil {
			r.pool.Close()
		}
	})
}

func newRealtimeLifecycleRuntime(ctx context.Context, hub *wsHub) (*realtimeLifecycleRuntime, error) {
	get := func(key string) string { return strings.TrimSpace(os.Getenv(key)) }
	database := get("DATABASE_URL")
	if database == "" {
		return nil, errors.New("Realtime requires its own DATABASE_URL and lifecycle migration")
	}
	cfg, listen, configured, err := realtimeprincipal.ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	if !configured {
		return nil, errors.New("Realtime requires its protected Space lifecycle listener")
	}
	pool, err := pgxpool.New(ctx, database)
	if err != nil {
		return nil, err
	}
	r := &realtimeLifecycleRuntime{pool: pool}
	failed := func(err error) (*realtimeLifecycleRuntime, error) {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		r.Close(closeCtx)
		return nil, err
	}
	var ready bool
	if err = pool.QueryRow(ctx, `SELECT to_regclass('realtime_manifest_heads') IS NOT NULL AND to_regclass('realtime_manifest_items') IS NOT NULL AND to_regclass('realtime_admissions') IS NOT NULL AND to_regclass('realtime_lifecycle_operations') IS NOT NULL AND to_regclass('realtime_incarnations') IS NOT NULL`).Scan(&ready); err != nil || !ready {
		return failed(errors.New("Realtime lifecycle schema unavailable"))
	}
	issuer, err := loadRealtimeLifecycleIssuer(get("REALTIME_PRINCIPAL_PRIVATE_KEY_FILE"), get("REALTIME_PRINCIPAL_KEY_ID"), get("REALTIME_PRINCIPAL_JWKS_FILE"))
	if err != nil {
		return failed(err)
	}
	certificate, err := tls.LoadX509KeyPair(get("REALTIME_CHAT_LIFECYCLE_CERT_FILE"), get("REALTIME_CHAT_LIFECYCLE_KEY_FILE"))
	if err != nil {
		return failed(errors.New("Realtime Chat lifecycle client identity unavailable"))
	}
	rootsPEM, err := os.ReadFile(get("REALTIME_CHAT_LIFECYCLE_CA_FILE"))
	if err != nil {
		return failed(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(rootsPEM) {
		return failed(errors.New("Realtime Chat lifecycle CA invalid"))
	}
	addr, serverName := get("REALTIME_CHAT_LIFECYCLE_ADDR"), get("REALTIME_CHAT_LIFECYCLE_SERVER_NAME")
	if addr == "" || serverName == "" {
		return failed(errors.New("Realtime protected Chat target required"))
	}
	r.chat, err = grpc.NewClient(addr, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: serverName, Certificates: []tls.Certificate{certificate}})))
	if err != nil {
		return failed(err)
	}
	r.store = &lifecycle.Store{Pool: pool}
	r.incarnation = uuid.New()
	boot, _ := os.ReadFile("/proc/sys/kernel/random/boot_id")
	processStart, _ := os.ReadFile("/proc/self/stat")
	identity := fmt.Sprintf("boot=%s;runtime=%s;pid=%d;start=%s;incarnation=%s", strings.TrimSpace(string(boot)), get("HOSTNAME"), os.Getpid(), strings.TrimSpace(string(processStart)), r.incarnation)
	if err = r.store.Open(ctx, r.incarnation, identity); err != nil {
		return failed(err)
	}
	r.executor = lifecycle.NewExecutor(r.store, r.incarnation)
	hub.lifecycle = &deliveryLifecycle{store: r.store, executor: r.executor, resolver: nativeChatLifecycleResolver{client: chatv1.NewChatRealtimeLifecycleServiceClient(r.chat), issuer: issuer}, serving: &r.serving}
	r.transport, err = realtimeprincipal.New(context.WithoutCancel(ctx), cfg)
	if err != nil {
		return failed(err)
	}
	if err = ctx.Err(); err != nil {
		return failed(err)
	}
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return failed(err)
	}
	r.server = grpc.NewServer(r.transport.ServerOptions()...)
	realtimev1.RegisterRealtimeLifecycleServiceServer(r.server, &realtimeLifecycleGRPC{store: r.store})
	// The startup deadline bounds preparation, not the lifetime of background
	// key refresh and settlement. Main's Close owns their shutdown explicitly.
	run, cancel := context.WithCancel(context.WithoutCancel(ctx))
	r.stop = cancel
	r.done = make(chan struct{})
	go func() {
		defer close(r.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		retention := time.NewTicker(time.Hour)
		defer retention.Stop()
		for {
			select {
			case <-run.Done():
				return
			case <-ticker.C:
				call, stop := context.WithTimeout(run, 3*time.Second)
				_ = r.executor.RetrySettlements(call)
				stop()
			case <-retention.C:
				call, stop := context.WithTimeout(run, 3*time.Second)
				_, _ = r.store.CompactReceipts(call)
				stop()
			}
		}
	}()
	r.serving.Store(true)
	go func() {
		if err := r.server.Serve(listener); err != nil {
			r.serving.Store(false)
		}
	}()
	return r, nil
}

type deniedLifecycleRunner struct{}

func (deniedLifecycleRunner) Run(context.Context, uuid.UUID, func(*lifecycle.Scope)) error {
	return lifecycle.ErrDenied
}

func loadRealtimeLifecycleIssuer(path, kid, jwksFile string) (*principal.Issuer, error) {
	if path == "" || kid == "" || jwksFile == "" {
		return nil, errors.New("Realtime lifecycle signing identity and current/next JWKS required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("invalid Realtime signing key")
	}
	var key *rsa.PrivateKey
	if block.Type == "RSA PRIVATE KEY" {
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	} else {
		var parsed any
		parsed, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		if err == nil {
			key, _ = parsed.(*rsa.PrivateKey)
		}
	}
	if err != nil || key == nil || key.N.BitLen() < 2048 {
		return nil, errors.New("invalid Realtime RSA signing key")
	}
	document, err := os.ReadFile(jwksFile)
	if err != nil {
		return nil, err
	}
	keys, err := principal.ParseJWKS(document)
	if err != nil || len(keys) != 2 {
		return nil, errors.New("Realtime current/next JWKS required")
	}
	current, ok := keys[kid]
	if !ok || current.N.Cmp(key.N) != 0 || current.E != key.E {
		return nil, errors.New("Realtime signing key does not match published JWKS")
	}
	for id, next := range keys {
		if next.N.BitLen() < 2048 || id != kid && next.N.Cmp(key.N) == 0 {
			return nil, errors.New("Realtime JWKS rotation keys invalid")
		}
	}
	return principal.NewIssuer(principal.IssuerConfig{Issuer: "realtime", KeyID: kid, PrivateKey: key})
}

// Compile-time proof that the resolver request is a protobuf message; no raw
// metadata or caller header can manufacture Chat lifecycle enrollment.
var _ proto.Message = (*chatv1.ResolveChatLifecycleRequest)(nil)
