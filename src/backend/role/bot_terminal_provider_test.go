//go:build role_bot_terminal

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/grpcsvc"
	"voice/backend/role/internal/principalruntime"
	"voice/backend/role/internal/store"
)

// Test-only provider. All authority and persistence below are the real af1 Role
// handler/store, strict verifier, mTLS listener, official26 migration and Redis.
type terminalProvider struct {
	t                         *testing.T
	ctx                       context.Context
	admin, app                *pgxpool.Pool
	config                    principalruntime.Config
	key                       *rsa.PrivateKey
	ca, clientCert, clientKey []byte
	transport                 credentials.TransportCredentials
	runtime                   *principalruntime.Runtime
	legacy, protected         *grpc.Server
	conn                      *grpc.ClientConn
	client                    rolev1.RoleServiceClient
	address                   string
}

func newTerminalProvider(t *testing.T, ctx context.Context) *terminalProvider {
	t.Helper()
	require.Equal(t, "linux", runtime.GOOS, "private Linux only; never a host test listener")
	f := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join("..", "migrations", "role_db"))
	f.Run(true, "goto", "26")
	name, password := "terminal_"+strings.ReplaceAll(uuid.NewString(), "-", ""), uuid.NewString()
	id := pgx.Identifier{name}.Sanitize()
	_, err := f.Pool.Exec(ctx, "CREATE ROLE "+id+" LOGIN PASSWORD '"+password+"' NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION")
	require.NoError(t, err)
	_, err = f.Pool.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+id+"; GRANT SELECT ON ALL TABLES IN SCHEMA public TO "+id+"; GRANT INSERT ON public.role_bot_global_terminals TO "+id)
	require.NoError(t, err)
	cfg := f.Pool.Config().Copy()
	cfg.ConnConfig.User, cfg.ConnConfig.Password = name, password
	app, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(app.Close)
	var login string
	var privileged bool
	require.NoError(t, app.QueryRow(ctx, "SELECT session_user,rolsuper OR rolbypassrls FROM pg_roles WHERE rolname=session_user").Scan(&login, &privileged))
	require.Equal(t, name, login)
	require.False(t, privileged)

	replay, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "redis:7-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForListeningPort("6379/tcp")}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		require.NoError(t, replay.Terminate(c))
	})
	host, err := replay.Host(ctx)
	require.NoError(t, err)
	port, err := replay.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwk := map[string]string{"kid": "current", "kty": "RSA", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}
	nextJWK := map[string]string{"kid": "next", "kty": "RSA", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(next.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(next.E)).Bytes())}
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{jwk, nextJWK}})
	}))
	t.Cleanup(jwks.Close)
	dir := t.TempDir()
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: jwks.TLS.Certificates[0].Certificate[0]})
	serverDER, err := x509.MarshalPKCS8PrivateKey(jwks.TLS.Certificates[0].PrivateKey)
	require.NoError(t, err)
	serverCert, serverKey, clientCA := filepath.Join(dir, "server.pem"), filepath.Join(dir, "server-key.pem"), filepath.Join(dir, "client.pem")
	require.NoError(t, os.WriteFile(serverCert, ca, 0600))
	require.NoError(t, os.WriteFile(serverKey, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: serverDER}), 0600))
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(18326), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, IsCA: true, BasicConstraintsValid: true}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &clientKey.PublicKey, clientKey)
	require.NoError(t, err)
	clientPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	clientDER, err := x509.MarshalPKCS8PrivateKey(clientKey)
	require.NoError(t, err)
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientDER})
	require.NoError(t, os.WriteFile(clientCA, clientPEM, 0600))
	pair, err := tls.X509KeyPair(clientPEM, clientKeyPEM)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	p := &terminalProvider{t: t, ctx: ctx, admin: f.Pool, app: app, key: key, ca: ca, clientCert: clientPEM, clientKey: clientKeyPEM,
		transport: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{pair}}),
		config:    principalruntime.Config{JWKSURLs: map[string]string{"space": jwks.URL, "gameintegration": jwks.URL, "voice": jwks.URL, "bot": jwks.URL}, RefreshAfter: time.Minute, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: time.Second, ReplayAddr: net.JoinHostPort(host, port.Port()), JWKSCAFile: serverCert, TLSCertFile: serverCert, TLSKeyFile: serverKey, ClientCAFile: clientCA, ListenAddr: "127.0.0.1:0", BotLifecycleEnabled: true}}
	p.start()
	t.Cleanup(p.stop)
	return p
}

func (p *terminalProvider) start() {
	var err error
	p.runtime, err = principalruntime.New(p.ctx, p.config)
	require.NoError(p.t, err)
	p.legacy, p.protected = newRoleGRPCServers(nil, &grpcsvc.RoleGRPC{Store: &store.RoleStore{Pool: p.app}}, p.runtime)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(p.t, err)
	p.address = lis.Addr().String()
	go func() { _ = p.protected.Serve(lis) }()
	p.conn, err = grpc.NewClient(p.address, grpc.WithTransportCredentials(p.transport.Clone()))
	require.NoError(p.t, err)
	p.client = rolev1.NewRoleServiceClient(p.conn)
}

func (p *terminalProvider) stop() {
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
	if p.protected != nil {
		p.protected.Stop()
		p.protected = nil
	}
	if p.legacy != nil {
		p.legacy.Stop()
		p.legacy = nil
	}
	if p.runtime != nil {
		require.NoError(p.t, p.runtime.Close())
		p.runtime = nil
	}
}

func (p *terminalProvider) signed(req proto.Message, method, id, issuer string) context.Context {
	return sourceCallContext(p.t, p.key, issuer, method, id, req)
}

func (p *terminalProvider) register(req *rolev1.RegisterBotGlobalTerminalRequest) *rolev1.RegisterBotGlobalTerminalResponse {
	r, err := p.client.RegisterBotGlobalTerminal(p.signed(req, rolev1.RoleService_RegisterBotGlobalTerminal_FullMethodName, req.DecisionId, "bot"), req)
	require.NoError(p.t, err)
	p.assertReceipt(req, r)
	return r
}

func (p *terminalProvider) assertReceipt(req *rolev1.RegisterBotGlobalTerminalRequest, r *rolev1.RegisterBotGlobalTerminalResponse) {
	t := p.t
	require.NotNil(t, r)
	require.NotNil(t, r.Receipt)
	v := r.Receipt
	require.EqualValues(t, 1, v.ProtocolVersion)
	require.Equal(t, "role", v.Participant)
	require.Equal(t, req.BotId, v.BotId)
	require.Equal(t, req.DecisionId, v.DecisionId)
	require.Equal(t, rolev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_BOT_ACTOR_ROLE_MUTATIONS_FENCED, v.Outcome)
	u, err := uuid.Parse(v.ReceiptId)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, u)
	require.Equal(t, u.String(), v.ReceiptId)
	require.NotNil(t, v.RegisteredAt)
	require.NoError(t, v.RegisteredAt.CheckValid())
	var bot, decision, receipt string
	var at time.Time
	require.NoError(t, p.admin.QueryRow(p.ctx, "SELECT bot_id::text,decision_id::text,receipt_id::text,registered_at FROM role_bot_global_terminals WHERE bot_id=$1", req.BotId).Scan(&bot, &decision, &receipt, &at))
	require.Equal(t, req.BotId, bot)
	require.Equal(t, req.DecisionId, decision)
	require.Equal(t, v.ReceiptId, receipt)
	require.True(t, at.Equal(v.RegisteredAt.AsTime()))
	var n int
	require.NoError(t, p.admin.QueryRow(p.ctx, "SELECT count(*) FROM role_bot_global_terminals WHERE bot_id=$1 OR decision_id=$2", req.BotId, req.DecisionId).Scan(&n))
	require.Equal(t, 1, n)
}

func (p *terminalProvider) assertFence(bot string) {
	// Setup identities are not capabilities. Probe only the existing local kernel;
	// no unsupported BotActor public writer is opened or called as admitted.
	actor, space, installation := uuid.New(), uuid.New(), uuid.New()
	err := p.admin.QueryRow(p.ctx, "SELECT actor_profile_id FROM role_bot_actor_bindings WHERE bot_id=$1", bot).Scan(&actor)
	if err == pgx.ErrNoRows {
		_, err = p.admin.Exec(p.ctx, "INSERT INTO role_bot_actor_bindings(bot_id,actor_profile_id) VALUES($1,$2)", bot, actor)
	}
	require.NoError(p.t, err)
	afterGuard := false
	err = (&store.RoleStore{Pool: p.app}).WithinSpaces(p.ctx, []uuid.UUID{space}, func(s *store.RoleStore) error {
		if err := s.CheckBotInstallationRemovalFence(p.ctx, uuid.MustParse(bot), actor, space, installation); err != nil {
			return err
		}
		afterGuard = true
		return nil
	})
	require.ErrorIs(p.t, err, store.ErrBotRemovalDenied)
	require.False(p.t, afterGuard)
}

func (p *terminalProvider) assertClosedWriters(bot string) {
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: p.key})
	require.NoError(p.t, err)
	space, actor, installation := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, v := range []struct {
		method, scope string
		request       proto.Message
	}{
		{rolev1.RoleService_CreateRole_FullMethodName, "SPACE_MANAGE_ROLES", &rolev1.CreateRoleRequest{SpaceId: space, Name: "must-not-exist"}},
		{rolev1.RoleService_AssignRole_FullMethodName, "MEMBER_ASSIGN_ROLES", &rolev1.AssignRoleRequest{SpaceId: space, ProfileId: actor, RoleId: uuid.NewString()}},
		{rolev1.RoleService_RevokeRole_FullMethodName, "MEMBER_ASSIGN_ROLES", &rolev1.RevokeRoleRequest{SpaceId: space, ProfileId: actor, RoleId: uuid.NewString()}},
	} {
		hash, err := principal.RequestHash(v.request)
		require.NoError(p.t, err)
		id := uuid.NewString()
		token, err := issuer.IssueBotActor(principal.BotActorInput{Audience: "role", RPC: v.method, RequestID: id, RequestHash: hash, BotID: bot, ActorProfileID: actor, SpaceID: space, InstallationID: installation, BotScope: v.scope})
		require.NoError(p.t, err)
		ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(p.ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), 2*time.Second)
		err = p.conn.Invoke(ctx, v.method, v.request, &rolev1.CreateRoleResponse{})
		cancel()
		require.Error(p.t, err, "unsupported BotActor public writer stays closed")
		require.NotEqual(p.t, codes.Unimplemented, status.Code(err), "missing handler is not a denial control")
	}
}

func TestRoleBotTerminalActualProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private actual Role26 PG/Redis")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
	defer cancel()
	p := newTerminalProvider(t, ctx)
	req := &rolev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: uuid.NewString(), DecisionId: uuid.NewString()}
	method := rolev1.RoleService_RegisterBotGlobalTerminal_FullMethodName
	callCtx := p.signed(req, method, req.DecisionId, "bot")
	first, err := p.client.RegisterBotGlobalTerminal(callCtx, req)
	require.NoError(t, err)
	p.assertReceipt(req, first)
	_, err = p.client.RegisterBotGlobalTerminal(callCtx, req)
	require.Error(t, err, "same JWT must not replay")
	require.True(t, proto.Equal(first, p.register(req)), "fresh transport retry returns original durable receipt")
	p.stop()
	p.start()
	_, err = p.client.RegisterBotGlobalTerminal(callCtx, req)
	require.Error(t, err, "original JWT stays spent across service restart through actual Redis")
	require.True(t, proto.Equal(first, p.register(req)), "same database and real Redis survive service reinstantiation")
	for _, r := range []*rolev1.RegisterBotGlobalTerminalRequest{{ProtocolVersion: 1, BotId: uuid.NewString(), DecisionId: req.DecisionId}, {ProtocolVersion: 1, BotId: req.BotId, DecisionId: uuid.NewString()}} {
		_, err = p.client.RegisterBotGlobalTerminal(p.signed(r, method, r.DecisionId, "bot"), r)
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
	}
	_, err = p.client.RegisterBotGlobalTerminal(p.signed(req, method, req.DecisionId, "space"), req)
	require.Error(t, err)
	changed := proto.Clone(req).(*rolev1.RegisterBotGlobalTerminalRequest)
	changed.BotId = uuid.NewString()
	_, err = p.client.RegisterBotGlobalTerminal(p.signed(req, method, req.DecisionId, "bot"), changed)
	require.Error(t, err, "signed digest cannot authorize changed request bytes")
	wrong := proto.Clone(req).(*rolev1.RegisterBotGlobalTerminalRequest)
	wrong.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	_, err = p.client.RegisterBotGlobalTerminal(p.signed(wrong, method, wrong.DecisionId, "bot"), wrong)
	require.Error(t, err)
	raw, err := grpc.NewClient(p.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	c, cancelCall := context.WithTimeout(ctx, time.Second)
	_, err = rolev1.NewRoleServiceClient(raw).RegisterBotGlobalTerminal(c, req)
	cancelCall()
	require.Error(t, err)
	require.NoError(t, raw.Close())
	for _, method := range []string{rolev1.RoleService_CreateRole_FullMethodName, rolev1.RoleService_AssignRole_FullMethodName, rolev1.RoleService_RevokeRole_FullMethodName} {
		_, err = p.client.RegisterBotGlobalTerminal(p.signed(req, method, req.DecisionId, "bot"), req)
		require.Error(t, err, "different mutation method cannot authorize the root RPC")
	}
	var columns []string
	require.NoError(t, p.admin.QueryRow(ctx, "SELECT array_agg(column_name::text ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema='public' AND table_name='role_bot_global_terminals'").Scan(&columns))
	require.Equal(t, []string{"bot_id", "decision_id", "receipt_id", "registered_at"}, columns)
	var count int
	for _, table := range []string{"role_bot_actor_bindings", "role_bot_global_removals", "role_bot_installation_removals", "role_bot_removal_operations", "role_space_lifecycle", "roles"} {
		require.NoError(t, p.admin.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()).Scan(&count))
		require.Zero(t, count, "root-only result has no Space/actor/cleanup effects")
	}
	p.assertFence(req.BotId)
	p.assertClosedWriters(req.BotId)
	p.hold(req, first)
	if dir := os.Getenv("VOICE_BOT_ROLE26_HANDOFF_DIR"); dir != "" {
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, entries, "owned ready, stop, completion and private key files removed before return")
	}
}
