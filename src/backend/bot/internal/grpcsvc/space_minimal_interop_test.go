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

type minimalSpaceReplyFault struct {
	spacev1.SpaceServiceClient
	calls int
}

func (c *minimalSpaceReplyFault) RegisterBotGlobalTerminal(ctx context.Context, q *spacev1.RegisterBotGlobalTerminalRequest, options ...grpc.CallOption) (*spacev1.RegisterBotGlobalTerminalResponse, error) {
	c.calls++
	r, err := c.SpaceServiceClient.RegisterBotGlobalTerminal(ctx, q, options...)
	if err != nil {
		return nil, err
	}
	if c.calls == 1 {
		return nil, status.Error(codes.Unavailable, "test lost ACK after real minimal Space commit")
	}
	if c.calls == 2 {
		r = proto.Clone(r).(*spacev1.RegisterBotGlobalTerminalResponse)
		r.Receipt.DecisionId = uuid.NewString()
	}
	return r, nil
}

func TestGlobalSpaceMinimalActualParticipantLostACKRecovery(t *testing.T) {
	dir := os.Getenv("VOICE_BOT_MINIMAL48_HANDOFF_DIR")
	if dir == "" {
		t.Skip("requires caller-owned immutable Space48 serving fixture")
	}
	var ready struct {
		SchemaVersion  int    `json:"schema_version"`
		CatalogVersion int    `json:"catalog_version"`
		SourcePin      string `json:"source_pin"`
		Address        string `json:"address"`
		ServerName     string `json:"server_name"`
		CAFile         string `json:"ca_file"`
		CertFile       string `json:"client_cert_file"`
		KeyFile        string `json:"client_key_file"`
		SignerFile     string `json:"signer_key_file"`
		KeyID          string `json:"key_id"`
	}
	require.Eventually(t, func() bool {
		raw, err := os.ReadFile(filepath.Join(dir, "ready.json"))
		return err == nil && json.Unmarshal(raw, &ready) == nil
	}, 120*time.Second, 50*time.Millisecond)
	require.Equal(t, 1, ready.SchemaVersion)
	require.Equal(t, 48, ready.CatalogVersion)
	require.Equal(t, os.Getenv("VOICE_BOT_MINIMAL48_SOURCE_PIN"), ready.SourcePin)
	require.Len(t, ready.SourcePin, 40)
	require.NoFileExists(t, filepath.Join(dir, "complete.json"))
	ca, err := os.ReadFile(ready.CAFile)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	pair, err := tls.LoadX509KeyPair(ready.CertFile, ready.KeyFile)
	require.NoError(t, err)
	conn, err := grpc.NewClient(ready.Address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: ready.ServerName, Certificates: []tls.Certificate{pair}})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	keyBytes, err := os.ReadFile(ready.SignerFile)
	require.NoError(t, err)
	block, _ := pem.Decode(keyBytes)
	require.NotNil(t, block)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	key, ok := parsed.(*rsa.PrivateKey)
	require.True(t, ok)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: ready.KeyID, PrivateKey: key})
	require.NoError(t, err)
	ctx := t.Context()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join("..", "..", "..", "migrations", "bot_db"))
	database.Run(true, "up")
	st := &store.BotStore{Pool: database.Pool}
	owner, actor := uuid.New(), uuid.New()
	bot, token, err := st.CreateBot(ctx, owner, "Actual zero-Space terminal", "", `[]`, actor)
	require.NoError(t, err)
	q := &botv1.DeleteBotRequest{BotId: bot.ID.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	hash, err := principal.RequestHash(q)
	require.NoError(t, err)
	// Controlled verified-owner admission, actual Bot persistence and remote
	// production Space receiver. No public Gateway/Auth session is claimed.
	root, err := st.BeginBotDeletionWithAdmission(ctx, bot.ID, owner, store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_DeleteBot_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	_, err = st.GetBotByTokenHash(ctx, store.HashToken(token))
	require.Error(t, err)
	effects, err := st.BotDeletionEffects(ctx, root.OperationID)
	require.NoError(t, err)
	require.Empty(t, effects)
	fault := &minimalSpaceReplyFault{SpaceServiceClient: spacev1.NewSpaceServiceClient(conn)}
	svc := &BotGRPC{Store: st, PrincipalIssuer: issuer, CleanupSpace: fault}
	var frozen []byte
	var saved spacev1.RegisterBotGlobalTerminalResponse
	for n := 0; n < 4; n++ {
		_, err = database.Pool.Exec(ctx, `UPDATE bot_deletion_operations SET next_attempt_at=clock_timestamp()-interval '1 second' WHERE operation_id=$1`, root.OperationID)
		require.NoError(t, err)
		worked, err := svc.ProcessNextBotDeletion(ctx)
		require.True(t, worked)
		require.Equal(t, codes.Unavailable, status.Code(err), "Role remains independent and unavailable")
		var state string
		var request, receipt []byte
		require.NoError(t, database.Pool.QueryRow(ctx, `SELECT state,request_bytes,receipt_bytes FROM bot_deletion_terminal_effects WHERE operation_id=$1 AND participant='space'`, root.OperationID).Scan(&state, &request, &receipt))
		require.NotEmpty(t, request)
		if frozen == nil {
			frozen = append([]byte(nil), request...)
		}
		require.Equal(t, frozen, request)
		if n < 2 {
			require.Equal(t, "PENDING", state)
			require.Empty(t, receipt)
		} else {
			require.Equal(t, "COMPLETE", state)
			require.NoError(t, proto.Unmarshal(receipt, &saved))
			require.Equal(t, root.OperationID.String(), saved.GetReceipt().GetDecisionId())
			require.Equal(t, bot.ID.String(), saved.GetReceipt().GetBotId())
			require.Equal(t, spacev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED, saved.GetReceipt().GetOutcome())
		}
		var roleState, rootState string
		require.NoError(t, database.Pool.QueryRow(ctx, `SELECT state FROM bot_deletion_terminal_effects WHERE operation_id=$1 AND participant='role'`, root.OperationID).Scan(&roleState))
		require.Equal(t, "PENDING", roleState)
		require.NoError(t, database.Pool.QueryRow(ctx, `SELECT state FROM bot_deletion_operations WHERE operation_id=$1`, root.OperationID).Scan(&rootState))
		require.Equal(t, "RETRY_WAIT", rootState)
		// Service/store reinstantiation shares the same Pool, not an OS restart.
		svc = &BotGRPC{Store: &store.BotStore{Pool: database.Pool}, PrincipalIssuer: issuer, CleanupSpace: fault}
	}
	require.Equal(t, 3, fault.calls, "complete saved Space result avoids fourth RPC")
	current, err := st.GetBotByID(ctx, bot.ID)
	require.NoError(t, err)
	require.Equal(t, "disabled", current.Status)
	completion, err := json.Marshal(map[string]any{"schema_version": 1, "bot_id": bot.ID.String(), "decision_id": root.OperationID.String(), "receipt_id": saved.GetReceipt().GetReceiptId()})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "complete.tmp"), completion, 0600))
	require.NoError(t, os.Rename(filepath.Join(dir, "complete.tmp"), filepath.Join(dir, "complete.json")))
	t.Log("real Space48 minimal result persisted through lost ACK and corrupt result; ROLE and root remain pending")
}
