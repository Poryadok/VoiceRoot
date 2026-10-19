package grpcsvc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
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

func TestNativeBotProtectedTransport(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated Linux native PostgreSQL/Redis/mTLS")
	}
	ctx := context.Background()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	database.Run(true, "goto", "34")
	st := &store.SpaceStore{Pool: database.Pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Protected native Bot", "", "public")
	require.NoError(t, err)
	database.Run(true, "up")
	certFile, keyFile, caFile, clientCert, roots := nativeBotTLSMaterial(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
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
	handoff := os.Getenv("VOICE_BOT_NATIVE_HANDOFF_DIR")
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
	outgoing := func(req proto.Message, remove bool) context.Context {
		method := spacev1.SpaceService_AddBotMember_FullMethodName
		var op string
		if remove {
			method = spacev1.SpaceService_RemoveBotMember_FullMethodName
			op = req.(*spacev1.RemoveBotMemberRequest).InstallationIntent.OperationId
		} else {
			op = req.(*spacev1.AddBotMemberRequest).InstallationIntent.OperationId
		}
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		token, err := signer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: op, RequestHash: hash})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", op))
	}
	profile := uuid.NewString()
	if handoff != "" {
		host := os.Getenv("VOICE_BOT_NATIVE_ADVERTISED_HOST")
		require.NotEmpty(t, host, "explicit private Docker alias required")
		_, port, err := net.SplitHostPort(listener.Addr().String())
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(handoff, 0700))
		files := map[string]string{"ca_file": caFile, "client_cert_file": clientCertFile, "client_key_file": clientKeyFile}
		fixture := map[string]any{"address": net.JoinHostPort(host, port), "server_name": "localhost", "space_id": space.ID.String(), "profile_id": profile, "key_id": "current"}
		for field, path := range files {
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			dest := filepath.Join(handoff, filepath.Base(path))
			require.NoError(t, os.WriteFile(dest, raw, 0600))
			fixture[field] = dest
		}
		signerFile := filepath.Join(handoff, "bot-signer.pem")
		signerKey, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(signerFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: signerKey}), 0600))
		fixture["signer_key_file"] = signerFile
		raw, err := json.Marshal(fixture)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(handoff, "ready.json.tmp"), raw, 0600))
		require.NoError(t, os.Rename(filepath.Join(handoff, "ready.json.tmp"), filepath.Join(handoff, "ready.json")))
		var expected struct {
			InstallOperationID string `json:"install_operation_id"`
			Generation         uint64 `json:"generation"`
		}
		deadline := time.NewTimer(90 * time.Second)
		defer deadline.Stop()
		tick := time.NewTicker(50 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-deadline.C:
				t.Fatal("bounded Bot producer handoff did not complete")
			case <-tick.C:
				raw, err := os.ReadFile(filepath.Join(handoff, "complete.json"))
				if os.IsNotExist(err) {
					continue
				}
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(raw, &expected))
				require.EqualValues(t, 2, expected.Generation)
				observed, err := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: profile})
				require.NoError(t, err)
				require.Equal(t, spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE, observed.GetOrdinary().GetState())
				require.Equal(t, expected.InstallOperationID, observed.GetOrdinary().GetPublishedBinding().GetOrdinary().GetPublicationIdentity())
				require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_BOT_INSTALL, observed.GetOrdinary().GetPublicationKind())
				var generation, floor uint64
				require.NoError(t, database.Pool.QueryRow(ctx, `SELECT generation,retired_through FROM space_bot_installation_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).Scan(&generation, &floor))
				require.EqualValues(t, 2, generation)
				require.EqualValues(t, 1, floor)
				t.Log("real Bot producer/native Space generation2 and retired predecessor verified")
				return
			}
		}
	}
	add := &spacev1.AddBotMemberRequest{SpaceId: space.ID.String(), ProfileId: profile, InstallationIntent: &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: uuid.NewString(), InstallationId: uuid.NewString(), SpaceId: space.ID.String(), ActorProfileId: profile, Generation: 1}}
	tokenContext := outgoing(add, false)
	_, err = client.AddBotMember(tokenContext, add)
	require.NoError(t, err)
	_, err = client.AddBotMember(tokenContext, add)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "real Redis denies reused JTI")
	_, err = client.AddBotMember(outgoing(add, false), add)
	require.NoError(t, err, "fresh transport retry reuses exact native installation")
	cancel := &spacev1.RemoveBotMemberRequest{SpaceId: add.SpaceId, ProfileId: profile, InstallationIntent: proto.Clone(add.InstallationIntent).(*spacev1.BotInstallationIntent), InstallOperationId: add.InstallationIntent.OperationId}
	cancel.InstallationIntent.OperationId = uuid.NewString()
	_, err = client.RemoveBotMember(outgoing(cancel, true), cancel)
	require.NoError(t, err)
	_, err = client.AddBotMember(outgoing(add, false), add)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "canceled Add cannot activate producer")
	nextAdd := proto.Clone(add).(*spacev1.AddBotMemberRequest)
	nextAdd.InstallationIntent.OperationId = uuid.NewString()
	nextAdd.InstallationIntent.InstallationId = uuid.NewString()
	nextAdd.InstallationIntent.Generation = 2
	_, err = client.AddBotMember(outgoing(nextAdd, false), nextAdd)
	require.NoError(t, err)
	_, err = client.RemoveBotMember(outgoing(cancel, true), cancel)
	require.NoError(t, err)
	observed, err := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: add.SpaceId, ProfileId: profile})
	require.NoError(t, err)
	require.Equal(t, nextAdd.InstallationIntent.OperationId, observed.GetOrdinary().GetPublishedBinding().GetOrdinary().GetPublicationIdentity())
	withoutCert := connect(nil)
	denied, stop := context.WithTimeout(outgoing(nextAdd, false), time.Second)
	defer stop()
	_, err = withoutCert.AddBotMember(denied, nextAdd)
	require.Error(t, err)
	_, err = client.GetSpace(outgoing(nextAdd, false), &spacev1.GetSpaceRequest{SpaceId: add.SpaceId})
	require.NotEqual(t, codes.OK, status.Code(err), "dedicated server exposes no ordinary method")
	require.NoError(t, redis.Stop(ctx, nil))
	failed, stopFailed := context.WithTimeout(outgoing(nextAdd, false), 5*time.Second)
	defer stopFailed()
	_, err = client.AddBotMember(failed, nextAdd)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "real Redis outage cannot grant native membership")
}

func nativeBotTLSMaterial(t *testing.T) (string, string, string, tls.Certificate, *x509.CertPool) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "botmembership-test-ca"}, IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600))
	issue := func(name string, serial int64, usage x509.ExtKeyUsage) (string, string, tls.Certificate) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
			KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter}
		der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
		require.NoError(t, err)
		pk, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		certFile, keyFile := filepath.Join(dir, name+".pem"), filepath.Join(dir, name+"-key.pem")
		require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
		require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk}), 0600))
		pair, err := tls.LoadX509KeyPair(certFile, keyFile)
		require.NoError(t, err)
		return certFile, keyFile, pair
	}
	certFile, keyFile, _ := issue("server", 2, x509.ExtKeyUsageServerAuth)
	_, _, client := issue("client", 3, x509.ExtKeyUsageClientAuth)
	parsedCA, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(parsedCA)
	return certFile, keyFile, caFile, client, roots
}
