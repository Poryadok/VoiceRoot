//go:build spacebanintegration

package store

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
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
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/memberadmission"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/usercapture"
)

type liveUserCaptureFixture struct {
	Address      string `json:"address"`
	CA           string `json:"ca"`
	Cert         string `json:"cert"`
	Key          string `json:"key"`
	SpaceSigner  string `json:"space_signer"`
	Profile      string `json:"profile"`
	BirthProfile string `json:"birth_profile"`
	Account      string `json:"account"`
}

// This proves the real Space Ban transaction calls the running User binary
// through the signed mTLS capture client and qualified source. Gateway, Role
// and Auth are controlled fixtures, not deployed-service continuity evidence.
func TestNativeBanThroughUserProcessCapture(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL and User process")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	directory := filepath.Join(repoRoot(t), "src/backend/migrations/space_db")
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, directory, "postgres:16-alpine")
	db.Run(true, "goto", "34")
	db.Run(true, "goto", "45")
	transition, err := (&SpaceStore{Pool: db.Pool}).TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	require.True(t, transition.Changed)
	db.Run(true, "goto", "50")
	_, err = (&SpaceStore{Pool: db.Pool}).TransitionNativeMembershipCatalog50(ctx)
	require.NoError(t, err)
	owner := migrateBotFixture51(t, ctx, db.Pool, directory)
	serving := projectionServingLogin(t, ctx, owner)
	st := &SpaceStore{Pool: serving}
	create, actorAccount, actorProfile := signedProjectionCreator(t, st, &spacev1.CreateSpaceRequest{Name: "qualified User capture Ban", Visibility: "public"})
	space, err := create()
	require.NoError(t, err)

	userRoot := filepath.Join(repoRoot(t), "src/backend/user")
	binary := filepath.Join(t.TempDir(), "user-capture-fixture.exe")
	build := exec.CommandContext(ctx, "go", "test", "-c", "-o", binary, "./internal/sourcecustody")
	build.Dir = userRoot
	output, err := build.CombinedOutput()
	require.NoError(t, err, "%s", output)
	child := exec.CommandContext(ctx, binary, "-test.run=^TestUserProcessServesQualifiedSpaceCapture$", "-test.timeout=7m")
	child.Dir = filepath.Join(userRoot, "internal/sourcecustody")
	child.Env = append(os.Environ(), "VOICE_USER_CAPTURE_TEST_SERVE=1", "VOICE_CAPTURE_ACTOR_PROFILE="+actorProfile.String(), "VOICE_CAPTURE_ACTOR_ACCOUNT="+actorAccount.String())
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
	var fixture liveUserCaptureFixture
	require.NoError(t, json.Unmarshal([]byte(line), &fixture))
	targetProfile, err := uuid.Parse(fixture.Profile)
	require.NoError(t, err)
	birthProfile, err := uuid.Parse(fixture.BirthProfile)
	require.NoError(t, err)
	targetAccount, err := uuid.Parse(fixture.Account)
	require.NoError(t, err)
	ca, err := os.ReadFile(fixture.CA)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	certificate, err := tls.LoadX509KeyPair(fixture.Cert, fixture.Key)
	require.NoError(t, err)
	keyPEM, err := os.ReadFile(fixture.SpaceSigner)
	require.NoError(t, err)
	block, rest := pem.Decode(keyPEM)
	require.NotNil(t, block)
	require.Empty(t, rest)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	spaceKey, ok := parsed.(*rsa.PrivateKey)
	require.True(t, ok)
	spaceIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: spaceKey})
	require.NoError(t, err)
	capture, conn, err := usercapture.New(usercapture.TransportConfig{Address: fixture.Address, ServerName: "localhost", Roots: roots, Certificate: certificate}, spaceIssuer)
	require.NoError(t, err)
	defer conn.Close()
	st.AccountCaptures = capture

	for _, member := range []uuid.UUID{targetProfile, birthProfile} {
		intent, err := st.PrepareMemberAdmission(ctx, MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, ProfileID: member, AccountID: targetAccount})
		require.NoError(t, err)
		role := uuid.New()
		receipt, err := memberadmission.EnsureReceiptFingerprint(intent.EnsureBinding(), role.String())
		require.NoError(t, err)
		_, err = st.PublishMemberAdmission(ctx, intent.Input.OperationID, role, receipt[:], nil)
		require.NoError(t, err)
	}
	ban := &spacev1.BanMemberRequest{SpaceId: space.ID.String(), AccountId: targetAccount.String()}
	operation := uuid.New()
	nativeBanIntentField(t, ban, operation)
	gatewayKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	gateway, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: gatewayKey})
	require.NoError(t, err)
	hash, err := principal.RequestHash(ban)
	require.NoError(t, err)
	id := uuid.NewString()
	token, err := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_BanMember_FullMethodName, RequestID: id, RequestHash: hash, AccountID: actorAccount.String(), ProfileID: actorProfile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &gatewayKey.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: func(context.Context, string, int64) error { return nil }}
	_, err = verifier.Unary()(incoming, ban, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_BanMember_FullMethodName}, func(verified context.Context, _ any) (any, error) {
		call, err := lifecycleprincipal.RequireModerationBan(verified, ban)
		if err != nil {
			return nil, err
		}
		return nil, st.BanAccount(verified, AccountBanInput{SpaceID: space.ID, AccountID: targetAccount, ActorProfileID: actorProfile, InitiatingCall: call, Request: ban, Authorize: func(context.Context) error { return nil }})
	})
	require.NoError(t, err, "Space Ban must finish through the actual User capture endpoint")
	var banned, remaining, audit, matched int
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_bans WHERE space_id=$1 AND account_id=$2`, space.ID, targetAccount).Scan(&banned))
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=ANY($2::uuid[])`, space.ID, []uuid.UUID{targetProfile, birthProfile}).Scan(&remaining))
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_banned'`, space.ID).Scan(&audit))
	require.NoError(t, serving.QueryRow(ctx, `SELECT cardinality(matched_profiles) FROM space_account_ban_operations WHERE actor_profile_id=$1 AND client_operation_id=$2`, actorProfile, operation).Scan(&matched))
	require.Equal(t, 1, banned)
	require.Zero(t, remaining)
	require.Equal(t, 1, audit)
	require.Equal(t, 2, matched)
}
