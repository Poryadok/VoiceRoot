package testfixture_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"bufio"
	"bytes"
	"encoding/json"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io"
	"os/exec"
	userv1 "voice.app/voice/user/v1"
	messagingfixture "voice/backend/messaging/testfixture"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

// This suite uses the real production TLS/JWKS/verifier/Redis admission and
// actual User handler/database, rather than constructing verified contexts.
func TestMessagingCompactionUsesActualUserDeletionAuthority(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated User subprocess and PostgreSQL")
	}
	ctx, stop := context.WithTimeout(context.Background(), 3*time.Minute)
	defer stop()
	repository := deletionFixtureRepository(t)
	executable := filepath.Join(t.TempDir(), "user-deletion-fixture")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	buildCtx, cancelBuild := context.WithTimeout(ctx, 90*time.Second)
	defer cancelBuild()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", executable, filepath.Join(repository, "src/backend/messaging/testfixture/testdata/user_deletion_fixture/main.go"))
	build.Dir = filepath.Join(repository, "src/backend/user")
	buildOutput, buildErr := build.CombinedOutput()
	require.NoError(t, buildErr, "build isolated User fixture: %s", buildOutput)
	pool := integrationtest.StartPostgres(t, ctx, "deletiontransport", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, repository)
	id := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'transport-profile','4242','transport')`, id, uuid.New())
	require.NoError(t, err)
	cert, roots, certPath, keyPath := deletionTransportIdentity(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks := httptest.NewUnstartedServer(principal.JWKSHandlerKeys([]principal.JWKSKey{{KeyID: "current", PublicKey: &key.PublicKey}, {KeyID: "next", PublicKey: &next.PublicKey}}))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	jwks.StartTLS()
	t.Cleanup(jwks.Close)
	replay := miniredis.RunT(t)

	start := func() (userv1.UserServiceClient, string) {
		payload, err := json.Marshal(struct{ DSN, CertPath, KeyPath, JWKSURL, ReplayAddr string }{pool.Config().ConnString(), certPath, keyPath, jwks.URL, replay.Addr()})
		require.NoError(t, err)
		command := exec.CommandContext(ctx, executable)
		command.Stdin = bytes.NewReader(payload)
		command.Stderr = os.Stderr
		output, err := command.StdoutPipe()
		require.NoError(t, err)
		require.NoError(t, command.Start())
		t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
		reader := bufio.NewReader(output)
		type ready struct {
			address string
			err     error
		}
		startup := make(chan ready, 1)
		go func() { address, err := reader.ReadString('\n'); startup <- ready{address, err} }()
		var address string
		select {
		case result := <-startup:
			require.NoError(t, result.err)
			address = strings.TrimSpace(result.address)
		case <-time.After(15 * time.Second):
			t.Fatal("User fixture readiness timeout")
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		_, _, parseErr := net.SplitHostPort(address)
		require.NoError(t, parseErr)
		go func() { _, _ = io.Copy(io.Discard, reader) }()
		conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{cert}})))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		return userv1.NewUserServiceClient(conn), address
	}
	first, _ := start()
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)

	messagingPool := integrationtest.StartPostgres(t, ctx, "messagingcompaction", "")
	for _, file := range []string{"000001_init.up.sql", "000007_thread_index.up.sql", "000037_message_views.up.sql"} {
		integrationtest.ApplySQLFile(t, ctx, messagingPool, repository, "src/backend/migrations/messaging_db/"+file)
	}
	deleted, frozen, missing := uuid.New(), uuid.New(), uuid.New()
	for i, p := range []uuid.UUID{deleted, frozen} {
		_, err = pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name,deleted_at,frozen_at) VALUES($1,$2,$3,'4242','transport',CASE WHEN $4 THEN now() END,CASE WHEN $5 THEN now() END)`, p, uuid.New(), []string{"deleted-profile", "frozen-profile"}[i], i == 0, i == 1)
		require.NoError(t, err)
	}
	messageID := uuid.New()
	_, err = messagingPool.Exec(ctx, `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content) VALUES($1,$2,'group',$3,'private compaction fixture')`, messageID, uuid.New(), id)
	require.NoError(t, err)
	for _, p := range []uuid.UUID{id, deleted, frozen, missing} {
		_, err = messagingPool.Exec(ctx, `INSERT INTO message_views(message_id,profile_id) VALUES($1,$2)`, messageID, p)
		require.NoError(t, err)
	}
	count := func() int {
		var n int
		require.NoError(t, messagingPool.QueryRow(ctx, `SELECT count(*) FROM message_views`).Scan(&n))
		return n
	}
	require.Equal(t, 4, count())
	// Removing the User authority table emulates a domain outage behind a healthy
	// authenticated transport. No partially validated chunk may be deleted.
	_, err = pool.Exec(ctx, `ALTER TABLE profiles RENAME TO profiles_unavailable`)
	require.NoError(t, err)
	require.Error(t, messagingfixture.CompactProfileViews(ctx, messagingPool, first, signer))
	require.Equal(t, 4, count())
	_, err = pool.Exec(ctx, `ALTER TABLE profiles_unavailable RENAME TO profiles`)
	require.NoError(t, err)
	require.NoError(t, messagingfixture.CompactProfileViews(ctx, messagingPool, first, signer))
	require.Equal(t, 3, count(), "only the explicit soft-deleted profile loses its view")
	var survivors []uuid.UUID
	rows, err := messagingPool.Query(ctx, `SELECT profile_id FROM message_views ORDER BY profile_id`)
	require.NoError(t, err)
	for rows.Next() {
		var p uuid.UUID
		require.NoError(t, rows.Scan(&p))
		survivors = append(survivors, p)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	require.ElementsMatch(t, []uuid.UUID{id, frozen, missing}, survivors)
	require.NoError(t, messagingfixture.CompactProfileViews(ctx, messagingPool, first, signer))
	require.Equal(t, 3, count(), "successful retry is idempotent")
}

func deletionTransportIdentity(t *testing.T) (tls.Certificate, *x509.CertPool, string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "private-deletion-fixture"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	identity, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "identity.crt"), filepath.Join(dir, "identity.key")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))
	return identity, roots, certPath, keyPath
}

// Locate source checkout without relying on process working directory or host paths.
func deletionFixtureRepository(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	for directory := filepath.Dir(file); ; directory = filepath.Dir(directory) {
		if _, err := os.Stat(filepath.Join(directory, "src/backend/user/go.mod")); err == nil {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatalf("repository root not found from %s", file)
		}
	}
}
