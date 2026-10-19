package grpcsvc

import (
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
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
)

func botPinProducerHandoff(t *testing.T, server *BotPinCleanupRuntime, pins *store.PinsStore, target *messagingv1.ApplyBotChatPinRemovalRequest, key *rsa.PrivateKey, certificate tls.Certificate, caFile string, message uuid.UUID) bool {
	t.Helper()
	dir := os.Getenv("VOICE_BOT_PIN_PRODUCER_HANDOFF_DIR")
	if dir == "" {
		return false
	}
	require.NoError(t, os.Mkdir(dir, 0700))
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })
	certFile, keyFile := writeGISMappingClientCert(t, certificate)
	private, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	signerFile := filepath.Join(dir, "test-signer.pem")
	require.NoError(t, os.WriteFile(signerFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), 0600))
	ready := struct {
		Address, ServerName, CAFile, CertFile, KeyFile, SignerFile, ChatID, SpaceID, ActorID string
		Observation                                                                          *messagingv1.BotRemovalChatScopeObservation
	}{server.Address(), "localhost", caFile, certFile, keyFile, signerFile, target.ChatId, target.SpaceId, target.ActorProfileId, target.Attachments[0].CurrentScope}
	body, err := json.Marshal(ready)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ready.json"), body, 0600))
	deadline := time.Now().Add(90 * time.Second)
	repinned := false
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(dir, "repin.request")); err == nil && !repinned {
			require.NoError(t, pins.UpsertPin(t.Context(), uuid.MustParse(target.ChatId), message, uuid.New()))
			repinned = true
			require.NoError(t, os.WriteFile(filepath.Join(dir, "repin.ready"), []byte("ready"), 0600))
		}
		if raw, err := os.ReadFile(filepath.Join(dir, "complete.json")); err == nil {
			var complete struct{ OperationID string }
			require.NoError(t, json.Unmarshal(raw, &complete))
			require.NotEqual(t, uuid.Nil, uuid.MustParse(complete.OperationID))
			require.True(t, repinned)
			var receipts, removed, revision int
			require.NoError(t, pins.Pool.QueryRow(t.Context(), `SELECT count(*),max(cleanup_revision) FROM bot_pin_removal_receipts WHERE operation_id=$1`, complete.OperationID).Scan(&receipts, &revision))
			require.Equal(t, 1, receipts)
			require.Equal(t, 1, revision)
			require.NoError(t, pins.Pool.QueryRow(t.Context(), `SELECT count(*) FROM bot_pin_removal_attachment_coverage WHERE origin_operation_id=$1`, complete.OperationID).Scan(&removed))
			require.Equal(t, 1, removed)
			remaining, err := pins.ListPins(t.Context(), uuid.MustParse(target.ChatId))
			require.NoError(t, err)
			require.Len(t, remaining, 1, "lost ACK recovery preserves later human repin")
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("bounded producer handoff timed out")
	return true
}
