package matchsquadprincipal

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

const (
	createMethod      = "/voice.chat.v1.MatchSquadChatService/CreateMatchSquadChat"
	teardownMethod    = "/voice.chat.v1.MatchSquadChatService/TeardownMatchSquadChat"
	dependencyTimeout = 2 * time.Second
)

var errDependencyUnavailable = errors.New("MatchSquad principal dependency unavailable")

type Runtime struct {
	resolver *principal.JWKSResolver
	replay   *redis.Client
	server   credentials.TransportCredentials
	client   *http.Client
}

func New(ctx context.Context, cfg Config) (*Runtime, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	serverCert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, errors.New("load MatchSquad principal TLS server identity")
	}
	caPEM, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return nil, errors.New("read MatchSquad principal client CA")
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("MatchSquad principal client CA contains no certificates")
	}
	serverTLS := credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs})
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	jwksCA, err := os.ReadFile(cfg.JWKSCAFile)
	if err != nil || !roots.AppendCertsFromPEM(jwksCA) {
		return nil, errors.New("MatchSquad principal JWKS CA is unavailable or invalid")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	httpClient := &http.Client{Transport: transport, Timeout: dependencyTimeout, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("MatchSquad principal JWKS redirects are forbidden")
	}}
	fetch := func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != "matchmaking" {
			return nil, errors.New("untrusted service principal issuer")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.JWKSURL, nil)
		if err != nil {
			return nil, errors.New("build MatchSquad principal JWKS request")
		}
		response, err := httpClient.Do(request)
		if err != nil {
			return nil, fmt.Errorf("%w: JWKS fetch", errDependencyUnavailable)
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%w: JWKS returned status %d", errDependencyUnavailable, response.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 128*1024+1))
		if err != nil || len(body) > 128*1024 {
			return nil, fmt.Errorf("%w: JWKS response invalid", errDependencyUnavailable)
		}
		return body, nil
	}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: fetch, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, errors.New("configure MatchSquad principal JWKS resolver")
	}
	replay := redis.NewClient(&redis.Options{Addr: cfg.ReplayRedisAddr, Password: cfg.ReplayRedisPassword})
	pingCtx, cancel := context.WithTimeout(ctx, dependencyTimeout)
	defer cancel()
	if err := replay.Ping(pingCtx).Err(); err != nil {
		_ = replay.Close()
		transport.CloseIdleConnections()
		return nil, errors.New("MatchSquad principal replay Redis unavailable")
	}
	return &Runtime{resolver: resolver, replay: replay, server: serverTLS, client: httpClient}, nil
}

func (r *Runtime) Verify(ctx context.Context, token, method, requestID, requestHash string) (principal.Principal, error) {
	if r == nil || r.resolver == nil || r.replay == nil {
		return principal.Principal{}, errors.New("MatchSquad principal runtime unavailable")
	}
	if method != createMethod && method != teardownMethod {
		return principal.Principal{}, errors.New("method is not allowed on MatchSquad listener")
	}
	p, err := principal.VerifyService(ctx, token, principal.VerifyConfig{
		ExpectedIssuer: "matchmaking", ExpectedAudience: "chat", ExpectedRPC: method,
		ExpectedRequestID: requestID, ExpectedRequestHash: requestHash,
		KeyResolver: r.resolver.Resolve, ReplayGuard: r.recordReplay,
	})
	if err != nil {
		return principal.Principal{}, err
	}
	if p.Kind != "service" || p.Subject != "service:matchmaking" || p.Issuer != "matchmaking" || p.Audience != "chat" || p.RPC != method || p.RequestID != requestID || p.RequestHash != requestHash || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
		return principal.Principal{}, errors.New("MatchSquad matchmaking service principal required")
	}
	return p, nil
}

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.Creds(r.server), grpc.ChainUnaryInterceptor(r.strictUnaryInterceptor)}
}

func (r *Runtime) strictUnaryInterceptor(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	return strictUnaryInterceptor(r)(ctx, request, info, handler)
}

type verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

func strictUnaryInterceptor(v verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != createMethod && info.FullMethod != teardownMethod {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on MatchSquad listener")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil || !message.ProtoReflect().IsValid() || hasUnknownFields(message) {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		operationID, ok := operationID(message)
		if !ok || operationID != transport.RequestID {
			return nil, status.Error(codes.Unauthenticated, "invalid principal binding")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		if v == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		verified, err := v.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			if errors.Is(err, errDependencyUnavailable) {
				return nil, status.Error(codes.Unavailable, "principal verifier unavailable")
			}
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}

func operationID(message proto.Message) (string, bool) {
	switch request := message.(type) {
	case *chatv1.CreateMatchSquadChatRequest:
		return request.GetOperationId(), true
	case *chatv1.TeardownMatchSquadChatRequest:
		return request.GetTeardownOperationId(), true
	default:
		return "", false
	}
}

func hasUnknownFields(message proto.Message) bool {
	reflect := message.ProtoReflect()
	if len(reflect.GetUnknown()) != 0 {
		return true
	}
	unknown := false
	reflect.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.IsMap() && field.MapValue().Message() != nil {
			value.Map().Range(func(_ protoreflect.MapKey, item protoreflect.Value) bool {
				unknown = hasUnknownFields(item.Message().Interface())
				return !unknown
			})
		} else if field.IsList() && field.Message() != nil {
			list := value.List()
			for i := 0; i < list.Len() && !unknown; i++ {
				unknown = hasUnknownFields(list.Get(i).Message().Interface())
			}
		} else if field.Message() != nil {
			unknown = hasUnknownFields(value.Message().Interface())
		}
		return !unknown
	})
	return unknown
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, jwtID string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid MatchSquad principal replay lifetime")
	}
	ttl = ((ttl + time.Millisecond - 1) / time.Millisecond) * time.Millisecond
	digest := sha256.Sum256([]byte(issuer + "\x00" + jwtID))
	key := "chat:match-squad:principal:replay:" + fmt.Sprintf("%x", digest)
	result, err := r.replay.SetArgs(ctx, key, "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, redis.Nil) {
		return errors.New("MatchSquad principal replay detected")
	}
	if err != nil || result != "OK" {
		return fmt.Errorf("%w: replay storage", errDependencyUnavailable)
	}
	return nil
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	if r.client != nil && r.client.Transport != nil {
		if transport, ok := r.client.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
	if r.replay != nil {
		return r.replay.Close()
	}
	return nil
}
