package grpcsvc

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/botmembershipprincipal"
	"voice/backend/space/internal/store"
)

func TestNativeBotGlobal46ProtectedTransport(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private Linux PostgreSQL18/Redis/mTLS/protected HTTPS JWKS")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), "postgres:18-alpine")
	db.Run(true, "goto", "34")
	admin := &store.SpaceStore{Pool: db.Pool}
	first, err := admin.CreateSpace(ctx, uuid.New(), "Global Bot transport", "", "public")
	require.NoError(t, err)
	other, err := admin.CreateSpace(ctx, uuid.New(), "Global Bot cross Space", "", "public")
	require.NoError(t, err)
	db.Run(true, "up")
	_, err = admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	for _, space := range []uuid.UUID{first.ID, other.ID} {
		require.NoError(t, admin.InitializeNativeMembership(ctx, space))
	}
	st := &store.SpaceStore{Pool: nativeBotServingLogin(t, ctx, db.Pool)}
	certFile, keyFile, caFile, clientCert, roots := nativeBotTLSMaterial(t)
	key := nativeBotFixtureSigner(t, "VOICE_BOT_NATIVE_SIGNER_KEY_FILE")
	nextKey := nativeBotFixtureSigner(t, "VOICE_BOT_NATIVE_NEXT_SIGNER_KEY_FILE")
	keys := []map[string]string{}
	for _, entry := range []struct {
		id  string
		key *rsa.PrivateKey
	}{{"current", key}, {"next", nextKey}} {
		keys = append(keys, map[string]string{"kty": "RSA", "kid": entry.id, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(entry.key.N.Bytes()), "e": "AQAB"})
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}))
	identity, err := tls.LoadX509KeyPair(certFile, keyFile)
	require.NoError(t, err)
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{identity}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	jwks.StartTLS()
	t.Cleanup(jwks.Close)
	redis, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "redis:7-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForLog("Ready to accept connections")}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, redis.Terminate(context.Background())) })
	redisAddr, err := redis.Endpoint(ctx, "")
	require.NoError(t, err)
	dir := t.TempDir()
	clientCertFile, clientKeyFile := filepath.Join(dir, "client.pem"), filepath.Join(dir, "client-key.pem")
	clientKey, err := x509.MarshalPKCS8PrivateKey(clientCert.PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(clientCertFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCert.Certificate[0]}), 0600))
	require.NoError(t, os.WriteFile(clientKeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientKey}), 0600))
	cfg := botmembershipprincipal.Config{ListenAddr: "127.0.0.1:0", CertFile: certFile, KeyFile: keyFile, ClientCAFile: caFile, ReplayAddr: redisAddr, JWKSCAFile: caFile, JWKSClientCertFile: clientCertFile, JWKSClientKeyFile: clientKeyFile, JWKSURLs: map[string]string{"bot": jwks.URL + "/bot"}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second}
	start := func() (*botmembershipprincipal.Runtime, *grpc.Server, net.Listener, spacev1.SpaceServiceClient) {
		runtime, err := botmembershipprincipal.New(ctx, cfg)
		require.NoError(t, err)
		listener, err := net.Listen("tcp", cfg.ListenAddr)
		require.NoError(t, err)
		server := grpc.NewServer(runtime.ServerOptions()...)
		RegisterBotMembershipServer(server, &BotMembershipGRPC{Store: st})
		go func() { _ = server.Serve(listener) }()
		connection, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{clientCert}})))
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = connection.Close()
			server.Stop()
			_ = listener.Close()
			require.NoError(t, runtime.Close())
		})
		return runtime, server, listener, spacev1.NewSpaceServiceClient(connection)
	}
	runtime, server, listener, client := start()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	sign := func(r proto.Message, method, operation string) context.Context {
		hash, err := principal.RequestHash(r)
		require.NoError(t, err)
		token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: operation, RequestHash: hash})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operation))
	}
	bounded := func(ctx context.Context) (context.Context, context.CancelFunc) {
		return context.WithTimeout(ctx, 3*time.Second)
	}
	profile := uuid.NewString()
	add := &spacev1.AddBotMemberRequest{SpaceId: first.ID.String(), ProfileId: profile, InstallationIntent: &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: uuid.NewString(), InstallationId: uuid.NewString(), SpaceId: first.ID.String(), ActorProfileId: profile, Generation: 1}}
	call, stop := bounded(sign(add, spacev1.SpaceService_AddBotMember_FullMethodName, add.InstallationIntent.OperationId))
	_, err = client.AddBotMember(call, add)
	stop()
	require.NoError(t, err)
	cancel := &spacev1.RemoveBotMemberRequest{SpaceId: add.SpaceId, ProfileId: profile, InstallationIntent: proto.Clone(add.InstallationIntent).(*spacev1.BotInstallationIntent), InstallOperationId: add.InstallationIntent.OperationId}
	cancel.InstallationIntent.OperationId = uuid.NewString()
	decision := uuid.NewString()
	rootSHA, frozenSHA := sha256.Sum256([]byte(decision)), sha256.Sum256([]byte(uuid.NewString()))
	request := &spacev1.ApplyBotInstallationRemovalRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), DecisionId: decision, DecisionSha256: rootSHA[:], Purpose: "global_delete", BotId: add.InstallationIntent.BotId, ActorProfileId: profile, SpaceId: add.SpaceId, NativeCancellation: cancel, FrozenScopeSha256: frozenSHA[:]}
	call, stop = bounded(ctx)
	_, err = client.ApplyBotInstallationRemoval(call, request)
	stop()
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	wrong := sign(request, spacev1.SpaceService_RemoveBotMember_FullMethodName, request.OperationId)
	call, stop = bounded(wrong)
	_, err = client.ApplyBotInstallationRemoval(call, request)
	stop()
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	firstToken := sign(request, spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName, request.OperationId)
	call, stop = bounded(firstToken)
	result, err := client.ApplyBotInstallationRemoval(call, request)
	stop()
	require.NoError(t, err)
	require.NotNil(t, result.Receipt)
	receipt := result.Receipt
	require.True(t, receipt.GlobalNativeAddsFenced)
	require.Equal(t, "COMPLETED", receipt.Outcome)
	require.Equal(t, request.OperationId, receipt.OperationId)
	require.Equal(t, request.DecisionId, receipt.DecisionId)
	require.Equal(t, request.BotId, receipt.BotId)
	require.Equal(t, request.ActorProfileId, receipt.ActorProfileId)
	require.Equal(t, request.SpaceId, receipt.SpaceId)
	require.Equal(t, "space", receipt.Participant)
	requestRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	require.NoError(t, err)
	nestedRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(cancel)
	require.NoError(t, err)
	requestSHA, nestedSHA := sha256.Sum256(requestRaw), sha256.Sum256(nestedRaw)
	require.Equal(t, requestSHA[:], receipt.RequestSha256)
	require.Equal(t, nestedSHA[:], receipt.EvidenceSha256)
	call, stop = bounded(firstToken)
	_, err = client.ApplyBotInstallationRemoval(call, request)
	stop()
	require.Equal(t, codes.Unauthenticated, status.Code(err), "actual Redis denies reused global JWT/JTI")
	server.Stop()
	_ = listener.Close()
	require.NoError(t, runtime.Close())
	_, _, _, restarted := start()
	call, stop = bounded(firstToken)
	_, err = restarted.ApplyBotInstallationRemoval(call, request)
	stop()
	require.Equal(t, codes.Unauthenticated, status.Code(err), "listener restart cannot reset shared Redis replay proof")
	call, stop = bounded(sign(request, spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName, request.OperationId))
	saved, err := restarted.ApplyBotInstallationRemoval(call, request)
	stop()
	require.NoError(t, err)
	require.True(t, proto.Equal(receipt, saved.Receipt))
	for _, space := range []string{first.ID.String(), other.ID.String()} {
		next := proto.Clone(add).(*spacev1.AddBotMemberRequest)
		next.SpaceId = space
		next.InstallationIntent.SpaceId = space
		next.InstallationIntent.OperationId = uuid.NewString()
		next.InstallationIntent.InstallationId = uuid.NewString()
		if space == first.ID.String() {
			next.InstallationIntent.Generation = 2
		}
		call, stop = bounded(sign(next, spacev1.SpaceService_AddBotMember_FullMethodName, next.InstallationIntent.OperationId))
		_, err = restarted.AddBotMember(call, next)
		stop()
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
	}
	var exact bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM native_membership_guard.bot_terminals WHERE bot_id=$1 AND decision_id=$2)
 AND EXISTS(SELECT 1 FROM native_membership_guard.bot_removal_receipts WHERE operation_id=$3 AND request_sha256=$4 AND evidence_sha256=$5)
 AND NOT EXISTS(SELECT 1 FROM public.space_members WHERE profile_id=$6) AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`, request.BotId, request.DecisionId, request.OperationId, requestSHA[:], nestedSHA[:], profile).Scan(&exact))
	require.True(t, exact)
	require.NoError(t, redis.Stop(ctx, nil))
	call, stop = bounded(sign(request, spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName, request.OperationId))
	_, err = restarted.ApplyBotInstallationRemoval(call, request)
	stop()
	require.Equal(t, codes.Unauthenticated, status.Code(err), "replay outage cannot return a positive saved global receipt")
}
