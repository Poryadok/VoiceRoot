//go:build spacebanintegration

package grpcsvc

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/usercapture"
)

// The protected Space gRPC receiver, native Ban transaction, signed capture
// client and User process are real. Gateway and Auth remain controlled inputs.
func TestPublicSpaceBanThroughUserProcessCapture(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL and User process")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	owner, ownerAccount := uuid.New(), uuid.New()
	userRoot := filepath.Join(repoRoot(t), "src/backend/user")
	binary := filepath.Join(t.TempDir(), "user-capture-fixture.exe")
	build := exec.CommandContext(ctx, "go", "test", "-c", "-o", binary, "./internal/sourcecustody")
	build.Dir = userRoot
	output, err := build.CombinedOutput()
	require.NoError(t, err, "%s", output)
	child := exec.CommandContext(ctx, binary, "-test.run=^TestUserProcessServesQualifiedSpaceCapture$", "-test.timeout=7m")
	child.Dir = filepath.Join(userRoot, "internal/sourcecustody")
	child.Env = append(os.Environ(), "VOICE_USER_CAPTURE_TEST_SERVE=1", "VOICE_CAPTURE_ACTOR_PROFILE="+owner.String(), "VOICE_CAPTURE_ACTOR_ACCOUNT="+ownerAccount.String())
	stdin, err := child.StdinPipe()
	require.NoError(t, err)
	stdout, err := child.StdoutPipe()
	require.NoError(t, err)
	var stderr bytes.Buffer
	child.Stderr = &stderr
	require.NoError(t, child.Start())
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	joined := false
	t.Cleanup(func() {
		_ = stdin.Close()
		if !joined {
			select {
			case <-done:
			case <-time.After(15 * time.Second):
				_ = child.Process.Kill()
				<-done
			}
		}
	})
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "VOICE_CAPTURE_READY ") {
				ready <- strings.TrimPrefix(scanner.Text(), "VOICE_CAPTURE_READY ")
				return
			}
		}
		close(ready)
	}()
	var line string
	select {
	case line = <-ready:
		require.NotEmpty(t, line, "User fixture ended without an endpoint")
	case err := <-done:
		joined = true
		t.Fatalf("User fixture exited before readiness: %v: %s", err, stderr.String())
	case <-time.After(4 * time.Minute):
		t.Fatal("User fixture readiness timed out")
	}
	var user struct {
		Address     string `json:"address"`
		CA          string `json:"ca"`
		Cert        string `json:"cert"`
		Key         string `json:"key"`
		SpaceSigner string `json:"space_signer"`
		Profile     string `json:"profile"`
		Account     string `json:"account"`
	}
	require.NoError(t, json.Unmarshal([]byte(line), &user))
	profile, err := uuid.Parse(user.Profile)
	require.NoError(t, err)
	account, err := uuid.Parse(user.Account)
	require.NoError(t, err)
	ca, err := os.ReadFile(user.CA)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	cert, err := tls.LoadX509KeyPair(user.Cert, user.Key)
	require.NoError(t, err)
	keyPEM, err := os.ReadFile(user.SpaceSigner)
	require.NoError(t, err)
	block, rest := pem.Decode(keyPEM)
	require.NotNil(t, block)
	require.Empty(t, rest)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	key, ok := parsed.(*rsa.PrivateKey)
	require.True(t, ok)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	capture, conn, err := usercapture.New(usercapture.TransportConfig{Address: user.Address, ServerName: "localhost", Roots: roots, Certificate: cert}, issuer)
	require.NoError(t, err)
	defer conn.Close()
	f := newNativeModerationFixtureWithProfiles(t, owner, ownerAccount, profile, account)
	f.st.AccountCaptures = capture
	require.NoError(t, f.ban(t, account, nil))
	var bans, members, audit int
	require.NoError(t, f.st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_bans WHERE space_id=$1 AND account_id=$2`, f.space, account).Scan(&bans))
	require.NoError(t, f.st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, f.space, profile).Scan(&members))
	require.NoError(t, f.st.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_banned'`, f.space).Scan(&audit))
	require.Equal(t, 1, bans)
	require.Zero(t, members)
	require.Equal(t, 1, audit)
}
