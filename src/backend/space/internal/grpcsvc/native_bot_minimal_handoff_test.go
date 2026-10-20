package grpcsvc

import (
	"context"
	"crypto/rsa"
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
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

// Optional synchronization with the real Bot producer in its own private Linux
// namespace. JSON supplies no authority; the database and protected RPCs are the
// oracles. This zero-Space path proves a fence, never installation cleanup.
func nativeBotMinimal48Handoff(t *testing.T, ctx context.Context, audit *pgxpool.Pool, client spacev1.SpaceServiceClient, address string, key *rsa.PrivateKey, caFile, certFile, keyFile string) bool {
	t.Helper()
	dir := os.Getenv("VOICE_BOT_MINIMAL48_HANDOFF_DIR")
	if dir == "" {
		return false
	}
	host, pin := os.Getenv("VOICE_BOT_NATIVE_ADVERTISED_HOST"), os.Getenv("VOICE_BOT_MINIMAL48_SOURCE_PIN")
	require.NotEmpty(t, host, "explicit private Docker alias required")
	require.Len(t, pin, 40, "caller must bind the immutable provider source")
	_, port, err := net.SplitHostPort(address)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0700))
	for _, name := range []string{"ready.json", "complete.json"} {
		_, err = os.Stat(filepath.Join(dir, name))
		require.True(t, os.IsNotExist(err), "handoff directory must have no stale coordination files")
	}
	ready := map[string]any{"schema_version": 1, "catalog_version": 48, "address": net.JoinHostPort(host, port), "server_name": "localhost", "key_id": "current", "source_pin": pin}
	owned := []string{}
	t.Cleanup(func() {
		for _, path := range owned {
			_ = os.Remove(path)
		}
	})
	for name, source := range map[string]string{"ca_file": caFile, "client_cert_file": certFile, "client_key_file": keyFile} {
		raw, e := os.ReadFile(source)
		require.NoError(t, e)
		destination := filepath.Join(dir, "minimal48-"+name+".pem")
		require.NoError(t, os.WriteFile(destination, raw, 0600))
		owned = append(owned, destination)
		ready[name] = destination
	}
	private, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	signerFile := filepath.Join(dir, "minimal48-bot-signer.pem")
	require.NoError(t, os.WriteFile(signerFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), 0600))
	owned = append(owned, signerFile, filepath.Join(dir, "ready.json"), filepath.Join(dir, "ready.json.tmp"))
	ready["signer_key_file"] = signerFile
	raw, err := json.Marshal(ready)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ready.json.tmp"), raw, 0600))
	require.NoError(t, os.Rename(filepath.Join(dir, "ready.json.tmp"), filepath.Join(dir, "ready.json")))
	var complete struct {
		SchemaVersion int    `json:"schema_version"`
		BotID         string `json:"bot_id"`
		DecisionID    string `json:"decision_id"`
		ReceiptID     string `json:"receipt_id"`
	}
	deadline := time.NewTimer(90 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	waiting := true
	for waiting {
		select {
		case <-deadline.C:
			t.Fatal("bounded minimal48 producer handoff did not complete")
		case <-tick.C:
			raw, err = os.ReadFile(filepath.Join(dir, "complete.json"))
			if os.IsNotExist(err) {
				continue
			}
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, &complete))
			waiting = false
		}
	}
	require.Equal(t, 1, complete.SchemaVersion)
	for _, id := range []string{complete.BotID, complete.DecisionID, complete.ReceiptID} {
		parsed, e := uuid.Parse(id)
		require.NoError(t, e)
		require.NotEqual(t, uuid.Nil, parsed)
		require.Equal(t, parsed.String(), id)
	}
	var receiptID uuid.UUID
	var registered time.Time
	require.NoError(t, audit.QueryRow(ctx, `SELECT receipt_id,registered_at FROM native_membership_guard.bot_terminal_registrations WHERE bot_id=$1 AND decision_id=$2`, complete.BotID, complete.DecisionID).Scan(&receiptID, &registered))
	require.Equal(t, complete.ReceiptID, receiptID.String())
	expected := &spacev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, BotId: complete.BotID, DecisionId: complete.DecisionID, ReceiptId: complete.ReceiptID, RegisteredAt: timestamppb.New(registered), Outcome: spacev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	signed := func(message proto.Message, method, operation string) (context.Context, context.CancelFunc) {
		hash, e := principal.RequestHash(message)
		require.NoError(t, e)
		token, e := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: operation, RequestHash: hash})
		require.NoError(t, e)
		return context.WithTimeout(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operation)), 3*time.Second)
	}
	request := &spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: complete.BotID, DecisionId: complete.DecisionID}
	call, stop := signed(request, spacev1.SpaceService_RegisterBotGlobalTerminal_FullMethodName, request.DecisionId)
	response, err := client.RegisterBotGlobalTerminal(call, request)
	stop()
	require.NoError(t, err)
	require.True(t, proto.Equal(expected, response.Receipt))
	for generation := uint64(1); generation <= 2; generation++ {
		space, profile := uuid.NewString(), uuid.NewString()
		add := &spacev1.AddBotMemberRequest{SpaceId: space, ProfileId: profile, InstallationIntent: &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: complete.BotID, InstallationId: uuid.NewString(), SpaceId: space, ActorProfileId: profile, Generation: generation}}
		call, stop = signed(add, spacev1.SpaceService_AddBotMember_FullMethodName, add.InstallationIntent.OperationId)
		_, err = client.AddBotMember(call, add)
		stop()
		require.Equal(t, codes.FailedPrecondition, status.Code(err), "permanent fence precedes absent-Space lookup, including new generation/Space")
	}
	var clean bool
	require.NoError(t, audit.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.spaces) AND NOT EXISTS(SELECT 1 FROM native_membership_guard.bot_terminals) AND NOT EXISTS(SELECT 1 FROM native_membership_guard.bot_removal_receipts48) AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames) AND (SELECT count(*)=1 FROM native_membership_guard.bot_terminal_registrations)`).Scan(&clean))
	require.True(t, clean, "only the minimal root fence exists; no invented source cleanup")
	t.Log("actual Bot producer minimal receipt persisted; zero-Space fence and fresh signed Add denials verified")
	return true
}
