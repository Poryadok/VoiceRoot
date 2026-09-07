package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestGatewaySessionEpochStrictConfigDefaultsToCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value *string
	}{
		{name: "unset"},
		{name: "exact false", value: configString("false")},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			configureSessionEpochEnv(t, tc.value, "")

			config, err := loadGatewayConfigFromEnvChecked()
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			if config.sessionEpochStrict {
				t.Fatal("session epoch strict unexpectedly enabled")
			}
			if config.sessionEpochFloor != nil {
				t.Fatalf("session epoch floor = %T, want nil in compatibility mode", config.sessionEpochFloor)
			}
		})
	}
}

func TestGatewaySessionEpochStrictConfigRejectsNonExactValues(t *testing.T) {
	for _, value := range []string{
		"", " true", "true ", " false", "false ",
		"TRUE", "True", "FALSE", "False", "1", "typo",
	} {
		value := value
		t.Run(value, func(t *testing.T) {
			configureSessionEpochEnv(t, &value, "127.0.0.1:6379")

			if _, err := loadGatewayConfigFromEnvChecked(); err == nil {
				t.Fatalf("strict value %q unexpectedly accepted", value)
			}
		})
	}
}

func TestGatewaySessionEpochStrictConfigRequiresRedisAndBuildsCanonicalAuthFloor(t *testing.T) {
	strict := "true"
	t.Run("requires Redis before serving", func(t *testing.T) {
		configureSessionEpochEnv(t, &strict, "")

		if _, err := loadGatewayConfigFromEnvChecked(); err == nil {
			t.Fatal("strict config without Redis unexpectedly succeeded")
		}
	})

	t.Run("uses Auth-owned Redis floor without prefix override or startup ping", func(t *testing.T) {
		configureSessionEpochEnv(t, &strict, "198.51.100.11:6379")
		t.Setenv("GATEWAY_REDIS_PASSWORD", "shared-password")

		config, err := loadGatewayConfigFromEnvChecked()
		if err != nil {
			t.Fatalf("load strict config: %v", err)
		}
		if !config.sessionEpochStrict {
			t.Fatal("session epoch strict not enabled")
		}
		floor, ok := config.sessionEpochFloor.(*redisSessionEpochFloor)
		if !ok || floor == nil {
			t.Fatalf("session epoch floor = %T, want *redisSessionEpochFloor", config.sessionEpochFloor)
		}
		// The epoch floor is Auth-owned and has no Gateway prefix override.
		if floor.addr != "198.51.100.11:6379" || floor.password != "shared-password" || floor.prefix != sessionEpochFloorPrefix || floor.timeout != 2*time.Second {
			t.Fatalf("floor = %#v, want shared Redis addr/password, Auth prefix, and 2s timeout", floor)
		}
	})
}

func TestGatewayBootstrapRejectsInvalidStrictConfigBeforeServerConstruction(t *testing.T) {
	for _, tc := range []struct {
		name   string
		strict string
		redis  string
	}{
		{name: "malformed strict value", strict: "TRUE", redis: "198.51.100.13:6379"},
		{name: "strict without Redis", strict: "true", redis: ""},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			configureSessionEpochEnv(t, &tc.strict, tc.redis)

			serverConstructed := 0
			server, err := newGatewayServerFromEnv(":8080", func(handler http.Handler) *http.Server {
				serverConstructed++
				return &http.Server{Addr: ":8080", Handler: handler}
			})
			if err == nil {
				t.Fatal("invalid strict config unexpectedly bootstrapped a server")
			}
			if server != nil {
				t.Fatalf("server = %#v, want nil on startup config error", server)
			}
			if serverConstructed != 0 {
				t.Fatalf("server constructor calls = %d, want 0", serverConstructed)
			}
		})
	}
}

func TestGatewayBootstrapRejectsInvalidGRPCUpstreamsBeforeServerConstruction(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{name: "malformed JSON", raw: `{"users":`},
		{name: "non-string upstream value", raw: `{"users":123}`},
		{name: "null upstream value", raw: `{"users":null}`},
		{name: "empty upstream address", raw: `{"users":""}`},
		{name: "whitespace upstream address", raw: `{"users":" \t "}`},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			configureSessionEpochEnv(t, configString("false"), "")
			t.Setenv("GATEWAY_USERS_GRPC_ADDR", "")
			t.Setenv("GATEWAY_GRPC_UPSTREAMS_JSON", tc.raw)

			serverConstructed := 0
			server, err := newGatewayServerFromEnv(":8080", func(handler http.Handler) *http.Server {
				serverConstructed++
				return &http.Server{Addr: ":8080", Handler: handler}
			})
			if err == nil {
				t.Fatal("invalid gRPC upstream configuration unexpectedly bootstrapped a server")
			}
			if !strings.Contains(err.Error(), "GATEWAY_GRPC_UPSTREAMS_JSON") {
				t.Fatalf("bootstrap error = %v, want GATEWAY_GRPC_UPSTREAMS_JSON context", err)
			}
			if server != nil {
				t.Fatalf("server = %#v, want nil on startup config error", server)
			}
			if serverConstructed != 0 {
				t.Fatalf("server constructor calls = %d, want 0", serverConstructed)
			}
		})
	}
}

func TestGatewayBootstrapAllowsFutureGRPCNamespace(t *testing.T) {
	configureSessionEpochEnv(t, configString("false"), "")
	t.Setenv("GATEWAY_GRPC_UPSTREAMS_JSON", `{"future-service":"future:9090"}`)

	serverConstructed := 0
	server, err := newGatewayServerFromEnv(":8080", func(handler http.Handler) *http.Server {
		serverConstructed++
		return &http.Server{Addr: ":8080", Handler: handler}
	})
	if err != nil {
		t.Fatalf("future namespace must remain forward-compatible: %v", err)
	}
	if serverConstructed != 1 {
		t.Fatalf("server constructor calls = %d, want 1", serverConstructed)
	}
	if err := server.Close(); err != nil {
		t.Fatalf("close server: %v", err)
	}
}

func TestGatewayBootstrapWaitsForConfiguredUserGRPCBeforeServerConstruction(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve unavailable grpc address: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close reserved grpc address: %v", err)
	}

	t.Setenv("GATEWAY_SESSION_EPOCH_STRICT", "false")
	t.Setenv("GATEWAY_REDIS_ADDR", "")
	t.Setenv("GATEWAY_USERS_GRPC_ADDR", "")
	t.Setenv("GATEWAY_GRPC_UPSTREAMS_JSON", `{"users":"`+addr+`"}`)
	t.Setenv("GRPC_DIAL_TIMEOUT", "100ms")

	serverConstructed := 0
	server, err := newGatewayServerFromEnv(":8080", func(handler http.Handler) *http.Server {
		serverConstructed++
		return &http.Server{Addr: ":8080", Handler: handler}
	})
	if err == nil {
		if server != nil {
			_ = server.Close()
		}
		t.Fatal("unavailable user grpc unexpectedly bootstrapped a server")
	}
	if server != nil {
		t.Fatalf("server = %#v, want nil when user grpc is unavailable", server)
	}
	if serverConstructed != 0 {
		t.Fatalf("server constructor calls = %d, want 0 before User gRPC readiness", serverConstructed)
	}
}

func TestGatewayBootstrapAcceptsReadyConfiguredUserGRPC(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for user grpc: %v", err)
	}
	grpcServer := grpc.NewServer()
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = listener.Close()
	})

	t.Setenv("GATEWAY_SESSION_EPOCH_STRICT", "false")
	t.Setenv("GATEWAY_REDIS_ADDR", "")
	t.Setenv("GATEWAY_USERS_GRPC_ADDR", "")
	t.Setenv("GATEWAY_GRPC_UPSTREAMS_JSON", `{"users":"`+listener.Addr().String()+`"}`)
	t.Setenv("GRPC_DIAL_TIMEOUT", time.Second.String())

	server, err := newGatewayServerFromEnv(":8080", func(handler http.Handler) *http.Server {
		return &http.Server{Addr: ":8080", Handler: handler}
	})
	if err != nil {
		t.Fatalf("bootstrap ready User gRPC: %v", err)
	}
	if server == nil {
		t.Fatal("server = nil after User gRPC becomes ready")
	}
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown server: %v", err)
	}
}

func TestGatewayServerShutdownDrainsHandlersBeforeClosingUpstreams(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen gateway: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	handlerStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	upstreamsClosed := make(chan struct{})
	shutdownStarted := make(chan struct{})
	server := &gatewayServer{
		Server: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(handlerStarted)
			<-releaseHandler
			w.WriteHeader(http.StatusNoContent)
		})},
		closeClients: func() { close(upstreamsClosed) },
	}
	server.RegisterOnShutdown(func() { close(shutdownStarted) })
	go func() { _ = server.Serve(listener) }()

	responseDone := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + listener.Addr().String())
		if err == nil {
			err = response.Body.Close()
		}
		responseDone <- err
	}()
	select {
	case <-handlerStarted:
	case <-time.After(time.Second):
		t.Fatal("gateway handler did not start")
	}

	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- server.Shutdown(context.Background()) }()
	select {
	case <-shutdownStarted:
	case <-time.After(time.Second):
		t.Fatal("http shutdown did not begin")
	}
	select {
	case <-upstreamsClosed:
		t.Fatal("gRPC upstreams closed before the active HTTP handler drained")
	default:
	}

	close(releaseHandler)
	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("shutdown gateway: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("gateway shutdown did not finish after handler drained")
	}
	if err := <-responseDone; err != nil {
		t.Fatalf("gateway response: %v", err)
	}
	select {
	case <-upstreamsClosed:
	case <-time.After(time.Second):
		t.Fatal("gRPC upstreams were not closed after gateway shutdown")
	}
}

func TestGatewayMainUsesCheckedConfigBootstrap(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(mainSource), "newGatewayServerFromEnv(") {
		t.Fatal("main must use newGatewayServerFromEnv so startup config errors prevent serving")
	}
}

func TestGatewaySessionEpochJWKSConfigUsesStrictnessOption(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	const issuer = "voice-auth"
	const audience = "voice-client"
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{configRSAJWK("key-1", &key.PublicKey)},
		})
	}))
	t.Cleanup(jwks.Close)

	base := map[string]any{
		"sub": "account-1", "iss": issuer, "aud": audience,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	withEpoch := cloneConfigJWTPayload(base)
	withEpoch["session_epoch"] = int64(7)
	malformedEpoch := cloneConfigJWTPayload(base)
	malformedEpoch["session_epoch"] = "7"

	for _, tc := range []struct {
		name      string
		strict    *string
		redisAddr string
		payload   map[string]any
		wantCode  string
		wantEpoch int64
	}{
		{name: "compatibility permits legacy JWT", payload: base, wantEpoch: 0},
		{name: "compatibility permits positive epoch", payload: withEpoch, wantEpoch: 7},
		{name: "compatibility rejects malformed present epoch", payload: malformedEpoch, wantCode: "invalid_token"},
		{name: "strict rejects legacy JWT", strict: configString("true"), redisAddr: "198.51.100.12:6379", payload: base, wantCode: "invalid_token"},
		{name: "strict permits positive epoch", strict: configString("true"), redisAddr: "198.51.100.12:6379", payload: withEpoch, wantEpoch: 7},
		{name: "strict rejects malformed epoch", strict: configString("true"), redisAddr: "198.51.100.12:6379", payload: malformedEpoch, wantCode: "invalid_token"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			configureSessionEpochEnv(t, tc.strict, tc.redisAddr)
			t.Setenv("GATEWAY_JWKS_URL", jwks.URL)
			t.Setenv("GATEWAY_JWT_ISSUER", issuer)
			t.Setenv("GATEWAY_JWT_AUDIENCE", audience)

			config, err := loadGatewayConfigFromEnvChecked()
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			token := signConfigJWT(t, "key-1", key, tc.payload)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			claims, code := config.tokenValidator.Validate(req)
			if code != tc.wantCode {
				t.Fatalf("JWT validation code = %q, want %q", code, tc.wantCode)
			}
			if code == "" && claims.SessionEpoch != tc.wantEpoch {
				t.Fatalf("session epoch = %d, want %d", claims.SessionEpoch, tc.wantEpoch)
			}
		})
	}
}

func configureSessionEpochEnv(t *testing.T, strict *string, redisAddr string) {
	t.Helper()
	t.Setenv("GATEWAY_AUTH_MODE", "")
	t.Setenv("GATEWAY_STATIC_TOKENS_JSON", "")
	t.Setenv("GATEWAY_JWKS_URL", "")
	t.Setenv("GATEWAY_JWT_ISSUER", "")
	t.Setenv("GATEWAY_JWT_AUDIENCE", "")
	t.Setenv("GATEWAY_REDIS_ADDR", redisAddr)
	t.Setenv("GATEWAY_REDIS_PASSWORD", "")
	if strict == nil {
		t.Setenv("GATEWAY_SESSION_EPOCH_STRICT", "placeholder")
		if err := os.Unsetenv("GATEWAY_SESSION_EPOCH_STRICT"); err != nil {
			t.Fatalf("unset strict env: %v", err)
		}
		return
	}
	t.Setenv("GATEWAY_SESSION_EPOCH_STRICT", *strict)
}

func configString(value string) *string {
	return &value
}

func cloneConfigJWTPayload(payload map[string]any) map[string]any {
	clone := make(map[string]any, len(payload))
	for key, value := range payload {
		clone[key] = value
	}
	return clone
}

func configRSAJWK(kid string, key *rsa.PublicKey) map[string]string {
	return map[string]string{
		"kty": "RSA",
		"kid": kid,
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}
}

func signConfigJWT(t *testing.T, kid string, key *rsa.PrivateKey, payload map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	if err != nil {
		t.Fatalf("marshal JWT header: %v", err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal JWT payload: %v", err)
	}
	signed := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	digest := sha256.Sum256([]byte(signed))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(signature)
}
