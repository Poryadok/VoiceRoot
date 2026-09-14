package principalruntime

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	filev1 "voice.app/voice/file/v1"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/principal"
)

type protectedFileFixture struct {
	filev1.UnimplementedFileServiceServer
	calls atomic.Int32
}

func (f *protectedFileFixture) ValidateStoryMedia(ctx context.Context, _ *filev1.ValidateStoryMediaRequest) (*filev1.ValidateStoryMediaResponse, error) {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Kind != "service" || p.Issuer != "story" || p.Subject != "service:story" {
		return nil, status.Error(codes.Internal, "interceptor did not install verified Story principal")
	}
	f.calls.Add(1)
	return &filev1.ValidateStoryMediaResponse{}, nil
}
func (f *protectedFileFixture) GetFileMetadata(context.Context, *filev1.GetFileMetadataRequest) (*filev1.GetFileMetadataResponse, error) {
	f.calls.Add(1)
	return &filev1.GetFileMetadataResponse{}, nil
}

type transportFixture struct {
	runtime      *Runtime
	redis        *miniredis.Miniredis
	service      *protectedFileFixture
	client       filev1.FileServiceClient
	active, next *rsa.PrivateKey
	address      string
	jwksStatus   atomic.Int32
}

func newTransportFixture(t *testing.T, cacheShort bool) *transportFixture {
	t.Helper()
	f := &transportFixture{service: &protectedFileFixture{}}
	var err error
	f.active, err = rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f.next, err = rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwk := func(kid string, key *rsa.PrivateKey) map[string]string {
		return map[string]string{"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}
	}
	document, err := json.Marshal(map[string]any{"keys": []any{jwk("current", f.active), jwk("next", f.next)}})
	require.NoError(t, err)
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if code := f.jwksStatus.Load(); code != 0 {
			w.WriteHeader(int(code))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(document)
	}))
	t.Cleanup(jwks.Close)
	temp := t.TempDir()
	jwksCA := filepath.Join(temp, "jwks-ca.pem")
	require.NoError(t, os.WriteFile(jwksCA, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: jwks.Certificate().Raw}), 0o600))
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &serverKey.PublicKey, serverKey)
	require.NoError(t, err)
	certPath, keyPath := filepath.Join(temp, "server.pem"), filepath.Join(temp, "server-key.pem")
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	require.NoError(t, os.WriteFile(certPath, certificate, 0o600))
	encodedKey, err := x509.MarshalPKCS8PrivateKey(serverKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedKey}), 0o600))
	f.redis = miniredis.RunT(t)
	cfg := Config{JWKSURLs: map[string]string{"story": jwks.URL, "gateway": jwks.URL}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second, ReplayAddr: f.redis.Addr(), JWKSCAFile: jwksCA, TLSCertFile: certPath, TLSKeyFile: keyPath, ListenAddr: "127.0.0.1:0"}
	if cacheShort {
		cfg.RefreshAfter = 10 * time.Millisecond
		cfg.HardExpiry = 40 * time.Millisecond
		cfg.UnknownKIDCooldown = 5 * time.Millisecond
	}
	f.runtime, err = New(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.runtime.Close()) })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	f.address = listener.Addr().String()
	server := grpc.NewServer(f.runtime.ServerOptions()...)
	filev1.RegisterFileServiceServer(server, f.service)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certificate))
	conn, err := grpc.NewClient(f.address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost"})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	f.client = filev1.NewFileServiceClient(conn)
	return f
}

func storyMediaRequest() *filev1.ValidateStoryMediaRequest {
	return &filev1.ValidateStoryMediaRequest{FileId: "7f1e69c1-0dfa-49fc-a356-980e128d686d", AuthorProfileId: "d097d722-6c61-4565-9483-65db9bc7909f", ExpectedStoryType: storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO}
}

func signedCredential(t *testing.T, key *rsa.PrivateKey, kid string, mutate func(map[string]any)) string {
	t.Helper()
	hash, err := principal.RequestHash(storyMediaRequest())
	require.NoError(t, err)
	now := time.Now().Unix()
	claims := map[string]any{"principal_type": "service", "iss": "story", "sub": "service:story", "aud": "file", "rpc": filev1.FileService_ValidateStoryMedia_FullMethodName, "request_id": "request-1", "request_hash": hash, "iat": now, "nbf": now, "exp": now + 30, "jti": time.Now().Format(time.RFC3339Nano)}
	if mutate != nil {
		mutate(claims)
	}
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func callMedia(t *testing.T, f *transportFixture, token string, req *filev1.ValidateStoryMediaRequest) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := f.client.ValidateStoryMedia(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "request-1")), req)
	return err
}

func TestProtectedListenerVerifiesSignedClaimsBeforeHandler(t *testing.T) {
	f := newTransportFixture(t, false)
	forged, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	cases := []struct {
		name   string
		key    *rsa.PrivateKey
		kid    string
		mutate func(map[string]any)
		want   codes.Code
	}{
		{"valid current", f.active, "current", nil, codes.OK},
		{"valid next", f.next, "next", nil, codes.OK},
		{"forged signature", forged, "current", nil, codes.Unauthenticated},
		{"unknown key", f.active, "unknown", nil, codes.Unauthenticated},
		{"untrusted issuer", f.active, "current", func(c map[string]any) { c["iss"] = "stranger"; c["sub"] = "service:stranger" }, codes.Unauthenticated},
		{"trusted disallowed service", f.active, "current", func(c map[string]any) { c["iss"] = "gateway"; c["sub"] = "service:gateway" }, codes.PermissionDenied},
		{"wrong subject", f.active, "current", func(c map[string]any) { c["sub"] = "service:gateway" }, codes.Unauthenticated},
		{"wrong principal type", f.active, "current", func(c map[string]any) { c["principal_type"] = "delegated_user" }, codes.Unauthenticated},
		{"user claims", f.active, "current", func(c map[string]any) {
			c["account_id"] = "account"
			c["profile_id"] = "profile"
			c["session_epoch"] = 1
		}, codes.Unauthenticated},
		{"wrong audience", f.active, "current", func(c map[string]any) { c["aud"] = "story" }, codes.Unauthenticated},
		{"wrong rpc", f.active, "current", func(c map[string]any) { c["rpc"] = filev1.FileService_GetFileMetadata_FullMethodName }, codes.Unauthenticated},
		{"wrong hash", f.active, "current", func(c map[string]any) {
			c["request_hash"] = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		}, codes.Unauthenticated},
		{"wrong request id", f.active, "current", func(c map[string]any) { c["request_id"] = "other" }, codes.Unauthenticated},
		{"expired", f.active, "current", func(c map[string]any) {
			now := time.Now().Unix()
			c["iat"] = now - 31
			c["nbf"] = now - 31
			c["exp"] = now - 1
		}, codes.Unauthenticated},
		{"future issued at", f.active, "current", func(c map[string]any) {
			now := time.Now().Unix()
			c["iat"] = now + 20
			c["nbf"] = now + 20
			c["exp"] = now + 40
		}, codes.Unauthenticated},
		{"future not before", f.active, "current", func(c map[string]any) { c["nbf"] = time.Now().Unix() + 20 }, codes.Unauthenticated},
		{"overlong", f.active, "current", func(c map[string]any) { c["exp"] = time.Now().Unix() + 31 }, codes.Unauthenticated},
		{"missing jti", f.active, "current", func(c map[string]any) { delete(c, "jti") }, codes.Unauthenticated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := f.service.calls.Load()
			err := callMedia(t, f, signedCredential(t, tc.key, tc.kid, tc.mutate), storyMediaRequest())
			require.Equal(t, tc.want, status.Code(err))
			if tc.want == codes.OK {
				require.Equal(t, before+1, f.service.calls.Load())
			} else {
				require.Equal(t, before, f.service.calls.Load(), "deny must occur before handler/store")
			}
		})
	}
}

func TestProtectedListenerRejectsReplayChangedRequestAndOrdinaryMethods(t *testing.T) {
	f := newTransportFixture(t, false)
	token := signedCredential(t, f.active, "current", nil)
	require.NoError(t, callMedia(t, f, token, storyMediaRequest()))
	require.Equal(t, codes.Unauthenticated, status.Code(callMedia(t, f, token, storyMediaRequest())))
	changed := storyMediaRequest()
	changed.AuthorProfileId = "743a3dfe-62d5-42fb-af74-54c6b3400001"
	require.Equal(t, codes.Unauthenticated, status.Code(callMedia(t, f, signedCredential(t, f.active, "current", nil), changed)))
	unknown := storyMediaRequest()
	unknown.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	require.Equal(t, codes.InvalidArgument, status.Code(callMedia(t, f, signedCredential(t, f.active, "current", nil), unknown)))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := f.client.GetFileMetadata(ctx, &filev1.GetFileMetadataRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, int32(1), f.service.calls.Load())
}

func TestProtectedListenerWrongCAFailsBeforeHandler(t *testing.T) {
	f := newTransportFixture(t, false)
	conn, err := grpc.NewClient(f.address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: x509.NewCertPool(), ServerName: "localhost"})))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	token := signedCredential(t, f.active, "current", nil)
	_, err = filev1.NewFileServiceClient(conn).ValidateStoryMedia(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "request-1")), storyMediaRequest())
	require.Error(t, err)
	require.Zero(t, f.service.calls.Load())
}

func TestProtectedListenerJWKSAndReplayOutageFailClosed(t *testing.T) {
	t.Run("JWKS unavailable without cached key", func(t *testing.T) {
		f := newTransportFixture(t, false)
		f.jwksStatus.Store(http.StatusServiceUnavailable)
		err := callMedia(t, f, signedCredential(t, f.active, "current", nil), storyMediaRequest())
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Zero(t, f.service.calls.Load())
	})
	t.Run("Redis unavailable", func(t *testing.T) {
		f := newTransportFixture(t, false)
		require.NoError(t, callMedia(t, f, signedCredential(t, f.active, "current", nil), storyMediaRequest()))
		f.redis.Close()
		err := callMedia(t, f, signedCredential(t, f.active, "current", nil), storyMediaRequest())
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Equal(t, int32(1), f.service.calls.Load())
	})
	t.Run("hard expired last good key", func(t *testing.T) {
		f := newTransportFixture(t, true)
		require.NoError(t, callMedia(t, f, signedCredential(t, f.active, "current", nil), storyMediaRequest()))
		f.jwksStatus.Store(http.StatusServiceUnavailable)
		require.Eventually(t, func() bool {
			return status.Code(callMedia(t, f, signedCredential(t, f.active, "current", nil), storyMediaRequest())) == codes.Unauthenticated
		}, 2*time.Second, 20*time.Millisecond)
		before := f.service.calls.Load()
		require.Equal(t, codes.Unauthenticated, status.Code(callMedia(t, f, signedCredential(t, f.active, "current", nil), storyMediaRequest())))
		require.Equal(t, before, f.service.calls.Load())
	})
}

func TestReplayGuardUsesFileNamespaceAndSharedAtomicAdmission(t *testing.T) {
	server := miniredis.RunT(t)
	first := redis.NewClient(&redis.Options{Addr: server.Addr()})
	second := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = first.Close(); _ = second.Close() }()
	a, b := &Runtime{replay: first}, &Runtime{replay: second}
	expiry := time.Now().Add(20 * time.Second)
	require.NoError(t, a.recordReplay(context.Background(), "story", "same-jti", expiry))
	require.Error(t, b.recordReplay(context.Background(), "story", "same-jti", expiry))
	keys := server.Keys()
	require.Len(t, keys, 1)
	require.Contains(t, keys[0], "file:principal:replay:")
	require.Positive(t, server.TTL(keys[0]))
	require.LessOrEqual(t, server.TTL(keys[0]), 20*time.Second)
	require.NoError(t, first.Close())
	require.Error(t, a.recordReplay(context.Background(), "story", "another-jti", expiry))
}
