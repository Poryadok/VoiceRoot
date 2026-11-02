package sourcecustody

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Child is invoked only by the owned PostgreSQL crash test below.
func TestOpenCrashChild(t *testing.T) {
	if os.Getenv("VOICE_OPEN_CRASH_CHILD") != "1" {
		return
	}
	ctx := context.Background()
	control, err := pgxpool.New(ctx, os.Getenv("VOICE_OPEN_CONTROL_DSN"))
	require.NoError(t, err)
	defer control.Close()
	admin, err := pgxpool.New(ctx, os.Getenv("VOICE_OPEN_ADMIN_DSN"))
	require.NoError(t, err)
	defer admin.Close()
	trace := &openingCommitTrace{entered: make(chan struct{})}
	config, err := pgxpool.ParseConfig(os.Getenv("VOICE_OPEN_SOURCE_DSN"))
	require.NoError(t, err)
	config.ConnConfig.Tracer = trace
	config.MaxConns = 1
	config.MinConns = 0
	source, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer source.Close()
	var binding Binding
	var digest []byte
	require.NoError(t, control.QueryRow(ctx, `SELECT epoch,source_id,history_generation,user_source_generation,keyset_digest FROM user_custody_state WHERE singleton`).Scan(&binding.Epoch, &binding.SourceID, &binding.HistoryGeneration, &binding.UserSourceGeneration, &digest))
	copy(binding.KeysetDigest[:], digest)
	var physical PhysicalSource
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM user_custody_physical_source WHERE source_id=$1 AND epoch=$2`, binding.SourceID, binding.Epoch).Scan(&physical.SystemIdentifier, &physical.Database, &physical.DatabaseOID))
	controller, err := Start(ctx, control, binding)
	require.NoError(t, err)
	challenge, err := controller.ActivationChallenge()
	require.NoError(t, err)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	claim := OpenAttestation{Challenge: challenge, ControlSystem: controller.controlSystem, ControlDatabase: controller.controlDatabase, ControlDatabaseOID: controller.controlDatabaseOID, Binding: binding, Physical: physical, WriterRole: "custody_app", SourceEvidenceDigest: [32]byte{3}, WriterManifestDigest: [32]byte{1}, AuthEvidenceDigest: [32]byte{2}, IssuedAtUnix: now.Unix(), ExpiresAtUnix: now.Add(4 * time.Minute).Unix()}
	payload, err := OpenAttestationPayload(claim)
	require.NoError(t, err)
	claim.Signature = ed25519.Sign(private, payload)
	_, keys := migrationFixture(t)
	if os.Getenv("VOICE_OPEN_CRASH_PHASE") == "prepare-enabled" {
		controller.afterPrepareEnabled = func() {
			fmt.Println("PREPARE-ENABLED-BLOCKED")
			_ = os.Stdout.Sync()
			select {}
		}
		_ = controller.PrepareOpenSource(ctx, admin, claim, public)
		t.Fatal("preparation returned before the crash point")
	}
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	trace.armed.Store(true)
	result := make(chan error, 1)
	go func() { result <- controller.OpenQualified(ctx, admin, source, keys, claim, public) }()
	select {
	case <-trace.entered:
		fmt.Println("OPEN-COMMIT-BLOCKED")
		_ = os.Stdout.Sync()
	case err := <-result:
		t.Fatalf("opening ended before source commit: %v", err)
	case <-time.After(20 * time.Second):
		t.Fatal("opening did not reach source commit")
	}
	<-result
}

func TestOpenCrashRestartClosesAndFences(t *testing.T) {
	runOpeningCrash(t, "source-commit")
}

func TestPrepareOpenCrashRestartFences(t *testing.T) {
	runOpeningCrash(t, "prepare-enabled")
}

func runOpeningCrash(t *testing.T, crashPhase string) {
	t.Helper()
	ctx, control, admin, source, _, controller, claim, _, _, _ := openSourceFixture(t)
	require.NoError(t, controller.Shutdown(ctx))
	source.Close()
	controlDSN := control.Config().ConnConfig.ConnString()
	adminDSN := admin.Config().ConnConfig.ConnString()
	sourceURL, err := url.Parse(adminDSN)
	require.NoError(t, err)
	sourceURL.Path = "/" + claim.Physical.Database
	sourceURL.RawPath = ""
	command := exec.Command(os.Args[0], "-test.run=^TestOpenCrashChild$")
	command.Env = append(os.Environ(), "VOICE_OPEN_CRASH_CHILD=1", "VOICE_OPEN_CRASH_PHASE="+crashPhase, "VOICE_OPEN_CONTROL_DSN="+controlDSN, "VOICE_OPEN_ADMIN_DSN="+adminDSN, "VOICE_OPEN_SOURCE_DSN="+sourceURL.String())
	stdout, err := command.StdoutPipe()
	require.NoError(t, err)
	command.Stderr = os.Stderr
	require.NoError(t, command.Start())
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	joined := false
	t.Cleanup(func() {
		if !joined {
			_ = command.Process.Kill()
			<-done
		}
	})
	line := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			line <- scanner.Text()
		} else {
			close(line)
		}
	}()
	select {
	case value := <-line:
		if crashPhase == "prepare-enabled" {
			require.Equal(t, "PREPARE-ENABLED-BLOCKED", value)
		} else {
			require.Equal(t, "OPEN-COMMIT-BLOCKED", value)
		}
	case err := <-done:
		joined = true
		t.Fatalf("child exited before the crash point: %v", err)
	case <-time.After(25 * time.Second):
		t.Fatal("child did not reach the crash point")
	}
	var state, phase string
	var login, allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	if crashPhase == "prepare-enabled" {
		require.Equal(t, "CLOSED", state)
	} else {
		require.Equal(t, "OPEN", state)
	}
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	if crashPhase == "prepare-enabled" {
		require.Equal(t, "PREPARING_OPEN", phase)
	} else {
		require.Equal(t, "ACTIVE", phase)
	}
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname='custody_app'`).Scan(&login))
	require.False(t, login, "the source transaction has not committed LOGIN")
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID).Scan(&allowed))
	require.True(t, allowed, "the crash must occur after maintenance access is enabled")
	require.NoError(t, command.Process.Kill())
	select {
	case <-done:
		joined = true
	case <-time.After(10 * time.Second):
		t.Fatal("child did not exit after termination")
	}
	var restarted *Controller
	require.Eventually(t, func() bool {
		restarted, err = Start(ctx, control, claim.Binding)
		return err == nil
	}, 10*time.Second, 50*time.Millisecond)
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, restarted.Shutdown(ctx))
	request := CreationRequest{SourceID: claim.Binding.SourceID, Epoch: claim.Binding.Epoch, ControlSystemID: claim.ControlSystem, SourceSystemID: claim.Physical.SystemIdentifier, Database: claim.Physical.Database}
	require.NoError(t, RecoverInterruptedActivation(ctx, control, admin, request, claim.Physical, claim.WriterRole))
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname='custody_app'`).Scan(&login))
	require.False(t, login)
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERED_CLOSED", phase)
}
