package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/chatcreation"
	"voice/backend/pkg/principal"
)

const authJavaFixturePin = "908ac14261dea22fc570ff114dffdea27dcffb97"

// Wait for the positive control's new transport before starting its RPC deadline.
// Negative TLS controls must not use this helper because they cannot become ready.
func waitForChatAuthControlConnection(ctx context.Context, conn *grpc.ClientConn) error {
	conn.Connect()
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return nil
		}
		if state == connectivity.Shutdown {
			return fmt.Errorf("Chat/Auth control connection shut down")
		}
		if !conn.WaitForStateChange(ctx, state) {
			if err := ctx.Err(); err != nil {
				return err
			}
			return fmt.Errorf("Chat/Auth control connection did not become ready")
		}
	}
}

// Candidate mode is only meaningful when the CI job builds the classpath from
// this clean checkout. Legacy component proofs keep their original fixed pin.
func authJavaCandidateSource(repo, source, classpath string) error {
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(source) {
		return fmt.Errorf("invalid candidate source")
	}
	git := func(args ...string) (string, error) {
		command := exec.Command("git", append([]string{"-C", repo}, args...)...)
		command.Env = append(os.Environ(), "GIT_NO_REPLACE_OBJECTS=1")
		out, err := command.Output()
		return strings.TrimSpace(string(out)), err
	}
	head, err := git("rev-parse", "HEAD")
	if err != nil || head != source {
		return fmt.Errorf("candidate checkout identity differs")
	}
	dirty, err := git("status", "--porcelain", "--untracked-files=all")
	if err != nil || dirty != "" {
		return fmt.Errorf("candidate inputs are not clean")
	}
	entries := strings.Split(classpath, ":")
	if len(entries) < 2 {
		return fmt.Errorf("candidate classpath is incomplete")
	}
	for i, relative := range []string{"src/backend/auth/target/test-classes", "src/backend/auth/target/classes"} {
		expected, e1 := filepath.EvalSymlinks(filepath.Join(repo, filepath.FromSlash(relative)))
		actual, e2 := filepath.EvalSymlinks(entries[i])
		if e1 != nil || e2 != nil || expected != actual {
			return fmt.Errorf("candidate classpath does not use checkout output")
		}
	}
	return nil
}

type authJavaReady struct {
	Schema   int       `json:"schema_version"`
	Run      string    `json:"run_id"`
	Auth     string    `json:"auth_source_sha"`
	Launcher string    `json:"launcher_source_sha"`
	Host     string    `json:"host"`
	Port     int       `json:"port"`
	Name     string    `json:"tls_server_name"`
	CA       string    `json:"ca_file"`
	Cert     string    `json:"client_cert_file"`
	Key      string    `json:"client_key_file"`
	RPC      string    `json:"rpc"`
	Account  string    `json:"account_id"`
	Unknown  string    `json:"unknown_account_id"`
	Deleted  string    `json:"deleted_account_id"`
	Durable  int64     `json:"durable_epoch"`
	Retained int64     `json:"retained_epoch"`
	Floor    int64     `json:"expected_floor"`
	Started  time.Time `json:"started_at"`
	Expires  time.Time `json:"expires_at"`
}

// Explicitly opt-in. Runs a separate actual Java process, not a Go provider fake.
// The outer private Linux orchestrator owns the fresh DB/Redis and compilation.
func TestChatAuthActualJavaInteroperability(t *testing.T) {
	cp := os.Getenv("BE131_AUTH_JAVA_CLASSPATH")
	if cp == "" {
		t.Skip("requires owned private Java/PG/Redis orchestration")
	}
	source := authJavaFixturePin
	if candidate := os.Getenv("BE131_AUTH_CANDIDATE_SHA"); candidate != "" {
		repo, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		require.NoError(t, err)
		require.NoError(t, authJavaCandidateSource(strings.TrimSpace(string(repo)), candidate, cp))
		source = candidate
	}
	require.Equal(t, source, os.Getenv("BE131_AUTH_JAVA_SOURCE"))
	for _, entry := range strings.Split(cp, ":") {
		_, err := os.Stat(entry)
		require.NoError(t, err, "actual compiled classpath entry")
	}
	root := t.TempDir()
	out := filepath.Join(root, "run")
	require.NoError(t, os.Mkdir(out, 0700))
	command := func(args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		b, err := exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
		require.NoError(t, err, "fixture command %s failed (%d bytes suppressed)", args[0], len(b))
	}
	serverCert, serverKey := filepath.Join(root, "server.pem"), filepath.Join(root, "server.key")
	clientCert, clientKey := filepath.Join(root, "chat.pem"), filepath.Join(root, "chat.key")
	command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1", "-subj", "/CN=localhost", "-addext", "subjectAltName=DNS:localhost,IP:127.0.0.1", "-addext", "basicConstraints=critical,CA:TRUE", "-keyout", serverKey, "-out", serverCert)
	command("openssl", "req", "-new", "-newkey", "rsa:2048", "-nodes", "-subj", "/CN=chat-interop", "-keyout", clientKey, "-out", filepath.Join(root, "chat.csr"))
	ext := filepath.Join(root, "chat.ext")
	require.NoError(t, os.WriteFile(ext, []byte("basicConstraints=CA:FALSE\nkeyUsage=digitalSignature\nextendedKeyUsage=clientAuth\nsubjectAltName=URI:urn:voice:test:chat-interop\n"), 0600))
	command("openssl", "x509", "-req", "-in", filepath.Join(root, "chat.csr"), "-CA", serverCert, "-CAkey", serverKey, "-set_serial", "44", "-days", "1", "-extfile", ext, "-out", clientCert)
	trust := filepath.Join(root, "trust.p12")
	command("keytool", "-importcert", "-noprompt", "-alias", "fixture-ca", "-file", serverCert, "-keystore", trust, "-storetype", "PKCS12", "-storepass", "fixture-test-password")
	for _, f := range []string{"jdbc-password", "trust-password"} {
		value := "test"
		if f == "trust-password" {
			value = "fixture-test-password"
		}
		require.NoError(t, os.WriteFile(filepath.Join(root, f), []byte(value), 0600))
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	foreign, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	foreignNext, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "chat", KeyID: "chat-current", PrivateKey: key})
	require.NoError(t, err)
	jwk := func(id string, k *rsa.PrivateKey) chatPrincipalJWK {
		return chatPrincipalJWK{Kty: "RSA", Kid: id, Use: "sig", Alg: "RS256", N: base64.RawURLEncoding.EncodeToString(k.N.Bytes()), E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.E)).Bytes())}
	}
	document := chatPrincipalJWKS{Keys: []chatPrincipalJWK{jwk("chat-current", key), jwk("chat-next", next)}}
	foreignDocument := chatPrincipalJWKS{Keys: []chatPrincipalJWK{jwk("space-current", foreign), jwk("space-next", foreignNext)}}
	var fetches atomic.Int64
	var httpStatus atomic.Int64
	httpStatus.Store(200)
	https := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/chat" {
			fetches.Add(1)
			w.WriteHeader(int(httpStatus.Load()))
			_ = json.NewEncoder(w).Encode(document)
		} else {
			_ = json.NewEncoder(w).Encode(foreignDocument)
		}
	}))
	cert, err := tls.LoadX509KeyPair(serverCert, serverKey)
	require.NoError(t, err)
	https.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}
	https.StartTLS()
	t.Cleanup(https.Close)
	urls, _ := json.Marshal(map[string]string{"chat": https.URL + "/chat", "space": https.URL + "/space", "gateway": https.URL + "/gateway"})
	runID := uuid.NewString()
	durable, retained := int64(7), int64(9)
	if s := os.Getenv("BE131_AUTH_DURABLE"); s != "" {
		durable, err = strconv.ParseInt(s, 10, 64)
		require.NoError(t, err)
	}
	if s := os.Getenv("BE131_AUTH_RETAINED"); s != "" {
		retained, err = strconv.ParseInt(s, 10, 64)
		require.NoError(t, err)
	}
	values := map[string]string{"RUN_ID": runID, "INPUT_ROOT": root, "AUTH_SOURCE_SHA": source, "LAUNCHER_SOURCE_SHA": source, "HOST": "localhost", "TLS_SERVER_NAME": "localhost", "CA_FILE": serverCert, "CLIENT_CERT_FILE": clientCert, "CLIENT_KEY_FILE": clientKey, "JDBC_URL": os.Getenv("BE131_AUTH_JDBC_URL"), "JDBC_USERNAME": "test", "JDBC_PASSWORD_FILE": filepath.Join(root, "jdbc-password"), "REDIS_HOST": os.Getenv("BE131_AUTH_REDIS_HOST"), "JWKS_TRUSTSTORE_FILE": trust, "JWKS_TRUSTSTORE_PASSWORD_FILE": filepath.Join(root, "trust-password"), "DURABLE_EPOCH": strconv.FormatInt(durable, 10), "RETAINED_EPOCH": strconv.FormatInt(retained, 10)}
	if port := os.Getenv("BE131_AUTH_REDIS_PORT"); port != "" {
		values["REDIS_PORT"] = port
	}
	logFile, err := os.Create(filepath.Join(root, "java.log"))
	require.NoError(t, err)
	child := exec.Command("java", "-cp", cp, "voice.backend.auth.sessionepoch.AuthChatFloorInteropFixture", out, "180")
	child.Env = os.Environ()
	for k, v := range values {
		child.Env = append(child.Env, "VOICE_AUTH_CHAT_FLOOR_INTEROP_"+k+"="+v)
	}
	child.Env = append(child.Env, "AUTH_GRPC_TLS_CERT_FILE="+serverCert, "AUTH_GRPC_TLS_KEY_FILE="+serverKey, "AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE="+serverCert, "S2S_JWKS_URLS_JSON="+string(urls), "S2S_UNKNOWN_KID_COOLDOWN=1ms")
	child.Stdout = logFile
	child.Stderr = logFile
	require.NoError(t, child.Start())
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		require.NoError(t, child.Process.Signal(syscall.SIGTERM))
		select {
		case <-done:
		case <-time.After(12 * time.Second):
			_ = child.Process.Kill()
			t.Error("Java graceful cleanup deadline")
		}
		require.NoError(t, logFile.Close())
	}
	t.Cleanup(func() {
		stop()
		if p := os.Getenv("BE131_AUTH_PROOF_DIR"); p != "" {
			require.NoError(t, os.MkdirAll(p, 0700))
			for _, f := range []struct{ from, to string }{{filepath.Join(root, "java.log"), "java.log"}, {filepath.Join(out, "closed.json"), "closed.json"}} {
				b, e := os.ReadFile(f.from)
				if e == nil {
					require.NoError(t, os.WriteFile(filepath.Join(p, f.to), b, 0600))
				}
			}
		}
	})
	readyPath := filepath.Join(out, "ready.json")
	deadline := time.Now().Add(45 * time.Second)
	var raw []byte
	for time.Now().Before(deadline) {
		raw, err = os.ReadFile(readyPath)
		if err == nil {
			break
		}
		select {
		case e := <-done:
			stopped = true
			t.Fatalf("Java exited before readiness: %v (see owned java.log)", e)
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.NoError(t, err, "Java setup must publish actual readiness")
	var ready authJavaReady
	require.NoError(t, json.Unmarshal(raw, &ready))
	require.Equal(t, 1, ready.Schema)
	require.Equal(t, runID, ready.Run)
	require.Equal(t, source, ready.Auth)
	require.Equal(t, source, ready.Launcher)
	require.Equal(t, authv1.AuthService_GetChatSessionEpochFloor_FullMethodName, ready.RPC)
	require.Equal(t, "localhost", ready.Host)
	require.Equal(t, "localhost", ready.Name)
	require.Greater(t, ready.Port, 0)
	require.Less(t, ready.Port, 65536)
	for actual, expected := range map[string]string{ready.CA: serverCert, ready.Cert: clientCert, ready.Key: clientKey} {
		require.Equal(t, expected, actual)
		resolved, e := filepath.EvalSymlinks(actual)
		require.NoError(t, e)
		require.True(t, strings.HasPrefix(resolved, root+string(os.PathSeparator)))
	}
	require.Equal(t, durable, ready.Durable)
	require.Equal(t, retained, ready.Retained)
	require.Equal(t, max(durable, retained), ready.Floor)
	require.Equal(t, 180*time.Second, ready.Expires.Sub(ready.Started))
	require.True(t, ready.Expires.After(time.Now()))
	require.Zero(t, fetches.Load(), "ready alone is not JWKS success")
	if p := os.Getenv("BE131_AUTH_PROOF_DIR"); p != "" {
		require.NoError(t, os.MkdirAll(p, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(p, "ready.json"), raw, 0600))
	}
	for k, v := range map[string]string{"GRPC_ADDR": fmt.Sprintf("%s:%d", ready.Host, ready.Port), "TLS_CA_FILE": ready.CA, "TLS_SERVER_NAME": ready.Name, "CLIENT_CERT_FILE": ready.Cert, "CLIENT_KEY_FILE": ready.Key} {
		t.Setenv("CHAT_CREATE_AUTH_"+k, v)
	}
	checker, closeFn, err := newChatCreateAuthCheckerFromEnv(issuer)
	require.NoError(t, err)
	t.Cleanup(func() { _ = closeFn() })
	op := uuid.NewString()
	incoming := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer not-forwarded-ingress", "x-request-id", op))
	require.NoError(t, checker.CheckSessionEpoch(incoming, ready.Account, ready.Floor))
	require.Positive(t, fetches.Load(), "actual Go request caused Java HTTPS JWKS fetch")
	twoNegativesOnly := os.Getenv("BE131_AUTH_TWO_NEGATIVES") == "true"
	if !twoNegativesOnly {
		require.NoError(t, checker.CheckSessionEpoch(incoming, ready.Account, ready.Floor+1))
		require.ErrorIs(t, checker.CheckSessionEpoch(incoming, ready.Account, ready.Floor-1), chatcreation.ErrInitiator)
		for _, account := range []string{ready.Unknown, ready.Deleted} {
			require.ErrorIs(t, checker.CheckSessionEpoch(incoming, account, ready.Floor), chatcreation.ErrInitiator)
		}
	}
	// Actual Go->Java caller controls, using the same mTLS credentials but fresh
	// exact service signatures except the intentionally repeated credential.
	roots := x509.NewCertPool()
	ca, err := os.ReadFile(serverCert)
	require.NoError(t, err)
	require.True(t, roots.AppendCertsFromPEM(ca))
	clientIdentity, err := tls.LoadX509KeyPair(clientCert, clientKey)
	require.NoError(t, err)
	dial := func(withCert bool) *grpc.ClientConn {
		cfg := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: ready.Name}
		if withCert {
			cfg.Certificates = []tls.Certificate{clientIdentity}
		}
		conn, e := grpc.NewClient(fmt.Sprintf("localhost:%d", ready.Port), grpc.WithTransportCredentials(credentials.NewTLS(cfg)))
		require.NoError(t, e)
		t.Cleanup(func() { _ = conn.Close() })
		if withCert {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			require.NoError(t, waitForChatAuthControlConnection(ctx, conn), "Chat/Auth control transport must become ready")
		}
		return conn
	}
	controlConnection := dial(true)
	client := authv1.NewAuthServiceClient(controlConnection)
	req := &authv1.GetChatSessionEpochFloorRequest{AccountId: ready.Account}
	signBinding := func(i *principal.Issuer, r *authv1.GetChatSessionEpochFloorRequest, rpc string) context.Context {
		hash, e := principal.RequestHash(r)
		require.NoError(t, e)
		token, e := i.IssueService(principal.ServiceInput{Audience: "auth", RPC: rpc, RequestID: op, RequestHash: hash})
		require.NoError(t, e)
		return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", op))
	}
	sign := func(i *principal.Issuer, r *authv1.GetChatSessionEpochFloorRequest) context.Context {
		return signBinding(i, r, ready.RPC)
	}
	call := func(ctx context.Context, r *authv1.GetChatSessionEpochFloorRequest) error {
		ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		_, e := client.GetChatSessionEpochFloor(ctx, r)
		if status.Code(e) == codes.DeadlineExceeded {
			t.Logf("Chat/Auth control deadline: gRPC state=%s, JWKS fetches=%d", controlConnection.GetState(), fetches.Load())
		}
		return e
	}
	checkClosed := func() {
		stop()
		closed, e := os.ReadFile(filepath.Join(out, "closed.json"))
		require.NoError(t, e)
		var receipt map[string]any
		require.NoError(t, json.Unmarshal(closed, &receipt))
		require.Equal(t, runID, receipt["run_id"])
		for _, k := range []string{"server_terminated", "ready_removed", "clients_closed"} {
			require.Equal(t, true, receipt[k])
		}
	}
	// These are receiver negatives, not constructor/precheck substitutes. Preserve
	// status and safe certificate evidence before a semantic assertion can fail.
	report := map[string]any{"run_id": runID, "actual_rpc": ready.RPC, "signed_rpc": ready.RPC + "/wrong-binding", "certificate_policy": "Chat client-chain CA trust; no Chat URI-SAN selector in reviewed908"}
	writeReport := func() {
		b, e := json.MarshalIndent(report, "", "  ")
		require.NoError(t, e)
		if dir := os.Getenv("BE131_AUTH_PROOF_DIR"); dir != "" {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "two-negatives.json"), b, 0600))
		}
	}
	require.NoError(t, call(sign(issuer, req), req), "same actual Java receiver valid baseline before wrong RPC")
	report["valid_control_before_rpc"] = "OK"
	wrongRPCError := call(signBinding(issuer, req, ready.RPC+"/wrong-binding"), req)
	report["wrong_rpc_expected"] = codes.Unauthenticated.String()
	report["wrong_rpc_actual"] = status.Code(wrongRPCError).String()
	writeReport()
	require.Equal(t, codes.Unauthenticated, status.Code(wrongRPCError), "correct Chat signature/body/audience with wrong RPC must reach actual Java denial")
	require.NoError(t, call(sign(issuer, req), req), "same receiver valid baseline before wrong client identity")
	report["valid_control_before_certificate"] = "OK"
	wrongCA, wrongCAKey := filepath.Join(root, "wrong-ca.pem"), filepath.Join(root, "wrong-ca.key")
	wrongCert, wrongKey := filepath.Join(root, "wrong-client.pem"), filepath.Join(root, "wrong-client.key")
	command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1", "-subj", "/CN=untrusted-fixture-ca", "-addext", "basicConstraints=critical,CA:TRUE", "-keyout", wrongCAKey, "-out", wrongCA)
	command("openssl", "req", "-new", "-newkey", "rsa:2048", "-nodes", "-subj", "/CN=wrong-service", "-keyout", wrongKey, "-out", filepath.Join(root, "wrong-client.csr"))
	wrongExt := filepath.Join(root, "wrong-client.ext")
	require.NoError(t, os.WriteFile(wrongExt, []byte("basicConstraints=CA:FALSE\nkeyUsage=digitalSignature\nextendedKeyUsage=clientAuth\nsubjectAltName=URI:urn:voice:test:wrong-service\n"), 0600))
	command("openssl", "x509", "-req", "-in", filepath.Join(root, "wrong-client.csr"), "-CA", wrongCA, "-CAkey", wrongCAKey, "-set_serial", "45", "-days", "1", "-extfile", wrongExt, "-out", wrongCert)
	wrongIdentity, err := tls.LoadX509KeyPair(wrongCert, wrongKey)
	require.NoError(t, err)
	wrongLeaf, err := x509.ParseCertificate(wrongIdentity.Certificate[0])
	require.NoError(t, err)
	wrongRoots := x509.NewCertPool()
	wrongCABytes, err := os.ReadFile(wrongCA)
	require.NoError(t, err)
	require.True(t, wrongRoots.AppendCertsFromPEM(wrongCABytes))
	_, err = wrongLeaf.Verify(x509.VerifyOptions{Roots: wrongRoots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
	require.NoError(t, err, "wrong identity is a valid clientAuth certificate under its own CA")
	_, err = wrongLeaf.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
	var unknownCA x509.UnknownAuthorityError
	require.ErrorAs(t, err, &unknownCA, "actual intended rejection boundary is Auth client-chain CA trust")
	certificateFacts := func(der []byte) map[string]any {
		leaf, e := x509.ParseCertificate(der)
		require.NoError(t, e)
		return map[string]any{"der_sha256": fmt.Sprintf("%x", sha256.Sum256(der)), "subject": leaf.Subject.String(), "issuer": leaf.Issuer.String(), "serial": leaf.SerialNumber.String(), "uri_sans": leaf.URIs, "extended_key_usage": leaf.ExtKeyUsage, "not_after": leaf.NotAfter}
	}
	report["valid_client_certificate"] = certificateFacts(clientIdentity.Certificate[0])
	report["wrong_client_certificate"] = certificateFacts(wrongIdentity.Certificate[0])
	report["server_ca_unchanged"] = true
	report["expected_boundary"] = "actual Java TLS client-chain trust, not an asserted Chat service URI binding"
	var selected, verifiedServer atomic.Int64
	badTLS := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: ready.Name,
		// Force the wrong certificate to be presented even when the server's CA
		// list would make Go silently omit it. This is not the missing-cert case.
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			selected.Add(1)
			return &wrongIdentity, nil
		},
		VerifyConnection: func(tls.ConnectionState) error { verifiedServer.Add(1); return nil }}
	badConnection, err := grpc.NewClient(fmt.Sprintf("localhost:%d", ready.Port), grpc.WithTransportCredentials(credentials.NewTLS(badTLS)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = badConnection.Close() })
	badContext, badCancel := context.WithTimeout(sign(issuer, req), 4*time.Second)
	_, wrongCertificateError := authv1.NewAuthServiceClient(badConnection).GetChatSessionEpochFloor(badContext, req)
	badCancel()
	report["wrong_certificate_expected"] = codes.Unavailable.String()
	report["wrong_certificate_actual"] = status.Code(wrongCertificateError).String()
	report["wrong_certificate_selected"] = selected.Load()
	report["correct_server_verified"] = verifiedServer.Load()
	writeReport()
	require.Positive(t, selected.Load(), "wrong certificate was actually selected, not silently omitted")
	require.Positive(t, verifiedServer.Load(), "correct server CA/name validation succeeded")
	require.Equal(t, codes.Unavailable, status.Code(wrongCertificateError), "actual Java TLS rejects the present untrusted client identity")
	require.NoError(t, call(sign(issuer, req), req), "valid identity still succeeds after actual wrong-certificate denial")
	report["valid_control_after_negatives"] = "OK"
	writeReport()
	if twoNegativesOnly {
		checkClosed()
		t.Log("actual Java wrong-signed-RPC Unauthenticated and PRESENT wrong-client-chain Unavailable; valid controls OK; Chat URI binding not claimed")
		return
	}
	repeated := sign(issuer, req)
	require.NoError(t, call(repeated, req))
	require.Equal(t, codes.Unauthenticated, status.Code(call(repeated, req)), "actual Redis JTI replay")
	require.Equal(t, codes.Unauthenticated, status.Code(call(sign(issuer, req), &authv1.GetChatSessionEpochFloorRequest{AccountId: ready.Unknown})), "full body mismatch")
	malformed := &authv1.GetChatSessionEpochFloorRequest{AccountId: "bad"}
	require.Equal(t, codes.InvalidArgument, status.Code(call(sign(issuer, malformed), malformed)))
	other, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "space-current", PrivateKey: foreign})
	require.NoError(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(call(sign(other, req), req)), "valid foreign issuer cannot borrow Chat method")
	noCert := authv1.NewAuthServiceClient(dial(false))
	ctx, cancel := context.WithTimeout(sign(issuer, req), 2*time.Second)
	_, err = noCert.GetChatSessionEpochFloor(ctx, req)
	cancel()
	require.Error(t, err, "missing client identity denied")
	// Synthetic verified Gateway peer for the CreateVerifier composition only.
	gatewayKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	gateway, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "gateway", PrivateKey: gatewayKey})
	require.NoError(t, err)
	create := &chatv1.CreateChatRequest{Type: chatv1.ChatType_CHAT_TYPE_GROUP, SpaceId: proto.String(uuid.NewString()), Name: proto.String("interop"), RequestId: op}
	createHash, err := principal.RequestHash(create)
	require.NoError(t, err)
	effects := 0
	verifier := chatcreation.CreateVerifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &gatewayKey.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: checker.CheckSessionEpoch}
	createCall := func(epoch int64) error {
		token, e := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "chat", RPC: chatv1.ChatService_CreateChat_FullMethodName, RequestID: op, RequestHash: createHash, AccountID: ready.Account, ProfileID: uuid.NewString(), AccountType: "regular", SessionEpoch: epoch, ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, e)
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", op))
		ctx = peer.NewContext(ctx, &peer.Peer{AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{{}}}}}})
		_, e = verifier.Unary()(ctx, create, &grpc.UnaryServerInfo{FullMethod: chatv1.ChatService_CreateChat_FullMethodName}, func(context.Context, any) (any, error) { effects++; return nil, nil })
		return e
	}
	require.NoError(t, createCall(ready.Floor))
	require.Equal(t, 1, effects)
	require.Equal(t, codes.Unauthenticated, status.Code(createCall(ready.Floor-1)))
	require.Equal(t, 1, effects)
	httpStatus.Store(503)
	outage, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "chat", KeyID: "outage-kid", PrivateKey: key})
	require.NoError(t, err)
	before := fetches.Load()
	time.Sleep(10 * time.Millisecond)
	require.Equal(t, codes.Unavailable, status.Code(call(sign(outage, req), req)))
	require.Greater(t, fetches.Load(), before, "actual HTTPS outage reached by Java")
	checkClosed()
	require.Equal(t, codes.Unavailable, status.Code(createCall(ready.Floor)))
	require.Equal(t, 1, effects, "endpoint outage denies before receiver effects")
	t.Logf("actual Go->Java max=%d/%d=%d, equality/lower/absent/malformed/body/issuer/replay/mTLS/JWKS and receiver-outage cases passed; synthetic ingress peer only", durable, retained, ready.Floor)
}
