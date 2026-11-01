package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	userv1 "voice.app/voice/user/v1"
	"voice/backend/notification/internal/presence"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

// This fixture starts the actual User entry point, stores and listener wiring.
// miniredis is a Redis protocol fixture, not a production Redis deployment.
// Only User's unrelated Auth deletion dependency is a narrow wire-level stub.
type userPresenceRuntimeFixture struct {
	public               userv1.UserServiceClient
	private              userv1.UserServiceClient
	checker              *presence.GRPCChecker
	signingKey           *rsa.PrivateKey
	accountID, profileID uuid.UUID
	binary, sourceDir    string
	env                  []string
	process              *userPresenceChild
}

type userPresenceChild struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
	once sync.Once
}

// Keep raw child output private and bounded; only fixed categories are returned.
type userPresenceDiagnostic struct {
	mu        sync.Mutex
	data      []byte
	truncated bool
}

func (d *userPresenceDiagnostic) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(p)
	const limit = 16384
	available := limit - len(d.data)
	if len(p) > available {
		p = p[:available]
		d.truncated = true
	}
	d.data = append(d.data, p...)
	return n, nil
}

func (d *userPresenceDiagnostic) summary() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	category := "unclassified startup failure"
	switch {
	case bytes.Contains(d.data, []byte("address already in use")):
		category = "listener address in use"
	case bytes.Contains(d.data, []byte("connection refused")):
		category = "dependency connection refused"
	case bytes.Contains(d.data, []byte("x509:")):
		category = "certificate verification failure"
	case bytes.Contains(d.data, []byte("context deadline exceeded")):
		category = "dependency deadline exceeded"
	}
	if d.truncated {
		category += " (output truncated)"
	}
	return category
}

var errUserPresenceExited = errors.New("User child exited before readiness")

func waitUserPresenceReady(ctx context.Context, done <-chan struct{}, interval time.Duration, probe func(context.Context) bool) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return errUserPresenceExited
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if probe(ctx) {
			select {
			case <-done:
				return errUserPresenceExited
			case <-ctx.Done():
				return ctx.Err()
			default:
				return nil
			}
		}
		select {
		case <-done:
			return errUserPresenceExited
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (p *userPresenceChild) stop(t *testing.T) {
	t.Helper()
	p.once.Do(func() {
		select {
		case <-p.done:
			t.Errorf("User child exited before owned shutdown: %v", p.err)
			return
		default:
		}
		_ = p.cmd.Process.Signal(os.Interrupt)
		select {
		case <-p.done:
			require.NoError(t, p.err, "actual User shuts down on interrupt")
		case <-time.After(10 * time.Second):
			_ = p.cmd.Process.Kill()
			select {
			case <-p.done:
			case <-time.After(5 * time.Second):
				t.Error("owned User child did not join after kill")
			}
			t.Error("actual User exceeded graceful shutdown deadline")
		}
	})
}

func userPresenceAddress(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}

func newUserPresenceRuntimeFixture(t *testing.T) *userPresenceRuntimeFixture {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userpresenceproof", "") // skips short before building any child
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	userDir := filepath.Join(filepath.Dir(source), "..", "user")
	repoRoot := filepath.Clean(filepath.Join(userDir, "..", "..", ".."))
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, repoRoot)
	f := &userPresenceRuntimeFixture{accountID: uuid.New(), profileID: uuid.New(), sourceDir: userDir}
	_, err := pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'presenceproof','0001','Presence proof')`, f.profileID, f.accountID)
	require.NoError(t, err)
	redis := miniredis.RunT(t)
	authAddr := startUserPresenceAuthDeletionStub(t, f.accountID)

	temp := t.TempDir()
	keysDir := filepath.Join(temp, "signing")
	require.NoError(t, os.Mkdir(keysDir, 0700))
	for _, kid := range []string{"current", "next"} {
		key, keyErr := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, keyErr)
		encoded, marshalErr := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, marshalErr)
		require.NoError(t, os.WriteFile(filepath.Join(keysDir, kid+".pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0600))
		if kid == "current" {
			f.signingKey = key
		}
	}
	certFile, keyFile, roots := userPresenceTLSFiles(t, temp)
	publicAddr, privateAddr, httpAddr := userPresenceAddress(t), userPresenceAddress(t), userPresenceAddress(t)
	jwksListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = jwksListener.Close() })
	config := map[string]string{
		"NOTIFICATION_PRINCIPAL_SIGNING_KEYS_DIR": keysDir, "NOTIFICATION_PRINCIPAL_ACTIVE_KID": "current",
		"NOTIFICATION_PRINCIPAL_JWKS_LISTEN":        jwksListener.Addr().String(),
		"NOTIFICATION_PRINCIPAL_JWKS_TLS_CERT_FILE": certFile, "NOTIFICATION_PRINCIPAL_JWKS_TLS_KEY_FILE": keyFile,
		"USER_NOTIFICATION_PRINCIPAL_GRPC_ADDR": privateAddr, "USER_NOTIFICATION_PRINCIPAL_TLS_CA_FILE": certFile,
		"USER_NOTIFICATION_PRINCIPAL_TLS_SERVER_NAME": "localhost",
	}
	checker, jwks, err := presence.LoadAuthenticatedGRPCCheckerFromEnv(func(k string) (string, bool) { v, found := config[k]; return v, found })
	require.NoError(t, err)
	f.checker = checker
	t.Cleanup(func() { require.NoError(t, checker.Close()) })
	jwksDone := make(chan error, 1)
	go func() { jwksDone <- jwks.Server.ServeTLS(jwksListener, jwks.CertFile, jwks.KeyFile) }()
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, jwks.Server.Shutdown(shutdownCtx))
		select {
		case serveErr := <-jwksDone:
			require.ErrorIs(t, serveErr, http.ErrServerClosed)
		case <-time.After(5 * time.Second):
			t.Error("Notification JWKS server did not join")
		}
	})
	urls, err := json.Marshal(map[string]string{"notification": "https://" + jwksListener.Addr().String() + "/.well-known/jwks.json"})
	require.NoError(t, err)
	// Inherit only OS execution essentials: no external service or NATS settings.
	for _, name := range []string{"PATH", "HOME", "TMPDIR", "TMP", "TEMP", "SYSTEMROOT"} {
		if value, found := os.LookupEnv(name); found {
			f.env = append(f.env, name+"="+value)
		}
	}
	settings := map[string]string{
		"VOICE_RELEASE_PROFILE": "development",
		"DATABASE_URL":          pool.Config().ConnConfig.ConnString(), "USER_GRPC_ADDR": publicAddr, "LISTEN_ADDR": httpAddr,
		"AUTH_GRPC_ADDR": authAddr, "USER_REDIS_ADDR": redis.Addr(),
		"USER_NOTIFICATION_PRINCIPAL_REPLAY_REDIS_ADDR": redis.Addr(),
		"USER_NOTIFICATION_PRINCIPAL_TLS_CERT_FILE":     certFile, "USER_NOTIFICATION_PRINCIPAL_TLS_KEY_FILE": keyFile,
		"USER_NOTIFICATION_PRINCIPAL_GRPC_LISTEN": privateAddr,
		"S2S_JWKS_CA_FILE":                        certFile, "S2S_JWKS_URLS_JSON": string(urls),
	}
	for k, v := range settings {
		f.env = append(f.env, k+"="+v)
	}
	f.binary = filepath.Join(temp, "user-presence-proof")
	if runtime.GOOS == "windows" {
		f.binary += ".exe"
	}
	buildCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-mod=readonly", "-o", f.binary, ".")
	build.Dir = userDir
	output, err := build.CombinedOutput()
	if len(output) > 2000 {
		output = output[len(output)-2000:]
	}
	require.NoError(t, err, "build actual User entry point: %s", output)
	publicConn, err := grpc.NewClient(publicAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, publicConn.Close()) })
	f.public = userv1.NewUserServiceClient(publicConn)
	privateConn, err := grpc.NewClient(privateAddr, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{RootCAs: roots, ServerName: "localhost", MinVersion: tls.VersionTLS12})))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, privateConn.Close()) })
	f.private = userv1.NewUserServiceClient(privateConn)
	f.start(t)
	return f
}

func (f *userPresenceRuntimeFixture) start(t *testing.T) {
	t.Helper()
	cmd := exec.Command(f.binary)
	cmd.Dir, cmd.Env = f.sourceDir, f.env
	// Never emit child output, environment, or generated key material.
	diagnostic := &userPresenceDiagnostic{}
	cmd.Stdout, cmd.Stderr = diagnostic, diagnostic
	require.NoError(t, cmd.Start())
	p := &userPresenceChild{cmd: cmd, done: make(chan struct{})}
	f.process = p
	go func() { p.err = cmd.Wait(); close(p.done) }()
	t.Cleanup(func() { p.stop(t) })
	readyCtx, cancelReady := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReady()
	err := waitUserPresenceReady(readyCtx, p.done, 50*time.Millisecond, func(ctx context.Context) bool {
		callCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		_, err := f.public.GetBulkPresence(callCtx, &userv1.GetBulkPresenceRequest{ProfileIds: []string{f.profileID.String()}})
		if err != nil {
			return false
		}
		_, err = f.checker.IsOnline(callCtx, f.profileID)
		return err == nil
	})
	require.NoError(t, err, "actual User public store and TLS/principal routing path become ready: %s", diagnostic.summary())
}

func userPresenceTLSFiles(t *testing.T, dir string) (string, string, *x509.CertPool) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "presence-proof"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	certPath, keyPath := filepath.Join(dir, "tls-cert.pem"), filepath.Join(dir, "tls-key.pem")
	require.NoError(t, os.WriteFile(certPath, cert, 0600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0600))
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(cert))
	return certPath, keyPath, roots
}

func startUserPresenceAuthDeletionStub(t *testing.T, accountID uuid.UUID) string {
	t.Helper()
	// Auth lives in another module. Match its one required proto wire contract
	// without importing User internals or changing Notification dependencies.
	field := func(name string) *descriptorpb.FieldDescriptorProto {
		return &descriptorpb.FieldDescriptorProto{Name: proto.String(name), Number: proto.Int32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()}
	}
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{Name: proto.String("presence-proof-auth.proto"), Package: proto.String("voice.auth.v1"), Syntax: proto.String("proto3"), MessageType: []*descriptorpb.DescriptorProto{
		{Name: proto.String("FilterDeletedAccountIDsRequest"), Field: []*descriptorpb.FieldDescriptorProto{field("account_ids")}},
		{Name: proto.String("FilterDeletedAccountIDsResponse"), Field: []*descriptorpb.FieldDescriptorProto{field("deleted_account_ids")}},
	}}, nil)
	require.NoError(t, err)
	server := grpc.NewServer()
	server.RegisterService(&grpc.ServiceDesc{ServiceName: "voice.auth.v1.AuthService", HandlerType: (*interface{})(nil), Methods: []grpc.MethodDesc{{MethodName: "FilterDeletedAccountIDs", Handler: func(_ any, ctx context.Context, decode func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
		req := dynamicpb.NewMessage(file.Messages().Get(0))
		if err := decode(req); err != nil {
			return nil, err
		}
		md, _ := metadata.FromIncomingContext(ctx)
		if strings.Join(md.Get("x-voice-internal-caller"), ",") != "user" {
			return nil, status.Error(codes.PermissionDenied, "User caller required")
		}
		ids := req.Get(req.Descriptor().Fields().ByNumber(1)).List()
		if ids.Len() != 1 || ids.Get(0).String() != accountID.String() {
			return nil, status.Error(codes.InvalidArgument, "fixture account required")
		}
		return dynamicpb.NewMessage(file.Messages().Get(1)), nil
	}}}}, struct{}{})
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- server.Serve(l) }()
	t.Cleanup(func() {
		server.Stop()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Error("Auth dependency stub did not join")
		}
	})
	return l.Addr().String()
}

// Always calls the real checker. Holding its second actual outage result lets
// the test inspect the first explicit NAK redelivery before restarting User.
type userPresenceOutageBarrier struct {
	checker *presence.GRPCChecker
	armed   atomic.Bool
	errors  atomic.Int32
	second  chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *userPresenceOutageBarrier) unblock() { b.once.Do(func() { close(b.release) }) }
func (b *userPresenceOutageBarrier) IsOnline(ctx context.Context, id uuid.UUID) (bool, error) {
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	online, err := b.checker.IsOnline(callCtx, id)
	cancel()
	if err != nil && b.armed.Load() && b.errors.Add(1) == 2 {
		close(b.second)
		select {
		case <-b.release:
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	return online, err
}

func (f *userPresenceRuntimeFixture) signedContext(t *testing.T, issuerName string, bound *userv1.GetNotificationRoutingPresenceRequest) context.Context {
	t.Helper()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuerName, KeyID: "current", PrivateKey: f.signingKey})
	require.NoError(t, err)
	hash, err := principal.RequestHash(bound)
	require.NoError(t, err)
	requestID := uuid.NewString()
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "user", RPC: userv1.UserService_GetNotificationRoutingPresence_FullMethodName, RequestID: requestID, RequestHash: hash})
	require.NoError(t, err)
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
}

func (f *userPresenceRuntimeFixture) setPresence(t *testing.T, state string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-user-id", f.accountID.String(), "x-voice-profile-id", f.profileID.String(), "x-voice-subscription-tier", "premium"))
	_, err := f.public.UpdatePresence(ctx, &userv1.UpdatePresenceRequest{Status: state, GameTitle: proto.String("private game"), CustomStatus: proto.String("private custom"), CallInfoJson: proto.String(`{"call_id":"private-call"}`)})
	require.NoError(t, err, fmt.Sprintf("write %s through actual User", state))
	response, err := f.public.GetPresence(ctx, &userv1.GetPresenceRequest{ProfileId: f.profileID.String()})
	require.NoError(t, err)
	self := response.GetPresenceStatus()
	require.NotNil(t, self)
	require.Equal(t, state, self.GetStatus(), "owner confirms the actual Redis-backed live state before public masking")
	require.Equal(t, "private game", self.GetGameTitle())
	require.Equal(t, "private custom", self.GetCustomStatus())
	require.Equal(t, `{"call_id":"private-call"}`, self.GetCallInfoJson())
	require.NotNil(t, self.GetLastSeen())
}
