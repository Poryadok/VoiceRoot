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
	"google.golang.org/grpc/credentials"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

// This lower-seam oracle consumes a separately built real Chat source. It does
// not manufacture public Role admission or count as a Gateway install proof.
func TestBotChatSourceCrossProcessPersistenceAndRetiredRetry(t *testing.T) {
	directory := os.Getenv("VOICE_BOT_CHAT_HANDOFF_DIR")
	if directory == "" {
		t.Skip("explicit private source fixture only")
	}
	ctx := t.Context()
	var f struct {
		FixtureOnly bool   `json:"fixture_only"`
		Address     string `json:"address"`
		ServerName  string `json:"server_name"`
		Space       string `json:"space_id"`
		Chat        string `json:"chat_id"`
		CA          string `json:"ca_file"`
		Cert        string `json:"client_cert_file"`
		Key         string `json:"client_key_file"`
		Signer      string `json:"signer_key_file"`
		KeyID       string `json:"signer_key_id"`
	}
	ready := filepath.Join(directory, "ready.json")
	require.Eventually(t, func() bool { body, err := os.ReadFile(ready); return err == nil && json.Unmarshal(body, &f) == nil }, 30*time.Second, 40*time.Millisecond)
	require.True(t, f.FixtureOnly)
	space, err := uuid.Parse(f.Space)
	require.NoError(t, err)
	chat, err := uuid.Parse(f.Chat)
	require.NoError(t, err)
	ca, err := os.ReadFile(f.CA)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	certificate, err := tls.LoadX509KeyPair(f.Cert, f.Key)
	require.NoError(t, err)
	connection, err := grpc.NewClient(f.Address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: f.ServerName, Certificates: []tls.Certificate{certificate}})))
	require.NoError(t, err)
	defer connection.Close()
	key, err := principal.LoadRSAPrivateKeyFile(f.Signer)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: f.KeyID, PrivateKey: key})
	require.NoError(t, err)
	pool := integrationtest.StartPostgres(t, ctx, "bot-chat-source-caller", "")
	paths, err := filepath.Glob("../../../migrations/bot_db/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	for _, path := range paths {
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err, path)
	}
	st := &store.BotStore{Pool: pool}
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Actual protected Chat caller", "", `[]`, uuid.New())
	require.NoError(t, err)
	service := &BotGRPC{Store: st, ChatScope: chatv1.NewChatScopeAuthorityServiceClient(connection), PrincipalIssuer: issuer}
	accepted, err := service.qualifyChatScopes(ctx, space, []uuid.UUID{chat})
	require.NoError(t, err)
	intent, err := st.PrepareInstallation(accepted, bot.ID, space, uuid.New(), []uuid.UUID{chat})
	require.NoError(t, err)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_chat_scope_observations WHERE bot_id=$1 AND intent_operation_id=$2 AND source_exists AND source_live`, bot.ID, intent.OperationID).Scan(&count))
	require.Equal(t, 1, count)
	var installed int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_installations WHERE bot_id=$1`, bot.ID).Scan(&installed))
	require.Zero(t, installed, "source was durable before any native Space Add")
	require.NoError(t, os.WriteFile(filepath.Join(directory, "retire-chat"), []byte("retire actual source"), 0600))
	require.Eventually(t, func() bool { _, err := os.Stat(filepath.Join(directory, "retired-chat")); return err == nil }, 8*time.Second, 30*time.Millisecond)
	_, err = service.qualifyChatScopes(ctx, space, []uuid.UUID{chat})
	require.Error(t, err, "retired source cannot admit new work")
	restarted := &store.BotStore{Pool: pool}
	again, err := restarted.ReusableInstallationIntent(context.Background(), bot.ID, space, []uuid.UUID{chat})
	require.NoError(t, err)
	require.NotNil(t, again)
	require.Equal(t, intent.OperationID, again.OperationID)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_chat_scope_observations WHERE bot_id=$1`, bot.ID).Scan(&count))
	require.Equal(t, 1, count, "recovery uses original accepted source without manufacturing a new observation")
	require.NoError(t, os.WriteFile(filepath.Join(directory, "complete.json"), []byte(`{"complete":true}`), 0600))
}
