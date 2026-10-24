package installprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"testing"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/principal"
)

type sessionFake struct {
	floor int64
	err   error
	calls int
	key   *rsa.PrivateKey
	t     *testing.T
}

func (f *sessionFake) GetBotSessionEpochFloor(ctx context.Context, req *authv1.GetBotSessionEpochFloorRequest, _ ...grpc.CallOption) (*authv1.GetBotSessionEpochFloorResponse, error) {
	f.calls++
	md, _ := metadata.FromOutgoingContext(ctx)
	if len(md) != 2 {
		f.t.Fatal("caller metadata escaped")
	}
	incoming := metadata.NewIncomingContext(ctx, md)
	transport, err := principal.IncomingMetadata(incoming)
	if err != nil {
		f.t.Fatal(err)
	}
	hash, _ := principal.RequestHash(req)
	_, err = principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "auth", ExpectedRPC: authv1.AuthService_GetBotSessionEpochFloor_FullMethodName, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &f.key.PublicKey, nil }})
	if err != nil {
		f.t.Fatal(err)
	}
	return &authv1.GetBotSessionEpochFloorResponse{SessionEpochFloor: f.floor}, f.err
}
func TestSessionCheckerUsesFreshProtectedBotFloor(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	if err != nil {
		t.Fatal(err)
	}
	fake := &sessionFake{floor: 2, key: key, t: t}
	checker := &SessionChecker{Client: fake, Issuer: issuer}
	account := uuid.NewString()
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "old", "x-voice-user-id", "raw"))
	if err = checker.Check(ctx, account, 2); err != nil {
		t.Fatal(err)
	}
	if err = checker.Check(ctx, account, 1); err == nil {
		t.Fatal("stale session accepted")
	}
	fake.err = errors.New("offline")
	if err = checker.Check(ctx, account, 2); err == nil {
		t.Fatal("cached allow survived Auth failure")
	}
	if fake.calls != 3 {
		t.Fatalf("expected fresh observation per check: %d", fake.calls)
	}
}
func TestInstallationRuntimeConfigPresenceIsStrict(t *testing.T) {
	env := map[string]string{}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	if _, enabled, err := LoadConfig(lookup); enabled || err != nil {
		t.Fatal("unset configuration enabled")
	}
	env["BOT_INSTALL_GRPC_LISTEN"] = ""
	if _, enabled, err := LoadConfig(lookup); !enabled || err == nil {
		t.Fatal("blank setting disabled verifier")
	}
	for _, key := range []string{"BOT_INSTALL_GRPC_LISTEN", "BOT_INSTALL_TLS_CERT_FILE", "BOT_INSTALL_TLS_KEY_FILE", "BOT_INSTALL_TLS_CLIENT_CA_FILE", "BOT_INSTALL_GATEWAY_JWKS_CA_FILE"} {
		env[key] = "configured"
	}
	env["BOT_INSTALL_GATEWAY_JWKS_URL"] = "https://gateway/.well-known/principal-jwks.json"
	env["BOT_PRINCIPAL_REPLAY_REDIS_URL"] = "redis://redis:6379"
	if _, enabled, err := LoadConfig(lookup); !enabled || err != nil {
		t.Fatal(err)
	}
	env["BOT_INSTALL_GATEWAY_JWKS_URL"] = "http://gateway/jwks"
	if _, _, err := LoadConfig(lookup); err == nil {
		t.Fatal("insecure JWKS accepted")
	}
}
