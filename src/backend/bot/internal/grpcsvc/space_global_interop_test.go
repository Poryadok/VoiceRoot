package grpcsvc

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
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
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

type globalSpaceReplyFault struct {
	spacev1.SpaceServiceClient
	loseACK, corrupt bool
}

func (c *globalSpaceReplyFault) ApplyBotInstallationRemoval(ctx context.Context, q *spacev1.ApplyBotInstallationRemovalRequest, options ...grpc.CallOption) (*spacev1.ApplyBotInstallationRemovalResponse, error) {
	response, err := c.SpaceServiceClient.ApplyBotInstallationRemoval(ctx, q, options...)
	if err != nil {
		return nil, err
	}
	if c.loseACK {
		c.loseACK = false
		return nil, status.Error(codes.Unavailable, "test lost global Space ACK after real participant commit")
	}
	if c.corrupt {
		response = proto.Clone(response).(*spacev1.ApplyBotInstallationRemovalResponse)
		response.Receipt.GlobalNativeAddsFenced = false
	}
	return response, nil
}

func TestGlobalSpaceActualParticipantLostACKRecovery(t *testing.T) {
	dir := os.Getenv("VOICE_BOT_GLOBAL46_HANDOFF_DIR")
	if dir == "" {
		t.Skip("requires caller-owned immutable native46 serving fixture")
	}
	var ready struct {
		ProtocolVersion int    `json:"protocol_version"`
		SchemaVersion   int    `json:"schema_version"`
		Address         string `json:"address"`
		ServerName      string `json:"server_name"`
		SpaceID         string `json:"space_id"`
		ProfileID       string `json:"profile_id"`
		KeyID           string `json:"key_id"`
		SignerFile      string `json:"signer_key_file"`
		CAFile          string `json:"ca_file"`
		CertFile        string `json:"client_cert_file"`
		KeyFile         string `json:"client_key_file"`
	}
	require.Eventually(t, func() bool {
		raw, err := os.ReadFile(filepath.Join(dir, "ready.json"))
		return err == nil && json.Unmarshal(raw, &ready) == nil
	}, 120*time.Second, 50*time.Millisecond, "own native46 serving fixture readiness")
	require.Equal(t, 1, ready.ProtocolVersion)
	require.Equal(t, 46, ready.SchemaVersion)
	require.NoFileExists(t, filepath.Join(dir, "complete.json"), "fresh handoff directory, never stale completion")
	ca, err := os.ReadFile(ready.CAFile)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	pair, err := tls.LoadX509KeyPair(ready.CertFile, ready.KeyFile)
	require.NoError(t, err)
	connection, err := grpc.NewClient(ready.Address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: ready.ServerName, Certificates: []tls.Certificate{pair}})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	rawKey, err := os.ReadFile(ready.SignerFile)
	require.NoError(t, err)
	block, _ := pem.Decode(rawKey)
	require.NotNil(t, block)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	key, ok := parsed.(*rsa.PrivateKey)
	require.True(t, ok)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: ready.KeyID, PrivateKey: key})
	require.NoError(t, err)
	actual := spacev1.NewSpaceServiceClient(connection)
	fault := &globalSpaceReplyFault{SpaceServiceClient: actual, loseACK: true}
	ctx := t.Context()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join("..", "..", "..", "migrations", "bot_db"))
	database.Run(true, "up")
	st := &store.BotStore{Pool: database.Pool}
	owner, ownerProfile, installer := uuid.New(), uuid.New(), uuid.New()
	space := uuid.MustParse(ready.SpaceID)
	bot, _, err := st.CreateBot(ctx, owner, "Native46 global cleanup", "", `[]`, uuid.MustParse(ready.ProfileID))
	require.NoError(t, err)
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, installer, nil)
	require.NoError(t, err)
	svc := &BotGRPC{Store: st, PrincipalIssuer: issuer, InstallationSpace: actual, CleanupSpace: fault}
	require.NoError(t, svc.addInstallationMember(ctx, intent), "real protected native Add follows committed Bot PREPARING")
	require.NoError(t, st.CompleteInstallation(ctx, intent, nil))
	q := &botv1.DeleteBotRequest{BotId: bot.ID.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	hash, err := principal.RequestHash(q)
	require.NoError(t, err)
	// Real Bot persistence and target transport, controlled already-verified
	// owner admission. This does not attest a public Gateway/Auth session.
	root, err := st.BeginBotDeletionWithAdmission(ctx, bot.ID, owner, store.DeletionAdmission{AccountID: owner, ProfileID: ownerProfile, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_DeleteBot_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	worked, err := svc.ProcessNextBotDeletion(ctx)
	require.True(t, worked)
	require.Equal(t, codes.Unavailable, status.Code(err))
	effects, err := st.BotDeletionEffects(ctx, root.OperationID)
	require.NoError(t, err)
	var frozen store.BotDeletionEffect
	for _, effect := range effects {
		if effect.Kind == "SPACE_CLEANUP" {
			frozen = effect
		}
	}
	require.NotEqual(t, uuid.Nil, frozen.EffectID)
	require.Equal(t, "PENDING", frozen.State)
	require.NotEmpty(t, frozen.RequestBytes)
	require.Empty(t, frozen.ReceiptBytes)
	require.Equal(t, codes.FailedPrecondition, status.Code(svc.addInstallationMember(ctx, intent)), "target terminal already rejects historical Add after lost ACK")
	// Reinstantiate the service/store on the SAME Pool. This is not an OS restart.
	recovered := &BotGRPC{Store: &store.BotStore{Pool: database.Pool}, PrincipalIssuer: issuer, InstallationSpace: actual, CleanupSpace: fault}
	due := func() {
		_, e := database.Pool.Exec(ctx, `UPDATE bot_deletion_operations SET next_attempt_at=clock_timestamp()-interval '1 second' WHERE operation_id=$1`, root.OperationID)
		require.NoError(t, e)
	}
	fault.corrupt = true
	due()
	worked, err = recovered.ProcessNextBotDeletion(ctx)
	require.True(t, worked)
	require.Equal(t, codes.Unavailable, status.Code(err))
	effects, err = st.BotDeletionEffects(ctx, root.OperationID)
	require.NoError(t, err)
	for _, effect := range effects {
		if effect.EffectID == frozen.EffectID {
			require.Equal(t, "PENDING", effect.State)
			require.Empty(t, effect.ReceiptBytes)
			require.Equal(t, frozen.RequestBytes, effect.RequestBytes)
		}
	}
	fault.corrupt = false
	due()
	worked, err = recovered.ProcessNextBotDeletion(ctx)
	require.True(t, worked)
	require.Equal(t, codes.Unavailable, status.Code(err), "real Space completion cannot settle unconfigured Role")
	effects, err = st.BotDeletionEffects(ctx, root.OperationID)
	require.NoError(t, err)
	var receipt spacev1.ApplyBotInstallationRemovalResponse
	for _, effect := range effects {
		if effect.EffectID == frozen.EffectID {
			require.Equal(t, "COMPLETE", effect.State)
			require.Equal(t, frozen.RequestBytes, effect.RequestBytes)
			require.NoError(t, proto.Unmarshal(effect.ReceiptBytes, &receipt))
			require.True(t, receipt.GetReceipt().GetGlobalNativeAddsFenced())
		}
		if effect.Kind == "ROLE_REMOVE" {
			require.Equal(t, "PENDING", effect.State)
			require.Empty(t, effect.ReceiptBytes)
		}
	}
	var rootState string
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT state FROM bot_deletion_operations WHERE operation_id=$1`, root.OperationID).Scan(&rootState))
	require.Equal(t, "RETRY_WAIT", rootState)
	current, err := st.GetBotByID(ctx, bot.ID)
	require.NoError(t, err)
	require.Equal(t, "disabled", current.Status)
	require.NotNil(t, receipt.Receipt)
	completed := map[string]string{"operation_id": frozen.EffectID.String(), "receipt_id": receipt.Receipt.ReceiptId, "bot_id": bot.ID.String(), "decision_id": root.OperationID.String()}
	raw, err := json.Marshal(completed)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "complete.json.tmp"), raw, 0600))
	require.NoError(t, os.Rename(filepath.Join(dir, "complete.json.tmp"), filepath.Join(dir, "complete.json")))
	t.Log("actual native46 participant receipt persisted after lost ACK/corrupt response/exact fresh-JWT recovery; Role/root remains pending")
}
