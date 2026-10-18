package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"net"
	"path/filepath"
	"testing"
	"time"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/evidence"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
)

type evidenceAuthFixture struct{ binding *automodv1.SubjectBinding }

func (f evidenceAuthFixture) ResolveSubject(_ context.Context, account, subject uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
	if f.binding.GetAccountId() != account.String() || f.binding.GetSubjectId() != subject.String() {
		return nil, status.Error(codes.NotFound, "unknown subject")
	}
	return &automodv1.ResolveModerationSubjectResponse{Subject: proto.Clone(f.binding).(*automodv1.SubjectBinding), SessionEpoch: 1}, nil
}
func TestAutoModEvidenceRPC_CommittedOnlyAndExactReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000006_async_automod.up.sql"))
	account, subject, epoch := uuid.New(), uuid.New(), uuid.New()
	binding := &automodv1.SubjectBinding{ProtocolVersion: 1, AccountId: account.String(), SubjectId: subject.String(), LifecycleRevision: 1, State: automodv1.SubjectState_SUBJECT_STATE_LIVE}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	verifier := &socialprincipal.Verifier{Target: "moderation", Capability: "automod-evidence", Issuers: map[string]bool{"messaging": true, "auth": true}, Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }}
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.UnaryInterceptor(socialprincipal.StrictUnaryInterceptor(verifier)))
	gate := automod.NewBootGate()
	automodv1.RegisterModerationEvidenceServiceServer(server, &evidence.Service{Store: automod.NewStore(pool), Auth: evidenceAuthFixture{binding}, Gate: gate})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	conn, err := grpc.NewClient("passthrough:///automod-private-test", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := automodv1.NewModerationEvidenceServiceClient(conn)
	signed := func(request proto.Message, method string) context.Context {
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		id := uuid.NewString()
		token, err := issuer.IssueService(principal.ServiceInput{Audience: "moderation", RPC: method, RequestID: id, RequestHash: hash})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	}
	provision := &automodv1.ProvisionSubjectRequest{Subject: binding, SourceEpoch: epoch.String()}
	raw := metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-internal-caller", "messaging", "x-voice-user-id", account.String()))
	_, err = client.ProvisionSubject(raw, provision)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	admission := &automodv1.GetAccountSendRestrictionRequest{AccountId: account.String(), SubjectId: subject.String()}
	_, err = client.GetAccountSendRestriction(signed(admission, automodv1.ModerationEvidenceService_GetAccountSendRestriction_FullMethodName), admission)
	require.Equal(t, codes.Unavailable, status.Code(err), "valid private credentials cannot bypass the current-boot privacy barrier")
	boot, generation := gate.Identity()
	for phase := automod.BootLocalIssuer; phase <= automod.BootEffectsReady; phase++ {
		require.NoError(t, gate.Advance(automod.BootReceipt{BootID: boot, Generation: generation, Phase: phase, ProofID: uuid.New()}))
	}
	before, err := client.GetAccountSendRestriction(signed(admission, automodv1.ModerationEvidenceService_GetAccountSendRestriction_FullMethodName), admission)
	require.NoError(t, err)
	require.False(t, before.GetBanned())
	require.Nil(t, before.MutedUntil)
	digest := sha256.Sum256([]byte("account keyed pattern"))
	at := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	event := func(seq uint64) *automodv1.IngestCommittedMessageEvidenceRequest {
		return &automodv1.IngestCommittedMessageEvidenceRequest{Record: &automodv1.IngestCommittedMessageEvidenceRequest_Evidence{Evidence: &automodv1.CommittedMessageEvidence{
			ProtocolVersion: 1, Subject: binding, SourceEpoch: epoch.String(), Sequence: seq, EventId: uuid.NewString(), RecordedAt: timestamppb.New(at.Add(time.Duration(seq) * time.Second)), MessageId: uuid.NewString(), ChatId: uuid.NewString(), SenderProfileId: uuid.NewString(), OperationId: uuid.NewString(), Eligibility: automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_ELIGIBLE, Digests: []*automodv1.PatternDigest{{KeyId: "current", HmacSha256: digest[:]}}, LinkCount: 3}}}
	}
	first := event(1)
	_, err = client.IngestCommittedMessageEvidence(signed(first, automodv1.ModerationEvidenceService_IngestCommittedMessageEvidence_FullMethodName), first)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "unprovisioned evidence must not invent a stream baseline")
	_, err = client.ProvisionSubject(signed(provision, automodv1.ModerationEvidenceService_ProvisionSubject_FullMethodName), provision)
	require.NoError(t, err)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_actions`).Scan(&count))
	require.Zero(t, count)
	receipt, err := client.IngestCommittedMessageEvidence(signed(first, automodv1.ModerationEvidenceService_IngestCommittedMessageEvidence_FullMethodName), first)
	require.NoError(t, err)
	require.Equal(t, automodv1.EvidenceOutcome_EVIDENCE_OUTCOME_APPLIED, receipt.GetOutcome())
	muted, err := client.GetAccountSendRestriction(signed(admission, automodv1.ModerationEvidenceService_GetAccountSendRestriction_FullMethodName), admission)
	require.NoError(t, err)
	require.False(t, muted.GetBanned())
	require.NotNil(t, muted.GetMutedUntil())
	receipt, err = client.IngestCommittedMessageEvidence(signed(first, automodv1.ModerationEvidenceService_IngestCommittedMessageEvidence_FullMethodName), first)
	require.NoError(t, err)
	require.Equal(t, automodv1.EvidenceOutcome_EVIDENCE_OUTCOME_DUPLICATE, receipt.GetOutcome())
	second := event(2)
	_, err = client.IngestCommittedMessageEvidence(signed(second, automodv1.ModerationEvidenceService_IngestCommittedMessageEvidence_FullMethodName), second)
	require.NoError(t, err)
	banned, err := client.GetAccountSendRestriction(signed(admission, automodv1.ModerationEvidenceService_GetAccountSendRestriction_FullMethodName), admission)
	require.NoError(t, err)
	require.True(t, banned.GetBanned())
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM sanctions WHERE type='perm_ban'`).Scan(&count))
	require.Equal(t, 1, count)
	final, err := client.ProvisionSubject(signed(provision, automodv1.ModerationEvidenceService_ProvisionSubject_FullMethodName), provision)
	require.NoError(t, err)
	require.EqualValues(t, 2, final.GetThroughSequence(), "provision retry never resets the actual cursor")
}
