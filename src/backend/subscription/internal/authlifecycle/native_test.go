package authlifecycle

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// This starts the actual Java source/TLS/JWKS/Redis/PostgreSQL factory, not a Go
// gRPC replacement. Configure the compiled Auth test classpath for native runs.
func TestNativeJavaOwnerAcceptsActualSubscriptionIssuer(t *testing.T) {
	if testing.Short() {
		t.Skip("native Java Auth/Redis/PostgreSQL fixture")
	}
	classpathFile := os.Getenv("VOICE_AUTH_SUBSCRIPTION_NATIVE_CLASSPATH_FILE")
	if classpathFile == "" {
		t.Skip("requires compiled native Auth test fixture classpath")
	}
	deps, err := os.ReadFile(classpathFile)
	require.NoError(t, err)
	auth, err := filepath.Abs("../../../auth")
	require.NoError(t, err)
	cp := filepath.Join(auth, "target", "test-classes") + string(os.PathListSeparator) + filepath.Join(auth, "target", "classes") + string(os.PathListSeparator) + strings.TrimSpace(string(deps))
	require.NotContains(t, cp, "\"")
	require.NotContains(t, cp, "\n")
	tmp := t.TempDir()
	argfile := filepath.Join(tmp, "java.args")
	require.NoError(t, os.WriteFile(argfile, []byte("-cp\n\""+strings.ReplaceAll(cp, "\\", "/")+"\"\nvoice.backend.auth.principal.SubscriptionLifecycleInteropFixture\n"), 0600))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	command := exec.CommandContext(ctx, "java", "@"+argfile)
	command.Env = append(os.Environ(), "VOICE_AUTH_SUBSCRIPTION_FIXTURE_DIR="+tmp)
	input, err := command.StdinPipe()
	require.NoError(t, err)
	output, err := command.StdoutPipe()
	require.NoError(t, err)
	var diagnostics lockedDiagnostics
	command.Stderr = &diagnostics
	require.NoError(t, command.Start())
	t.Cleanup(func() {
		drained := make(chan struct{})
		go func() { _, _ = io.Copy(io.Discard, output); close(drained) }()
		_, _ = fmt.Fprintln(input, "STOP")
		_ = input.Close()
		done := make(chan error, 1)
		go func() { <-drained; done <- command.Wait() }()
		select {
		case err := <-done:
			require.NoError(t, err, "%s", diagnostics.String())
		case <-time.After(20 * time.Second):
			_ = command.Process.Kill()
			<-done
			t.Error("owned Java fixture did not stop")
		}
	})
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var fixture struct {
		Address           string `json:"address"`
		ServerName        string `json:"server_name"`
		CAFile            string `json:"ca_file"`
		CertificateFile   string `json:"certificate_file"`
		KeyFile           string `json:"key_file"`
		ClientURI         string `json:"client_uri"`
		SigningKeyFile    string `json:"signing_key_file"`
		SigningKeyID      string `json:"signing_key_id"`
		AccountID         string `json:"account_id"`
		PostgresContainer string `json:"postgres_container"`
		RedisContainer    string `json:"redis_container"`
	}
	ready := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VOICE_AUTH_SUBSCRIPTION_FIXTURE=") {
			require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "VOICE_AUTH_SUBSCRIPTION_FIXTURE=")), &fixture))
			ready = true
			break
		}
	}
	require.True(t, ready, "native Java fixture did not become ready: %s", diagnostics.String())
	t.Logf("actual Java fixture containers PostgreSQL=%s Redis=%s", fixture.PostgresContainer, fixture.RedisContainer)
	admin, writer, generation := ingressDB(t)
	client, err := New(Config{Address: fixture.Address, ServerName: fixture.ServerName, CAFile: fixture.CAFile, CertificateFile: fixture.CertificateFile, KeyFile: fixture.KeyFile, ClientURI: fixture.ClientURI, SigningKeyFile: fixture.SigningKeyFile, SigningKeyID: fixture.SigningKeyID, Generation: generation})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	observed, err := client.Observe(ctx, fixture.AccountID, 1)
	require.NoError(t, err, "actual common Go service issuer must interoperate with the Java verifier")
	require.Equal(t, fixture.AccountID, observed.Snapshot().GetAccountId())
	require.EqualValues(t, 1, observed.Snapshot().GetHead().GetSourceRevision())
	require.NoError(t, Ingest(ctx, writer, observed))
	require.NoError(t, Ingest(ctx, writer, observed))
	_, err = fmt.Fprintln(input, "RESTORE")
	require.NoError(t, err)
	restored := false
	for scanner.Scan() {
		if scanner.Text() == "VOICE_AUTH_SUBSCRIPTION_RESTORED=true" {
			restored = true
			break
		}
	}
	require.True(t, restored)
	after, err := client.Observe(ctx, fixture.AccountID, 1)
	require.NoError(t, err)
	require.Equal(t, "active", after.Snapshot().GetAccountStatus())
	require.EqualValues(t, 2, after.Snapshot().GetHead().GetSourceRevision())
	require.NoError(t, Ingest(ctx, writer, after))
	var revision int
	var state string
	require.NoError(t, admin.QueryRow(ctx, "SELECT auth_revision,state FROM subscription_auth_account_observations WHERE account_id=$1", fixture.AccountID).Scan(&revision, &state))
	require.Equal(t, 2, revision)
	require.Equal(t, "OBSERVED", state)
	expired, stop := context.WithCancel(ctx)
	stop()
	_, err = client.Observe(expired, fixture.AccountID, 2)
	require.ErrorIs(t, err, ErrUnavailable)
}

type lockedDiagnostics struct {
	mutex  sync.Mutex
	buffer bytes.Buffer
}

func (d *lockedDiagnostics) Write(data []byte) (int, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.buffer.Write(data)
}
func (d *lockedDiagnostics) String() string {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.buffer.String()
}
