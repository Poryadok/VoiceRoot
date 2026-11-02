package sourcecustody

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

// Test subprocess entry. Stdin closure cancels the service on Windows; this
// exercises orderly ServeClosed cleanup, not the CLI's OS signal integration.
func TestCustodyProcessChild(t *testing.T) {
	if os.Getenv("VOICE_CUSTODY_TEST_CHILD") != "1" {
		return
	}
	cfg, err := LoadServerConfig(os.LookupEnv)
	if err != nil {
		t.Fatal("private fixture configuration refused")
	}
	cfg.testAllowSyntheticOpen = os.Getenv("VOICE_CUSTODY_TEST_SYNTHETIC") == "1"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _, _ = io.Copy(io.Discard, os.Stdin); cancel() }()
	err = ServeClosed(ctx, cfg, func(addr string) { fmt.Println("READY " + addr) })
	if err != nil {
		t.Fatal("private fixture controller refused")
	}
}

type processPKI struct {
	caFile, serverCert, serverKey string
	client                        tls.Certificate
	roots                         *x509.CertPool
}

func processTLS(t *testing.T, dir, label string) processPKI {
	t.Helper()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	ca, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)
	issue := func(serial int64, usage x509.ExtKeyUsage) (tls.Certificate, []byte, []byte) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
		der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
		require.NoError(t, err)
		pk, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk})
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		require.NoError(t, err)
		return cert, certPEM, keyPEM
	}
	_, serverPEM, serverKey := issue(2, x509.ExtKeyUsageServerAuth)
	client, _, _ := issue(3, x509.ExtKeyUsageClientAuth)
	caPath, certPath, keyPath := filepath.Join(dir, label+"-ca.pem"), filepath.Join(dir, label+"-server.pem"), filepath.Join(dir, label+"-server.key")
	for path, data := range map[string][]byte{caPath: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), certPath: serverPEM, keyPath: serverKey} {
		require.NoError(t, os.WriteFile(path, data, 0600))
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	return processPKI{caFile: caPath, serverCert: certPath, serverKey: keyPath, client: client, roots: roots}
}

// TLS proxy supplies an encrypted Redis connection; miniredis remains a
// behavioral fixture and does not replace hosted real-Redis evidence.
func tlsRedisProxy(t *testing.T, pki processPKI, upstream string) string {
	t.Helper()
	cert, err := tls.LoadX509KeyPair(pki.serverCert, pki.serverKey)
	require.NoError(t, err)
	lis, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = lis.Close() })
	go func() {
		for {
			incoming, err := lis.Accept()
			if err != nil {
				return
			}
			go func() {
				defer incoming.Close()
				outgoing, err := net.DialTimeout("tcp", upstream, time.Second)
				if err != nil {
					return
				}
				defer outgoing.Close()
				done := make(chan struct{})
				go func() { _, _ = io.Copy(outgoing, incoming); close(done) }()
				_, _ = io.Copy(incoming, outgoing)
				_ = incoming.Close()
				_ = outgoing.Close()
				<-done
			}()
		}
	}()
	return lis.Addr().String()
}

func TestCustodyProcessStartsClosedAndRejectsSignedStream(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private TLS PostgreSQL and child process")
	}
	ctx := context.Background()
	dir := t.TempDir()
	integrationtest.ConfigureDockerTesting()
	pgTLS := processTLS(t, dir, "pg")
	pgConfig := filepath.Join(dir, "postgres.conf")
	require.NoError(t, os.WriteFile(pgConfig, []byte("listen_addresses='*'\nssl=on\nssl_ca_file='/tmp/testcontainers-go/postgres/ca_cert.pem'\nssl_cert_file='/tmp/testcontainers-go/postgres/server.cert'\nssl_key_file='/tmp/testcontainers-go/postgres/server.key'\n"), 0600))
	// WithSSLCert's fixture entrypoint requires shell pipefail, supported by
	// the upstream postgres Alpine image (Debian /bin/sh is dash).
	pg, err := postgres.Run(ctx, "postgres:16-alpine", postgres.BasicWaitStrategies(), postgres.WithDatabase("custody_test"), postgres.WithUsername("fixture"), postgres.WithPassword("fixture_only"), postgres.WithConfigFile(pgConfig), postgres.WithSSLCert(pgTLS.caFile, pgTLS.serverCert, pgTLS.serverKey))
	if pg != nil {
		t.Cleanup(func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			require.NoError(t, pg.Terminate(cleanup))
		})
	}
	require.NoError(t, err)
	raw, err := pg.ConnectionString(ctx)
	require.NoError(t, err)
	u, err := url.Parse(raw)
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(u.Host)
	require.NoError(t, err)
	u.Host = net.JoinHostPort("127.0.0.1", port)
	q := u.Query()
	q.Set("sslmode", "verify-full")
	q.Set("sslrootcert", pgTLS.caFile)
	u.RawQuery = q.Encode()
	pool, err := pgxpool.New(ctx, u.String())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx))
	guarded, err := OpenUserSourcePool(ctx, u.String())
	require.NoError(t, err)
	guarded.Close()
	unsafeURL := *u
	unsafeQuery := unsafeURL.Query()
	unsafeQuery.Set("sslmode", "disable")
	unsafeURL.RawQuery = unsafeQuery.Encode()
	_, err = OpenUserSourcePool(ctx, unsafeURL.String())
	require.Error(t, err, "capture source must use verified TLS")
	binding := Binding{Epoch: 1, SourceID: uuid.NewString(), HistoryGeneration: uuid.New(), UserSourceGeneration: uuid.New(), KeysetDigest: [32]byte{1}}
	_, err = pool.Exec(ctx, `CREATE TABLE user_custody_state(singleton boolean PRIMARY KEY CHECK(singleton),state text NOT NULL,epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO user_custody_state VALUES(true,'OPEN',$1,$2,$3,$4,$5)`, binding.Epoch, binding.SourceID, binding.HistoryGeneration, binding.UserSourceGeneration, binding.KeysetDigest[:])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE TABLE user_custody_physical_source(source_id text NOT NULL,epoch bigint NOT NULL,system_identifier text NOT NULL,database_name text NOT NULL,database_oid oid NOT NULL,PRIMARY KEY(source_id,epoch))`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO user_custody_physical_source VALUES($1,$2,'123456789','source_fixture',7654)`, binding.SourceID, binding.Epoch)
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	public := func(kid string, k *rsa.PublicKey) map[string]string {
		return map[string]string{"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(k.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.E)).Bytes())}
	}
	doc, err := json.Marshal(map[string]any{"keys": []any{public("current", &key.PublicKey), public("next", &next.PublicKey)}})
	require.NoError(t, err)
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(doc) }))
	t.Cleanup(jwks.Close)
	jwksCA := filepath.Join(dir, "jwks-ca.pem")
	require.NoError(t, os.WriteFile(jwksCA, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: jwks.Certificate().Raw}), 0600))
	replay := miniredis.RunT(t)
	redisTLS := processTLS(t, dir, "redis")
	redisAddr := tlsRedisProxy(t, redisTLS, replay.Addr())
	serverTLS := processTLS(t, dir, "grpc")
	env := map[string]string{"CUSTODY_CONTROL_DATABASE_URL": u.String(), "CUSTODY_LISTEN_ADDR": "127.0.0.1:0", "CUSTODY_TLS_CERT_FILE": serverTLS.serverCert, "CUSTODY_TLS_KEY_FILE": serverTLS.serverKey, "CUSTODY_TLS_CLIENT_CA_FILE": serverTLS.caFile, "CUSTODY_USER_JWKS_URL": jwks.URL, "CUSTODY_USER_JWKS_CA_FILE": jwksCA, "CUSTODY_REDIS_ADDR": redisAddr, "CUSTODY_REDIS_TLS_CA_FILE": redisTLS.caFile, "CUSTODY_REDIS_TLS_SERVER_NAME": "localhost", "CUSTODY_EPOCH": "1", "CUSTODY_SOURCE_ID": binding.SourceID, "CUSTODY_HISTORY_GENERATION": binding.HistoryGeneration.String(), "CUSTODY_USER_SOURCE_GENERATION": binding.UserSourceGeneration.String(), "CUSTODY_KEYSET_DIGEST": fmt.Sprintf("%x", binding.KeysetDigest), "CUSTODY_HEARTBEAT": "100ms", "VOICE_CUSTODY_TEST_CHILD": "1"}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCustodyProcessChild$")
	for _, item := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(item), "PG") && !strings.HasPrefix(item, "CUSTODY_") {
			cmd.Env = append(cmd.Env, item)
		}
	}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	cmd.Stderr = io.Discard
	require.NoError(t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	joined := false
	t.Cleanup(func() {
		_ = stdin.Close()
		if !joined {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
				<-done
			}
		}
	})
	lines := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			lines <- scanner.Text()
		} else {
			close(lines)
		}
	}()
	var address string
	select {
	case line := <-lines:
		require.True(t, strings.HasPrefix(line, "READY "), "child did not publish READY")
		address = strings.TrimPrefix(line, "READY ")
	case err := <-done:
		joined = true
		t.Fatalf("child exited before readiness: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("child did not become ready")
	}
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: serverTLS.roots, ServerName: "localhost", Certificates: []tls.Certificate{serverTLS.client}})))
	require.NoError(t, err)
	defer conn.Close()
	req := &userv1.AcquireUserSourcePermitRequest{Protocol: 2, Binding: &userv1.UserSourcePermitBinding{Epoch: uint64(binding.Epoch), SourceId: binding.SourceID, HistoryGeneration: binding.HistoryGeneration.String(), UserSourceGeneration: binding.UserSourceGeneration.String(), KeysetDigest: binding.KeysetDigest[:]}}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "user", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	legacy := &userv1.AcquireUserSourcePermitRequest{Protocol: 1, Binding: req.Binding}
	legacyHash, err := principal.RequestHash(legacy)
	require.NoError(t, err)
	legacyToken, err := issuer.IssueService(principal.ServiceInput{Audience: "user-source-custody", RPC: userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName, RequestID: "legacy-fixture", RequestHash: legacyHash})
	require.NoError(t, err)
	legacyCtx, legacyCancel := context.WithTimeout(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+legacyToken, "x-request-id", "legacy-fixture")), 5*time.Second)
	legacyStream, err := userv1.NewUserSourceCustodyServiceClient(conn).AcquireUserSourcePermit(legacyCtx, legacy)
	require.NoError(t, err)
	_, err = legacyStream.Recv()
	legacyCancel()
	require.Error(t, err, "the production listener must reject lock-only grants")
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "user-source-custody", RPC: userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName, RequestID: "fixture-request", RequestHash: hash})
	require.NoError(t, err)
	call, cancel := context.WithTimeout(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "fixture-request")), 5*time.Second)
	defer cancel()
	stream, err := userv1.NewUserSourceCustodyServiceClient(conn).AcquireUserSourcePermit(call, req)
	require.NoError(t, err)
	_, err = stream.Recv()
	require.Error(t, err)
	require.NotEmpty(t, replay.Keys(), "valid signed request must consume replay before CLOSED denial")
	_, err = pool.Exec(ctx, `UPDATE user_custody_state SET state='OPEN' WHERE singleton`)
	require.NoError(t, err)
	directID := "direct-open-is-not-attestation"
	directToken, err := issuer.IssueService(principal.ServiceInput{Audience: "user-source-custody", RPC: userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName, RequestID: directID, RequestHash: hash})
	require.NoError(t, err)
	directCtx, directCancel := context.WithTimeout(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+directToken, "x-request-id", directID)), 5*time.Second)
	directStream, err := userv1.NewUserSourceCustodyServiceClient(conn).AcquireUserSourcePermit(directCtx, req)
	require.NoError(t, err)
	_, err = directStream.Recv()
	directCancel()
	require.Error(t, err, "a SQL OPEN must not authorize the production listener")
	require.NoError(t, stdin.Close())
	select {
	case err := <-done:
		joined = true
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("controller failed to shut down")
	}
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	var leaders int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND mode='ExclusiveLock' AND granted AND objsubid=1 AND classid=(6219270163691208961::bigint >> 32)::oid AND objid=(6219270163691208961::bigint & 4294967295)::oid`).Scan(&leaders))
	require.Zero(t, leaders)

	t.Run("configured-operator-listener", func(t *testing.T) {
		var controlSystem string
		require.NoError(t, pool.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&controlSystem))
		_, err := pool.Exec(ctx, `CREATE TABLE user_custody_creation(singleton boolean PRIMARY KEY,phase text,source_id text,epoch bigint,control_system_id text,source_system_id text,database_name text,database_oid oid,history_generation uuid,user_source_generation uuid,keyset_digest bytea)`)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO user_custody_creation VALUES(true,'INITIALIZED_CLOSED',$1,$2,$3,'123456789','source_fixture',7654,$4,$5,$6)`, binding.SourceID, binding.Epoch, controlSystem, binding.HistoryGeneration, binding.UserSourceGeneration, binding.KeysetDigest[:])
		require.NoError(t, err)
		operatorTLS := processTLS(t, dir, "operator-process")
		public, _, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		publicDER, err := x509.MarshalPKIXPublicKey(public)
		require.NoError(t, err)
		publicPath := filepath.Join(dir, "operator-public.pem")
		require.NoError(t, os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0600))
		key := make([]byte, 32)
		for i := range key {
			key[i] = 0x71
		}
		keyPath := filepath.Join(dir, "identity-key")
		require.NoError(t, os.WriteFile(keyPath, key, 0600))
		fingerprint := sha256.Sum256(key)
		manifest, err := json.Marshal(map[string]any{"purpose": "voice.user.identity-association.v1", "revision": 1, "active_version": 1, "versions": []any{map[string]any{"version": 1, "sha256": fmt.Sprintf("%x", fingerprint), "file": "identity-key"}}})
		require.NoError(t, err)
		manifestPath := filepath.Join(dir, "identity-manifest.json")
		require.NoError(t, os.WriteFile(manifestPath, manifest, 0600))
		reserved, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		operatorAddress := reserved.Addr().String()
		require.NoError(t, reserved.Close())
		operatorEnv := map[string]string{"CUSTODY_OPERATOR_LISTEN": operatorAddress, "CUSTODY_OPERATOR_TLS_CERT_FILE": operatorTLS.serverCert, "CUSTODY_OPERATOR_TLS_KEY_FILE": operatorTLS.serverKey, "CUSTODY_OPERATOR_CLIENT_CA_FILE": operatorTLS.caFile, "CUSTODY_OPERATOR_CLIENT_CERT_SHA256": fmt.Sprintf("%x", sha256.Sum256(operatorTLS.client.Certificate[0])), "CUSTODY_OPERATOR_SIGNER_PUBLIC_KEY_FILE": publicPath, "CUSTODY_SOURCE_ADMIN_DATABASE_URL": u.String(), "CUSTODY_SOURCE_DATABASE_URL": u.String(), "CUSTODY_USER_KEY_MANIFEST": manifestPath, "CUSTODY_USER_KEYS_DIR": dir}
		cfg, err := LoadServerConfig(func(name string) (string, bool) {
			if value, ok := operatorEnv[name]; ok {
				return value, true
			}
			value, ok := env[name]
			return value, ok
		})
		require.NoError(t, err)
		serverCtx, stop := context.WithCancel(ctx)
		defer stop()
		ready := make(chan string, 1)
		finished := make(chan error, 1)
		go func() { finished <- ServeClosed(serverCtx, cfg, func(addr string) { ready <- addr }) }()
		select {
		case <-ready:
		case err := <-finished:
			t.Fatalf("server failed before readiness: %v", err)
		case <-time.After(15 * time.Second):
			t.Fatal("server did not become ready")
		}
		transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: operatorTLS.roots, Certificates: []tls.Certificate{operatorTLS.client}}}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
		response, err := client.Get("https://" + operatorAddress + "/challenge")
		require.NoError(t, err)
		var challenge activationChallenge
		require.NoError(t, json.NewDecoder(response.Body).Decode(&challenge))
		require.NoError(t, response.Body.Close())
		require.Equal(t, binding, challenge.Binding)
		require.NotEqual(t, [32]byte{}, challenge.Challenge)
		stop()
		select {
		case err := <-finished:
			require.NoError(t, err)
		case <-time.After(15 * time.Second):
			t.Fatal("server did not stop")
		}
	})

	t.Run("crash-restart-closes-stale-open", func(t *testing.T) {
		// Only this private fixture directly writes OPEN to create an active
		// stream. No production opener or source qualification is implied.
		start := func() (string, io.WriteCloser, *exec.Cmd, <-chan error, *bool) {
			child := exec.Command(os.Args[0], "-test.run=^TestCustodyProcessChild$")
			for _, item := range os.Environ() {
				if !strings.HasPrefix(strings.ToUpper(item), "PG") && !strings.HasPrefix(item, "CUSTODY_") {
					child.Env = append(child.Env, item)
				}
			}
			for k, v := range env {
				child.Env = append(child.Env, k+"="+v)
			}
			child.Env = append(child.Env, "VOICE_CUSTODY_TEST_SYNTHETIC=1")
			output, err := child.StdoutPipe()
			require.NoError(t, err)
			input, err := child.StdinPipe()
			require.NoError(t, err)
			child.Stderr = io.Discard
			require.NoError(t, child.Start())
			done := make(chan error, 1)
			go func() { done <- child.Wait() }()
			joined := false
			t.Cleanup(func() {
				_ = input.Close()
				if !joined {
					select {
					case <-done:
					case <-time.After(5 * time.Second):
						_ = child.Process.Kill()
						<-done
					}
				}
			})
			line := make(chan string, 1)
			go func() {
				scanner := bufio.NewScanner(output)
				if scanner.Scan() {
					line <- scanner.Text()
				} else {
					close(line)
				}
			}()
			select {
			case value := <-line:
				require.True(t, strings.HasPrefix(value, "READY "))
				return strings.TrimPrefix(value, "READY "), input, child, done, &joined
			case err := <-done:
				joined = true
				t.Fatalf("child exited before READY: %v", err)
			case <-time.After(10 * time.Second):
				t.Fatal("child readiness timed out")
			}
			return "", nil, nil, nil, nil
		}
		_, err := pool.Exec(ctx, `UPDATE user_custody_state SET state='OPEN' WHERE singleton`)
		require.NoError(t, err)
		address, input, child, done, joined := start()
		require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
		require.Equal(t, "CLOSED", state)
		_, err = pool.Exec(ctx, `UPDATE user_custody_state SET state='OPEN' WHERE singleton`)
		require.NoError(t, err)
		transport, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: serverTLS.roots, ServerName: "localhost", Certificates: []tls.Certificate{serverTLS.client}})))
		require.NoError(t, err)
		defer transport.Close()
		id := "crash-before-restart"
		token, err := issuer.IssueService(principal.ServiceInput{Audience: "user-source-custody", RPC: userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName, RequestID: id, RequestHash: hash})
		require.NoError(t, err)
		call, cancel := context.WithTimeout(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), 20*time.Second)
		defer cancel()
		stream, err := userv1.NewUserSourceCustodyServiceClient(transport).AcquireUserSourcePermit(call, req)
		require.NoError(t, err)
		frame, err := stream.Recv()
		require.NoError(t, err)
		require.Equal(t, userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_GRANTED, frame.Kind)
		require.NoError(t, child.Process.Kill())
		select {
		case <-done:
			*joined = true
		case <-time.After(5 * time.Second):
			t.Fatal("killed child did not exit")
		}
		_ = input.Close()
		for err == nil {
			_, err = stream.Recv()
		}
		require.Error(t, err)
		require.NoError(t, call.Err())
		require.Eventually(t, func() bool {
			return pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND mode='ExclusiveLock' AND granted AND objsubid=1 AND classid=(6219270163691208961::bigint >> 32)::oid AND objid=(6219270163691208961::bigint & 4294967295)::oid`).Scan(&leaders) == nil && leaders == 0
		}, 5*time.Second, 25*time.Millisecond)
		require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
		require.Equal(t, "OPEN", state, "fixture must remain stale OPEN after crash")
		_, input2, _, done2, joined2 := start()
		require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
		require.Equal(t, "CLOSED", state, "restart must overwrite stale OPEN before readiness")
		require.NoError(t, input2.Close())
		select {
		case err := <-done2:
			*joined2 = true
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("restarted child did not shut down")
		}
	})

	t.Run("listener-failure-with-active-stream", func(t *testing.T) {
		// OPEN is set only in this isolated fixture to exercise cleanup of an
		// already-held permit; it is not a source qualification or opener.
		cfg, err := LoadServerConfig(func(name string) (string, bool) { v, ok := env[name]; return v, ok })
		require.NoError(t, err)
		listenerCh := make(chan net.Listener, 1)
		cfg.testAllowSyntheticOpen = true
		cfg.testListen = func(network, address string) (net.Listener, error) {
			l, err := net.Listen(network, address)
			if err == nil {
				listenerCh <- l
			}
			return l, err
		}
		serverCtx, stopServer := context.WithCancel(ctx)
		defer stopServer()
		ready := make(chan string, 1)
		finished := make(chan error, 1)
		go func() { finished <- ServeClosed(serverCtx, cfg, func(addr string) { ready <- addr }) }()
		joined := false
		defer func() {
			stopServer()
			if !joined {
				select {
				case <-finished:
				case <-time.After(5 * time.Second):
					t.Error("server did not join")
				}
			}
		}()
		var address string
		select {
		case address = <-ready:
		case err := <-finished:
			joined = true
			t.Fatalf("server stopped before readiness: %v", err)
		case <-time.After(10 * time.Second):
			t.Fatal("server did not become ready")
		}
		listener := <-listenerCh
		defer listener.Close()
		_, err = pool.Exec(ctx, `UPDATE user_custody_state SET state='OPEN' WHERE singleton`)
		require.NoError(t, err)
		transport, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: serverTLS.roots, ServerName: "localhost", Certificates: []tls.Certificate{serverTLS.client}})))
		require.NoError(t, err)
		defer transport.Close()
		requestID := "listener-failure"
		token, err := issuer.IssueService(principal.ServiceInput{Audience: "user-source-custody", RPC: userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName, RequestID: requestID, RequestHash: hash})
		require.NoError(t, err)
		call, cancel := context.WithTimeout(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), 20*time.Second)
		defer cancel()
		stream, err := userv1.NewUserSourceCustodyServiceClient(transport).AcquireUserSourcePermit(call, req)
		require.NoError(t, err)
		frame, err := stream.Recv()
		require.NoError(t, err)
		require.Equal(t, userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_GRANTED, frame.Kind)
		require.NoError(t, listener.Close())
		select {
		case err = <-finished:
			joined = true
			require.Error(t, err, "listener failure must propagate")
		case <-time.After(8 * time.Second):
			t.Fatal("listener failure did not terminate server")
		}
		for err == nil {
			_, err = stream.Recv()
		}
		require.Error(t, err, "active stream must terminate")
		require.NoError(t, call.Err(), "stream must stop before its client deadline")
		var held int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND granted AND objsubid=1 AND classid=(6219270163691208961::bigint >> 32)::oid AND objid IN ((6219270163691208961::bigint & 4294967295)::oid,(6219270163691208962::bigint & 4294967295)::oid)`).Scan(&held))
		require.Zero(t, held, "leader and active permit must be released before ServeClosed returns")
	})

	// Each negative starts a fresh child with private control state. A timeout is
	// never accepted as a clean fail-closed result.
	for _, tc := range []struct {
		name           string
		changes        map[string]string
		controlReached bool
	}{
		{"postgres-plaintext", map[string]string{"CUSTODY_CONTROL_DATABASE_URL": strings.Replace(u.String(), "sslmode=verify-full", "sslmode=disable", 1)}, false},
		{"postgres-wrong-ca", map[string]string{"CUSTODY_CONTROL_DATABASE_URL": func() string {
			copyURL := *u
			query := copyURL.Query()
			query.Set("sslrootcert", serverTLS.caFile)
			copyURL.RawQuery = query.Encode()
			return copyURL.String()
		}()}, false},
		{"invalid-server-cert", map[string]string{"CUSTODY_TLS_CERT_FILE": serverTLS.caFile}, false},
		{"redis-unavailable", map[string]string{"CUSTODY_REDIS_ADDR": "127.0.0.1:1"}, true},
		{"redis-wrong-ca", map[string]string{"CUSTODY_REDIS_TLS_CA_FILE": serverTLS.caFile}, true},
		{"jwks-unavailable", map[string]string{"CUSTODY_USER_JWKS_URL": "https://127.0.0.1:1/jwks"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, `UPDATE user_custody_state SET state='OPEN' WHERE singleton`)
			require.NoError(t, err)
			values := make(map[string]string, len(env))
			for k, v := range env {
				values[k] = v
			}
			for k, v := range tc.changes {
				values[k] = v
			}
			bounded, stop := context.WithTimeout(ctx, 10*time.Second)
			defer stop()
			child := exec.CommandContext(bounded, os.Args[0], "-test.run=^TestCustodyProcessChild$")
			for _, item := range os.Environ() {
				if !strings.HasPrefix(strings.ToUpper(item), "PG") && !strings.HasPrefix(item, "CUSTODY_") {
					child.Env = append(child.Env, item)
				}
			}
			for k, v := range values {
				child.Env = append(child.Env, k+"="+v)
			}
			input, err := child.StdinPipe()
			require.NoError(t, err)
			defer input.Close()
			output, err := child.Output()
			require.Error(t, err)
			require.NoError(t, bounded.Err(), "timeout is not proof of fail-closed startup")
			require.NotContains(t, string(output), "READY ")
			require.NotContains(t, string(output), "fixture_only")
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				require.NotContains(t, string(exitErr.Stderr), "fixture_only")
			}
			if tc.controlReached {
				require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
				require.Equal(t, "CLOSED", state)
			}
		})
	}
}
