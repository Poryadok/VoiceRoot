package grpcsvc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/gameprotocol"
	"voice/backend/messaging/internal/s2s"
	"voice/backend/messaging/internal/store"
)

const gameMappingTestPath = "/internal/v1/game-integrations/resource-mappings/authorize-chat"

func TestApplyGameMessageUsesGISMappingClientBeforePermitAndCommit(t *testing.T) {
	for _, allow := range []bool{false, true} {
		name := "deny"
		if allow {
			name = "allow"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			pool := startPostgresForTest(t, ctx)
			applyBaseMessagingMigrations(t, ctx, pool)
			applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000016_game_message_revisions.up.sql"))
			applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000017_game_message_execution_permits.up.sql"))

			now := time.Now().UTC().Truncate(time.Millisecond)
			compact, message, assertion, authPrivate := freshSignedCreate(t, now, allow)
			profileID := uuid.New()
			events := []string{}
			fixture, serverURL := newGISMappingTestClient(t, allow, &events)
			permitBase := &testGamePermitIssuer{key: authPrivate, keyID: "auth-current", profileID: profileID, now: now}
			permits := &recordingGamePermitIssuer{inner: permitBase, events: &events}
			chat := &recordingGameChatGuard{gameAuthorityChatGuard: &gameAuthorityChatGuard{profile: profileID, chat: message.ChatID}, events: &events}
			files := &recordingGameFiles{events: &events}
			resourceAuthority, err := s2s.NewGameResourceMappingAuthorizationClient(s2s.GameResourceMappingAuthorizationConfig{
				Endpoint: serverURL, TLSCertFile: fixture.certFile, TLSKeyFile: fixture.keyFile,
				CAFile: fixture.caFile, WorkloadKeyBase64: base64.StdEncoding.EncodeToString(fixture.workloadKey),
			})
			require.NoError(t, err)
			defer resourceAuthority.Close()
			processor := &VerifiedGameMessageProcessor{
				Store:    &store.MessagesStore{Pool: pool},
				AuthKeys: testGameAuthKeys{keys: map[string]*rsa.PublicKey{"auth-current": &authPrivate.PublicKey}, at: now},
				Permits:  permits,
				Bindings: &AuthBackedGameBindingAuthority{Chats: chat, ResourceMappings: resourceAuthority},
				Files:    files, Clock: func() time.Time { return now },
			}
			req := &messagingv1.ApplyGameMessageRequest{CompactJws: compact, DeviceAuthorityAssertion: assertion}
			svc := &MessagingGRPC{GameMessages: processor}
			_, err = svc.ApplyGameMessage(gatewayApplyContext(t, req, nil), req)
			if !allow {
				require.Equal(t, codes.InvalidArgument, status.Code(err))
				require.Equal(t, []string{"mapping"}, events)
				require.Zero(t, permitBase.issued)
				require.Zero(t, chat.calls, "mapping denial must stop before membership")
				require.Zero(t, files.calls)
				assertNoGameMessageMutation(t, ctx, pool, message)
				return
			}

			require.NoError(t, err)
			require.Equal(t, []string{"mapping", "permit", "chat", "file"}, events)
			require.Equal(t, 1, permitBase.issued)
			require.Equal(t, 1, chat.calls)
			require.Equal(t, 1, files.calls)
			var revisions int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM game_message_revisions WHERE message_id=$1`, message.MessageID).Scan(&revisions))
			require.Equal(t, 1, revisions)
			var outcome, completionStatus string
			require.NoError(t, pool.QueryRow(ctx, `SELECT outcome,status FROM game_message_execution_permit_completions WHERE operation_id=$1`, message.OperationID).Scan(&outcome, &completionStatus))
			require.Equal(t, "committed", outcome)
			require.Equal(t, "completed", completionStatus)
		})
	}
}

type recordingGISMappingFixture struct {
	certFile, keyFile, caFile string
	workloadKey               []byte
}

func newGISMappingTestClient(t *testing.T, allow bool, events *[]string) (recordingGISMappingFixture, string) {
	t.Helper()
	ca, caPEM, serverCert, clientCert := createGISMappingTestCertificates(t)
	workloadKey := []byte("0123456789abcdef0123456789abcdef")
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*events = append(*events, "mapping")
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, gameMappingTestPath, r.URL.Path)
		require.NotEmpty(t, r.TLS.PeerCertificates)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var request map[string]string
		require.NoError(t, json.Unmarshal(body, &request))
		require.Len(t, request, 4)
		require.NotEmpty(t, request["application_id"])
		require.NotEmpty(t, request["environment_id"])
		require.NotEmpty(t, request["binding_id"])
		require.NotEmpty(t, request["chat_id"])
		timestamp, nonce := r.Header.Get("X-Voice-Timestamp"), r.Header.Get("X-Voice-Nonce")
		require.Equal(t, workloadRequestSignature(workloadKey, r.Method, r.URL.EscapedPath(), timestamp, nonce, body), r.Header.Get("X-Voice-Signature"))
		responseBody := []byte("{\"allowed\":false}\n")
		if allow {
			responseBody = []byte("{\"allowed\":true,\"mapping_revision\":1}\n")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Voice-Response-Timestamp", timestamp)
		w.Header().Set("X-Voice-Response-Nonce", nonce)
		w.Header().Set("X-Voice-Response-Signature", workloadResponseSignature(workloadKey, http.StatusOK, r.URL.EscapedPath(), timestamp, nonce, responseBody))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(responseBody)
	}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: ca}
	server.StartTLS()
	t.Cleanup(server.Close)
	url, err := url.Parse(server.URL)
	require.NoError(t, err)
	endpoint := "https://localhost:" + url.Port() + gameMappingTestPath
	certFile, keyFile := writeGISMappingClientCert(t, clientCert)
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caFile, caPEM, 0o600))
	return recordingGISMappingFixture{certFile: certFile, keyFile: keyFile, caFile: caFile, workloadKey: workloadKey}, endpoint
}

func assertNoGameMessageMutation(t *testing.T, ctx context.Context, pool *pgxpool.Pool, message gameprotocol.Message) {
	t.Helper()
	var revisions, completions int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM game_message_revisions WHERE message_id=$1`, message.MessageID).Scan(&revisions))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM game_message_execution_permit_completions WHERE operation_id=$1`, message.OperationID).Scan(&completions))
	require.Zero(t, revisions)
	require.Zero(t, completions)
}

type recordingGamePermitIssuer struct {
	inner  *testGamePermitIssuer
	events *[]string
}

func (p *recordingGamePermitIssuer) Issue(ctx context.Context, authority gameprotocol.DeviceAuthority, operation uuid.UUID, body []byte) (string, error) {
	*p.events = append(*p.events, "permit")
	return p.inner.Issue(ctx, authority, operation, body)
}
func (p *recordingGamePermitIssuer) Complete(ctx context.Context, permit, operation uuid.UUID, outcome string) error {
	return p.inner.Complete(ctx, permit, operation, outcome)
}

type recordingGameChatGuard struct {
	*gameAuthorityChatGuard
	events *[]string
}

func (g *recordingGameChatGuard) EnsureMember(ctx context.Context, chat, profile uuid.UUID) error {
	*g.events = append(*g.events, "chat")
	return g.gameAuthorityChatGuard.EnsureMember(ctx, chat, profile)
}

type recordingGameFiles struct {
	events *[]string
	calls  int
}

func (f *recordingGameFiles) VerifyGameAttachmentManifest(context.Context, uuid.UUID, uuid.UUID, []gameprotocol.Attachment) error {
	*f.events = append(*f.events, "file")
	f.calls++
	return nil
}

func workloadRequestSignature(key []byte, method, path, timestamp, nonce string, body []byte) string {
	digest := sha256.Sum256(body)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("v1\n" + method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex.EncodeToString(digest[:])))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func workloadResponseSignature(key []byte, status int, path, timestamp, nonce string, body []byte) string {
	digest := sha256.Sum256(body)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("v1\n" + strconv.Itoa(status) + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex.EncodeToString(digest[:])))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func createGISMappingTestCertificates(t *testing.T) (*x509.CertPool, []byte, tls.Certificate, tls.Certificate) {
	t.Helper()
	now := time.Now()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	server := issueGISMappingCert(t, caCert, caKey, "localhost", nil, []string{"localhost"})
	client := issueGISMappingCert(t, caCert, caKey, "messaging", []string{"spiffe://voice/service/messaging"}, nil)
	return pool, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), server, client
}

func issueGISMappingCert(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn string, uris, dns []string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: cn}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, DNSNames: dns}
	for _, raw := range uris {
		parsed, parseErr := url.Parse(raw)
		require.NoError(t, parseErr)
		template.URIs = append(template.URIs, parsed)
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func writeGISMappingClientCert(t *testing.T, cert tls.Certificate) (string, string) {
	t.Helper()
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "client.pem"), filepath.Join(dir, "client-key.pem")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0o600))
	keyDER, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
	return certPath, keyPath
}
