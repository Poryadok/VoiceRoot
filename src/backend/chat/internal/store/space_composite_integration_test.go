package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/chat/internal/chatcreation"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spacemutationlock"
)

func compositePG(t *testing.T) *pgxpool.Pool {
	t.Helper()
	f := integrationtest.NewSourceMigrationFixture(t, context.Background(), filepath.Join(chatRepoRoot(t), "src", "backend", "migrations", "chat_db"))
	f.Run(true, "goto", "27")
	return f.Pool
}

func TestSpaceCompositePurgeWaitsForActualDeliveryAndExpiresOwnEvidence(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := compositePG(t)
	s := &DMStore{Pool: pool}
	lifecycle := &SpaceLifecycleStore{Pool: pool}
	req := compositeCreateRequest()
	op, err := s.PrepareSpaceComposite(compositeCreator(t, req, uuid.NewString()), req)
	require.NoError(t, err)
	_, err = s.ResolveSpaceComposite(ctx, uuid.MustParse(op.Binding.OperationId), committedComposite(op))
	require.NoError(t, err)
	deletion := uuid.NewString()
	prepared, err := lifecycle.PrepareSpaceDeletionManifest(ctx, &chatv1.PrepareSpaceDeletionManifestRequest{ProtocolVersion: 1, SpaceId: req.GetSpaceId(), DeletionOperationId: deletion, ScheduleGeneration: 1})
	require.NoError(t, err)
	manifest := prepared.Receipt.ChatManifest
	for _, f := range []*commonv1.SpaceLifecycleFenceRequest{
		{ProtocolVersion: 1, SpaceId: req.GetSpaceId(), DeletionOperationId: deletion, Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest},
		{ProtocolVersion: 1, SpaceId: req.GetSpaceId(), DeletionOperationId: deletion, Generation: 2, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, Manifest: manifest},
	} {
		_, err = lifecycle.ApplySpaceLifecycleFence(ctx, &chatv1.ApplySpaceLifecycleFenceRequest{Fence: f})
		require.NoError(t, err)
	}
	purge := &chatv1.PurgeSpaceRequest{Purge: &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: req.GetSpaceId(), DeletionOperationId: deletion, Generation: 2, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_CHAT, Manifest: manifest, PurgeDecidedAt: timestamppb.Now()}}
	require.ErrorIs(t, lifecycle.ValidateSpacePurgeRequest(ctx, purge), ErrSpaceCompositeDrain, "no owner effects before durable delivery resolution")
	_, err = lifecycle.PurgeSpace(ctx, purge, spacePurgeEvidenceForTest(t, purge.Purge))
	require.ErrorIs(t, err, ErrSpaceCompositeDrain)
	for range 2 {
		e, claimErr := s.ClaimSpaceCompositeEffect(ctx, time.Minute)
		require.NoError(t, claimErr)
		require.NotNil(t, e)
		require.NoError(t, s.AckSpaceCompositeEffect(ctx, e))
	}
	require.NoError(t, lifecycle.ValidateSpacePurgeRequest(ctx, purge))
	finished, err := lifecycle.PurgeSpace(ctx, purge, spacePurgeEvidenceForTest(t, purge.Purge))
	require.NoError(t, err)
	var completion, expiry time.Time
	require.NoError(t, pool.QueryRow(ctx, "SELECT local_purge_completed_at,full_evidence_expires_at FROM chat_space_composite_operations WHERE operation_id=$1", op.Binding.OperationId).Scan(&completion, &expiry))
	require.True(t, completion.Equal(finished.Receipt.CompletedAt.AsTime()))
	require.True(t, expiry.Equal(completion.Add(30*24*time.Hour)))
	count, err := s.MinimizeSpaceCompositeEvidence(ctx, 10)
	require.NoError(t, err)
	require.Zero(t, count, "no TTL before own completion horizon")
	_, err = pool.Exec(ctx, "UPDATE chat_space_composite_operations SET local_purge_completed_at=statement_timestamp()-interval '30 days',full_evidence_expires_at=statement_timestamp() WHERE operation_id=$1", op.Binding.OperationId)
	require.NoError(t, err)
	_, err = s.LoadSpaceComposite(ctx, uuid.MustParse(op.Binding.OperationId))
	require.ErrorIs(t, err, ErrSpaceCompositeExpired)
	count, err = s.MinimizeSpaceCompositeEvidence(ctx, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	var personal bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT creator_profile_id IS NOT NULL OR client_request_id IS NOT NULL OR create_request_sha256 IS NOT NULL
		OR binding_bytes IS NOT NULL OR response_bytes IS NOT NULL OR outcome_bytes IS NOT NULL FROM chat_space_composite_operations WHERE operation_id=$1`, op.Binding.OperationId).Scan(&personal))
	require.False(t, personal)
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_create_requests WHERE operation_id=$1", op.Binding.OperationId).Scan(&n))
	require.Zero(t, n)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_space_composite_outbox WHERE operation_id=$1 AND payload IS NOT NULL", op.Binding.OperationId).Scan(&n))
	require.Zero(t, n)
}

func TestSpaceCompositeRollbackGuardsOldSnapshotAndFirstWriter(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := compositePG(t)
	down, err := os.ReadFile(filepath.Join(chatRepoRoot(t), "src", "backend", "migrations", "chat_db", "000027_space_composite_creations.down.sql"))
	require.NoError(t, err)
	// Establish an old empty RR snapshot before a first authority writer commits.
	old, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer func() { _ = old.Rollback(ctx) }()
	var n int
	require.NoError(t, old.QueryRow(ctx, "SELECT count(*) FROM chat_space_composite_operations").Scan(&n))
	require.Zero(t, n)
	req := compositeCreateRequest()
	s := &DMStore{Pool: pool}
	_, err = s.PrepareSpaceComposite(compositeCreator(t, req, uuid.NewString()), req)
	require.NoError(t, err)
	_, err = old.Exec(ctx, string(down))
	require.ErrorContains(t, err, "fresh READ COMMITTED")
	require.NoError(t, old.Rollback(ctx))
	rc, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = rc.Exec(ctx, string(down))
	require.ErrorContains(t, err, "retained composite authority")
	require.NoError(t, rc.Rollback(ctx))
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_space_composite_operations").Scan(&n))
	require.Equal(t, 1, n)
}

func TestSpaceCompositeRollbackSerializesEmptyFirstWriter(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool := compositePG(t)
	down, err := os.ReadFile(filepath.Join(chatRepoRoot(t), "src", "backend", "migrations", "chat_db", "000027_space_composite_creations.down.sql"))
	require.NoError(t, err)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(ctx, "LOCK TABLE chat_space_composite_operations IN ACCESS EXCLUSIVE MODE")
	require.NoError(t, err)
	req := compositeCreateRequest()
	creator := compositeCreator(t, req, uuid.NewString())
	finished := make(chan error, 1)
	go func() { _, e := (&DMStore{Pool: pool}).PrepareSpaceComposite(creator, req); finished <- e }()
	require.Eventually(t, func() bool {
		var blocked bool
		e := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='chat_space_composite_operations'::regclass AND NOT granted)`).Scan(&blocked)
		return e == nil && blocked
	}, 5*time.Second, 10*time.Millisecond, "first writer waits on the same relation DOWN locks before its empty probe")
	_, err = tx.Exec(ctx, string(down))
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	require.Error(t, <-finished, "old writer cannot commit authority after guarded schema removal")
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_create_requests").Scan(&n))
	require.Zero(t, n)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chats").Scan(&n))
	require.Zero(t, n)
}

func TestSpaceCompositeRecoveryLeaseAndConcurrentFinalize(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := compositePG(t)
	s := &DMStore{Pool: pool}
	req := compositeCreateRequest()
	op, err := s.PrepareSpaceComposite(compositeCreator(t, req, uuid.NewString()), req)
	require.NoError(t, err)
	first, err := s.ClaimSpaceComposite(ctx, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, first)
	none, err := s.ClaimSpaceComposite(ctx, time.Minute)
	require.NoError(t, err)
	require.Nil(t, none)
	_, err = pool.Exec(ctx, "UPDATE chat_space_composite_operations SET lease_expires_at=clock_timestamp()-interval '1 second' WHERE operation_id=$1", op.Binding.OperationId)
	require.NoError(t, err)
	second, err := s.ClaimSpaceComposite(ctx, time.Minute)
	require.NoError(t, err)
	require.NotEqual(t, first.Token, second.Token)
	_, err = s.ResolveClaimedSpaceComposite(ctx, first, committedComposite(op))
	require.ErrorIs(t, err, ErrSpaceCompositeLease, "expired worker cannot activate after reclaim")
	var state string
	require.NoError(t, pool.QueryRow(ctx, "SELECT state FROM chat_space_composite_operations WHERE operation_id=$1", op.Binding.OperationId).Scan(&state))
	require.Equal(t, "PREPARED", state, "lease expiry is not abort or completion")
	result := make(chan error, 2)
	for range 2 {
		go func() { _, e := s.ResolveClaimedSpaceComposite(ctx, second, committedComposite(op)); result <- e }()
	}
	// Both authenticated identical completions can succeed as replay; effects
	// and Chat activation still occur exactly once in the same transaction.
	require.NoError(t, <-result)
	require.NoError(t, <-result)
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chats WHERE id=$1", op.Binding.ChatId).Scan(&n))
	require.Equal(t, 1, n)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_space_composite_outbox WHERE eligibility='READY'").Scan(&n))
	require.Equal(t, 2, n)
	none, err = s.ClaimSpaceComposite(ctx, time.Minute)
	require.NoError(t, err)
	require.Nil(t, none)
}
func compositeCreateRequest() *chatv1.CreateChatRequest {
	space, name := uuid.NewString(), " prepared "
	return &chatv1.CreateChatRequest{Type: chatv1.ChatType_CHAT_TYPE_GROUP, SpaceId: &space, Name: &name, RequestId: uuid.NewString()}
}

func compositeCreator(t *testing.T, req *chatv1.CreateChatRequest, creator string) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{
		Kind: "delegated_user", Issuer: "gateway", Subject: creator, AccountID: creator, ProfileID: creator,
		Audience: "chat", RPC: chatv1.ChatService_CreateChat_FullMethodName, RequestID: req.RequestId,
		RequestHash: hash, SessionEpoch: 3, AccountType: "regular", ExpiresAt: time.Now().Add(30 * time.Second),
	})
}

func TestSpaceCompositeExpiredInitiatorAfterLockCannotPrepare(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool := compositePG(t)
	req := compositeCreateRequest()
	authority := compositeCreator(t, req, uuid.NewString())
	p, ok := principal.FromContext(authority)
	require.True(t, ok)
	p.ExpiresAt = time.Now().Add(time.Second)
	authority = principal.WithVerified(ctx, p)
	lock, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = lock.Rollback(context.Background()) }()
	_, err = lock.Exec(ctx, "SELECT pg_advisory_xact_lock($1::bigint)", spacemutationlock.Key(uuid.MustParse(req.GetSpaceId())))
	require.NoError(t, err)
	finished := make(chan error, 1)
	go func() { _, e := (&DMStore{Pool: pool}).PrepareSpaceComposite(authority, req); finished <- e }()
	require.Eventually(t, func() bool {
		var n int
		return pool.QueryRow(ctx, "SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND NOT granted").Scan(&n) == nil && n > 0
	}, time.Second, 5*time.Millisecond)
	require.Eventually(t, func() bool { return time.Now().After(p.ExpiresAt) }, 2*time.Second, 5*time.Millisecond)
	require.NoError(t, lock.Commit(ctx))
	require.ErrorIs(t, <-finished, chatcreation.ErrInitiator)
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_space_composite_operations").Scan(&n))
	require.Zero(t, n)
}

func committedComposite(op *SpaceCompositeOperation) *spacev1.SpaceChatCreationOutcome {
	b := proto.Clone(op.Binding).(*spacev1.SpaceChatCreationBinding)
	kind := b.ChatType
	return &spacev1.SpaceChatCreationOutcome{Binding: b, State: spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED,
		Node: &spacev1.SpaceTreeNode{Id: b.NodeId, SpaceId: b.SpaceId, Kind: "text_chat", LinkedChat: &chatv1.ChatRef{Id: b.ChatId, Type: &kind}}}
}

func TestSpaceCompositePrepareHiddenReplayAndFinalize(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := compositePG(t)
	s := &DMStore{Pool: pool}
	req, actor := compositeCreateRequest(), uuid.NewString()
	first, err := s.PrepareSpaceComposite(compositeCreator(t, req, actor), req)
	require.NoError(t, err)
	require.Equal(t, "PREPARED", first.State)
	require.Equal(t, "prepared", first.Response.Chat.GetName())
	require.Equal(t, actor, first.Binding.CreatorProfileId)
	hidden, err := s.FindChatByID(ctx, uuid.MustParse(first.Binding.ChatId))
	require.NoError(t, err)
	require.Nil(t, hidden, "prepared Chat is not an ordinary/public Chat row")
	var held int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_space_composite_outbox WHERE eligibility='HELD'").Scan(&held))
	require.Equal(t, 2, held)
	again, err := s.PrepareSpaceComposite(compositeCreator(t, req, actor), req)
	require.NoError(t, err)
	require.True(t, proto.Equal(first.Binding, again.Binding))
	changed := proto.Clone(req).(*chatv1.CreateChatRequest)
	name := "changed"
	changed.Name = &name
	_, err = s.PrepareSpaceComposite(compositeCreator(t, changed, actor), changed)
	require.ErrorIs(t, err, ErrChatCreateRequestConflict)
	bad := committedComposite(first)
	bad.Binding.AuditEventId = uuid.NewString()
	_, err = s.ResolveSpaceComposite(ctx, uuid.MustParse(first.Binding.OperationId), bad)
	require.ErrorIs(t, err, ErrSpaceCompositeEvidence)
	unknown := committedComposite(first)
	unknown.Node.ProtoReflect().SetUnknown([]byte{0xf8, 0x07, 0x01})
	_, err = s.ResolveSpaceComposite(ctx, uuid.MustParse(first.Binding.OperationId), unknown)
	require.ErrorIs(t, err, ErrSpaceCompositeEvidence, "unknown nested outcome fields cannot activate a prepared Chat")
	finished, err := s.ResolveSpaceComposite(ctx, uuid.MustParse(first.Binding.OperationId), committedComposite(first))
	require.NoError(t, err)
	require.Equal(t, "COMPLETE", finished.State)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_space_composite_outbox WHERE eligibility='READY'").Scan(&held))
	require.Equal(t, 2, held)
	visible, err := s.FindChatByID(ctx, uuid.MustParse(first.Binding.ChatId))
	require.NoError(t, err)
	require.NotNil(t, visible)
	_, err = pool.Exec(ctx, "UPDATE chats SET name='later' WHERE id=$1", visible.ID)
	require.NoError(t, err)
	replay, err := s.PrepareSpaceComposite(compositeCreator(t, req, actor), req)
	require.NoError(t, err)
	require.True(t, proto.Equal(finished.Response, replay.Response), "keyed create replays original result, not later metadata")
}

func TestSpaceCompositeConcurrentKeyedPrepareReservesOneBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	pool := compositePG(t)
	s := &DMStore{Pool: pool}
	req := compositeCreateRequest()
	ctx := compositeCreator(t, req, uuid.NewString())
	type result struct {
		op  *SpaceCompositeOperation
		err error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for range 2 {
		go func() { <-start; op, err := s.PrepareSpaceComposite(ctx, req); results <- result{op, err} }()
	}
	close(start)
	a, b := <-results, <-results
	require.NoError(t, a.err)
	require.NoError(t, b.err)
	require.True(t, proto.Equal(a.op.Binding, b.op.Binding))
	require.True(t, proto.Equal(a.op.Response, b.op.Response))
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_space_composite_operations").Scan(&n))
	require.Equal(t, 1, n)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_create_requests").Scan(&n))
	require.Equal(t, 1, n)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_space_composite_outbox WHERE eligibility='HELD'").Scan(&n))
	require.Equal(t, 2, n)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chats").Scan(&n))
	require.Zero(t, n)
}

func TestSpaceCompositePrepareRollbackAndVerifiedAbort(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := compositePG(t)
	s := &DMStore{Pool: pool}
	_, err := pool.Exec(ctx, "ALTER TABLE chat_space_composite_outbox ADD CONSTRAINT reject_prepare CHECK(false) NOT VALID")
	require.NoError(t, err)
	req, actor := compositeCreateRequest(), uuid.NewString()
	_, err = s.PrepareSpaceComposite(compositeCreator(t, req, actor), req)
	require.Error(t, err)
	var count int
	for _, table := range []string{"chat_create_requests", "chat_space_composite_operations", "chat_space_composite_outbox", "chats"} {
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
		require.Zero(t, count, table)
	}
	_, err = pool.Exec(ctx, "ALTER TABLE chat_space_composite_outbox DROP CONSTRAINT reject_prepare")
	require.NoError(t, err)
	op, err := s.PrepareSpaceComposite(compositeCreator(t, req, actor), req)
	require.NoError(t, err)
	abort := &spacev1.SpaceChatCreationOutcome{Binding: proto.Clone(op.Binding).(*spacev1.SpaceChatCreationBinding),
		State: spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_ABORTED, FailureCode: uint32(codes.PermissionDenied)}
	stopped, err := s.ResolveSpaceComposite(ctx, uuid.MustParse(op.Binding.OperationId), abort)
	require.NoError(t, err)
	require.Equal(t, "ABORTED", stopped.State)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chats").Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_space_composite_outbox WHERE eligibility='CANCELLED'").Scan(&count))
	require.Equal(t, 2, count)
	_, err = s.ResolveSpaceComposite(ctx, uuid.MustParse(op.Binding.OperationId), committedComposite(op))
	require.ErrorIs(t, err, ErrSpaceCompositeEvidence, "opposite terminal result never wins")
}

func TestSpaceCompositeFinalizeRefusesLostOrReboundHeldEffect(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := compositePG(t)
	s := &DMStore{Pool: pool}
	req := compositeCreateRequest()
	op, err := s.PrepareSpaceComposite(compositeCreator(t, req, uuid.NewString()), req)
	require.NoError(t, err)
	var payload []byte
	require.NoError(t, pool.QueryRow(ctx, "SELECT payload FROM chat_space_composite_outbox WHERE operation_id=$1 AND effect_kind='AUDIT'", op.Binding.OperationId).Scan(&payload))
	_, err = pool.Exec(ctx, "UPDATE chat_space_composite_outbox SET payload=$2 WHERE operation_id=$1 AND effect_kind='AUDIT'", op.Binding.OperationId, []byte{1})
	require.NoError(t, err)
	_, err = s.ResolveSpaceComposite(ctx, uuid.MustParse(op.Binding.OperationId), committedComposite(op))
	require.ErrorIs(t, err, ErrSpaceCompositeEvidence, "corrupt producer fact cannot activate Chat")
	_, err = pool.Exec(ctx, "UPDATE chat_space_composite_outbox SET payload=$2 WHERE operation_id=$1 AND effect_kind='AUDIT'", op.Binding.OperationId, payload)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "DELETE FROM chat_space_composite_outbox WHERE operation_id=$1 AND effect_kind='CHAT_CREATED'", op.Binding.OperationId)
	require.NoError(t, err)
	_, err = s.ResolveSpaceComposite(ctx, uuid.MustParse(op.Binding.OperationId), committedComposite(op))
	require.ErrorIs(t, err, ErrSpaceCompositeEvidence, "missing durable notification cannot be silently treated as complete")
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chats").Scan(&n))
	require.Zero(t, n)
}

func TestSpaceCompositeEffectsHeldAndStaleAcknowledgement(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := compositePG(t)
	s := &DMStore{Pool: pool}
	req := compositeCreateRequest()
	op, err := s.PrepareSpaceComposite(compositeCreator(t, req, uuid.NewString()), req)
	require.NoError(t, err)
	none, err := s.ClaimSpaceCompositeEffect(ctx, time.Minute)
	require.NoError(t, err)
	require.Nil(t, none, "HELD is never delivery eligible")
	_, err = s.ResolveSpaceComposite(ctx, uuid.MustParse(op.Binding.OperationId), committedComposite(op))
	require.NoError(t, err)
	first, err := s.ClaimSpaceCompositeEffect(ctx, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, first)
	_, err = pool.Exec(ctx, "UPDATE chat_space_composite_outbox SET lease_expires_at=clock_timestamp()-interval '1 second' WHERE event_id=$1", first.EventID)
	require.NoError(t, err)
	// Prevent the other effect from winning the ORDER BY during reclaim.
	_, err = pool.Exec(ctx, "UPDATE chat_space_composite_outbox SET next_attempt_at=clock_timestamp()+interval '1 hour' WHERE event_id<>$1", first.EventID)
	require.NoError(t, err)
	second, err := s.ClaimSpaceCompositeEffect(ctx, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, first.EventID, second.EventID)
	require.Equal(t, first.Payload, second.Payload, "a retry sends the exact durable original bytes")
	require.NotEqual(t, first.Token, second.Token)
	require.ErrorIs(t, s.AckSpaceCompositeEffect(ctx, first), ErrSpaceCompositeLease)
	require.NoError(t, s.AckSpaceCompositeEffect(ctx, second))
	var delivered int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_space_composite_outbox WHERE delivered_at IS NOT NULL").Scan(&delivered))
	require.Equal(t, 1, delivered)
}

func TestSpaceCompositeFreezeDrainsBeforeManifestAndRemainsHidden(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := compositePG(t)
	s := &DMStore{Pool: pool}
	lifecycle := &SpaceLifecycleStore{Pool: pool}
	req := compositeCreateRequest()
	op, err := s.PrepareSpaceComposite(compositeCreator(t, req, uuid.NewString()), req)
	require.NoError(t, err)
	freeze := &chatv1.PrepareSpaceDeletionManifestRequest{ProtocolVersion: 1, SpaceId: req.GetSpaceId(), DeletionOperationId: uuid.NewString(), ScheduleGeneration: 1}
	response, err := lifecycle.PrepareSpaceDeletionManifest(ctx, freeze)
	require.ErrorIs(t, err, ErrSpaceCompositeDrain, "PREPARED prevents immutable manifest acknowledgement")
	require.Nil(t, response)
	var state string
	var draining bool
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT state,draining_composites FROM chat_space_lifecycle_fences WHERE space_id=$1", req.GetSpaceId()).Scan(&state, &draining))
	require.Equal(t, "FROZEN", state)
	require.True(t, draining)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_space_lifecycle_operations WHERE operation_kind='PREPARE'").Scan(&n))
	require.Zero(t, n)
	other := proto.Clone(req).(*chatv1.CreateChatRequest)
	other.RequestId = uuid.NewString()
	_, err = s.PrepareSpaceComposite(compositeCreator(t, other, uuid.NewString()), other)
	require.ErrorIs(t, err, ErrSpaceLifecycleState)
	_, err = s.ResolveSpaceComposite(ctx, uuid.MustParse(op.Binding.OperationId), committedComposite(op))
	require.NoError(t, err, "verified COMMITTED must roll forward while frozen")
	hidden, err := s.FindChatByID(ctx, uuid.MustParse(op.Binding.ChatId))
	require.NoError(t, err)
	require.Nil(t, hidden)
	sealed, err := lifecycle.PrepareSpaceDeletionManifest(ctx, freeze)
	require.NoError(t, err)
	require.EqualValues(t, 1, sealed.Receipt.ChatManifest.ItemCount, "manifest includes committed drained Chat")
	replayed, err := lifecycle.PrepareSpaceDeletionManifest(ctx, freeze)
	require.NoError(t, err)
	require.True(t, proto.Equal(sealed, replayed))
	require.NoError(t, pool.QueryRow(ctx, "SELECT draining_composites FROM chat_space_lifecycle_fences WHERE space_id=$1", req.GetSpaceId()).Scan(&draining))
	require.False(t, draining)
}
