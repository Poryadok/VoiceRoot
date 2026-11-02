//go:build spacebanintegration

package grpcsvc

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
	"voice/backend/space/internal/usercapture"
)

type observedUserCapture struct {
	t      *testing.T
	client store.SpaceAccountCaptureClient
}

func (c observedUserCapture) CaptureSpaceMemberAccountBindings(ctx context.Context, request *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
	started := time.Now()
	response, err := c.client.CaptureSpaceMemberAccountBindings(ctx, request)
	if err != nil {
		c.t.Logf("User capture failed after %s: %v", time.Since(started), err)
	}
	return response, err
}

// Gateway's HTTP handler, the protected Space receiver and the User capture
// process are real. Login claims and Auth's parent authority are fixtures.
func TestPublicSpaceBanThroughUserProcessCapture(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL and User process")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	owner, ownerAccount := uuid.New(), uuid.New()
	userRoot := filepath.Join(repoRoot(t), "src/backend/user")
	binary := filepath.Join(t.TempDir(), "user-capture-fixture.exe")
	build := exec.CommandContext(ctx, "go", "test", "-c", "-o", binary, "./internal/sourcecustody")
	build.Dir = userRoot
	output, err := build.CombinedOutput()
	require.NoError(t, err, "%s", output)
	child := exec.CommandContext(ctx, binary, "-test.run=^TestUserProcessServesQualifiedSpaceCapture$", "-test.timeout=7m")
	child.Dir = filepath.Join(userRoot, "internal/sourcecustody")
	child.Env = append(os.Environ(), "VOICE_USER_CAPTURE_TEST_SERVE=1", "VOICE_CAPTURE_ACTOR_PROFILE="+owner.String(), "VOICE_CAPTURE_ACTOR_ACCOUNT="+ownerAccount.String())
	stdin, err := child.StdinPipe()
	require.NoError(t, err)
	stdout, err := child.StdoutPipe()
	require.NoError(t, err)
	var stderr bytes.Buffer
	child.Stderr = &stderr
	require.NoError(t, child.Start())
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	joined := false
	t.Cleanup(func() {
		_ = stdin.Close()
		if !joined {
			select {
			case <-done:
			case <-time.After(15 * time.Second):
				_ = child.Process.Kill()
				<-done
			}
		}
	})
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "VOICE_CAPTURE_READY ") {
				ready <- strings.TrimPrefix(scanner.Text(), "VOICE_CAPTURE_READY ")
				return
			}
		}
		close(ready)
	}()
	var line string
	select {
	case line = <-ready:
		require.NotEmpty(t, line, "User fixture ended without an endpoint")
	case err := <-done:
		joined = true
		t.Fatalf("User fixture exited before readiness: %v: %s", err, stderr.String())
	case <-time.After(4 * time.Minute):
		t.Fatal("User fixture readiness timed out")
	}
	var user struct {
		Address     string `json:"address"`
		CA          string `json:"ca"`
		Cert        string `json:"cert"`
		Key         string `json:"key"`
		SpaceSigner string `json:"space_signer"`
		Profile     string `json:"profile"`
		Account     string `json:"account"`
	}
	require.NoError(t, json.Unmarshal([]byte(line), &user))
	profile, err := uuid.Parse(user.Profile)
	require.NoError(t, err)
	account, err := uuid.Parse(user.Account)
	require.NoError(t, err)
	ca, err := os.ReadFile(user.CA)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	cert, err := tls.LoadX509KeyPair(user.Cert, user.Key)
	require.NoError(t, err)
	keyPEM, err := os.ReadFile(user.SpaceSigner)
	require.NoError(t, err)
	block, rest := pem.Decode(keyPEM)
	require.NotNil(t, block)
	require.Empty(t, rest)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	key, ok := parsed.(*rsa.PrivateKey)
	require.True(t, ok)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	capture, conn, err := usercapture.New(usercapture.TransportConfig{Address: user.Address, ServerName: "localhost", Roots: roots, Certificate: cert}, issuer)
	require.NoError(t, err)
	defer conn.Close()
	f := newNativeModerationFixtureWithMembership(t, owner, ownerAccount, profile, account, true)
	f.st.AccountCaptures = observedUserCapture{t: t, client: capture}
	member, err := f.st.IsSpaceMember(ctx, f.space, profile)
	require.NoError(t, err)
	require.True(t, member, "the managed target must be a member before Ban")
	tlsDir := t.TempDir()
	serverCert, caFile, certFile, keyFile, clientCert, clientKey := be041TLSFixture(t, tlsDir)
	gatewayKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	nextGatewayKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signingKey := filepath.Join(tlsDir, "gateway-signing.pem")
	require.NoError(t, os.WriteFile(signingKey, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(gatewayKey)}), 0600))
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{
			{"kty": "RSA", "kid": "current", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(gatewayKey.N.Bytes()), "e": "AQAB"},
			{"kty": "RSA", "kid": "next", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(nextGatewayKey.N.Bytes()), "e": "AQAB"},
		}})
	}))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}}
	jwks.StartTLS()
	t.Cleanup(jwks.Close)
	redis := miniredis.RunT(t)
	redis.Set("auth:session:min_epoch:"+ownerAccount.String(), "7")
	runtime, err := lifecycleprincipal.New(ctx, lifecycleprincipal.Config{Listen: "127.0.0.1:0", CertFile: certFile, KeyFile: keyFile, ClientCAFile: caFile, JWKSURL: jwks.URL, JWKSCAFile: caFile, RedisAddr: redis.Addr()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	options := append(runtime.ServerOptions(), grpc.ChainUnaryInterceptor(func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		result, callErr := handler(ctx, request)
		if callErr != nil {
			t.Logf("Space %s: %v", info.FullMethod, callErr)
		}
		return result, callErr
	}))
	server := grpc.NewServer(options...)
	spacev1.RegisterSpaceServiceServer(server, &PublicLifecycleSpace{SpaceGRPC: &SpaceGRPC{Store: f.st, Roles: f.roles, OwnershipRoles: f.protectedRoles, PrincipalIssuer: f.spaceIssuer}})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	gatewayBinary := filepath.Join(t.TempDir(), "gateway-fixture.exe")
	compile := exec.CommandContext(ctx, "go", "test", "-tags=spacebanintegration", "-p=1", "-c", "-o", gatewayBinary, ".")
	compile.Dir = filepath.Join(repoRoot(t), "src/backend/gateway")
	output, err = compile.CombinedOutput()
	require.NoError(t, err, "%s", output)
	gateway := exec.CommandContext(ctx, gatewayBinary, "-test.run=^TestSpaceUserBanGatewayProcess$", "-test.timeout=2m")
	gateway.Env = append(os.Environ(), "VOICE_BAN_SPACE_ADDR="+listener.Addr().String(), "VOICE_BAN_CA="+caFile, "VOICE_BAN_CLIENT_CERT="+clientCert, "VOICE_BAN_CLIENT_KEY="+clientKey, "VOICE_BAN_SIGNING_KEY="+signingKey, "VOICE_BAN_ACTOR_ACCOUNT="+ownerAccount.String(), "VOICE_BAN_ACTOR_PROFILE="+owner.String(), "VOICE_BAN_SPACE="+f.space.String(), "VOICE_BAN_TARGET_ACCOUNT="+account.String())
	output, err = gateway.CombinedOutput()
	require.NoError(t, err, "%s", output)
	var bans, members, retiredProjection, audit int
	require.NoError(t, f.st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_bans WHERE space_id=$1 AND account_id=$2`, f.space, account).Scan(&bans))
	require.NoError(t, f.st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='ACTIVE'`, f.space, profile).Scan(&members))
	require.NoError(t, f.st.Pool.QueryRow(ctx, `SELECT count(*) FROM community_roster_members WHERE space_id=$1 AND profile_id=$2 AND revoked_at IS NOT NULL`, f.space, profile).Scan(&retiredProjection))
	require.NoError(t, f.st.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_banned'`, f.space).Scan(&audit))
	require.Equal(t, 1, bans)
	require.Zero(t, members)
	require.Equal(t, 1, retiredProjection, "Ban retires the local Space projection")
	require.Equal(t, 1, audit)
	member, err = f.st.IsSpaceMember(ctx, f.space, profile)
	require.NoError(t, err)
	require.False(t, member, "the banned managed profile must lose Space access")
}
