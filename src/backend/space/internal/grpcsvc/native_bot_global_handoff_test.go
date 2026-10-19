package grpcsvc

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

// This optional, bounded fixture runs only in the caller's own private Linux
// namespace. JSON synchronizes the real producer; authority and terminal facts
// are checked against the actual protected receiver and its database.
func nativeBotGlobal46Handoff(t *testing.T, ctx context.Context, audit *pgxpool.Pool, listener net.Listener, client spacev1.SpaceServiceClient, key *rsa.PrivateKey, caFile, clientCertFile, clientKeyFile string, first, other uuid.UUID) bool {
	t.Helper()
	dir := os.Getenv("VOICE_BOT_GLOBAL46_HANDOFF_DIR")
	if dir == "" {
		return false
	}
	host := os.Getenv("VOICE_BOT_NATIVE_ADVERTISED_HOST")
	require.NotEmpty(t, host, "explicit private Docker alias required")
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0700))
	profile := uuid.NewString()
	ready := map[string]any{"protocol_version": 1, "schema_version": 46, "address": net.JoinHostPort(host, port), "server_name": "localhost", "space_id": first.String(), "other_space_id": other.String(), "profile_id": profile, "key_id": "current"}
	for field, path := range map[string]string{"ca_file": caFile, "client_cert_file": clientCertFile, "client_key_file": clientKeyFile} {
		raw, e := os.ReadFile(path)
		require.NoError(t, e)
		destination := filepath.Join(dir, "space46-"+filepath.Base(path))
		require.NoError(t, os.WriteFile(destination, raw, 0600))
		ready[field] = destination
	}
	rawKey, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	signerFile := filepath.Join(dir, "space46-bot-signer.pem")
	require.NoError(t, os.WriteFile(signerFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: rawKey}), 0600))
	ready["signer_key_file"] = signerFile
	raw, err := json.Marshal(ready)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ready.json.tmp"), raw, 0600))
	require.NoError(t, os.Rename(filepath.Join(dir, "ready.json.tmp"), filepath.Join(dir, "ready.json")))
	t.Cleanup(func() { _ = os.Remove(filepath.Join(dir, "ready.json")); _ = os.Remove(signerFile) })
	var completed struct {
		OperationID string `json:"operation_id"`
		ReceiptID   string `json:"receipt_id"`
		BotID       string `json:"bot_id"`
		DecisionID  string `json:"decision_id"`
	}
	deadline := time.NewTimer(90 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	waiting := true
	for waiting {
		select {
		case <-deadline.C:
			t.Fatal("bounded native46 global producer handoff did not complete")
		case <-tick.C:
			raw, err = os.ReadFile(filepath.Join(dir, "complete.json"))
			if os.IsNotExist(err) {
				continue
			}
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, &completed))
			waiting = false
		}
	}
	for _, value := range []string{completed.OperationID, completed.ReceiptID, completed.BotID, completed.DecisionID} {
		_, err = uuid.Parse(value)
		require.NoError(t, err)
	}
	var requestRaw, receiptRaw []byte
	require.NoError(t, audit.QueryRow(ctx, `SELECT request_bytes,receipt_bytes FROM native_membership_guard.bot_removal_receipts WHERE operation_id=$1 AND space_id=$2`, completed.OperationID, first).Scan(&requestRaw, &receiptRaw))
	request, receipt := &spacev1.ApplyBotInstallationRemovalRequest{}, &spacev1.BotInstallationRemovalReceipt{}
	require.NoError(t, proto.Unmarshal(requestRaw, request))
	require.NoError(t, proto.Unmarshal(receiptRaw, receipt))
	require.Equal(t, completed.ReceiptID, receipt.ReceiptId)
	require.Equal(t, completed.BotID, request.BotId)
	require.Equal(t, completed.DecisionID, request.DecisionId)
	require.Equal(t, profile, request.ActorProfileId)
	require.Equal(t, first.String(), request.SpaceId)
	require.Equal(t, "global_delete", request.Purpose)
	require.True(t, receipt.GlobalNativeAddsFenced)
	require.Equal(t, "COMPLETED", receipt.Outcome)
	requestSHA := sha256.Sum256(requestRaw)
	nestedRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request.NativeCancellation)
	require.NoError(t, err)
	nestedSHA := sha256.Sum256(nestedRaw)
	require.Equal(t, requestSHA[:], receipt.RequestSha256)
	require.Equal(t, nestedSHA[:], receipt.EvidenceSha256)
	var terminal bool
	require.NoError(t, audit.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM native_membership_guard.bot_terminals WHERE bot_id=$1 AND decision_id=$2 AND decision_sha256=$3)
 AND NOT EXISTS(SELECT 1 FROM public.space_members WHERE profile_id=$4)
 AND EXISTS(SELECT 1 FROM public.space_bot_installation_heads WHERE space_id=$5 AND profile_id=$4 AND state='CANCELED' AND retired_through >= $6)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`, request.BotId, request.DecisionId, request.DecisionSha256, profile, first, request.NativeCancellation.InstallationIntent.Generation).Scan(&terminal))
	require.True(t, terminal, "actual global marker, settled native head and no residual membership/frame")
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	signed := func(message proto.Message, method, operation string) (context.Context, context.CancelFunc) {
		hash, e := principal.RequestHash(message)
		require.NoError(t, e)
		token, e := signer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: operation, RequestHash: hash})
		require.NoError(t, e)
		return context.WithTimeout(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operation)), 3*time.Second)
	}
	call, stop := signed(request, spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName, request.OperationId)
	saved, err := client.ApplyBotInstallationRemoval(call, request)
	stop()
	require.NoError(t, err)
	require.True(t, proto.Equal(receipt, saved.Receipt))
	old := &spacev1.AddBotMemberRequest{SpaceId: request.SpaceId, ProfileId: profile, InstallationIntent: proto.Clone(request.NativeCancellation.InstallationIntent).(*spacev1.BotInstallationIntent)}
	old.InstallationIntent.OperationId = request.NativeCancellation.InstallOperationId
	for index := 0; index < 3; index++ {
		add := proto.Clone(old).(*spacev1.AddBotMemberRequest)
		if index > 0 {
			add.InstallationIntent.OperationId = uuid.NewString()
			add.InstallationIntent.InstallationId = uuid.NewString()
			add.InstallationIntent.Generation++
		}
		if index == 2 {
			add.SpaceId, add.InstallationIntent.SpaceId = other.String(), other.String()
			add.InstallationIntent.Generation = 1
		}
		call, stop = signed(add, spacev1.SpaceService_AddBotMember_FullMethodName, add.InstallationIntent.OperationId)
		_, err = client.AddBotMember(call, add)
		stop()
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
	}
	t.Log("actual producer receipt/root/native cancellation and global denial of old/new native Add verified")
	return true
}
