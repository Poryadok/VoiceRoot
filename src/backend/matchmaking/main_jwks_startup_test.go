package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const matchmakingJWKSChildMode = "MATCHMAKING_MAIN_JWKS_TEST_CHILD"

func TestMatchmakingJWKSStartsBeforeSpaceProviderReadiness(t *testing.T) {
	if os.Getenv(matchmakingJWKSChildMode) == "1" {
		runMatchmakingJWKSChild()
		return
	}
	databaseURL := matchmakingJWKSDatabase(t)

	keysDir := t.TempDir()
	writeMatchmakingRotationKeys(t, keysDir)
	certFile, keyFile := writeMatchmakingJWKSIdentity(t)
	spaceListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("reserve unavailable Space provider address")
	}
	defer spaceListener.Close()
	spaceAccepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := spaceListener.Accept()
		if acceptErr == nil {
			spaceAccepted <- conn
		}
	}()
	listenProbe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("reserve Matchmaking JWKS test address")
	}
	jwksAddr := listenProbe.Addr().String()
	if err := listenProbe.Close(); err != nil {
		t.Fatal("release Matchmaking JWKS test address")
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal("locate Matchmaking test executable")
	}
	command := exec.Command(executable, "-test.run=^TestMatchmakingJWKSStartsBeforeSpaceProviderReadiness$")
	command.Env = []string{
		matchmakingJWKSChildMode + "=1",
		"MATCHMAKING_TEST_KEYS_DIR=" + keysDir,
		"MATCHMAKING_TEST_TLS_CERT=" + certFile,
		"MATCHMAKING_TEST_TLS_KEY=" + keyFile,
		"MATCHMAKING_TEST_LISTEN_ADDR=" + jwksAddr,
		"MATCHMAKING_TEST_SPACE_ADDR=" + spaceListener.Addr().String(),
		"MATCHMAKING_TEST_DATABASE_URL=" + databaseURL,
	}
	if runtime.GOOS == "windows" {
		command.Env = append(command.Env, "SYSTEMROOT="+os.Getenv("SYSTEMROOT"))
	}
	var childOutput bytes.Buffer
	command.Stdout = &childOutput
	command.Stderr = &childOutput
	if err := command.Start(); err != nil {
		t.Fatal("start Matchmaking process")
	}
	defer func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	}()

	var accepted net.Conn
	select {
	case accepted = <-spaceAccepted:
		defer accepted.Close()
	case <-time.After(8 * time.Second):
		_ = command.Process.Kill()
		_ = command.Wait()
		output := childOutput.String()
		categories := make([]string, 0, 4)
		for marker, name := range map[string]string{
			"Matchmaking principal JWKS listening":        "jwks_listener_started",
			"MatchSquad protected provider configuration": "matchsquad_provider_config_failed",
			"postgres:":            "postgres_initialization_failed",
			"space grpc dial:":     "space_readiness_timed_out",
			"DATABASE_URL not set": "database_branch_skipped",
			"gRPC listening":       "grpc_service_started",
			"listening":            "health_service_started",
		} {
			if strings.Contains(output, marker) {
				categories = append(categories, name)
			}
		}
		t.Fatalf("Matchmaking did not attempt its unready Space provider connection; startup_categories=%v", categories)
	}

	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatal("load test JWKS identity")
	}
	root, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		t.Fatal("parse test JWKS trust root")
	}
	trustedRoots := x509.NewCertPool()
	trustedRoots.AddCert(root)
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    trustedRoots,
		}},
	}
	requestCtx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet,
		"https://"+jwksAddr+"/internal/v1/principal/jwks.json", nil)
	if err != nil {
		t.Fatal("construct JWKS request")
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal("Matchmaking JWKS was not available while Space gRPC remained unready")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Matchmaking JWKS returned HTTP %d while Space gRPC remained unready", response.StatusCode)
	}
}

func runMatchmakingJWKSChild() {
	keysDir := os.Getenv("MATCHMAKING_TEST_KEYS_DIR")
	certFile := os.Getenv("MATCHMAKING_TEST_TLS_CERT")
	keyFile := os.Getenv("MATCHMAKING_TEST_TLS_KEY")
	listenAddr := os.Getenv("MATCHMAKING_TEST_LISTEN_ADDR")
	spaceAddr := os.Getenv("MATCHMAKING_TEST_SPACE_ADDR")
	databaseURL := os.Getenv("MATCHMAKING_TEST_DATABASE_URL")
	// Winsock needs SYSTEMROOT even when the service fixture isolates its env.
	systemRoot := os.Getenv("SYSTEMROOT")
	os.Clearenv()
	if runtime.GOOS == "windows" {
		_ = os.Setenv("SYSTEMROOT", systemRoot)
	}
	for key, value := range map[string]string{
		"DATABASE_URL":                              databaseURL,
		"SPACE_GRPC_ADDR":                           spaceAddr,
		"LISTEN_ADDR":                               "127.0.0.1:0",
		"MATCHMAKING_GRPC_LISTEN":                   "127.0.0.1:0",
		"MATCHMAKING_MATCH_SQUAD_CHAT_GRPC_ADDR":    spaceAddr,
		"MATCHMAKING_MATCH_SQUAD_VOICE_GRPC_ADDR":   spaceAddr,
		"MATCHMAKING_MATCH_SQUAD_CHAT_CA_FILE":      certFile,
		"MATCHMAKING_MATCH_SQUAD_VOICE_CA_FILE":     certFile,
		"MATCHMAKING_MATCH_SQUAD_CLIENT_CERT_FILE":  certFile,
		"MATCHMAKING_MATCH_SQUAD_CLIENT_KEY_FILE":   keyFile,
		"MATCHMAKING_MATCH_SQUAD_CHAT_SERVER_NAME":  "matchmaking",
		"MATCHMAKING_MATCH_SQUAD_VOICE_SERVER_NAME": "matchmaking",
		"MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR":    keysDir,
		"MATCHMAKING_PRINCIPAL_ACTIVE_KID":          "current",
		"MATCHMAKING_PRINCIPAL_JWKS_LISTEN_ADDR":    listenAddr,
		"MATCHMAKING_PRINCIPAL_JWKS_TLS_CERT_FILE":  certFile,
		"MATCHMAKING_PRINCIPAL_JWKS_TLS_KEY_FILE":   keyFile,
		"GRPC_DIAL_TIMEOUT":                         "10s",
	} {
		if err := os.Setenv(key, value); err != nil {
			os.Exit(2)
		}
	}
	main()
}

// The compatibility root must validate actual pre-cut database admission before
// publishing JWKS. Keep remote readiness unready, but provide that real local
// prerequisite rather than depending on a pool that never connects.
func matchmakingJWKSDatabase(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("BE011_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("real PostgreSQL unavailable: set BE011_TEST_PG_DSN")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("open local PostgreSQL fixture")
	}
	name := "mm_jwks_" + uuid.NewString()[:8]
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+name); err != nil {
		admin.Close()
		t.Fatal("create isolated JWKS fixture database")
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DROP DATABASE `+name)
		admin.Close()
	})
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatal("parse local PostgreSQL fixture")
		}
		parsed.Path, parsed.RawPath = "/"+name, ""
		query := parsed.Query()
		query.Del("dbname")
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return dsn + " dbname=" + name
}

func writeMatchmakingRotationKeys(t *testing.T, directory string) {
	t.Helper()
	for _, kid := range []string{"current", "next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal("generate Matchmaking signing key")
		}
		der, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			t.Fatal("encode Matchmaking signing key")
		}
		path := filepath.Join(directory, kid+".pem")
		if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
			t.Fatal("write Matchmaking signing key")
		}
	}
}

func writeMatchmakingJWKSIdentity(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal("generate Matchmaking JWKS TLS key")
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		t.Fatal("generate Matchmaking JWKS TLS serial")
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "matchmaking"},
		DNSNames:              []string{"matchmaking"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal("create Matchmaking JWKS TLS certificate")
	}
	root := t.TempDir()
	certFile, keyFile := filepath.Join(root, "tls.crt"), filepath.Join(root, "tls.key")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal("write Matchmaking JWKS TLS certificate")
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal("encode Matchmaking JWKS TLS key")
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal("write Matchmaking JWKS TLS key")
	}
	return certFile, keyFile
}
