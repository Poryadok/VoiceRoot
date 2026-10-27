//go:build be109_public_chat

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	authv1 "voice.app/voice/auth/v1"
	chatv1 "voice.app/voice/chat/v1"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/chat/internal/chatcreation"
	"voice/backend/chat/internal/grpcsvc"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/principal"
)

const receiverSourcePin = "3b6e69f535598c83e203cece3340e9530ec968cb"

type observedFloor struct {
	delegate chatcreation.AuthFloorClient
	calls    atomic.Int64
	last     atomic.Int64
}

func (o *observedFloor) GetChatSessionEpochFloor(ctx context.Context, r *authv1.GetChatSessionEpochFloorRequest, opts ...grpc.CallOption) (*authv1.GetChatSessionEpochFloorResponse, error) {
	response, err := o.delegate.GetChatSessionEpochFloor(ctx, r, opts...)
	o.calls.Add(1)
	if err == nil && response != nil {
		o.last.Store(response.GetSessionEpochFloor())
	}
	return response, err
}

type observedEvents struct{ calls atomic.Int64 }

func (o *observedEvents) PublishChatCreated(context.Context, string, string) error {
	o.calls.Add(1)
	return nil
}
func (o *observedEvents) PublishChatUpdated(context.Context, string, []string) error {
	o.calls.Add(1)
	return nil
}
func (o *observedEvents) PublishChatDeleted(context.Context, *eventsv1.ChatStreamEvent) error {
	o.calls.Add(1)
	return nil
}
func (o *observedEvents) PublishChatMemberChanged(context.Context, string, string, string) error {
	o.calls.Add(1)
	return nil
}
func TestPublicPasswordChangeRevokesSessionsBeforeProtectedChatCreate(t *testing.T) {
	cp := os.Getenv("BE131_AUTH_JAVA_CLASSPATH")
	require.NotEmpty(t, cp, "actual Java fixture mandatory; never skip")
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
	gatewayKey, e := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, e)
	gatewayNext, e := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, e)
	botKey, e := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, e)
	botNext, e := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, e)
	gatewayDocument := chatPrincipalJWKS{Keys: []chatPrincipalJWK{jwk("gateway-current", gatewayKey), jwk("gateway-next", gatewayNext)}}
	botDocument := chatPrincipalJWKS{Keys: []chatPrincipalJWK{jwk("bot-current", botKey), jwk("bot-next", botNext)}}
	var fetches atomic.Int64
	var httpStatus atomic.Int64
	httpStatus.Store(200)
	https := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/chat" {
			fetches.Add(1)
			w.WriteHeader(int(httpStatus.Load()))
			_ = json.NewEncoder(w).Encode(document)
		} else if r.URL.Path == "/gateway" {
			_ = json.NewEncoder(w).Encode(gatewayDocument)
		} else if r.URL.Path == "/bot" {
			_ = json.NewEncoder(w).Encode(botDocument)
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
	values := map[string]string{"RUN_ID": runID, "INPUT_ROOT": root, "AUTH_SOURCE_SHA": receiverSourcePin, "LAUNCHER_SOURCE_SHA": receiverSourcePin, "HOST": "localhost", "TLS_SERVER_NAME": "localhost", "CA_FILE": serverCert, "CLIENT_CERT_FILE": clientCert, "CLIENT_KEY_FILE": clientKey, "JDBC_URL": os.Getenv("BE131_AUTH_JDBC_URL"), "JDBC_USERNAME": "test", "JDBC_PASSWORD_FILE": filepath.Join(root, "jdbc-password"), "REDIS_HOST": os.Getenv("BE131_AUTH_REDIS_HOST"), "JWKS_TRUSTSTORE_FILE": trust, "JWKS_TRUSTSTORE_PASSWORD_FILE": filepath.Join(root, "trust-password"), "DURABLE_EPOCH": strconv.FormatInt(durable, 10), "RETAINED_EPOCH": strconv.FormatInt(retained, 10)}
	logFile, err := os.Create(filepath.Join(root, "java.log"))
	require.NoError(t, err)
	child := exec.Command("java", "-cp", cp, "voice.backend.auth.sessionepoch.PublicPasswordChangeChatFixture", out, "180")
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
		if p := os.Getenv("RECEIVER_PROOF_DIR"); p != "" {
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
	require.Equal(t, receiverSourcePin, ready.Auth)
	require.Equal(t, receiverSourcePin, ready.Launcher)
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
	if p := os.Getenv("RECEIVER_PROOF_DIR"); p != "" {
		require.NoError(t, os.MkdirAll(p, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(p, "ready.json"), raw, 0600))
	}
	for k, v := range map[string]string{"GRPC_ADDR": fmt.Sprintf("%s:%d", ready.Host, ready.Port), "TLS_CA_FILE": ready.CA, "TLS_SERVER_NAME": ready.Name, "CLIENT_CERT_FILE": ready.Cert, "CLIENT_KEY_FILE": ready.Key} {
		t.Setenv("CHAT_CREATE_AUTH_"+k, v)
	}
	checker, closeFn, err := newChatCreateAuthCheckerFromEnv(issuer)
	require.NoError(t, err)
	t.Cleanup(func() { _ = closeFn() })

	observed := &observedFloor{delegate: checker.Client}
	checker.Client = observed
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	pool, e := pgxpool.New(ctx, os.Getenv("RECEIVER_CHAT_DATABASE_URL"))
	require.NoError(t, e)
	defer pool.Close()
	migrations, e := filepath.Glob("../migrations/chat_db/*.up.sql")
	require.NoError(t, e)
	sort.Strings(migrations)
	require.NotEmpty(t, migrations)
	for _, path := range migrations {
		sql, e := os.ReadFile(path)
		require.NoError(t, e)
		_, e = pool.Exec(ctx, string(sql))
		require.NoError(t, e, path)
	}
	counts := func() map[string]int64 {
		rows, e := pool.Query(ctx, "SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename")
		require.NoError(t, e)
		var tables []string
		for rows.Next() {
			var n string
			require.NoError(t, rows.Scan(&n))
			tables = append(tables, n)
		}
		require.NoError(t, rows.Err())
		rows.Close()
		result := map[string]int64{}
		for _, table := range tables {
			var n int64
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM "`+table+`"`).Scan(&n))
			result[table] = n
		}
		return result
	}
	runtime, e := chatcreation.NewCreateRuntime(ctx, chatcreation.CreateConfig{Listen: "127.0.0.1:0", CertFile: serverCert, KeyFile: serverKey, ClientCAFile: serverCert, JWKSURL: https.URL + "/gateway", BotJWKSURL: https.URL + "/bot", JWKSCAFile: serverCert, JWKSClientCertFile: clientCert, JWKSClientKeyFile: clientKey, RedisAddr: os.Getenv("RECEIVER_CHAT_REDIS_ADDR")}, checker.CheckSessionEpoch)
	require.NoError(t, e)
	defer runtime.Close()
	var ingress atomic.Int64
	// The outer observer sees actual mTLS-decoded requests before the production verifier.
	options := []grpc.ServerOption{grpc.UnaryInterceptor(func(ctx context.Context, r any, i *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		ingress.Add(1)
		return h(ctx, r)
	})}
	options = append(options, runtime.ServerOptions()...)
	events := &observedEvents{}
	server := grpc.NewServer(options...)
	chatv1.RegisterChatServiceServer(server, &grpcsvc.ChatGRPC{DM: &store.DMStore{Pool: pool}, ChatEvents: events})
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, e)
	go server.Serve(listener)
	defer server.Stop()
	roots := x509.NewCertPool()
	ca, e := os.ReadFile(serverCert)
	require.NoError(t, e)
	require.True(t, roots.AppendCertsFromPEM(ca))
	identity, e := tls.LoadX509KeyPair(clientCert, clientKey)
	require.NoError(t, e)
	connection, e := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{identity}})))
	require.NoError(t, e)
	defer connection.Close()
	client := chatv1.NewChatServiceClient(connection)
	gateway, e := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "gateway-current", PrivateKey: gatewayKey})
	require.NoError(t, e)

	privatePath := filepath.Join(out, "http-private.json")
	var private struct {
		URL     string   `json:"url"`
		Profile string   `json:"profile"`
		Access  []string `json:"access"`
		Refresh []string `json:"refresh"`
		JTI     []string `json:"jti"`
	}
	require.Eventually(t, func() bool { b, e := os.ReadFile(privatePath); return e == nil && json.Unmarshal(b, &private) == nil }, 15*time.Second, 50*time.Millisecond)
	require.Len(t, private.Access, 2)
	require.Len(t, private.Refresh, 2)
	require.Len(t, private.JTI, 2)
	profile := private.Profile
	httpClient := &http.Client{Timeout: 5 * time.Second}
	post := func(path, token, body string) (int, string) {
		request, e := http.NewRequestWithContext(ctx, "POST", private.URL+path, strings.NewReader(body))
		require.NoError(t, e)
		request.Header.Set("Content-Type", "application/json")
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response, e := httpClient.Do(request)
		require.NoError(t, e)
		defer response.Body.Close()
		b, e := io.ReadAll(io.LimitReader(response.Body, 8192))
		require.NoError(t, e)
		return response.StatusCode, string(b)
	}
	authPool, e := pgxpool.New(ctx, os.Getenv("RECEIVER_AUTH_DATABASE_URL"))
	require.NoError(t, e)
	defer authPool.Close()
	authSnapshot := func() (int64, string, int64, int64) {
		var epoch, total, active int64
		var hash string
		require.NoError(t, authPool.QueryRow(ctx, "SELECT session_epoch,password_hash FROM accounts WHERE id=$1", ready.Account).Scan(&epoch, &hash))
		require.NoError(t, authPool.QueryRow(ctx, "SELECT count(*),count(*) FILTER(WHERE revoked_at IS NULL) FROM refresh_tokens WHERE account_id=$1", ready.Account).Scan(&total, &active))
		return epoch, hash, total, active
	}
	authRedis := redis.NewClient(&redis.Options{Addr: os.Getenv("BE131_AUTH_REDIS_HOST") + ":6379", MaxRetries: -1})
	defer authRedis.Close()
	floorKey := "auth:session:min_epoch:" + ready.Account
	floorBefore, e := authRedis.Get(ctx, floorKey).Int64()
	require.NoError(t, e)
	require.Equal(t, ready.Floor, floorBefore)
	initialEpoch, initialHash, initialRows, initialActive := authSnapshot()
	require.Equal(t, int64(2), initialRows)
	require.Equal(t, initialRows, initialActive)
	for _, access := range private.Access {
		code, _ := post("/api/v1/auth/validate", access, "{}")
		require.Equal(t, 200, code)
	}

	call := func(epoch int64, label string) (*chatv1.CreateChatResponse, error) {
		rid := uuid.NewString()
		name := "receiver-" + label
		request := &chatv1.CreateChatRequest{Type: chatv1.ChatType_CHAT_TYPE_GROUP, Name: &name, RequestId: rid}
		hash, e := principal.RequestHash(request)
		require.NoError(t, e)
		token, e := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "chat", RPC: chatv1.ChatService_CreateChat_FullMethodName, RequestID: rid, RequestHash: hash, AccountID: ready.Account, ProfileID: profile, SessionEpoch: epoch, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, e)
		response, err := client.CreateChat(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", rid)), request)
		t.Logf("receiver_case=%s request_id=%s signed_epoch=%d receiver_ingress=%d auth_rpc_calls=%d auth_floor=%d grpc_code=%s body=%q", label, rid, epoch, ingress.Load(), observed.calls.Load(), observed.last.Load(), status.Code(err), status.Convert(err).Message())
		return response, err
	}
	positive, e := call(ready.Floor, "positive-before")
	require.NoError(t, e)
	require.NotEmpty(t, positive.GetChat().GetId())
	require.Equal(t, int64(ready.Floor), observed.last.Load())

	wrongCode, _ := post("/api/v1/auth/password/change", private.Access[0], `{"current_password":"wrong","new_password":"New fixture password 73"}`)
	require.Equal(t, 401, wrongCode)
	wrongEpoch, wrongHash, wrongRows, wrongActive := authSnapshot()
	require.Equal(t, initialEpoch, wrongEpoch)
	require.Equal(t, initialHash, wrongHash)
	require.Equal(t, initialRows, wrongRows)
	require.Equal(t, initialActive, wrongActive)
	floorWrong, e := authRedis.Get(ctx, floorKey).Int64()
	require.NoError(t, e)
	require.Equal(t, floorBefore, floorWrong)
	changedCode, changedBody := post("/api/v1/auth/password/change", private.Access[0], `{"current_password":"Old fixture password 73","new_password":"New fixture password 73"}`)
	require.Equal(t, 204, changedCode)
	require.Empty(t, changedBody, "password change must issue no replacement session")
	nextEpoch, nextHash, nextRows, nextActive := authSnapshot()
	require.Greater(t, nextEpoch, initialEpoch)
	require.Greater(t, nextEpoch, floorBefore)
	require.NotEqual(t, initialHash, nextHash)
	require.Equal(t, initialRows, nextRows)
	require.Zero(t, nextActive)
	newFloor, e := authRedis.Get(ctx, floorKey).Int64()
	require.NoError(t, e)
	require.Equal(t, nextEpoch, newFloor)
	ttl, e := authRedis.TTL(ctx, floorKey).Result()
	require.NoError(t, e)
	require.Equal(t, time.Duration(-1), ttl, "floor must have no TTL")
	blacklisted, e := authRedis.Exists(ctx, "jwt:blacklist:"+private.JTI[0]).Result()
	require.NoError(t, e)
	require.Equal(t, int64(1), blacklisted)
	for i, access := range private.Access {
		code, _ := post("/api/v1/auth/validate", access, "{}")
		require.Equal(t, 401, code)
		refreshBody, e := json.Marshal(map[string]string{"refresh_token": private.Refresh[i]})
		require.NoError(t, e)
		code, _ = post("/api/v1/auth/refresh", "", string(refreshBody))
		require.Equal(t, 401, code)
	}
	// Source-backed password hash verification is independently performed by the Java launcher readback.
	require.NoError(t, os.WriteFile(filepath.Join(out, "verify-password.request"), []byte("verify"), 0600))
	var passwordReadback struct {
		NewMatches bool `json:"new_matches"`
		OldMatches bool `json:"old_matches"`
	}
	require.Eventually(t, func() bool {
		b, e := os.ReadFile(filepath.Join(out, "password-readback.json"))
		return e == nil && json.Unmarshal(b, &passwordReadback) == nil
	}, 5*time.Second, 25*time.Millisecond)
	require.True(t, passwordReadback.NewMatches)
	require.False(t, passwordReadback.OldMatches)
	before := counts()
	beforeIngress, beforeRPC, beforeEvents := ingress.Load(), observed.calls.Load(), events.calls.Load()
	response, deny := call(ready.Floor, "stale-after")
	after := counts()
	report := map[string]any{"source": receiverSourcePin, "public_http_status": changedCode, "empty_204": changedBody == "", "durable_before": initialEpoch, "durable_after": nextEpoch, "floor_before": floorBefore, "floor_after": newFloor, "refresh_rows": nextRows, "refresh_active_after": nextActive, "old_access_rejections": 2, "old_refresh_rejections": 2, "blacklisted_change_jti": blacklisted == 1, "fresh_public_login": "OPEN: real User profile authority absent", "old_epoch": ready.Floor, "advanced_floor": observed.last.Load(), "receiver_ingress_delta": ingress.Load() - beforeIngress, "auth_rpc_delta": observed.calls.Load() - beforeRPC, "grpc_code": status.Code(deny).String(), "body": status.Convert(deny).Message(), "db_before": before, "db_after": after, "db_equal": reflect.DeepEqual(before, after), "publisher_calls_delta": events.calls.Load() - beforeEvents, "positive_chat_id": positive.GetChat().GetId()}
	raw, e = json.MarshalIndent(report, "", "  ")
	require.NoError(t, e)
	require.NoError(t, os.WriteFile(filepath.Join(os.Getenv("RECEIVER_PROOF_DIR"), "receiver-result.json"), raw, 0600))
	require.Nil(t, response)
	require.Equal(t, codes.Unauthenticated, status.Code(deny))
	require.Equal(t, ready.Floor+1, observed.last.Load())
	require.Equal(t, int64(1), ingress.Load()-beforeIngress)
	require.Equal(t, int64(1), observed.calls.Load()-beforeRPC)
	require.Equal(t, before, after)
	require.Equal(t, beforeEvents, events.calls.Load())
	fresh, e := call(nextEpoch, "receiver-new-epoch-control")
	require.NoError(t, e)
	require.NotEmpty(t, fresh.GetChat().GetId())
	require.NotEqual(t, positive.GetChat().GetId(), fresh.GetChat().GetId())
	t.Log("PUBLIC_PASSWORD_CHANGE_CHAT_PASS: HTTP204 real transaction durable/floor advance and all refresh revoke; old signed epoch receiver DENY; table COUNT maps and publisher unchanged; receiver new-epoch control OK; public fresh-login OPEN")
}
