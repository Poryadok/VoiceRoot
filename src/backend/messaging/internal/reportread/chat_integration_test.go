package reportread

import (
	"bytes"
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
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

type changedPolicyBody struct{ actual ChatPolicyClient }

func (c changedPolicyBody) CheckMessageReadEntitlement(ctx context.Context, r *chatv1.CheckMessageReadEntitlementRequest, options ...grpc.CallOption) (*chatv1.CheckMessageReadEntitlementResponse, error) {
	changed := proto.Clone(r).(*chatv1.CheckMessageReadEntitlementRequest)
	changed.ProfileId = uuid.NewString()
	return c.actual.CheckMessageReadEntitlement(ctx, changed, options...)
}

// The subprocess is the actual Chat main and the exact source being tested,
// built and executed only inside the owned Linux proof container. No fixture
// handler or fabricated principal supplies its native scope/retirement facts.
func TestReportOwnerActualChatMainTLSNative26MoveAndPolicy(t *testing.T) {
	if testing.Short() {
		t.Skip("requires actual private Chat process and PostgreSQL")
	}
	if runtime.GOOS != "linux" {
		t.Skip("isolated Linux process fixture")
	}
	ctx := context.Background()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(root, "src/backend/migrations/chat_db"))
	db.Run(true, "up")
	var version int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&version))
	require.Equal(t, 26, version)
	viewer, space, otherSpace, chat, managed := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	joined := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	_, err := db.Pool.Exec(ctx, `INSERT INTO chats(id,type,creator_profile_id,space_id) VALUES($1,'group',$2,$3)`, chat, viewer, space)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `INSERT INTO chats(id,type,creator_profile_id,managed_by_application_id,managed_environment_id,external_chat_key) VALUES($1,'group',NULL,$2,$3,'owner-report-policy')`, managed, uuid.New(), uuid.New())
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `INSERT INTO managed_chat_member_intervals(chat_id,profile_id,joined_at) VALUES($1,$2,$3)`, managed, viewer, joined)
	require.NoError(t, err)
	loginName := "ownerclient_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	identifier := pgx.Identifier{loginName}.Sanitize()
	_, err = db.Pool.Exec(ctx, `CREATE ROLE `+identifier+` LOGIN PASSWORD 'private-client-fixture' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;GRANT USAGE ON SCHEMA public TO `+identifier+`;GRANT SELECT ON ALL TABLES IN SCHEMA public TO `+identifier)
	require.NoError(t, err)
	login := db.Pool.Config().Copy()
	login.ConnConfig.User = loginName
	login.ConnConfig.Password = "private-client-fixture"
	// ConnString preserves the originally parsed URL; the fixture and the login
	// change its Database/User fields. Serialize those actual fields explicitly.
	loginURL := &url.URL{Scheme: "postgres", User: url.UserPassword(login.ConnConfig.User, login.ConnConfig.Password), Host: net.JoinHostPort(login.ConnConfig.Host, strconv.Itoa(int(login.ConnConfig.Port))), Path: "/" + login.ConnConfig.Database, RawQuery: "sslmode=disable"}
	t.Cleanup(func() {
		_, err := db.Pool.Exec(ctx, `DROP OWNED BY `+identifier+`;DROP ROLE `+identifier)
		require.NoError(t, err)
	})
	cert, keyFile, ca, clientCert, roots := ownerChatTLSMaterial(t)
	signing, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := []map[string]string{}
	for i, k := range []*rsa.PrivateKey{signing, next} {
		kid := "current"
		if i == 1 {
			kid = "next"
		}
		keys = append(keys, map[string]string{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(k.N.Bytes()), "e": "AQAB"})
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"keys": keys}))
	}))
	identity, err := tls.LoadX509KeyPair(cert, keyFile)
	require.NoError(t, err)
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{identity}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	jwks.StartTLS()
	t.Cleanup(jwks.Close)
	replay := miniredis.RunT(t)
	dir := t.TempDir()
	clientKey, err := x509.MarshalPKCS8PrivateKey(clientCert.PrivateKey)
	require.NoError(t, err)
	clientFile, clientKeyFile := filepath.Join(dir, "client.pem"), filepath.Join(dir, "client-key.pem")
	require.NoError(t, os.WriteFile(clientFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCert.Certificate[0]}), 0600))
	require.NoError(t, os.WriteFile(clientKeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientKey}), 0600))
	reserve := func() string {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := l.Addr().String()
		require.NoError(t, l.Close())
		return addr
	}
	policyAddr, scopeAddr := reserve(), reserve()
	binary := filepath.Join(dir, "chat-owner")
	build := exec.Command("go", "build", "-mod=readonly", "-o", binary, ".")
	build.Dir = filepath.Join(root, "src/backend/chat")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "build actual Chat: %s", output)
	processContext, cancelProcess := context.WithCancel(ctx)
	cmd := exec.CommandContext(processContext, binary)
	cmd.Env = append(os.Environ(), "DATABASE_URL="+loginURL.String(), "LISTEN_ADDR=127.0.0.1:0", "CHAT_GRPC_LISTEN=127.0.0.1:0", "NATS_URL=", "CHAT_REPORT_POLICY_GRPC_LISTEN="+policyAddr, "CHAT_REPORT_POLICY_TLS_CERT_FILE="+cert, "CHAT_REPORT_POLICY_TLS_KEY_FILE="+keyFile, "CHAT_REPORT_POLICY_CLIENT_CA_FILE="+ca, "CHAT_REPORT_POLICY_REPLAY_REDIS_ADDR="+replay.Addr(), `S2S_JWKS_URLS_JSON={"messaging":"`+jwks.URL+`"}`, "S2S_JWKS_CA_FILE="+ca, "S2S_JWKS_CLIENT_CERT_FILE="+clientFile, "S2S_JWKS_CLIENT_KEY_FILE="+clientKeyFile,
		"CHAT_SCOPE_MESSAGING_GRPC_LISTEN="+scopeAddr, "CHAT_SCOPE_MESSAGING_TLS_CERT_FILE="+cert, "CHAT_SCOPE_MESSAGING_TLS_KEY_FILE="+keyFile, "CHAT_SCOPE_MESSAGING_CLIENT_CA_FILE="+ca, "CHAT_SCOPE_MESSAGING_JWKS_URL="+jwks.URL, "CHAT_SCOPE_MESSAGING_JWKS_CA_FILE="+ca, "CHAT_SCOPE_MESSAGING_JWKS_CLIENT_CERT_FILE="+clientFile, "CHAT_SCOPE_MESSAGING_JWKS_CLIENT_KEY_FILE="+clientKeyFile, "CHAT_SCOPE_MESSAGING_REPLAY_REDIS_ADDR="+replay.Addr())
	var logs bytes.Buffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	require.NoError(t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		cancelProcess()
		select {
		case <-done:
			if t.Failed() {
				t.Logf("actual Chat process startup: %s", logs.String())
			}
		case <-time.After(5 * time.Second):
			t.Error("own Chat process did not exit")
		}
	})
	connect := func(addr string, certs []tls.Certificate) *grpc.ClientConn {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: certs})))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, conn.Close()) })
		return conn
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "current", PrivateKey: signing})
	require.NoError(t, err)
	policy := chatv1.NewChatServiceClient(connect(policyAddr, []tls.Certificate{clientCert}))
	o := &Owners{Issuer: issuer, ChatPolicy: policy, ChatScope: chatv1.NewChatScopeAuthorityServiceClient(connect(scopeAddr, []tls.Certificate{clientCert}))}
	defer func() {
		if t.Failed() {
			r := &chatv1.CheckMessageReadEntitlementRequest{ChatId: managed.String(), ProfileId: viewer.String(), MessageCreatedAt: timestamppb.New(joined)}
			call, cancel, signErr := o.signed(ctx, "chat", chatv1.ChatService_CheckMessageReadEntitlement_FullMethodName, uuid.NewString(), r)
			if signErr != nil {
				t.Logf("native caller signing diagnostic: %v", signErr)
				return
			}
			defer cancel()
			v, e := policy.CheckMessageReadEntitlement(call, r)
			t.Logf("native caller diagnostic: code=%s allowed=%v error=%v", status.Code(e), v.GetEntitled(), e)
		}
	}()
	require.Eventually(t, func() bool {
		allowed, err := o.ManagedEntitled(ctx, managed, viewer, joined)
		return err == nil && allowed
	}, 15*time.Second, 100*time.Millisecond, "actual Chat main readiness")
	var servingConnections int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND usename=$1`, loginName).Scan(&servingConnections))
	require.Positive(t, servingConnections, "only the actual Chat subprocess opens this independent serving LOGIN")
	allowed, err := o.ManagedEntitled(taintedContext(), managed, viewer, joined.Add(-time.Microsecond))
	require.NoError(t, err)
	require.False(t, allowed)
	allowed, err = o.ManagedEntitled(taintedContext(), managed, uuid.New(), joined)
	require.NoError(t, err)
	require.False(t, allowed)
	o.ChatPolicy = changedPolicyBody{actual: policy}
	_, err = o.ManagedEntitled(ctx, managed, viewer, joined)
	require.Equal(t, codes.Unavailable, status.Code(err))
	o.ChatPolicy = policy
	before, err := o.ReadChat(taintedContext(), space, chat)
	require.NoError(t, err)
	generation := before.ReportAttachmentGeneration
	_, err = db.Pool.Exec(ctx, `UPDATE chats SET type='channel',creator_profile_id=$2 WHERE id=$1`, chat, uuid.New())
	require.NoError(t, err)
	changedScope, err := o.ReadChat(ctx, space, chat)
	require.NoError(t, err)
	require.NotEqual(t, before.ChatScopeIncarnation, changedScope.ChatScopeIncarnation)
	require.Equal(t, generation, changedScope.ReportAttachmentGeneration)
	unchanged, err := o.ReadAttachment(ctx, chat, space, generation)
	require.NoError(t, err)
	require.Equal(t, chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNCHANGED, unchanged.Disposition)
	_, err = db.Pool.Exec(ctx, `UPDATE chats SET space_id=$2 WHERE id=$1`, chat, otherSpace)
	require.NoError(t, err)
	_, err = o.ReadChat(ctx, space, chat)
	require.ErrorIs(t, err, ErrNotVisible)
	retired, err := o.ReadAttachment(ctx, chat, space, generation)
	require.NoError(t, err)
	require.Equal(t, chatv1.SpaceReportAttachmentRetirementReason_SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_MOVED, retired.Reason)
	require.True(t, canonicalID(retired.OwnerEventId))
	_, err = db.Pool.Exec(ctx, `UPDATE chats SET space_id=$2 WHERE id=$1`, chat, space)
	require.NoError(t, err)
	returned, err := o.ReadChat(ctx, space, chat)
	require.NoError(t, err)
	require.Greater(t, returned.ReportAttachmentGeneration, generation)
	oldAgain, err := o.ReadAttachment(ctx, chat, space, generation)
	require.NoError(t, err)
	require.Equal(t, retired.OwnerEventId, oldAgain.OwnerEventId)
	require.Equal(t, retired.OwnerRevision, oldAgain.OwnerRevision)
	o.ChatPolicy = chatv1.NewChatServiceClient(connect(policyAddr, nil))
	_, err = o.ManagedEntitled(ctx, managed, viewer, joined)
	require.Equal(t, codes.Unavailable, status.Code(err))
	cancelProcess()
	select {
	case <-done:
		done <- nil
	case <-time.After(5 * time.Second):
		t.Fatal("actual owned Chat did not stop")
	}
}

func ownerChatTLSMaterial(t *testing.T) (string, string, string, tls.Certificate, *x509.CertPool) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "memberauthority-test-ca"}, IsCA: true, BasicConstraintsValid: true,
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
