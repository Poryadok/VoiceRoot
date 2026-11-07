package sourcecustody

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func buildProductionSupervisor(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "sourcecustody.exe")
	build := exec.Command("go", "build", "-o", binary, "./cmd/sourcecustody")
	build.Dir = filepath.Join("..", "..")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "build actual custody CLI: %s", output)
	return binary
}

type supervisorProcess struct {
	command *exec.Cmd
	done    chan error
	joined  bool
	stderr  *bytes.Buffer
}

func (p *supervisorProcess) killAndJoin(t *testing.T) {
	t.Helper()
	if p.joined {
		return
	}
	_ = p.command.Process.Kill()
	select {
	case <-p.done:
		p.joined = true
	case <-time.After(10 * time.Second):
		t.Fatal("owned supervisor process did not join")
	}
}

func startProductionSupervisor(t *testing.T, binary, controlURL, adminURL string, claim OpenAttestation, custodian ed25519.PublicKey) (*supervisorClient, *supervisorProcess, *SupervisorClientConfig) {
	t.Helper()
	dir := t.TempDir()
	pki := processTLS(t, dir, "supervisor")
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(public)
	require.NoError(t, err)
	custodianDER, err := x509.MarshalPKIXPublicKey(custodian)
	require.NoError(t, err)
	clientDER, err := x509.MarshalPKCS8PrivateKey(pki.client.PrivateKey)
	require.NoError(t, err)
	decision, err := json.Marshal(claim)
	require.NoError(t, err)
	files := map[string][]byte{
		"monitor-private.pem":   pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}),
		"monitor-public.pem":    pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}),
		"custodian-public.pem":  pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: custodianDER}),
		"controller-client.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pki.client.Certificate[0]}),
		"controller-client.key": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientDER}),
		"decision.json":         decision,
	}
	for name, data := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0600))
	}
	peer := sha256.Sum256(pki.client.Certificate[0])
	env := map[string]string{
		"CUSTODY_SUPERVISOR_LISTEN": "127.0.0.1:0", "CUSTODY_SUPERVISOR_TLS_CERT_FILE": pki.serverCert, "CUSTODY_SUPERVISOR_TLS_KEY_FILE": pki.serverKey, "CUSTODY_SUPERVISOR_CLIENT_CA_FILE": pki.caFile,
		"CUSTODY_SUPERVISOR_CLIENT_CERT_SHA256": hex.EncodeToString(peer[:]), "CUSTODY_SUPERVISOR_DECISION_FILE": filepath.Join(dir, "decision.json"), "CUSTODY_SUPERVISOR_SIGNER_PRIVATE_KEY_FILE": filepath.Join(dir, "monitor-private.pem"),
		"CUSTODY_SUPERVISOR_CUSTODIAN_PUBLIC_KEY_FILE": filepath.Join(dir, "custodian-public.pem"), "CUSTODY_SUPERVISOR_CONTROL_DATABASE_URL": controlURL, "CUSTODY_SUPERVISOR_SOURCE_ADMIN_DATABASE_URL": adminURL,
	}
	command := exec.Command(binary, "supervise")
	for _, item := range os.Environ() {
		name, _, _ := strings.Cut(item, "=")
		if !strings.HasPrefix(strings.ToUpper(name), "PG") && !strings.HasPrefix(name, "CUSTODY_") {
			command.Env = append(command.Env, item)
		}
	}
	for name, value := range env {
		command.Env = append(command.Env, name+"="+value)
	}
	stdout, err := command.StdoutPipe()
	require.NoError(t, err)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	require.NoError(t, command.Start())
	process := &supervisorProcess{command: command, done: make(chan error, 1), stderr: &stderr}
	go func() { process.done <- command.Wait() }()
	t.Cleanup(func() { process.killAndJoin(t) })
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
		if !strings.HasPrefix(line, "SUPERVISOR_READY ") {
			process.killAndJoin(t)
			t.Fatalf("production supervisor never became ready: %s", stderr.String())
		}
		address = strings.TrimPrefix(line, "SUPERVISOR_READY ")
	case err := <-process.done:
		process.joined = true
		t.Fatalf("production supervisor exited before readiness: %v: %s", err, stderr.String())
	case <-time.After(20 * time.Second):
		process.killAndJoin(t)
		t.Fatal("production supervisor readiness timed out")
	}
	cert, err := tls.LoadX509KeyPair(pki.serverCert, pki.serverKey)
	require.NoError(t, err)
	config := &SupervisorClientConfig{URL: "https://" + address, CAFile: pki.caFile, CertFile: filepath.Join(dir, "controller-client.pem"), KeyFile: filepath.Join(dir, "controller-client.key"), PublicKeyFile: filepath.Join(dir, "monitor-public.pem"), ServerCertSHA256: sha256.Sum256(cert.Certificate[0])}
	client, err := newSupervisorClient(config, custodian)
	require.NoError(t, err)
	t.Cleanup(client.client.CloseIdleConnections)
	return client, process, config
}

func openThroughOperator(t *testing.T, h *activationHandler, claim OpenAttestation) int {
	t.Helper()
	encoded, err := json.Marshal(claim)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "https://operator/open", bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	request.TLS = &tls.ConnectionState{HandshakeComplete: true, VerifiedChains: [][]*x509.Certificate{{{}}}}
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		h.controller.mu.Lock()
		t.Logf("operator deny: stopped=%v used=%v prepared=%v authorized=%v", h.controller.stopped, h.controller.activationUsed, h.controller.activationPrepared, h.controller.authorizedOpen)
		h.controller.mu.Unlock()
		if h.supervised != nil {
			select {
			case <-h.supervised.done:
				t.Logf("supervision recovery settled: %v", h.supervised.result)
			default:
				t.Log("supervision recovery pending")
			}
			t.Logf("supervision live=%v", h.supervised.session.live())
		}
	}
	return response.Code
}

func TestProductionSupervisorControlMismatchRequiresHoldAndFreshDecision(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two owned TLS PostgreSQL servers and the production supervisor executable")
	}
	binary := buildProductionSupervisor(t)
	ctx := context.Background()
	controlURL, control := recoveryTLSDatabase(t, ctx, "supervisor-control")
	adminURL, admin := recoveryTLSDatabase(t, ctx, "supervisor-source")
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_source_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	_, _, _, source, keys, controller, claim, public, _, _ := openSourceFixtureDatabases(t, ctx, control, admin, request, true, nil, false)
	client, monitor, _ := startProductionSupervisor(t, binary, controlURL, adminURL, claim, public)
	h := &activationHandler{controller: controller, admin: admin, source: source, keys: keys, signer: public, supervisor: client}
	t.Cleanup(func() {
		if h.supervised != nil {
			_ = h.supervised.stop()
		}
	})
	require.Equal(t, http.StatusNoContent, openThroughOperator(t, h, claim))
	permit, _, err := controller.AcquireAttested(ctx, claim.Binding)
	require.NoError(t, err)
	require.NoError(t, permit.Release(ctx), "release the immutable pin before intentionally changing its record")
	// The exact control binding changes while SQL remains reachable. The real
	// supervisor must detect it; a healthy process/TCP endpoint is insufficient.
	_, err = control.Exec(ctx, `UPDATE user_custody_physical_source SET database_oid=$3 WHERE source_id=$1 AND epoch=$2`, claim.Binding.SourceID, claim.Binding.Epoch, claim.Physical.DatabaseOID+1)
	require.NoError(t, err)
	select {
	case <-h.supervised.session.revoked():
	case <-time.After(8 * time.Second):
		t.Fatal("control mismatch did not revoke admission")
	}
	_, _, err = controller.AcquireAttested(ctx, claim.Binding)
	require.Error(t, err)
	select {
	case <-h.supervised.done:
	case <-time.After(40 * time.Second):
		t.Fatal("HOLD outcome did not settle")
	}
	require.Error(t, h.supervised.result, "a mismatched physical record must never be called recovered")
	var state, phase string
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.NotEqual(t, "RECOVERED_CLOSED", phase)
	require.Equal(t, http.StatusServiceUnavailable, openThroughOperator(t, h, claim), "a restored heartbeat/record cannot revive an incarnation")
	monitor.killAndJoin(t)
	_, err = control.Exec(ctx, `UPDATE user_custody_physical_source SET database_oid=$3 WHERE source_id=$1 AND epoch=$2`, claim.Binding.SourceID, claim.Binding.Epoch, claim.Physical.DatabaseOID)
	require.NoError(t, err)
	require.NoError(t, recoverSupervisedSource(control, admin, claim))
	require.Equal(t, http.StatusServiceUnavailable, openThroughOperator(t, h, claim), "verified recovery still requires a new leader and decision")
}

func TestProductionSupervisorPreparedTransactionRemainsHold(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two owned TLS PostgreSQL servers and production supervisor")
	}
	binary := buildProductionSupervisor(t)
	ctx := context.Background()
	controlURL, control := recoveryTLSDatabase(t, ctx, "prepared-supervisor-control")
	adminURL, admin := recoveryTLSDatabase(t, ctx, "prepared-supervisor-source", "max_prepared_transactions=5")
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_source_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	_, _, _, source, keys, controller, claim, public, _, _ := openSourceFixtureDatabases(t, ctx, control, admin, request, true, nil, false)
	client, monitor, _ := startProductionSupervisor(t, binary, controlURL, adminURL, claim, public)
	h := &activationHandler{controller: controller, admin: admin, source: source, keys: keys, signer: public, supervisor: client}
	t.Cleanup(func() {
		if h.supervised != nil {
			_ = h.supervised.stop()
		}
	})
	require.Equal(t, http.StatusNoContent, openThroughOperator(t, h, claim))
	_, err := source.Exec(ctx, `BEGIN; SELECT 1; PREPARE TRANSACTION 'supervisor_hold_fixture'`)
	require.NoError(t, err)
	select {
	case <-h.supervised.session.revoked():
	case <-time.After(8 * time.Second):
		t.Fatal("prepared transaction did not revoke supervision")
	}
	select {
	case <-h.supervised.done:
	case <-time.After(40 * time.Second):
		t.Fatal("prepared transaction recovery did not settle to HOLD")
	}
	require.ErrorIs(t, h.supervised.result, ErrSupervisionHold)
	var state, phase string
	var prepared int
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.NotEqual(t, "RECOVERED_CLOSED", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT count(*) FROM pg_prepared_xacts WHERE database=$1`, claim.Physical.Database).Scan(&prepared))
	require.Equal(t, 1, prepared)
	require.Equal(t, http.StatusServiceUnavailable, openThroughOperator(t, h, claim))
	select {
	case err := <-monitor.done:
		monitor.joined = true
		require.Error(t, err)
		require.Contains(t, monitor.stderr.String(), "HOLD")
	case <-time.After(40 * time.Second):
		t.Fatal("independent supervisor did not report its HOLD outcome")
	}
	// The owned disposable cluster is discarded with the unresolved prepared
	// transaction intact. This test never fabricates a recovery acknowledgement.
}

func TestProductionSupervisorStalledOpenerRequiresRecoveryBeforeReopen(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two owned TLS PostgreSQL servers and production supervisor")
	}
	binary := buildProductionSupervisor(t)
	ctx := context.Background()
	controlURL, control := recoveryTLSDatabase(t, ctx, "stalled-supervisor-control")
	adminURL, admin := recoveryTLSDatabase(t, ctx, "stalled-supervisor-source")
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_source_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	_, _, _, source, keys, controller, claim, public, _, _ := openSourceFixtureDatabases(t, ctx, control, admin, request, true, nil, false)
	client, monitor, _ := startProductionSupervisor(t, binary, controlURL, adminURL, claim, public)
	h := &activationHandler{controller: controller, admin: admin, source: source, keys: keys, signer: public, supervisor: client}
	t.Cleanup(func() {
		if h.supervised != nil {
			_ = h.supervised.stop()
		}
	})
	entered, released := make(chan struct{}), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(released) }) }
	defer release()
	controller.afterPrepareEnabled = func() { close(entered); <-released }
	body, err := json.Marshal(claim)
	require.NoError(t, err)
	call := httptest.NewRequest(http.MethodPost, "https://operator/open", bytes.NewReader(body))
	call.Header.Set("Content-Type", "application/json")
	call.TLS = &tls.ConnectionState{HandshakeComplete: true, VerifiedChains: [][]*x509.Certificate{{{}}}}
	response := httptest.NewRecorder()
	finished := make(chan struct{})
	go func() { defer close(finished); h.ServeHTTP(response, call) }()
	t.Cleanup(func() {
		release()
		select {
		case <-finished:
		case <-time.After(35 * time.Second):
			t.Error("opening request did not join during cleanup")
		}
	})
	select {
	case <-entered:
	case <-finished:
		t.Fatalf("opening did not reach the controlled stall: %d", response.Code)
	case <-time.After(15 * time.Second):
		t.Fatal("opening never enabled maintenance")
	}
	monitor.killAndJoin(t)
	select {
	case <-h.supervised.session.revoked():
	case <-time.After(8 * time.Second):
		t.Fatal("supervision did not revoke while the opener was stalled")
	}
	var state, phase string
	var allowed, login bool
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "PREPARING_OPEN", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID).Scan(&allowed))
	require.True(t, allowed, "HOLD must not be misreported as a completed physical fence")
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=$1`, claim.WriterRole).Scan(&login))
	require.False(t, login)
	select {
	case <-h.supervised.done:
		t.Fatal("stalled opening has not established a settled recovery outcome")
	default:
	}
	release()
	select {
	case <-finished:
		require.Equal(t, http.StatusServiceUnavailable, response.Code)
	case <-time.After(35 * time.Second):
		t.Fatal("revoked opening did not settle after release")
	}
	select {
	case <-h.supervised.done:
		require.NoError(t, h.supervised.result)
	case <-time.After(40 * time.Second):
		t.Fatal("recovery did not settle after opener release")
	}
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERED_CLOSED", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	require.Equal(t, http.StatusServiceUnavailable, openThroughOperator(t, h, claim), "recovery does not revive the old decision")
}
