package spacemembership

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"os"
	"path/filepath"
	"testing"
	"time"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/bot/internal/installprincipal"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

type nativeRole struct {
	rolev1.RoleServiceClient
	calls    int
	response *rolev1.ApplyBotInstallationRemovalResponse
	key      *rsa.PrivateKey
	t        *testing.T
}

func (n *nativeRole) ApplyBotInstallationRemoval(ctx context.Context, q *rolev1.ApplyBotInstallationRemovalRequest, _ ...grpc.CallOption) (*rolev1.ApplyBotInstallationRemovalResponse, error) {
	n.calls++
	verifyNativeCall(n.t, ctx, n.key, "role", rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName, q.OperationId, q)
	if n.response == nil {
		body, _ := proto.MarshalOptions{Deterministic: true}.Marshal(q)
		evidence, _ := proto.MarshalOptions{Deterministic: true}.Marshal(q.GetNative())
		h, e := sha256.Sum256(body), sha256.Sum256(evidence)
		n.response = &rolev1.ApplyBotInstallationRemovalResponse{Receipt: &rolev1.BotInstallationRemovalReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: q.OperationId, DecisionId: q.DecisionId, Purpose: q.Purpose, BotId: q.BotId, ActorProfileId: q.ActorProfileId, SpaceId: q.SpaceId, EvidenceKind: "native", EvidenceSha256: e[:], FrozenScopeSha256: q.FrozenScopeSha256, RequestSha256: h[:], Participant: "role", Outcome: "COMPLETED", CompletedAt: timestamppb.Now()}}
	}
	return proto.Clone(n.response).(*rolev1.ApplyBotInstallationRemovalResponse), nil
}

type nativeMessaging struct {
	messagingv1.MessagingServiceClient
	calls    int
	lost     bool
	bad      bool
	response *messagingv1.ApplyBotChatPinRemovalResponse
	key      *rsa.PrivateKey
	t        *testing.T
}

func (n *nativeMessaging) ApplyBotChatPinRemoval(ctx context.Context, q *messagingv1.ApplyBotChatPinRemovalRequest, _ ...grpc.CallOption) (*messagingv1.ApplyBotChatPinRemovalResponse, error) {
	n.calls++
	verifyNativeCall(n.t, ctx, n.key, "messaging", messagingv1.MessagingService_ApplyBotChatPinRemoval_FullMethodName, q.OperationId, q)
	if n.response == nil {
		body, _ := proto.MarshalOptions{Deterministic: true}.Marshal(q)
		evidence, _ := proto.MarshalOptions{Deterministic: true}.Marshal(q.GetNative())
		h, e := sha256.Sum256(body), sha256.Sum256(evidence)
		receipt := &messagingv1.BotChatPinRemovalReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: q.OperationId, DecisionId: q.DecisionId, Purpose: q.Purpose, BotId: q.BotId, ActorProfileId: q.ActorProfileId, SpaceId: q.SpaceId, ChatId: q.ChatId, EvidenceKind: "native", EvidenceSha256: e[:], FrozenScopeSha256: q.FrozenScopeSha256, RequestSha256: h[:], Participant: "messaging", Outcome: "COMPLETED", ExecutionCleanupRevision: 1, CompletedAt: timestamppb.Now()}
		for _, a := range q.Attachments {
			digest, err := messagingAttachmentDigest(q, a)
			if err != nil {
				n.t.Fatal(err)
			}
			receipt.Targets = append(receipt.Targets, &messagingv1.BotChatPinRemovalTargetReceipt{AttachmentId: a.AttachmentId, EffectTargetSha256: digest, CleanupRevision: 1})
		}
		n.response = &messagingv1.ApplyBotChatPinRemovalResponse{Receipt: receipt}
	}
	if n.lost {
		n.lost = false
		return nil, errors.New("reply lost after controlled receiver commit")
	}
	copy := proto.Clone(n.response).(*messagingv1.ApplyBotChatPinRemovalResponse)
	if n.bad {
		copy.Receipt.Outcome = "PENDING"
	}
	return copy, nil
}

type nativeChat struct {
	chatv1.ChatScopeAuthorityServiceClient
	calls int
	key   *rsa.PrivateKey
	t     *testing.T
}

func (n *nativeChat) GetBotChatScopeAuthority(ctx context.Context, q *chatv1.GetBotChatScopeAuthorityRequest, _ ...grpc.CallOption) (*chatv1.GetBotChatScopeAuthorityResponse, error) {
	n.calls++
	verifyNativeCall(n.t, ctx, n.key, "chat", chatv1.ChatScopeAuthorityService_GetBotChatScopeAuthority_FullMethodName, q.OperationId, q)
	b, _ := proto.MarshalOptions{Deterministic: true}.Marshal(q)
	h := sha256.Sum256(b)
	return &chatv1.GetBotChatScopeAuthorityResponse{RequestSha256: h[:], Scope: &chatv1.GetSpaceChatScopeAuthorityResponse{ProtocolVersion: 1, OperationId: q.OperationId, ChatId: q.ChatId, ExpectedSpaceId: q.ExpectedSpaceId, ActualSpaceId: q.ExpectedSpaceId, ChatScopeIncarnation: uuid.NewString(), ChatAuthorityRevision: 1, Exists: true, Live: true, ChatType: chatv1.ChatType_CHAT_TYPE_GROUP, ManagedKind: chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY, LifecycleState: chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE}}, nil
}
func verifyNativeCall(t *testing.T, ctx context.Context, key *rsa.PrivateKey, audience, method, operation string, q proto.Message) {
	t.Helper()
	md, _ := metadata.FromOutgoingContext(ctx)
	if len(md) != 2 {
		t.Fatal("identity metadata forwarded")
	}
	transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(ctx, md))
	if err != nil {
		t.Fatal(err)
	}
	h, _ := principal.RequestHash(q)
	if _, err = principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: audience, ExpectedRPC: method, ExpectedRequestID: operation, ExpectedRequestHash: h, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }}); err != nil {
		t.Fatal(err)
	}
}
func effectsAdmin(t *testing.T, q proto.Message, method string) installprincipal.Administrator {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	issuer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	if e != nil {
		t.Fatal(e)
	}
	h, _ := principal.RequestHash(q)
	id := uuid.NewString()
	token, e := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "bot", RPC: method, RequestID: id, RequestHash: h, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 1, ClientExpiresAt: time.Now().Add(time.Minute)})
	if e != nil {
		t.Fatal(e)
	}
	v := &installprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: func(context.Context, string, int64) error { return nil }}
	var a installprincipal.Administrator
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	_, e = v.Unary()(ctx, q, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
		a, e = installprincipal.Require(ctx, method, q)
		return nil, e
	})
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func TestNativeMembershipTypedCleanupLostReplyAndMalformedReceipt(t *testing.T) {
	endpoint := os.Getenv("BOT_MEMBERSHIP_TEST_DATABASE_URL")
	if endpoint == "" {
		t.Skip("explicit native database endpoint required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, e := pgxpool.New(ctx, endpoint)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	schema := "effects_" + uuid.NewString()[:8]
	if _, e = pool.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") }()
	cfg, e := pgxpool.ParseConfig(endpoint)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	files, e := filepath.Glob("../../../migrations/bot_db/*.up.sql")
	if e != nil {
		t.Fatal(e)
	}
	for _, file := range files {
		b, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(ctx, string(b)); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = db.Exec(ctx, "CREATE TABLE schema_migrations(version bigint,dirty boolean);INSERT INTO schema_migrations VALUES(9,false)"); e != nil {
		t.Fatal(e)
	}
	st := &store.BotStore{Pool: db}
	var source uuid.UUID
	if e = db.QueryRow(ctx, "SELECT source_generation FROM bot_space_membership_source").Scan(&source); e != nil {
		t.Fatal(e)
	}
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	issuer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	if e != nil {
		t.Fatal(e)
	}
	role := &nativeRole{key: key, t: t}
	messaging := &nativeMessaging{key: key, t: t, lost: true}
	chat := &nativeChat{key: key, t: t}
	client := &Client{Store: st, Source: source, issuer: issuer, role: role, messaging: messaging, chat: chat}
	bot, _, e := st.CreateBot(ctx, uuid.New(), "native-effects", "", "[]", uuid.New())
	if e != nil {
		t.Fatal(e)
	}
	q := &botv1.InstallBotInSpaceRequest{BotId: bot.ID.String(), SpaceId: uuid.NewString(), AllowedChats: []*chatv1.ChatRef{{Id: uuid.NewString()}}}
	if _, e = db.Exec(ctx, `UPDATE bots SET scopes='["TEXT_CHAT_READ_HISTORY"]'::jsonb WHERE id=$1`, bot.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = st.PrepareInstall(ctx, source, effectsAdmin(t, q, botv1.BotService_InstallBotInSpace_FullMethodName), q); e == nil {
		t.Fatal("privileged scope installed without explicit acknowledgement")
	}
	q.AcknowledgePrivilegedScopes = true
	a := effectsAdmin(t, q, botv1.BotService_InstallBotInSpace_FullMethodName)
	r, e := st.PrepareInstall(ctx, source, a, q)
	if e != nil {
		t.Fatal(e)
	}
	if e = client.ValidateChats(ctx, r, q); e != nil {
		t.Fatal(e)
	}
	if e = client.ValidateChats(ctx, r, q); e != nil || chat.calls != 1 {
		t.Fatalf("original observation replaced: %v", e)
	}
	r, e = st.ClaimMembershipAttempt(ctx, source, r, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	r, e = st.RecordAddAcknowledged(ctx, source, r)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = st.CompleteInstallation(ctx, source, r); e != nil {
		t.Fatal(e)
	}
	if e = st.SetChatEnabled(ctx, r.BotID, uuid.New(), r.SpaceID, a.ProfileID(), true); e == nil {
		t.Fatal("raw whitelist insertion bypassed Chat authority snapshot")
	}
	cancelQ := &botv1.UninstallBotFromSpaceRequest{BotId: q.BotId, SpaceId: q.SpaceId}
	r, e = st.BeginCancel(ctx, source, effectsAdmin(t, cancelQ, botv1.BotService_UninstallBotFromSpace_FullMethodName), r.SpaceID, r.BotID)
	if e != nil {
		t.Fatal(e)
	}
	r, e = st.ClaimMembershipAttempt(ctx, source, r, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	r, e = st.RecordCancelAcknowledged(ctx, source, r)
	if e != nil {
		t.Fatal(e)
	}
	if e = client.Cleanup(ctx, r, nil); e == nil {
		t.Fatal("lost reply reported success")
	}
	if e = st.CompleteCancellation(ctx, source, r); e == nil {
		t.Fatal("unresolved effect released generation")
	}
	messaging.bad = true
	if e = client.Cleanup(ctx, r, nil); e == nil {
		t.Fatal("PENDING typed receipt accepted")
	}
	messaging.bad = false
	// New producer object uses the exact persisted request and completed Role result.
	client = &Client{Store: &store.BotStore{Pool: db}, Source: source, issuer: issuer, role: role, messaging: messaging, chat: chat}
	if e = client.Cleanup(ctx, r, nil); e != nil {
		t.Fatal(e)
	}
	if role.calls != 1 || messaging.calls != 3 {
		t.Fatalf("unexpected retries role=%d messaging=%d", role.calls, messaging.calls)
	}
	if e = st.CompleteCancellation(ctx, source, r); e != nil {
		t.Fatal(e)
	}
	for _, statement := range []string{
		"UPDATE bot_space_membership_chat_sources SET response_bytes='changed'::bytea",
		"DELETE FROM bot_space_membership_chat_sources",
		"UPDATE bot_space_membership_cleanup_decisions SET scope_bytes='changed'::bytea",
		"UPDATE bot_space_membership_calls SET request_id='changed'",
		"UPDATE bot_space_membership_calls SET state='PENDING' WHERE state='SUCCEEDED'",
	} {
		if _, e = db.Exec(ctx, statement); e == nil {
			t.Fatalf("immutable evidence accepted: %s", statement)
		}
	}
	next, e := st.PrepareInstall(ctx, source, effectsAdmin(t, q, botv1.BotService_InstallBotInSpace_FullMethodName), q)
	if e != nil || next.Generation != 2 || next.InstallationID == r.InstallationID {
		t.Fatalf("settled successor: %v", e)
	}
}
