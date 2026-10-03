package spaceprincipal

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/pkg/principal"
)

func TestComposeSpaceJWKSUsesPublishedIssuerRoute(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source location unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../../"))
	compose, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	service := strings.SplitN(string(compose), "\n  matchmaking:\n", 2)
	if len(service) != 2 {
		t.Fatal("Compose Matchmaking service missing")
	}
	section := strings.SplitN(service[1], "\n  search:\n", 2)[0]
	for _, line := range strings.Split(section, "\n") {
		if value, found := strings.CutPrefix(strings.TrimSpace(line), "S2S_JWKS_URLS_JSON: "); found {
			var endpoints map[string]string
			if err := json.Unmarshal([]byte(strings.Trim(value, "'")), &endpoints); err != nil {
				t.Fatal(err)
			}
			if endpoints["space"] != "https://space:8443/.well-known/jwks.json" {
				t.Fatal("Compose must fetch Space's published public JWKS route before enabling the principal listener")
			}
			return
		}
	}
	t.Fatal("Compose Space JWKS trust missing")
}

func TestConfigRequiresCompleteTLSReplayAndSpaceJWKS(t *testing.T) {
	cfg := Config{
		ListenAddr: ":9092", TLSCertFile: "server.crt", TLSKeyFile: "server.key", ClientCAFile: "client-ca.crt",
		JWKSURL: "https://space:8443/.well-known/principal-jwks.json", ReplayAddr: "redis:6379",
		RefreshAfter: timeSecond, HardExpiry: 2 * timeSecond, UnknownKIDCooldown: timeSecond,
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	for name, mutate := range map[string]func(*Config){
		"listener":     func(c *Config) { c.ListenAddr = "not-an-address" },
		"TLS identity": func(c *Config) { c.TLSKeyFile = "" },
		"client CA":    func(c *Config) { c.ClientCAFile = "" },
		"replay Redis": func(c *Config) { c.ReplayAddr = "" },
		"JWKS URL":     func(c *Config) { c.JWKSURL = "http://space:8443/jwks" },
		"cache order":  func(c *Config) { c.HardExpiry = timeSecond / 2 },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := cfg
			mutate(&invalid)
			if err := invalid.validate(); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}

func TestLoadFromEnvIsDisabledOnlyWhenNoPrincipalSettingExists(t *testing.T) {
	var previous = map[string]string{}
	var present = map[string]bool{}
	for _, name := range []string{
		"MATCHMAKING_SPACE_PRINCIPAL_GRPC_LISTEN", "MATCHMAKING_SPACE_PRINCIPAL_TLS_CERT_FILE",
		"MATCHMAKING_SPACE_PRINCIPAL_TLS_KEY_FILE", "MATCHMAKING_SPACE_PRINCIPAL_CLIENT_CA_FILE",
		"MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_ADDR", "MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_PASSWORD",
		"S2S_JWKS_URLS_JSON", "S2S_JWKS_CA_FILE", "S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN",
	} {
		previous[name], present[name] = os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for name := range previous {
			if present[name] {
				_ = os.Setenv(name, previous[name])
			} else {
				_ = os.Unsetenv(name)
			}
		}
	})
	if _, enabled, err := LoadFromEnv(); err != nil || enabled {
		t.Fatalf("empty config = enabled %v, err %v", enabled, err)
	}
	t.Setenv("MATCHMAKING_SPACE_PRINCIPAL_TLS_CERT_FILE", "partial")
	if _, enabled, err := LoadFromEnv(); err == nil || !enabled {
		t.Fatalf("partial config must fail closed, enabled %v err %v", enabled, err)
	}
}

func TestLifecycleInterceptorsIsolateProtectedRPCsAndBindRequest(t *testing.T) {
	const lifecycleMethod = matchmakingv1.MatchmakingService_ApplySpaceLifecycleFence_FullMethodName
	const publicMethod = matchmakingv1.MatchmakingService_ListGames_FullMethodName
	ordinary := OrdinaryUnaryInterceptor()
	called := false
	handler := func(context.Context, any) (any, error) { called = true; return "ok", nil }
	if _, err := ordinary(context.Background(), &matchmakingv1.ApplySpaceLifecycleFenceRequest{}, &grpc.UnaryServerInfo{FullMethod: lifecycleMethod}, handler); status.Code(err) != codes.Unavailable || called {
		t.Fatalf("ordinary listener lifecycle result: err=%v called=%v", err, called)
	}
	if _, err := ordinary(context.Background(), &matchmakingv1.ListGamesRequest{}, &grpc.UnaryServerInfo{FullMethod: publicMethod}, handler); err != nil || !called {
		t.Fatalf("ordinary listener public result: err=%v called=%v", err, called)
	}
	called = false
	verifier := &recordingVerifier{principal: principal.Principal{
		Kind: "service", Issuer: "space", Subject: "service:space", Audience: "matchmaking",
		RPC: lifecycleMethod, RequestID: "request-1", AccountID: "", ProfileID: "", SessionEpoch: 0,
	}}
	strict := StrictUnaryInterceptor(verifier)
	if _, err := strict(context.Background(), &matchmakingv1.ApplySpaceLifecycleFenceRequest{}, &grpc.UnaryServerInfo{FullMethod: publicMethod}, handler); status.Code(err) != codes.PermissionDenied || called {
		t.Fatalf("protected listener public result: err=%v called=%v", err, called)
	}
	request := &matchmakingv1.ApplySpaceLifecycleFenceRequest{}
	hash, err := principal.RequestHash(request)
	if err != nil {
		t.Fatal(err)
	}
	verifier.principal.RequestHash = hash
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer signed", "x-request-id", "request-1"))
	response, err := strict(ctx, request, &grpc.UnaryServerInfo{FullMethod: lifecycleMethod}, handler)
	if err != nil || response != "ok" || !called {
		t.Fatalf("valid protected request: response=%v err=%v called=%v", response, err, called)
	}
	if verifier.method != lifecycleMethod || verifier.requestID != "request-1" || verifier.requestHash != hash {
		t.Fatalf("verification binding = %q %q %q", verifier.method, verifier.requestID, verifier.requestHash)
	}
}

func TestStrictInterceptorRejectsMalformedAndUntrustedPrincipalsBeforeHandler(t *testing.T) {
	const method = matchmakingv1.MatchmakingService_PurgeSpace_FullMethodName
	verifier := &recordingVerifier{err: errors.New("bad signature")}
	strict := StrictUnaryInterceptor(verifier)
	request := &matchmakingv1.PurgeSpaceRequest{}
	handlerCalled := false
	handler := func(context.Context, any) (any, error) { handlerCalled = true; return nil, nil }
	for name, ctx := range map[string]context.Context{
		"missing metadata": context.Background(),
		"duplicate bearer": metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer a", "authorization", "Bearer b", "x-request-id", "id")),
		"raw authority":    metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer a", "x-request-id", "id", "x-account-id", "account")),
	} {
		t.Run(name, func(t *testing.T) {
			handlerCalled = false
			_, err := strict(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, handler)
			if status.Code(err) != codes.Unauthenticated || handlerCalled {
				t.Fatalf("err=%v handlerCalled=%v", err, handlerCalled)
			}
		})
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token", "x-request-id", "id"))
	if _, err := strict(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, handler); status.Code(err) != codes.Unauthenticated || handlerCalled {
		t.Fatalf("bad signature err=%v handlerCalled=%v", err, handlerCalled)
	}
	verifier.err = nil
	verifier.principal = principal.Principal{Kind: "service", Issuer: "space", Subject: "service:other", Audience: "matchmaking", RPC: method, RequestID: "id"}
	if _, err := strict(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, handler); status.Code(err) != codes.PermissionDenied || handlerCalled {
		t.Fatalf("wrong identity err=%v handlerCalled=%v", err, handlerCalled)
	}
}

type recordingVerifier struct {
	principal                      principal.Principal
	err                            error
	method, requestID, requestHash string
}

func (v *recordingVerifier) Verify(_ context.Context, _, method, requestID, requestHash string) (principal.Principal, error) {
	v.method, v.requestID, v.requestHash = method, requestID, requestHash
	return v.principal, v.err
}

const timeSecond = 1_000_000_000
