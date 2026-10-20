package grpcsvc

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/store"
)

type nativeP3Installed struct {
	SchemaVersion      int    `json:"schema_version"`
	BotID              string `json:"bot_id"`
	InstallationID     string `json:"installation_id"`
	InstallOperationID string `json:"install_operation_id"`
	BotGeneration      uint64 `json:"bot_generation"`
	Variant            string `json:"variant"`
	CancelOperationID  string `json:"cancel_operation_id,omitempty"`
}

type nativeP3Completed struct {
	SchemaVersion     int    `json:"schema_version"`
	BotID             string `json:"bot_id"`
	InstallationID    string `json:"installation_id"`
	DrainOperationID  string `json:"drain_operation_id"`
	NativeReceiptID   string `json:"native_receipt_id"`
	BotPurgeReceiptID string `json:"bot_purge_receipt_id,omitempty"` // synchronization only
}

func nativeP3WriteSignal(t *testing.T, dir, name string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path+".tmp", raw, 0600))
	require.NoError(t, os.Rename(path+".tmp", path))
}

func nativeP3WaitSignal(t *testing.T, dir, name string, deadline time.Time, value any) {
	t.Helper()
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(dir, "stop")); err == nil && name != "stop" {
			t.Fatal("caller stopped before provider postconditions")
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if os.IsNotExist(err) {
			time.Sleep(25 * time.Millisecond)
			continue
		}
		require.NoError(t, err)
		if name == "stop" {
			return
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		require.NoError(t, decoder.Decode(value))
		return
	}
	t.Fatalf("bounded P3 provider did not receive %s", name)
}

func nativeP3ExportReady(t *testing.T, dir, sourcePin, address, space, profile, ca, cert, keyFile string, key *rsa.PrivateKey, deadline time.Time) {
	t.Helper()
	require.Regexp(t, regexp.MustCompile(`^[0-9a-f]{40}$`), sourcePin, "caller binds the exact immutable provider source; metadata is not a source proof")
	require.NoError(t, os.MkdirAll(dir, 0700))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries, "fresh caller-owned private coordination directory required")
	host := os.Getenv("VOICE_BOT_NATIVE_ADVERTISED_HOST")
	require.NotEmpty(t, host)
	_, port, err := net.SplitHostPort(address)
	require.NoError(t, err)
	ready := map[string]any{"schema_version": 1, "catalog_version": 49, "source_pin": sourcePin, "address": net.JoinHostPort(host, port), "server_name": "localhost", "space_id": space, "profile_id": profile, "key_id": "current", "expires_at": deadline.UTC().Format(time.RFC3339Nano), "stop_file": filepath.Join(dir, "stop")}
	created := []string{"ready.json", "purge-ready.json", "verified.json", "bot-signer.pem", "ca.pem", "client.pem", "client-key.pem"}
	t.Cleanup(func() {
		for _, name := range created {
			_ = os.Remove(filepath.Join(dir, name))
			_ = os.Remove(filepath.Join(dir, name+".tmp"))
		}
	})
	for field, source := range map[string]string{"ca_file": ca, "client_cert_file": cert, "client_key_file": keyFile} {
		raw, err := os.ReadFile(source)
		require.NoError(t, err)
		name := map[string]string{"ca_file": "ca.pem", "client_cert_file": "client.pem", "client_key_file": "client-key.pem"}[field]
		destination := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(destination, raw, 0600))
		ready[field] = destination
	}
	raw, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	signer := filepath.Join(dir, "bot-signer.pem")
	require.NoError(t, os.WriteFile(signer, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw}), 0600))
	ready["signer_key_file"] = signer
	nativeP3WriteSignal(t, dir, "ready.json", ready)
}

func nativeP3VerifyInstalled(t *testing.T, ctx context.Context, admin, serving *store.SpaceStore, space, profile string, in nativeP3Installed) *spacev1.AddBotMemberRequest {
	t.Helper()
	require.Equal(t, 1, in.SchemaVersion)
	require.Positive(t, in.BotGeneration)
	for _, id := range []string{in.BotID, in.InstallationID, in.InstallOperationID} {
		parsed, err := uuid.Parse(id)
		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, parsed)
		require.Equal(t, parsed.String(), id)
	}
	require.Contains(t, []string{"installed", "ordinary_canceled"}, in.Variant)
	add := &spacev1.AddBotMemberRequest{SpaceId: space, ProfileId: profile, InstallationIntent: &spacev1.BotInstallationIntent{OperationId: in.InstallOperationID, BotId: in.BotID, InstallationId: in.InstallationID, SpaceId: space, ActorProfileId: profile, Generation: in.BotGeneration}}
	var state string
	var membership *uint64
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT state,membership_generation FROM public.space_bot_installation_attempts WHERE space_id=$1 AND profile_id=$2 AND bot_id=$3 AND installation_id=$4 AND install_operation_id=$5 AND generation=$6`, space, profile, in.BotID, in.InstallationID, in.InstallOperationID, in.BotGeneration).Scan(&state, &membership))
	require.NotNil(t, membership, "an actual published native incarnation is required, not a JSON-seeded intent")
	if in.Variant == "installed" {
		require.Equal(t, "PUBLISHED", state)
		observed, err := serving.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space, ProfileId: profile})
		require.NoError(t, err)
		require.Equal(t, spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE, observed.GetOrdinary().GetState())
		require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_BOT_INSTALL, observed.GetOrdinary().GetPublicationKind())
		require.Equal(t, in.InstallOperationID, observed.GetOrdinary().GetPublishedBinding().GetOrdinary().GetPublicationIdentity())
	} else {
		require.Equal(t, "CANCELED", state)
		require.NotEmpty(t, in.CancelOperationID)
		cancel := &spacev1.RemoveBotMemberRequest{SpaceId: space, ProfileId: profile, InstallOperationId: in.InstallOperationID, InstallationIntent: proto.Clone(add.InstallationIntent).(*spacev1.BotInstallationIntent)}
		cancel.InstallationIntent.OperationId = in.CancelOperationID
		raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(cancel)
		require.NoError(t, err)
		hash := sha256.Sum256(raw)
		var actual bool
		require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.space_bot_installation_cancellations WHERE operation_id=$1 AND install_operation_id=$2 AND space_id=$3 AND request_sha256=$4) AND NOT EXISTS(SELECT 1 FROM public.space_members WHERE space_id=$3 AND profile_id=$5 AND admission_operation_id=$2)`, in.CancelOperationID, in.InstallOperationID, space, hash[:], profile).Scan(&actual))
		require.True(t, actual)
	}
	return add // reconstructed view only; never inserted or used to manufacture an effect
}

func nativeP3ExportDecision(t *testing.T, dir, sourcePin, space string, decision nativeP3ProviderDecision) {
	t.Helper()
	encode := func(p proto.Message) string {
		raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(p)
		require.NoError(t, err)
		return base64.StdEncoding.EncodeToString(raw)
	}
	nativeP3WriteSignal(t, dir, "purge-ready.json", map[string]any{"schema_version": 1, "catalog_version": 49, "source_pin": sourcePin, "space_id": space, "deletion_operation_id": decision.OperationID.String(), "generation": decision.Generation, "manifest_type": "voice.common.v1.ManifestBinding", "manifest_base64": encode(decision.Manifest), "purge_decided_at": decision.PurgeDecidedAt.UTC().Format(time.RFC3339Nano), "frozen_fence_type": "voice.common.v1.SpaceLifecycleFenceRequest", "frozen_fence_base64": encode(decision.Frozen), "purge_decided_fence_type": "voice.common.v1.SpaceLifecycleFenceRequest", "purge_decided_fence_base64": encode(decision.Decided), "bot_purge_type": "voice.common.v1.SpacePurgeRequest", "bot_purge_base64": encode(decision.Purge), "upstream_mode": "controlled-role-and-fences-empty-chat"})
}

func nativeP3VerifyChild(t *testing.T, ctx context.Context, admin *store.SpaceStore, space, profile string, installed nativeP3Installed, decision nativeP3ProviderDecision, complete nativeP3Completed) (*spacev1.ReconcileBotPurgeInstallationRequest, *spacev1.BotPurgeInstallationReceipt) {
	t.Helper()
	require.Equal(t, 1, complete.SchemaVersion)
	require.Equal(t, installed.BotID, complete.BotID)
	require.Equal(t, installed.InstallationID, complete.InstallationID)
	var requestRaw, resultRaw, requestHash, resultHash []byte
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT request_bytes,receipt_bytes,request_sha256,receipt_sha256 FROM native_membership_guard.bot_purge_children WHERE operation_id=$1 AND receipt_id=$2 AND space_id=$3`, complete.DrainOperationID, complete.NativeReceiptID, space).Scan(&requestRaw, &resultRaw, &requestHash, &resultHash))
	rh, sh := sha256.Sum256(requestRaw), sha256.Sum256(resultRaw)
	require.Equal(t, rh[:], requestHash)
	require.Equal(t, sh[:], resultHash)
	r := &spacev1.ReconcileBotPurgeInstallationRequest{}
	receipt := &spacev1.BotPurgeInstallationReceipt{}
	require.NoError(t, proto.Unmarshal(requestRaw, r))
	require.NoError(t, proto.Unmarshal(resultRaw, receipt))
	require.Equal(t, complete.DrainOperationID, r.OperationId)
	require.Equal(t, space, r.SpaceId)
	require.Equal(t, decision.OperationID.String(), r.DeletionOperationId)
	require.Equal(t, decision.Generation, r.Generation)
	require.True(t, proto.Equal(decision.Manifest, r.Manifest))
	require.Equal(t, installed.BotID, r.BotId)
	require.Equal(t, installed.InstallationID, r.InstallationId)
	cancel := r.NativeCancellation
	require.NotNil(t, cancel)
	require.Equal(t, space, cancel.SpaceId)
	require.Equal(t, profile, cancel.ProfileId)
	require.Equal(t, installed.InstallOperationID, cancel.InstallOperationId)
	intent := cancel.InstallationIntent
	require.NotNil(t, intent)
	require.Equal(t, installed.BotID, intent.BotId)
	require.Equal(t, installed.InstallationID, intent.InstallationId)
	require.Equal(t, installed.BotGeneration, intent.Generation)
	require.Equal(t, space, intent.SpaceId)
	require.Equal(t, profile, intent.ActorProfileId)
	if installed.CancelOperationID != "" {
		require.Equal(t, installed.CancelOperationID, intent.OperationId)
	}
	require.Equal(t, complete.NativeReceiptID, receipt.ReceiptId)
	require.Equal(t, r.OperationId, receipt.OperationId)
	require.Equal(t, requestHash, receipt.RequestSha256)
	require.Equal(t, r.SpaceId, receipt.SpaceId)
	require.Equal(t, r.DeletionOperationId, receipt.DeletionOperationId)
	require.Equal(t, r.Generation, receipt.Generation)
	require.True(t, proto.Equal(r.Manifest, receipt.Manifest))
	require.Equal(t, r.BotId, receipt.BotId)
	require.Equal(t, r.InstallationId, receipt.InstallationId)
	require.True(t, proto.Equal(cancel, receipt.NativeCancellation))
	expected := spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_CANCELED
	if installed.Variant == "ordinary_canceled" {
		expected = spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_ALREADY_CANCELED
	}
	require.Equal(t, expected, receipt.Disposition)
	var settled bool
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.space_bot_installation_attempts WHERE install_operation_id=$1 AND state='CANCELED')
 AND EXISTS(SELECT 1 FROM public.space_bot_installation_heads WHERE space_id=$2 AND bot_id=$3 AND generation=$4 AND retired_through>=$4 AND state='CANCELED')
 AND NOT EXISTS(SELECT 1 FROM public.space_members WHERE space_id=$2 AND profile_id=$5)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE space_id=$2)
 AND EXISTS(SELECT 1 FROM public.space_lifecycle_aggregates WHERE space_id=$2 AND phase='PURGING' AND NOT local_purge_completed)
 AND NOT EXISTS(SELECT 1 FROM public.space_lifecycle_participants WHERE space_id=$2 AND participant_id=9 AND request_kind='PURGE' AND progress='COMPLETE')`, installed.InstallOperationID, space, installed.BotID, installed.BotGeneration, profile).Scan(&settled))
	require.True(t, settled, "own DB proves negative native effect, never a participant receipt from coordination JSON")
	return r, receipt
}
