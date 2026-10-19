package grpcsvc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

type uninstallRoleLostACK struct {
	rolev1.RoleServiceClient
	corrupt bool
}

func (c uninstallRoleLostACK) ApplyBotInstallationRemoval(ctx context.Context, q *rolev1.ApplyBotInstallationRemovalRequest, opts ...grpc.CallOption) (*rolev1.ApplyBotInstallationRemovalResponse, error) {
	r, err := c.RoleServiceClient.ApplyBotInstallationRemoval(ctx, q, opts...)
	if err != nil {
		return nil, err
	}
	if c.corrupt {
		r.Receipt.Purpose = "global_delete"
		return r, nil
	}
	return nil, status.Error(codes.Unavailable, "test loses committed Role response")
}

func TestUninstallRoleActualParticipantLostACKRecovery(t *testing.T) {
	dir := os.Getenv("VOICE_BOT_ROLE_UNINSTALL_HANDOFF_DIR")
	if dir == "" {
		t.Skip("requires owned source-only Role25/Space44 fixture")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "ready.json"))
	require.NoError(t, err)
	var ready struct {
		FixtureOnly  bool      `json:"fixture_only"`
		RoleSchema   int       `json:"role_schema"`
		SpaceSchema  int       `json:"space_schema"`
		RoleSource   string    `json:"role_source"`
		RoleAddress  string    `json:"role_address"`
		ServerName   string    `json:"tls_server_name"`
		CAFile       string    `json:"tls_ca_file"`
		CertFile     string    `json:"tls_client_cert_file"`
		KeyFile      string    `json:"tls_client_key_file"`
		SignerFile   string    `json:"bot_signer_key_file"`
		KeyID        string    `json:"bot_signer_key_id"`
		OwnerAccount string    `json:"owner_account_id"`
		OwnerProfile string    `json:"owner_profile_id"`
		SpaceID      string    `json:"space_id"`
		StopFile     string    `json:"stop_file"`
		ExpiresAt    time.Time `json:"expires_at"`
	}
	require.NoError(t, json.Unmarshal(raw, &ready))
	require.True(t, ready.FixtureOnly)
	require.Equal(t, 25, ready.RoleSchema)
	require.Equal(t, 44, ready.SpaceSchema)
	require.Equal(t, "eef1f09afb3e7c82565473e8ed894c312cacc0d3", ready.RoleSource)
	require.True(t, time.Now().Before(ready.ExpiresAt))
	t.Cleanup(func() { require.NoError(t, os.WriteFile(ready.StopFile, []byte("stop"), 0600)) })
	ca, err := os.ReadFile(ready.CAFile)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	pair, err := tls.LoadX509KeyPair(ready.CertFile, ready.KeyFile)
	require.NoError(t, err)
	connection, err := grpc.NewClient(ready.RoleAddress, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: ready.ServerName, Certificates: []tls.Certificate{pair}})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	key, err := principal.LoadRSAPrivateKeyFile(ready.SignerFile)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: ready.KeyID, PrivateKey: key})
	require.NoError(t, err)
	ctx := t.Context()
	actual := rolev1.NewRoleServiceClient(connection)
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join("..", "..", "..", "migrations", "bot_db"))
	database.Run(true, "up")
	st := &store.BotStore{Pool: database.Pool}
	owner, manager, space := uuid.MustParse(ready.OwnerAccount), uuid.MustParse(ready.OwnerProfile), uuid.MustParse(ready.SpaceID)
	svc := &BotGRPC{Store: st, PrincipalIssuer: issuer, InstallationRole: actual, CleanupRole: uninstallRoleLostACK{RoleServiceClient: actual}, InstallationSpace: uninstallMessagingNativeACK{}}
	// Real protected current manageBots decision precedes the lower fixture's
	// committed initiating facts; those facts are not a Gateway/Auth attestation.
	require.NoError(t, svc.checkInstallationPermission(ctx, space, manager))
	bot, _, err := st.CreateBot(ctx, owner, "Actual Role uninstall", "", `[]`, uuid.New())
	require.NoError(t, err)
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, manager, nil)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(ctx, intent, nil))
	q := &botv1.UninstallBotFromSpaceRequest{BotId: bot.ID.String(), SpaceId: space.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	hash, err := principal.RequestHash(q)
	require.NoError(t, err)
	_, err = st.BeginInstallationCancellationWithAdmission(ctx, bot.ID, space, manager, store.DeletionAdmission{AccountID: owner, ProfileID: manager, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_UninstallBotFromSpace_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	root, err := st.UninstallCleanup(ctx, intent.InstallationID)
	require.NoError(t, err)
	worked, err := svc.ProcessNextBotDeletion(ctx)
	require.True(t, worked)
	require.Equal(t, codes.Unavailable, status.Code(err))
	root, err = st.UninstallCleanup(ctx, intent.InstallationID)
	require.NoError(t, err)
	require.Equal(t, "RETRY_WAIT", root.State)
	children, err := st.LifecycleEffects(ctx, root)
	require.NoError(t, err)
	var frozen []byte
	for _, child := range children {
		if child.Kind == "ROLE_REMOVE" {
			require.Equal(t, "PENDING", child.State)
			require.Empty(t, child.ReceiptBytes)
			frozen = append([]byte(nil), child.RequestBytes...)
		}
	}
	require.NotEmpty(t, frozen)
	restarted := &BotGRPC{Store: &store.BotStore{Pool: database.Pool}, PrincipalIssuer: issuer, CleanupRole: uninstallRoleLostACK{RoleServiceClient: actual, corrupt: true}}
	due := func() {
		_, e := database.Pool.Exec(ctx, `UPDATE bot_uninstall_operations SET next_attempt_at=clock_timestamp()-interval '1 second' WHERE operation_id=$1`, root.OperationID)
		require.NoError(t, e)
	}
	due()
	worked, err = restarted.ProcessNextBotDeletion(ctx)
	require.True(t, worked)
	require.Equal(t, codes.Unavailable, status.Code(err))
	restarted.CleanupRole = actual
	due()
	worked, err = restarted.ProcessNextBotDeletion(ctx)
	require.True(t, worked)
	require.NoError(t, err)
	root, err = st.UninstallCleanup(ctx, intent.InstallationID)
	require.NoError(t, err)
	require.Equal(t, "COMPLETE", root.State)
	children, err = st.LifecycleEffects(ctx, root)
	require.NoError(t, err)
	for _, child := range children {
		require.Equal(t, "COMPLETE", child.State)
		if child.Kind == "ROLE_REMOVE" {
			require.Equal(t, frozen, child.RequestBytes)
			var response rolev1.ApplyBotInstallationRemovalResponse
			require.NoError(t, proto.Unmarshal(child.ReceiptBytes, &response))
			require.Equal(t, "space_uninstall", response.Receipt.Purpose)
			require.Zero(t, response.Receipt.DeletedRoleCount, "fixture has no Bot-created Role rows; no deletion-effect claim")
			require.Zero(t, response.Receipt.RemovedAssignmentCount)
		}
	}
	current, err := st.GetBotByID(ctx, bot.ID)
	require.NoError(t, err)
	require.Equal(t, "live", current.Status)
	var globals int
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_operations WHERE bot_id=$1`, bot.ID).Scan(&globals))
	require.Zero(t, globals)
	replacement, err := st.PrepareInstallation(ctx, bot.ID, space, manager, nil)
	require.NoError(t, err)
	require.NotEqual(t, intent.InstallationID, replacement.InstallationID)
	require.Greater(t, replacement.Generation, intent.Generation)
}
