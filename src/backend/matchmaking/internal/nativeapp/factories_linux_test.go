//go:build linux

package nativeapp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	jwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"voice/backend/matchmaking/internal/rosterconsume"
	w "voice/backend/pkg/mmadmissionwire"
	"voice/backend/pkg/principal"
)

type factoryTLSFixture struct {
	ca                                           string
	serverCert, serverKey, clientCert, clientKey string
	server, client                               tls.Certificate
	roots                                        *x509.CertPool
}

func nativeFactoryTLS(t *testing.T) factoryTLSFixture {
	t.Helper()
	dir := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "owned-native-factory-ca"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	require.NoError(t, err)
	ca, err = x509.ParseCertificate(der)
	require.NoError(t, err)
	caPath := filepath.Join(dir, "ca.crt")
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	makeLeaf := func(name string, usage x509.ExtKeyUsage) (string, string, tls.Certificate) {
		leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		leaf := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, ExtKeyUsage: []x509.ExtKeyUsage{usage}, KeyUsage: x509.KeyUsageDigitalSignature}
		encoded, err := x509.CreateCertificate(rand.Reader, leaf, ca, &leafKey.PublicKey, key)
		require.NoError(t, err)
		private, err := x509.MarshalPKCS8PrivateKey(leafKey)
		require.NoError(t, err)
		certPath, keyPath := filepath.Join(dir, name+".crt"), filepath.Join(dir, name+".key")
		require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: encoded}), 0600))
		require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), 0600))
		pair, err := tls.LoadX509KeyPair(certPath, keyPath)
		require.NoError(t, err)
		return certPath, keyPath, pair
	}
	sc, sk, server := makeLeaf("server", x509.ExtKeyUsageServerAuth)
	cc, ck, client := makeLeaf("client", x509.ExtKeyUsageClientAuth)
	return factoryTLSFixture{caPath, sc, sk, cc, ck, server, client, roots}
}

func fixtureAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}

func nativeFactoryBroker(t *testing.T, identity factoryTLSFixture) (string, string) {
	t.Helper()
	binary := os.Getenv("BE011_TEST_NATS_SERVER_BINARY")
	if binary == "" {
		t.Skip("owned native NATS server binary unavailable")
	}
	dir := t.TempDir()
	operator, err := nkeys.CreateOperator()
	require.NoError(t, err)
	account, err := nkeys.CreateAccount()
	require.NoError(t, err)
	opPub, _ := operator.PublicKey()
	accountPub, _ := account.PublicKey()
	system, err := nkeys.CreateAccount()
	require.NoError(t, err)
	systemPub, _ := system.PublicKey()
	opClaims := jwt.NewOperatorClaims(opPub)
	opClaims.SystemAccount = systemPub
	opToken, err := opClaims.Encode(operator)
	require.NoError(t, err)
	systemToken, err := jwt.NewAccountClaims(systemPub).Encode(operator)
	require.NoError(t, err)
	claims := jwt.NewAccountClaims(accountPub)
	claims.Limits = jwt.OperatorLimits{AccountLimits: jwt.AccountLimits{Conn: 64}, NatsLimits: jwt.NatsLimits{Subs: 1024, Data: 64 << 20, Payload: 1 << 20}, JetStreamLimits: jwt.JetStreamLimits{MemoryStorage: 64 << 20, DiskStorage: 512 << 20, Streams: 10, Consumer: 16, MaxAckPending: 1024}}
	accountToken, err := claims.Encode(operator)
	require.NoError(t, err)
	user, err := nkeys.CreateUser()
	require.NoError(t, err)
	userPub, _ := user.PublicKey()
	userClaims := jwt.NewUserClaims(userPub)
	userClaims.Expires = time.Now().Add(time.Hour).Unix()
	userClaims.Pub.Allow = []string{">"}
	userClaims.Sub.Allow = []string{">"}
	token, err := userClaims.Encode(account)
	require.NoError(t, err)
	seed, _ := user.Seed()
	creds, err := jwt.FormatUserConfig(token, seed)
	require.NoError(t, err)
	credsPath := filepath.Join(dir, "fixture.creds")
	require.NoError(t, os.WriteFile(credsPath, creds, 0600))
	operatorPath := filepath.Join(dir, "operator.jwt")
	require.NoError(t, os.WriteFile(operatorPath, []byte(opToken), 0600))
	address := fixtureAddress(t)
	config := filepath.Join(dir, "server.conf")
	text := fmt.Sprintf("listen: %q\nserver_name: owned-mm-native\njetstream { store_dir: %q }\noperator: %q\nresolver: MEMORY\nresolver_preload { %s: %q; %s: %q }\ntls { cert_file: %q; key_file: %q; ca_file: %q; verify: true }\n", address, filepath.Join(dir, "data"), operatorPath, accountPub, accountToken, systemPub, systemToken, identity.serverCert, identity.serverKey, identity.ca)
	require.NoError(t, os.WriteFile(config, []byte(text), 0600))
	log, err := os.Create(filepath.Join(dir, "server.log"))
	require.NoError(t, err)
	command := exec.Command(binary, "-c", config)
	command.Stdout = log
	command.Stderr = log
	require.NoError(t, command.Start())
	t.Cleanup(func() {
		_ = command.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- command.Wait() }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = command.Process.Kill()
			<-done
		}
		_ = log.Close()
	})
	options := []nats.Option{nats.UserCredentials(credsPath), nats.Secure(&tls.Config{MinVersion: tls.VersionTLS13, RootCAs: identity.roots, Certificates: []tls.Certificate{identity.client}, ServerName: "localhost"}), nats.Timeout(200 * time.Millisecond)}
	var connection *nats.Conn
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		connection, err = nats.Connect("tls://"+address, options...)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil || connection == nil {
		output, _ := os.ReadFile(filepath.Join(dir, "server.log"))
		t.Fatalf("owned fixture JWT TLS broker unavailable: %v; %s", err, output)
	}
	defer connection.Close()
	js, err := connection.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "matchmaking_events", Subjects: []string{"mm.search_started", "mm.search_cancelled", "mm.match_found", "mm.match_completed", "mm.rating_submitted", "mm.player_banned", "mm.search_nudge", "mm.search_timeout"}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy, MaxAge: 168 * time.Hour})
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: rosterconsume.Stream, Subjects: []string{rosterconsume.Subject}, Storage: nats.FileStorage})
	require.NoError(t, err)
	_, err = js.AddConsumer(rosterconsume.Stream, &nats.ConsumerConfig{Durable: rosterconsume.Durable, FilterSubject: rosterconsume.Subject, DeliverPolicy: nats.DeliverAllPolicy, AckPolicy: nats.AckExplicitPolicy})
	require.NoError(t, err)
	t.Logf("actual %s JWT operator realm + mTLS; broad fixture user only, not installed/canonical ACL qualification", connection.ConnectedServerVersion())
	return "tls://" + address, credsPath
}

func nativeFactoryConfiguration(t *testing.T, register ...func(*grpc.Server, []principal.JWKSKey)) (nativeConfig, *pgxpool.Pool, factoryTLSFixture) {
	t.Helper()
	addr := os.Getenv("BE011_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("owned native Redis unavailable")
	}
	identity := nativeFactoryTLS(t)
	broker, credsPath := nativeFactoryBroker(t, identity)
	keysDir := t.TempDir()
	var public []principal.JWKSKey
	for _, kid := range []string{"current", "next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		private, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(keysDir, kid+".pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), 0600))
		public = append(public, principal.JWKSKey{KeyID: kid, PublicKey: &key.PublicKey})
	}
	// Explicit same-key test upstreams, not independent production issuers.
	jwks := httptest.NewUnstartedServer(principal.JWKSHandlerKeys(public))
	jwks.TLS = &tls.Config{Certificates: []tls.Certificate{identity.server}, MinVersion: tls.VersionTLS13}
	jwks.StartTLS()
	t.Cleanup(jwks.Close)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	upstream := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{identity.server}, ClientCAs: identity.roots, ClientAuth: tls.RequireAndVerifyClientCert, MinVersion: tls.VersionTLS13})))
	for _, configure := range register {
		configure(upstream, public)
	}
	go func() { _ = upstream.Serve(listener) }()
	t.Cleanup(upstream.Stop)
	values := map[string]string{"DATABASE_URL": os.Getenv("BE011_TEST_PG_DSN"), "MATCHMAKING_REDIS_ADDR": addr, "NATS_URL": broker, "NATS_CREDS_FILE": credsPath, "NATS_CA_FILE": identity.ca, "NATS_CLIENT_CERT_FILE": identity.clientCert, "NATS_CLIENT_KEY_FILE": identity.clientKey, "NATS_SERVER_NAME": "localhost", "MATCHMAKING_SPACE_PRINCIPAL_GRPC_LISTEN": "127.0.0.1:0", "MATCHMAKING_SPACE_PRINCIPAL_TLS_CERT_FILE": identity.serverCert, "MATCHMAKING_SPACE_PRINCIPAL_TLS_KEY_FILE": identity.serverKey, "MATCHMAKING_SPACE_PRINCIPAL_CLIENT_CA_FILE": identity.ca, "MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_ADDR": addr, "S2S_JWKS_CA_FILE": identity.ca, "S2S_JWKS_URLS_JSON": fmt.Sprintf(`{"gateway":%q,"space":%q}`, jwks.URL, jwks.URL), "MATCHMAKING_VOICE_PARTY_SNAPSHOT_GRPC_ADDR": listener.Addr().String(), "MATCHMAKING_VOICE_PARTY_SNAPSHOT_CA_FILE": identity.ca, "MATCHMAKING_VOICE_PARTY_SNAPSHOT_CLIENT_CERT_FILE": identity.clientCert, "MATCHMAKING_VOICE_PARTY_SNAPSHOT_CLIENT_KEY_FILE": identity.clientKey, "MATCHMAKING_VOICE_PARTY_SNAPSHOT_SERVER_NAME": "localhost", "MATCHMAKING_MATCH_SQUAD_CHAT_GRPC_ADDR": listener.Addr().String(), "MATCHMAKING_MATCH_SQUAD_VOICE_GRPC_ADDR": listener.Addr().String(), "MATCHMAKING_MATCH_SQUAD_CHAT_CA_FILE": identity.ca, "MATCHMAKING_MATCH_SQUAD_VOICE_CA_FILE": identity.ca, "MATCHMAKING_MATCH_SQUAD_CLIENT_CERT_FILE": identity.clientCert, "MATCHMAKING_MATCH_SQUAD_CLIENT_KEY_FILE": identity.clientKey, "MATCHMAKING_MATCH_SQUAD_CHAT_SERVER_NAME": "localhost", "MATCHMAKING_MATCH_SQUAD_VOICE_SERVER_NAME": "localhost", "MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR": keysDir, "MATCHMAKING_PRINCIPAL_ACTIVE_KID": "current", "MATCHMAKING_PRINCIPAL_JWKS_LISTEN_ADDR": "127.0.0.1:0", "MATCHMAKING_PRINCIPAL_JWKS_TLS_CERT_FILE": identity.serverCert, "MATCHMAKING_PRINCIPAL_JWKS_TLS_KEY_FILE": identity.serverKey}
	pool := nativeDBFixture(t)
	d := descriptorFixture(t)
	d.Binding.SourceFactoryManifestHash, d.Binding.FreshRouteManifestHash = nativeCodeContracts()
	cfg := nativeConfig{descriptor: d, values: values}
	observed, err := observeConfigurationContracts(cfg)
	require.NoError(t, err)
	cfg.descriptor.ConfigContractHash = observed.Config
	cfg.descriptor.CredentialRefsManifestHash = observed.Credentials
	cfg.descriptor.UpstreamContractManifestHash = observed.Upstream
	cfg.descriptor.BrokerContractManifestHash = observed.Broker
	return cfg, pool, identity
}

func TestNativeRealFactoriesPrepareWithoutStartingDomainDelivery(t *testing.T) {
	cfg, pool, identity := nativeFactoryConfiguration(t)
	values := cfg.values
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// Direct closed-bootstrap fixture deliberately has no operational IPC client:
	// successful factory preparation must still be unable to mint OPEN authority.
	runtime, err := prepareFactories(ctx, &closedBootstrap{config: cfg, pool: pool, stamp: w.AdmissionStampV1{}}, nil)
	require.NoError(t, err)
	defer runtime.Close()
	require.NotNil(t, runtime.service.PartySnapshots)
	require.NotNil(t, runtime.service.ResponseValidator)
	require.Same(t, runtime.gate, runtime.provider.Worker.ForwardLifetime)
	require.Same(t, runtime.gate, runtime.projector.ForwardLifetime)
	release, err := runtime.gate.EnterForward(ctx, uuid.New(), [32]byte{1})
	require.Error(t, err)
	require.Nil(t, release)
	var inbox, parties int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_inbox`).Scan(&inbox))
	require.Zero(t, inbox)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM parties`).Scan(&parties))
	require.Zero(t, parties)
	runtime.Close()
	wrong := cfg
	wrong.values = make(map[string]string, len(values))
	for key, value := range values {
		wrong.values[key] = value
	}
	wrong.values["MATCHMAKING_VOICE_PARTY_SNAPSHOT_SERVER_NAME"] = "different.invalid"
	prepared, err := prepareFactories(ctx, &closedBootstrap{config: wrong, pool: pool}, nil)
	require.ErrorIs(t, err, errUnavailable)
	require.Nil(t, prepared, "a different endpoint/trust profile cannot borrow admitted hashes")
	other := nativeFactoryTLS(t)
	changedCert, err := os.ReadFile(other.clientCert)
	require.NoError(t, err)
	changedKey, err := os.ReadFile(other.clientKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(identity.clientCert, changedCert, 0600))
	require.NoError(t, os.WriteFile(identity.clientKey, changedKey, 0600))
	prepared, err = prepareFactories(ctx, &closedBootstrap{config: cfg, pool: pool}, nil)
	require.ErrorIs(t, err, errUnavailable)
	require.Nil(t, prepared, "same paths with replaced valid key pair cannot borrow the credential contract")
}
