package grpcsvc

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

// This fixture selects the real durable producer and protected Space transport.
// It does not fake a Role permission decision or qualify public HTTP install.
func TestNativeProducerActualSpacePersistenceCancelAndReinstall(t *testing.T) {
	dir := os.Getenv("VOICE_BOT_NATIVE_HANDOFF_DIR")
	if dir == "" {
		t.Skip("explicit actual Space receiver fixture only")
	}
	data, err := os.ReadFile(filepath.Join(dir, "ready.json"))
	require.NoError(t, err)
	var ready struct {
		Address        string `json:"address"`
		ServerName     string `json:"server_name"`
		SpaceID        string `json:"space_id"`
		ProfileID      string `json:"profile_id"`
		CAFile         string `json:"ca_file"`
		ClientCertFile string `json:"client_cert_file"`
		ClientKeyFile  string `json:"client_key_file"`
		SignerKeyFile  string `json:"signer_key_file"`
		KeyID          string `json:"key_id"`
	}
	require.NoError(t, json.Unmarshal(data, &ready))
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "bot_native_interop", "")
	_, source, _, _ := runtime.Caller(0)
	migrations := filepath.Join(filepath.Dir(source), "..", "..", "..", "migrations", "bot_db")
	entries, err := os.ReadDir(migrations)
	require.NoError(t, err)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".up.sql") {
			sql, err := os.ReadFile(filepath.Join(migrations, entry.Name()))
			require.NoError(t, err)
			_, err = pool.Exec(ctx, string(sql))
			require.NoError(t, err, entry.Name())
		}
	}
	st := &store.BotStore{Pool: pool}
	actor, space, installer := uuid.MustParse(ready.ProfileID), uuid.MustParse(ready.SpaceID), uuid.New()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Real Space producer", "", `[]`, actor)
	require.NoError(t, err)
	key, err := principal.LoadRSAPrivateKeyFile(ready.SignerKeyFile)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: ready.KeyID, PrivateKey: key})
	require.NoError(t, err)
	ca, err := os.ReadFile(ready.CAFile)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	certificate, err := tls.LoadX509KeyPair(ready.ClientCertFile, ready.ClientKeyFile)
	require.NoError(t, err)
	conn, err := grpc.NewClient(ready.Address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: ready.ServerName, Certificates: []tls.Certificate{certificate}})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	svc := NewBotGRPC(st, nil)
	svc.PrincipalIssuer = issuer
	svc.InstallationSpace = spacev1.NewSpaceServiceClient(conn)
	chat := uuid.New()
	first, err := st.PrepareInstallation(ctx, bot.ID, space, installer, []uuid.UUID{chat})
	require.NoError(t, err)
	require.EqualValues(t, 1, first.Generation)
	var persisted uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT installation_id FROM bot_installation_intents WHERE operation_id=$1 AND state='PREPARING'`, first.OperationID).Scan(&persisted))
	require.Equal(t, first.InstallationID, persisted, "durable identity precedes actual remote Add")
	require.NoError(t, svc.addInstallationMember(ctx, first))
	// Simulate lost acknowledgement before local completion, then recover from
	// persisted state through a new store object and a fresh transport JTI.
	restarted := &store.BotStore{Pool: pool}
	retry, err := restarted.PrepareInstallation(ctx, bot.ID, space, installer, []uuid.UUID{chat})
	require.NoError(t, err)
	require.Equal(t, first.OperationID, retry.OperationID)
	require.Equal(t, first.InstallationID, retry.InstallationID)
	require.NoError(t, svc.addInstallationMember(ctx, retry))
	require.NoError(t, restarted.CompleteInstallation(ctx, retry, []uuid.UUID{chat}))
	cancel, err := st.BeginInstallationCancellation(ctx, bot.ID, space, installer)
	require.NoError(t, err)
	allowed, err := st.IsChatWhitelisted(ctx, bot.ID, chat)
	require.NoError(t, err)
	require.False(t, allowed)
	require.NoError(t, svc.removeInstallationMember(ctx, cancel))
	require.NoError(t, st.CompleteInstallationCancellation(ctx, cancel))
	require.Equal(t, codes.FailedPrecondition, status.Code(svc.addInstallationMember(ctx, first)))
	require.ErrorIs(t, st.CompleteInstallation(ctx, first, []uuid.UUID{chat}), store.ErrInstallationConflict)
	second, err := st.PrepareInstallation(ctx, bot.ID, space, installer, []uuid.UUID{chat})
	require.NoError(t, err)
	require.EqualValues(t, 2, second.Generation)
	require.NotEqual(t, first.InstallationID, second.InstallationID)
	require.NoError(t, svc.addInstallationMember(ctx, second))
	require.NoError(t, st.CompleteInstallation(ctx, second, []uuid.UUID{chat}))
	require.NoError(t, svc.removeInstallationMember(ctx, cancel), "old exact cancel replay must preserve new native source")
	require.NoError(t, svc.addInstallationMember(ctx, second), "new source must still be current eligible after old cancel")
	other, _, err := st.CreateBot(ctx, uuid.New(), "Canceled before add", "", `[]`, uuid.New())
	require.NoError(t, err)
	late, err := st.PrepareInstallation(ctx, other.ID, space, installer, nil)
	require.NoError(t, err)
	negative, err := st.BeginInstallationCancellation(ctx, other.ID, space, installer)
	require.NoError(t, err)
	require.NoError(t, svc.removeInstallationMember(ctx, negative))
	require.Equal(t, codes.FailedPrecondition, status.Code(svc.addInstallationMember(ctx, late)))
	require.NoError(t, st.CompleteInstallationCancellation(ctx, negative))
	complete, err := json.Marshal(map[string]any{"install_operation_id": second.OperationID.String(), "generation": second.Generation})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "complete.json.tmp"), complete, 0600))
	require.NoError(t, os.Rename(filepath.Join(dir, "complete.json.tmp"), filepath.Join(dir, "complete.json")))
}
