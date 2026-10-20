package grpcsvc

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/botmembershipprincipal"
	"voice/backend/space/internal/store"
)

// The clock is private test PostgreSQL only. Go/JWT/Auth fixture clocks stay
// live; the first risk is whether a real ordinary native Add remains valid.
func TestNativeBotP3Provider49(t *testing.T) {
	ctx := context.Background()
	handoff := os.Getenv("VOICE_BOT_P3_HANDOFF_DIR")
	sourcePin := os.Getenv("SOURCE_PIN")
	deadline := time.Now().Add(4 * time.Minute)
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), nativeP3ProviderClockImage(t))
	db.Run(true, "goto", "34")
	admin := &store.SpaceStore{Pool: db.Pool}
	setNativeP3ProviderDBTime(t, admin, time.Now().UTC().Add(-8*24*time.Hour).Truncate(time.Second))
	owner, account := uuid.New(), uuid.New()
	space, err := admin.CreateSpace(ctx, owner, "P3 provider LIVE", "", "private")
	require.NoError(t, err)
	db.Run(true, "goto", "46")
	_, err = admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID))
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog47(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog48(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog49(ctx)
	require.NoError(t, err)
	st := &store.SpaceStore{Pool: nativeBotServingLogin(t, ctx, db.Pool)}
	certFile, keyFile, caFile, clientCert, roots := nativeBotTLSMaterial(t)
	key := nativeBotFixtureSigner(t, "VOICE_BOT_NATIVE_SIGNER_KEY_FILE")
	next := nativeBotFixtureSigner(t, "VOICE_BOT_NATIVE_NEXT_SIGNER_KEY_FILE")
	keys := []map[string]string{}
	for i, k := range []*rsa.PrivateKey{key, next} {
		kid := "current"
		if i == 1 {
			kid = "next"
		}
		keys = append(keys, map[string]string{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(k.N.Bytes()), "e": "AQAB"})
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}))
	serverIdentity, err := tls.LoadX509KeyPair(certFile, keyFile)
	require.NoError(t, err)
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverIdentity}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
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
	if handoff != "" {
		cfg.ListenAddr = "0.0.0.0:0"
	}
	runtime, err := botmembershipprincipal.New(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	server := grpc.NewServer(runtime.ServerOptions()...)
	RegisterBotMembershipServer(server, &BotMembershipGRPC{Store: st})
	t.Cleanup(server.Stop)
	go func() { _ = server.Serve(listener) }()
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	connect := func(certs []tls.Certificate) spacev1.SpaceServiceClient {
		connection, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: certs})))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, connection.Close()) })
		return spacev1.NewSpaceServiceClient(connection)
	}
	client := connect([]tls.Certificate{clientCert})

	profile := uuid.NewString()
	var installed nativeP3Installed
	if handoff != "" {
		nativeP3ExportReady(t, handoff, sourcePin, listener.Addr().String(), space.ID.String(), profile, caFile, clientCertFile, clientKeyFile, key, deadline)
		nativeP3WaitSignal(t, handoff, "installed.json", deadline, &installed)
	} else {
		add := &spacev1.AddBotMemberRequest{SpaceId: space.ID.String(), ProfileId: profile, InstallationIntent: &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: uuid.NewString(), InstallationId: uuid.NewString(), SpaceId: space.ID.String(), ActorProfileId: profile, Generation: 1}}
		hash, err := principal.RequestHash(add)
		require.NoError(t, err)
		token, err := signer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_AddBotMember_FullMethodName, RequestID: add.InstallationIntent.OperationId, RequestHash: hash})
		require.NoError(t, err)
		call := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", add.InstallationIntent.OperationId))
		_, err = client.AddBotMember(call, add)
		require.NoError(t, err, "actual signed Add49 with live Go/JWT clocks")
		installed = nativeP3Installed{SchemaVersion: 1, BotID: add.InstallationIntent.BotId, InstallationID: add.InstallationIntent.InstallationId, InstallOperationID: add.InstallationIntent.OperationId, BotGeneration: 1, Variant: "installed"}
	}
	add := nativeP3VerifyInstalled(t, ctx, admin, st, space.ID.String(), profile, installed)
	decision := nativeP3ProviderDecide(t, ctx, admin, st, space.ID, account, owner, space.Name)
	sign := func(method, operation string, r proto.Message) context.Context {
		h, err := principal.RequestHash(r)
		require.NoError(t, err)
		token, err := signer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: operation, RequestHash: h})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operation))
	}
	read := &spacev1.GetBotPurgeHandoffRequest{ProtocolVersion: 1, RequestId: uuid.NewString(), SpaceId: space.ID.String(), DeletionOperationId: decision.OperationID.String(), Generation: decision.Generation, Manifest: decision.Manifest}
	original, err := client.GetBotPurgeHandoff(sign(spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName, read.RequestId, read), read)
	require.NoError(t, err)
	require.True(t, proto.Equal(decision.Role, original.GetRole().GetOriginalReceipt()))
	require.Nil(t, original.Messaging, "Role-only empty-chat fixture: absence is not Messaging completion")

	if handoff != "" {
		nativeP3ExportDecision(t, handoff, sourcePin, space.ID.String(), decision)
		var complete nativeP3Completed
		nativeP3WaitSignal(t, handoff, "complete.json", deadline, &complete)
		request, receipt := nativeP3VerifyChild(t, ctx, admin, space.ID.String(), profile, installed, decision, complete)
		replay, err := client.ReconcileBotPurgeInstallation(sign(spacev1.SpaceService_ReconcileBotPurgeInstallation_FullMethodName, request.OperationId, request), request)
		require.NoError(t, err)
		require.True(t, proto.Equal(receipt, replay.Receipt))
		nativeP3WriteSignal(t, handoff, "verified.json", map[string]any{"schema_version": 1, "source_pin": sourcePin, "bot_id": installed.BotID, "installation_id": installed.InstallationID, "drain_operation_id": request.OperationId, "native_receipt_id": receipt.ReceiptId, "native_effect_verified": true, "participant_receipt_recorded": false})
		nativeP3WaitSignal(t, handoff, "stop", deadline, nil)
		return
	}
	cancel := &spacev1.RemoveBotMemberRequest{SpaceId: add.SpaceId, ProfileId: profile, InstallOperationId: add.InstallationIntent.OperationId, InstallationIntent: proto.Clone(add.InstallationIntent).(*spacev1.BotInstallationIntent)}
	cancel.InstallationIntent.OperationId = uuid.NewString()
	child := &spacev1.ReconcileBotPurgeInstallationRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), SpaceId: space.ID.String(), DeletionOperationId: decision.OperationID.String(), Generation: decision.Generation, Manifest: decision.Manifest, BotId: add.InstallationIntent.BotId, InstallationId: add.InstallationIntent.InstallationId, NativeCancellation: cancel}
	reconciled, err := client.ReconcileBotPurgeInstallation(sign(spacev1.SpaceService_ReconcileBotPurgeInstallation_FullMethodName, child.OperationId, child), child)
	require.NoError(t, err)
	require.Equal(t, spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_CANCELED, reconciled.Receipt.Disposition)
	replayed, err := client.ReconcileBotPurgeInstallation(sign(spacev1.SpaceService_ReconcileBotPurgeInstallation_FullMethodName, child.OperationId, child), child)
	require.NoError(t, err)
	require.True(t, proto.Equal(reconciled, replayed))

	verifiedRequest, verifiedReceipt := nativeP3VerifyChild(t, ctx, admin, space.ID.String(), profile, installed, decision, nativeP3Completed{SchemaVersion: 1, BotID: installed.BotID, InstallationID: installed.InstallationID, DrainOperationID: child.OperationId, NativeReceiptID: reconciled.Receipt.ReceiptId})
	require.True(t, proto.Equal(child, verifiedRequest))
	require.True(t, proto.Equal(reconciled.Receipt, verifiedReceipt))
}
