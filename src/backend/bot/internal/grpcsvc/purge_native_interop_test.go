package grpcsvc

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

type p3NativeTransportFault struct {
	spacev1.SpaceServiceClient
	calls, reads int
	request      []byte
}

func (c *p3NativeTransportFault) GetBotPurgeHandoff(ctx context.Context, q *spacev1.GetBotPurgeHandoffRequest, options ...grpc.CallOption) (*spacev1.GetBotPurgeHandoffResponse, error) {
	c.reads++
	return c.SpaceServiceClient.GetBotPurgeHandoff(ctx, q, options...)
}

func (c *p3NativeTransportFault) ReconcileBotPurgeInstallation(ctx context.Context, q *spacev1.ReconcileBotPurgeInstallationRequest, options ...grpc.CallOption) (*spacev1.ReconcileBotPurgeInstallationResponse, error) {
	c.calls++
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	if err != nil {
		return nil, err
	}
	if c.request == nil {
		c.request = bytes.Clone(body)
	}
	if !bytes.Equal(c.request, body) {
		return nil, status.Error(codes.Internal, "test observed changed retained P3 request")
	}
	r, err := c.SpaceServiceClient.ReconcileBotPurgeInstallation(ctx, q, options...)
	if err != nil {
		return nil, err
	}
	if c.calls == 1 {
		return nil, status.Error(codes.DeadlineExceeded, "test lost ACK after actual native49 P3 commit")
	}
	if c.calls == 2 {
		r = proto.Clone(r).(*spacev1.ReconcileBotPurgeInstallationResponse)
		r.Receipt.DeletionOperationId = uuid.NewString()
	}
	return r, nil
}

// Actual BotPG intent and recovery worker against a separately built immutable
// Space49 provider. Setup files carry no public ingress or receipt authority.
func TestPurgeNativeActual49RelayAndTypedResultRecovery(t *testing.T) {
	dir := os.Getenv("VOICE_BOT_P3_HANDOFF_DIR")
	if dir == "" {
		t.Skip("requires caller-owned immutable two-phase Space49 provider")
	}
	var ready struct {
		Schema  int       `json:"schema_version"`
		Catalog int       `json:"catalog_version"`
		Source  string    `json:"source_pin"`
		Address string    `json:"address"`
		Server  string    `json:"server_name"`
		CA      string    `json:"ca_file"`
		Cert    string    `json:"client_cert_file"`
		Key     string    `json:"client_key_file"`
		Signer  string    `json:"signer_key_file"`
		KID     string    `json:"key_id"`
		Space   string    `json:"space_id"`
		Profile string    `json:"profile_id"`
		Expires time.Time `json:"expires_at"`
		Stop    string    `json:"stop_file"`
	}
	wait := func(name string, v any) {
		t.Helper()
		require.Eventually(t, func() bool {
			b, e := os.ReadFile(filepath.Join(dir, name))
			return e == nil && json.Unmarshal(b, v) == nil
		}, 150*time.Second, 25*time.Millisecond, name)
	}
	write := func(name string, v any) {
		t.Helper()
		b, e := json.Marshal(v)
		require.NoError(t, e)
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p+".tmp", b, 0600))
		require.NoError(t, os.Rename(p+".tmp", p))
	}
	wait("ready.json", &ready)
	require.Equal(t, 1, ready.Schema)
	require.Equal(t, 49, ready.Catalog)
	require.Equal(t, os.Getenv("VOICE_BOT_P3_SOURCE_PIN"), ready.Source)
	require.Len(t, ready.Source, 40)
	require.True(t, time.Now().Before(ready.Expires))
	require.Equal(t, filepath.Join(dir, "stop"), ready.Stop)
	require.NoFileExists(t, filepath.Join(dir, "installed.json"))
	ca, err := os.ReadFile(ready.CA)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	pair, err := tls.LoadX509KeyPair(ready.Cert, ready.Key)
	require.NoError(t, err)
	conn, err := grpc.NewClient(ready.Address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: ready.Server, Certificates: []tls.Certificate{pair}})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	key, err := principal.LoadRSAPrivateKeyFile(ready.Signer)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: ready.KID, PrivateKey: key})
	require.NoError(t, err)
	ctx := t.Context()
	_, source, _, _ := runtime.Caller(0)
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(filepath.Dir(source), "..", "..", "..", "migrations", "bot_db"))
	database.Run(true, "up")
	st := &store.BotStore{Pool: database.Pool}
	actor, space := uuid.MustParse(ready.Profile), uuid.MustParse(ready.Space)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Actual native49 P3 caller", "", `[]`, actor)
	require.NoError(t, err)
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, uuid.New(), nil)
	require.NoError(t, err)
	client := &p3NativeTransportFault{SpaceServiceClient: spacev1.NewSpaceServiceClient(conn)}
	svc := &BotGRPC{Store: st, PrincipalIssuer: issuer, InstallationSpace: client}
	require.NoError(t, svc.addInstallationMember(ctx, intent))
	require.NoError(t, st.CompleteInstallation(ctx, intent, nil))
	write("installed.json", map[string]any{"schema_version": 1, "bot_id": bot.ID.String(), "installation_id": intent.InstallationID.String(), "install_operation_id": intent.OperationID.String(), "bot_generation": intent.Generation, "variant": "installed"})
	var decision struct {
		Schema       int    `json:"schema_version"`
		Source       string `json:"source_pin"`
		Space        string `json:"space_id"`
		ManifestType string `json:"manifest_type"`
		Manifest     string `json:"manifest_base64"`
		FrozenType   string `json:"frozen_fence_type"`
		Frozen       string `json:"frozen_fence_base64"`
		DecidedType  string `json:"purge_decided_fence_type"`
		Decided      string `json:"purge_decided_fence_base64"`
		PurgeType    string `json:"bot_purge_type"`
		Purge        string `json:"bot_purge_base64"`
		Upstream     string `json:"upstream_mode"`
	}
	wait("purge-ready.json", &decision)
	require.Equal(t, 1, decision.Schema)
	require.Equal(t, ready.Source, decision.Source)
	require.Equal(t, ready.Space, decision.Space)
	require.Equal(t, "controlled-role-and-fences-empty-chat", decision.Upstream)
	decode := func(typeName, encoded string, p proto.Message) {
		t.Helper()
		require.Equal(t, string(p.ProtoReflect().Descriptor().FullName()), typeName)
		raw, e := base64.StdEncoding.DecodeString(encoded)
		require.NoError(t, e)
		require.NoError(t, proto.Unmarshal(raw, p))
		canonical, e := (proto.MarshalOptions{Deterministic: true}).Marshal(p)
		require.NoError(t, e)
		require.Equal(t, raw, canonical)
	}
	manifest, frozen, decided, purge := new(commonv1.ManifestBinding), new(commonv1.SpaceLifecycleFenceRequest), new(commonv1.SpaceLifecycleFenceRequest), new(commonv1.SpacePurgeRequest)
	decode(decision.ManifestType, decision.Manifest, manifest)
	decode(decision.FrozenType, decision.Frozen, frozen)
	decode(decision.DecidedType, decision.Decided, decided)
	decode(decision.PurgeType, decision.Purge, purge)
	require.Zero(t, manifest.ItemCount)
	require.True(t, proto.Equal(manifest, purge.Manifest))
	require.Equal(t, ready.Space, purge.SpaceId)
	_, err = st.ApplySpaceLifecycleFence(ctx, frozen)
	require.NoError(t, err)
	_, err = st.ApplySpaceLifecycleFence(ctx, decided)
	require.NoError(t, err)
	_, err = st.PurgeSpace(ctx, purge)
	require.ErrorIs(t, err, store.ErrDeletionPending)
	var saved []byte
	for n := 0; n < 4; n++ {
		_, err = database.Pool.Exec(ctx, `UPDATE bot_space_purge_handoffs SET next_attempt_at=clock_timestamp() WHERE space_id=$1`, space)
		require.NoError(t, err)
		worked, e := svc.ProcessNextBotPurgeHandoff(ctx)
		require.Equal(t, n < 3, worked)
		if n == 0 {
			require.Equal(t, codes.DeadlineExceeded, status.Code(e))
		} else if n == 1 {
			require.Equal(t, codes.Unavailable, status.Code(e))
		} else {
			require.NoError(t, e)
		}
		var body []byte
		require.NoError(t, database.Pool.QueryRow(ctx, `SELECT coverage_bytes FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, space, bot.ID).Scan(&body))
		var envelope struct {
			Native []struct {
				Request []byte `json:"request_bytes"`
				Receipt []byte `json:"receipt_bytes"`
			} `json:"native"`
		}
		require.NoError(t, json.Unmarshal(body, &envelope))
		require.Len(t, envelope.Native, 1)
		if n < 2 {
			require.Empty(t, envelope.Native[0].Receipt)
		} else {
			require.NotEmpty(t, envelope.Native[0].Receipt)
			if saved == nil {
				saved = bytes.Clone(envelope.Native[0].Receipt)
			} else {
				require.Equal(t, saved, envelope.Native[0].Receipt)
			}
		}
	}
	require.Equal(t, 3, client.calls)
	require.Equal(t, 1, client.reads)
	r := new(spacev1.ReconcileBotPurgeInstallationResponse)
	require.NoError(t, proto.Unmarshal(saved, r))
	q := new(spacev1.ReconcileBotPurgeInstallationRequest)
	require.NoError(t, proto.Unmarshal(client.request, q))
	require.NoError(t, store.ValidateBotPurgeNativeReceipt(q, r))
	localReceipt, err := st.PurgeSpace(ctx, purge)
	require.NoError(t, err)
	require.Equal(t, commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, localReceipt.State)
	write("complete.json", map[string]any{"schema_version": 1, "bot_id": bot.ID.String(), "installation_id": intent.InstallationID.String(), "drain_operation_id": q.OperationId, "native_receipt_id": r.Receipt.ReceiptId, "bot_purge_receipt_id": localReceipt.ReceiptId})
	var verified map[string]any
	wait("verified.json", &verified)
	replay, err := st.PurgeSpace(ctx, purge)
	require.NoError(t, err)
	require.True(t, proto.Equal(localReceipt, replay))
	var receipts int
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_lifecycle_purge_receipts WHERE space_id=$1`, space).Scan(&receipts))
	require.Equal(t, 1, receipts)
	var intents, rootsCount int
	var botStatus string
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_installation_intents WHERE space_id=$1`, space).Scan(&intents))
	require.Zero(t, intents)
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_operations WHERE bot_id=$1`, bot.ID).Scan(&rootsCount))
	require.Zero(t, rootsCount)
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT status FROM bots WHERE id=$1`, bot.ID).Scan(&botStatus))
	require.Equal(t, "live", botStatus)
	require.NoError(t, os.WriteFile(ready.Stop, []byte("stop"), 0600))
}
