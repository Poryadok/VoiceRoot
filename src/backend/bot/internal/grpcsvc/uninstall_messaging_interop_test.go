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
	messagingv1 "voice.app/voice/messaging/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

func TestUninstallMessagingActualParticipantLostACKRecovery(t *testing.T) {
	dir := os.Getenv("VOICE_BOT_PIN_UNINSTALL_HANDOFF_DIR")
	if dir == "" {
		t.Skip("requires own private Messaging source fixture")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "ready.json"))
	require.NoError(t, err)
	var ready struct {
		Address, ServerName, CAFile, CertFile, KeyFile, SignerFile, ChatID, SpaceID, ActorID string
		Observation                                                                          *messagingv1.BotRemovalChatScopeObservation
	}
	require.NoError(t, json.Unmarshal(raw, &ready))
	ca, err := os.ReadFile(ready.CAFile)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	pair, err := tls.LoadX509KeyPair(ready.CertFile, ready.KeyFile)
	require.NoError(t, err)
	connection, err := grpc.NewClient(ready.Address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: ready.ServerName, Certificates: []tls.Certificate{pair}})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	keyBytes, err := os.ReadFile(ready.SignerFile)
	require.NoError(t, err)
	block, _ := pem.Decode(keyBytes)
	require.NotNil(t, block)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	key, ok := parsed.(*rsa.PrivateKey)
	require.True(t, ok)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	ctx := t.Context()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join("..", "..", "..", "migrations", "bot_db"))
	database.Run(true, "up")
	st := &store.BotStore{Pool: database.Pool}
	owner, installer := uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Messaging recovery", "", `[]`, uuid.MustParse(ready.ActorID))
	require.NoError(t, err)
	observation, err := store.NewChatScopeObservation(ready.Observation.Request, ready.Observation.Response, true)
	require.NoError(t, err)
	space, chat := uuid.MustParse(ready.SpaceID), uuid.MustParse(ready.ChatID)
	intent, err := st.PrepareInstallation(store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{observation}), bot.ID, space, installer, []uuid.UUID{chat})
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(ctx, intent, []uuid.UUID{chat}))
	deleteRequest := &botv1.UninstallBotFromSpaceRequest{BotId: bot.ID.String(), SpaceId: space.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(deleteRequest)
	require.NoError(t, err)
	hash, err := principal.RequestHash(deleteRequest)
	require.NoError(t, err)
	// Initiating manager facts and native Empty are controlled lower seams.
	// Actual purpose-specific Messaging authority, Chat SQL source and receipts
	// cross a real mTLS/JWKS/Redis receiver; this is not public Uninstall proof.
	_, err = st.BeginInstallationCancellationWithAdmission(ctx, bot.ID, space, installer, store.DeletionAdmission{AccountID: owner, ProfileID: installer, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_UninstallBotFromSpace_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	root, err := st.UninstallCleanup(ctx, intent.InstallationID)
	require.NoError(t, err)
	claim, err := st.ClaimUninstallCleanup(ctx, root.OperationID)
	require.NoError(t, err)
	effects, err := st.LifecycleEffects(ctx, claim)
	require.NoError(t, err)
	var effect store.BotDeletionEffect
	for _, candidate := range effects {
		if candidate.Kind == "MESSAGING_UNPIN" {
			effect = candidate
		}
	}
	require.NotEqual(t, uuid.Nil, effect.EffectID)
	actual := messagingv1.NewMessagingServiceClient(connection)
	svc := &BotGRPC{Store: st, PrincipalIssuer: issuer, CleanupMessaging: messagingLostACKClient{MessagingServiceClient: actual}, InstallationSpace: uninstallMessagingNativeACK{}}
	for _, candidate := range effects {
		if candidate.Kind == "SPACE_CLEANUP" {
			require.NoError(t, svc.applyNativeCancellation(ctx, claim, candidate))
		}
	}
	require.True(t, svc.nativeCancellationAccepted(ctx, claim))
	var nativeState string
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT state FROM bot_installation_intents WHERE installation_id=$1`, intent.InstallationID).Scan(&nativeState))
	require.Equal(t, "CANCELED", nativeState)
	err = svc.applyMessagingDeletion(ctx, claim, effect)
	require.Equal(t, codes.Unavailable, status.Code(err))
	frozenEffects, err := st.LifecycleEffects(ctx, claim)
	require.NoError(t, err)
	for _, candidate := range frozenEffects {
		if candidate.EffectID == effect.EffectID {
			effect = candidate
		}
	}
	require.NotEmpty(t, effect.RequestBytes)
	require.Empty(t, effect.ReceiptBytes)
	require.Equal(t, "PENDING", effect.State)
	frozen := append([]byte(nil), effect.RequestBytes...)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "repin.request"), []byte("repin"), 0600))
	require.Eventually(t, func() bool { _, err := os.Stat(filepath.Join(dir, "repin.ready")); return err == nil }, 3*time.Second, 10*time.Millisecond)
	// A fresh service on the same Pool with the persisted child source rejects a corrupt
	// completed response before saving it, then recovers the real exact receipt.
	restarted := &BotGRPC{Store: &store.BotStore{Pool: database.Pool}, PrincipalIssuer: issuer, CleanupMessaging: messagingLostACKClient{MessagingServiceClient: actual, corrupt: true}}
	err = restarted.applyMessagingDeletion(ctx, claim, effect)
	require.Equal(t, codes.Unavailable, status.Code(err))
	var receipts int
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_effects WHERE effect_id=$1 AND receipt_bytes IS NOT NULL`, effect.EffectID).Scan(&receipts))
	require.Zero(t, receipts)
	restarted.CleanupMessaging = actual
	require.NoError(t, st.RetryBotDeletion(ctx, claim))
	var worked bool
	require.Eventually(t, func() bool {
		var workerErr error
		worked, workerErr = restarted.ProcessNextBotDeletion(ctx)
		if worked {
			require.Equal(t, codes.Unavailable, status.Code(workerErr), "ordinary Messaging cleanup may complete while unconfigured Role remains pending")
		}
		return worked
	}, 5*time.Second, 20*time.Millisecond)
	final, err := st.LifecycleEffects(ctx, claim)
	require.NoError(t, err)
	for _, candidate := range final {
		if candidate.EffectID == effect.EffectID {
			require.Equal(t, "COMPLETE", candidate.State)
			require.Equal(t, frozen, candidate.RequestBytes)
			require.NotEmpty(t, candidate.ReceiptBytes)
		}
	}
	var globals int
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_operations WHERE bot_id=$1`, bot.ID).Scan(&globals))
	require.Zero(t, globals, "ordinary uninstall cannot manufacture a global root")
	current, err := st.GetBotByID(ctx, bot.ID)
	require.NoError(t, err)
	require.Equal(t, "live", current.Status)
	var saved messagingv1.ApplyBotChatPinRemovalRequest
	require.NoError(t, proto.Unmarshal(frozen, &saved))
	require.Equal(t, "space_uninstall", saved.Purpose)
	require.Equal(t, intent.InstallationID.String(), saved.GetNative().InstallationId)
	var rootState string
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT state FROM bot_uninstall_operations WHERE operation_id=$1`, root.OperationID).Scan(&rootState))
	require.Equal(t, "RETRY_WAIT", rootState, "Messaging completion cannot settle pending Role work; native cancellation alone was accepted")
	complete, err := json.Marshal(struct{ OperationID string }{effect.EffectID.String()})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "complete.json"), complete, 0600))
}

// This explicit Empty fixture does not claim actual Space receiver cancellation.
type uninstallMessagingNativeACK struct{ spacev1.SpaceServiceClient }

func (uninstallMessagingNativeACK) RemoveBotMember(context.Context, *spacev1.RemoveBotMemberRequest, ...grpc.CallOption) (*spacev1.RemoveBotMemberResponse, error) {
	return &spacev1.RemoveBotMemberResponse{}, nil
}
