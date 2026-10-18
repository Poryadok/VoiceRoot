package grpcsvc

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"io"
	"math/big"
	"net"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	moderationv1 "voice.app/voice/moderation/v1"
	"voice/backend/moderation/internal/reportintake"
	"voice/backend/moderation/internal/reporttargetclient"
	"voice/backend/moderation/internal/store"
	"voice/backend/pkg/principal"
)

func reportTargetProofTLS(t *testing.T) (tls.Certificate, string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return certificate, certPath, keyPath
}

// TestReportIntakeFourOwnerRuntime runs real owner handlers/stores in separate
// Linux subprocesses, their mTLS/JWKS/replay boundary, the signed client, durable
// intake/worker and real JetStream dispatch. Auth deletion is an explicit false
// fixture; its current-authority outage/deletion checks are separate User tests.
func TestReportIntakeFourOwnerRuntime(t *testing.T) {
	binaries := os.Getenv("REPORT_TARGET_PROOF_BINARY_DIR")
	if binaries == "" {
		t.Skip("compiled Linux owner fixtures required")
	}
	require.Equal(t, "linux", os.Getenv("REPORT_TARGET_PROOF_OS"))
	ctx := context.Background()
	pool := reportIntakePool(t)
	certificate, certPath, keyPath := reportTargetProofTLS(t)
	active, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks := httptest.NewUnstartedServer(principal.JWKSHandlerKeys([]principal.JWKSKey{{KeyID: "active", PublicKey: &active.PublicKey}, {KeyID: "next", PublicKey: &next.PublicKey}}))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}
	jwks.StartTLS()
	defer jwks.Close()
	replay := miniredis.RunT(t)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "moderation", KeyID: "active", PrivateKey: active})
	require.NoError(t, err)
	procs := map[string]*exec.Cmd{}
	targets := map[string][3]uuid.UUID{}
	var ownerPools []*pgxpool.Pool
	t.Cleanup(func() {
		for _, command := range procs {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
		for _, p := range ownerPools {
			p.Close()
		}
	})
	const prefix = "MODERATION_REPORT_TARGETS_"
	t.Setenv(prefix+"TLS_CA_FILE", certPath)
	t.Setenv(prefix+"CLIENT_CERT_FILE", certPath)
	t.Setenv(prefix+"CLIENT_KEY_FILE", keyPath)
	for _, owner := range []string{"user", "messaging", "space", "story"} {
		schema := "target_proof_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		_, err = pool.Exec(ctx, "CREATE SCHEMA "+schema)
		require.NoError(t, err)
		defer func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE") }()
		parsed, err := url.Parse(os.Getenv("REPORT_INTAKE_TEST_DSN"))
		require.NoError(t, err)
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		ownerPool, err := pgxpool.New(ctx, parsed.String())
		require.NoError(t, err)
		ownerPools = append(ownerPools, ownerPool)
		table := map[string]string{"user": "profiles", "messaging": "messages", "space": "spaces", "story": "stories"}[owner]
		ddl := "CREATE TABLE " + table + "(id uuid PRIMARY KEY,deleted_at timestamptz,account_id uuid,private boolean DEFAULT true,ghost_only boolean DEFAULT true,expired_at timestamptz DEFAULT now());"
		if owner == "user" {
			ddl += "CREATE TABLE user_account_lifecycle(account_id uuid PRIMARY KEY,state text);"
		}
		if owner == "space" {
			ddl += "CREATE TABLE ownership_journal(space_id uuid,state text);CREATE TABLE space_permanent_purge_fences(space_id uuid PRIMARY KEY);CREATE TABLE space_lifecycle_aggregates(space_id uuid PRIMARY KEY,phase text,scheduled_at timestamptz,purge_after timestamptz);"
		}
		_, err = ownerPool.Exec(ctx, ddl)
		require.NoError(t, err)
		ids := [3]uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
		targets[owner] = ids
		_, err = ownerPool.Exec(ctx, "INSERT INTO "+table+"(id,account_id) VALUES($1,$2)", ids[0], uuid.New())
		require.NoError(t, err)
		if owner == "space" {
			_, err = ownerPool.Exec(ctx, "INSERT INTO space_permanent_purge_fences VALUES($1)", ids[2])
		} else {
			_, err = ownerPool.Exec(ctx, "INSERT INTO "+table+"(id,deleted_at,account_id) VALUES($1,now(),$2)", ids[2], uuid.New())
		}
		require.NoError(t, err)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := listener.Addr().String()
		require.NoError(t, listener.Close())
		t.Setenv(prefix+strings.ToUpper(owner)+"_GRPC_ADDR", addr)
		t.Setenv(prefix+strings.ToUpper(owner)+"_TLS_SERVER_NAME", "localhost")
		envPrefix := strings.ToUpper(owner) + "_REPORT_TARGETS_"
		command := exec.Command(filepath.Join(binaries, owner), "-test.run=^TestReportTargetProofServer$", "-test.v", "-test.timeout=35s")
		command.Env = append(os.Environ(), "REPORT_TARGET_PROOF_SERVER=1", "REPORT_TARGET_PROOF_DSN="+parsed.String(), envPrefix+"GRPC_LISTEN="+addr, envPrefix+"TLS_CERT_FILE="+certPath, envPrefix+"TLS_KEY_FILE="+keyPath, envPrefix+"CLIENT_CA_FILE="+certPath, envPrefix+"JWKS_CA_FILE="+certPath, envPrefix+"MODERATION_JWKS_URL="+jwks.URL, envPrefix+"REPLAY_REDIS_ADDR="+replay.Addr())
		stdout, err := command.StdoutPipe()
		require.NoError(t, err)
		command.Stderr = os.Stderr
		require.NoError(t, command.Start())
		procs[owner] = command
		ready := make(chan bool, 1)
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				if scanner.Text() == "REPORT_TARGET_READY" {
					ready <- true
					_, _ = io.Copy(io.Discard, stdout)
					return
				}
			}
			ready <- false
		}()
		select {
		case ok := <-ready:
			require.True(t, ok, owner+" fixture startup")
		case <-time.After(5 * time.Second):
			t.Fatal(owner + " fixture startup timeout")
		}
	}
	client, closeClient, err := reporttargetclient.DialFromEnv(issuer)
	require.NoError(t, err)
	defer closeClient()
	for _, connection := range client.Connections {
		connection.(*grpc.ClientConn).Connect()
	}
	public, closePublic := startModerationGRPCTestServer(t, pool)
	defer closePublic()
	reports := &store.ReportStore{Pool: pool}
	for owner, ids := range targets {
		kind := owner
		if kind == "messaging" {
			kind = "message"
		}
		for _, id := range ids {
			description := "private evidence"
			bounded, cancel := context.WithTimeout(withReporterProfile(ctx, uuid.New()), time.Second)
			response, err := public.CreateReport(bounded, &moderationv1.CreateReportRequest{TargetType: kind, TargetId: id.String(), Category: "spam", Description: &description, EvidenceJson: `{}`})
			cancel()
			require.NoError(t, err)
			require.NotEmpty(t, response.GetReport().GetId())
			require.Empty(t, response.GetReport().GetTargetId())
			require.Empty(t, response.GetReport().GetStatus())
		}
	}
	queue, err := reports.ListReports(ctx, "", 100)
	require.NoError(t, err)
	require.Empty(t, queue)
	worker := &reportintake.Worker{Reports: reports, Targets: client}
	for i := 0; i < 12; i++ {
		worked, err := worker.ValidateOne(ctx)
		require.NoError(t, err)
		require.True(t, worked)
	}
	var valid, missing, deleted, effects int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE validation_state='VALID'),count(*) FILTER(WHERE validation_state='MISSING'),count(*) FILTER(WHERE validation_state='DELETED') FROM reports`).Scan(&valid, &missing, &deleted))
	require.Equal(t, 4, valid)
	require.Equal(t, 4, missing)
	require.Equal(t, 4, deleted)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_intake_outbox`).Scan(&effects))
	require.Equal(t, 8, effects)
	queue, err = reports.ListReports(ctx, "", 100)
	require.NoError(t, err)
	require.Len(t, queue, 4)
	var leaked int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE validation_state IN ('MISSING','DELETED') AND (description IS NOT NULL OR evidence <> '{}'::jsonb)`).Scan(&leaked))
	require.Zero(t, leaked)
	natsURL := os.Getenv("REPORT_INTAKE_TEST_NATS_URL")
	require.NotEmpty(t, natsURL)
	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)
	defer nc.Close()
	js, err := nc.JetStream()
	require.NoError(t, err)
	for _, cfg := range []*nats.StreamConfig{{Name: "moderation_events", Subjects: []string{"moderation.report_created", "moderation.sanction_applied", "moderation.appeal_submitted"}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour}, {Name: "analytics_events", Subjects: []string{"analytics.>"}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour}} {
		_ = js.DeleteStream(cfg.Name)
		_, err = js.AddStream(cfg)
		require.NoError(t, err)
		defer js.DeleteStream(cfg.Name)
	}
	publisher, err := reportintake.NewPublisher(natsURL)
	require.NoError(t, err)
	defer publisher.Close()
	for i := 0; i < 8; i++ {
		delivered, err := reports.DeliverIntakeEvent(ctx, publisher.Publish)
		require.NoError(t, err)
		require.True(t, delivered)
	}
	for _, stream := range []string{"moderation_events", "analytics_events"} {
		info, err := js.StreamInfo(stream)
		require.NoError(t, err)
		require.Equal(t, uint64(4), info.State.Msgs)
	}
	// Owner outage cannot affect public admission and does not become not-found.
	require.NoError(t, procs["story"].Process.Kill())
	response, err := public.CreateReport(withReporterProfile(ctx, uuid.New()), &moderationv1.CreateReportRequest{TargetType: "story", TargetId: uuid.NewString(), Category: "spam", EvidenceJson: `{}`})
	require.NoError(t, err)
	worked, err := worker.ValidateOne(ctx)
	require.NoError(t, err)
	require.True(t, worked)
	var pending string
	require.NoError(t, pool.QueryRow(ctx, `SELECT validation_state FROM reports WHERE id=$1`, response.GetReport().GetId()).Scan(&pending))
	require.Equal(t, "PENDING", pending)
}
