package sourcecustody

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
	"voice/backend/user/internal/identityassociation"
)

// A separately running monitor must fence the actual writer after the control
// leader disappears. This fixture proves no deployment or external supervision.
func TestWatchActivationFencesWriterAfterLeaderLoss(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated TLS PostgreSQL servers and a child process")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	integrationtest.ConfigureDockerTesting()
	controlURL, control := recoveryTLSDatabase(t, ctx, "watch-control")
	adminURL, admin := recoveryTLSDatabase(t, ctx, "watch-source")
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_source_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	dir, keys := migrationFixture(t)
	require.NoError(t, InitializeClosedSource(ctx, control, admin, request, physical, dir, keys))
	_, err = admin.Exec(ctx, `CREATE ROLE custody_app NOLOGIN PASSWORD 'fixture_only'; GRANT CONNECT ON DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` TO custody_app`)
	require.NoError(t, err)
	isolateWriterDatabases(t, admin, physical.Database)
	_, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	u, err := url.Parse(adminURL)
	require.NoError(t, err)
	u.Path = "/" + physical.Database
	source, err := openPinnedPostgres(ctx, u.String())
	require.NoError(t, err)
	_, err = source.Exec(ctx, `GRANT USAGE ON SCHEMA public TO custody_app;
GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO custody_app;
GRANT USAGE,SELECT,UPDATE ON ALL SEQUENCES IN SCHEMA public TO custody_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO custody_app;
GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO custody_app`)
	require.NoError(t, err)
	var binding Binding
	var digest []byte
	require.NoError(t, control.QueryRow(ctx, `SELECT epoch,source_id,history_generation,user_source_generation,keyset_digest FROM user_custody_state WHERE singleton`).Scan(&binding.Epoch, &binding.SourceID, &binding.HistoryGeneration, &binding.UserSourceGeneration, &digest))
	copy(binding.KeysetDigest[:], digest)
	identities, err := identityassociation.NewStore(source, keys, binding.HistoryGeneration)
	require.NoError(t, err)
	require.NoError(t, identities.ActivateGuards(ctx))
	source.Close()
	_, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`)
	require.NoError(t, err)
	source, err = openPinnedPostgres(ctx, u.String())
	require.NoError(t, err)
	defer source.Close()
	controller, err := Start(ctx, control, binding)
	require.NoError(t, err)
	defer func() { _ = controller.Shutdown(context.Background()) }()
	challenge, err := controller.ActivationChallenge()
	require.NoError(t, err)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	claim := OpenAttestation{Challenge: challenge, ControlSystem: controller.controlSystem, ControlDatabase: controller.controlDatabase, ControlDatabaseOID: controller.controlDatabaseOID, Binding: binding, Physical: physical, WriterRole: "custody_app", SourceEvidenceDigest: [32]byte{3}, WriterManifestDigest: [32]byte{1}, AuthEvidenceDigest: [32]byte{2}, IssuedAtUnix: now.Unix(), ExpiresAtUnix: now.Add(4 * time.Minute).Unix()}
	claim, err = SignOpenAttestation(claim, private)
	require.NoError(t, err)
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	require.NoError(t, controller.OpenQualified(ctx, admin, source, keys, claim, public))
	writerCfg, err := pgxpool.ParseConfig(u.String())
	require.NoError(t, err)
	writerCfg.ConnConfig.User, writerCfg.ConnConfig.Password = "custody_app", "fixture_only"
	writer, err := pgxpool.NewWithConfig(ctx, writerCfg)
	require.NoError(t, err)
	defer writer.Close()
	require.NoError(t, writer.Ping(ctx))
	binary := filepath.Join(t.TempDir(), "sourcecustody.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/sourcecustody")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "%s", output)
	child := exec.CommandContext(ctx, binary, "watch-activation")
	for _, item := range os.Environ() {
		name, _, _ := strings.Cut(item, "=")
		if !strings.HasPrefix(strings.ToUpper(name), "PG") && !strings.HasPrefix(name, "CUSTODY_") {
			child.Env = append(child.Env, item)
		}
	}
	child.Env = append(child.Env, "CUSTODY_CONTROL_DATABASE_URL="+controlURL, "CUSTODY_SOURCE_ADMIN_DATABASE_URL="+adminURL,
		"CUSTODY_CONTROL_SYSTEM_ID="+request.ControlSystemID, "CUSTODY_SOURCE_SYSTEM_ID="+request.SourceSystemID,
		"CUSTODY_SOURCE_DATABASE="+request.Database, "CUSTODY_SOURCE_DATABASE_OID="+strconv.FormatUint(uint64(physical.DatabaseOID), 10),
		"CUSTODY_SOURCE_ID="+request.SourceID, "CUSTODY_EPOCH=1", "CUSTODY_WRITER_ROLE=custody_app")
	wrong := exec.CommandContext(ctx, binary, "watch-activation")
	for _, item := range child.Env {
		if strings.HasPrefix(item, "CUSTODY_SOURCE_DATABASE_OID=") {
			item = "CUSTODY_SOURCE_DATABASE_OID=" + strconv.FormatUint(uint64(physical.DatabaseOID)+1, 10)
		}
		wrong.Env = append(wrong.Env, item)
	}
	require.Error(t, wrong.Run(), "a mismatched physical pin must not start a watcher")
	require.NoError(t, writer.Ping(ctx), "a mismatched watcher must not fence the actual writer")
	stdout, err := child.StdoutPipe()
	require.NoError(t, err)
	child.Stderr = io.Discard
	require.NoError(t, child.Start())
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	joined := false
	t.Cleanup(func() {
		if !joined {
			_ = child.Process.Kill()
			<-done
		}
	})
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			ready <- scanner.Text()
		} else {
			close(ready)
		}
	}()
	select {
	case line := <-ready:
		require.Equal(t, "WATCHING", line, "independent monitor must start while leader is live")
	case err := <-done:
		joined = true
		t.Fatalf("monitor exited before readiness: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatal("monitor did not become ready")
	}
	var terminated bool
	require.NoError(t, control.QueryRow(ctx, `SELECT pg_terminate_backend($1)`, controller.leader.Conn().PgConn().PID()).Scan(&terminated))
	require.True(t, terminated)
	select {
	case err := <-done:
		joined = true
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("monitor did not fence after leader loss")
	}
	var phase string
	var allowed, login bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERED_CLOSED", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn,(SELECT rolcanlogin FROM pg_roles WHERE rolname='custody_app') FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed, &login))
	require.False(t, allowed)
	require.False(t, login)
	require.Error(t, writer.Ping(ctx), "already authenticated writer must be terminated")
}
