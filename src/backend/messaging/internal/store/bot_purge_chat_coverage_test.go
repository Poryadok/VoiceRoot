package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func TestBotPurgeChatCoverageRequiresOwnSealedIrreversibleBinding(t *testing.T) {
	ctx := t.Context()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	space, operation := uuid.New(), uuid.New()
	chat := uuid.MustParse("00000000-0000-4000-8000-000000000010")
	foreign := uuid.MustParse("00000000-0000-4000-8000-000000000020")
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: messagingChatManifestSHA(space, operation, 7, []uuid.UUID{chat}), ItemCount: 1}
	_, err := s.ImportSpacePurgeManifestPage(ctx, SpacePurgeManifestPageInput{SpaceID: space, DeletionOperationID: operation, ScheduleGeneration: 7, Page: &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: manifest, ItemIds: []string{chat.String()}, PageSha256: make([]byte, 32)}, SealsManifest: true, RequestBytes: []byte("page"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("page-receipt")})
	require.NoError(t, err)
	fence := SpaceLifecycleFenceInput{SpaceID: space, DeletionOperationID: operation, Generation: 7, State: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest, RequestBytes: []byte("frozen"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("frozen-receipt")}
	_, err = s.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	input := BotPurgeChatCoverageInput{SpaceID: space, DeletionOperationID: operation, PurgeGeneration: 8, Manifest: manifest, Chats: []uuid.UUID{chat, foreign}}
	_, err = s.ObserveBotPurgeChatCoverage(ctx, input)
	require.ErrorIs(t, err, ErrSpaceManifestBinding, "restorable FROZEN is not P3 coverage authority")
	fence.Generation = 8
	fence.State = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	fence.RequestBytes = []byte("purge")
	fence.ReceiptBytes = []byte("purge-receipt")
	_, err = s.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	observed, err := s.ObserveBotPurgeChatCoverage(ctx, input)
	require.NoError(t, err, "own SEALED manifest and exact irreversible binding provide membership, not completion")
	require.EqualValues(t, 7, observed.SourceScheduleGeneration)
	require.Equal(t, []bool{true, false}, observed.Covered, "foreign chat is explicitly uncovered")
	for _, mutate := range []struct {
		name   string
		change func(*BotPurgeChatCoverageInput)
	}{
		{"wrong operation", func(q *BotPurgeChatCoverageInput) { q.DeletionOperationID = uuid.New() }},
		{"wrong generation", func(q *BotPurgeChatCoverageInput) { q.PurgeGeneration++ }},
		{"wrong manifest", func(q *BotPurgeChatCoverageInput) { q.Manifest.ManifestId = uuid.NewString() }},
		{"wrong root hash", func(q *BotPurgeChatCoverageInput) { q.Manifest.ManifestSha256[0] ^= 1 }},
		{"wrong item count", func(q *BotPurgeChatCoverageInput) { q.Manifest.ItemCount++ }},
		{"duplicate target", func(q *BotPurgeChatCoverageInput) { q.Chats = []uuid.UUID{chat, chat} }},
		{"unsorted target", func(q *BotPurgeChatCoverageInput) { q.Chats = []uuid.UUID{foreign, chat} }},
		{"zero target", func(q *BotPurgeChatCoverageInput) { q.Chats = []uuid.UUID{uuid.Nil} }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			q := input
			q.Manifest = proto.Clone(manifest).(*commonv1.ManifestBinding)
			q.Chats = append([]uuid.UUID(nil), input.Chats...)
			mutate.change(&q)
			_, err := s.ObserveBotPurgeChatCoverage(ctx, q)
			require.ErrorIs(t, err, ErrSpaceManifestBinding)
		})
	}
	var originalSeal time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT sealed_at FROM messaging_space_chat_manifests WHERE space_id=$1`, space).Scan(&originalSeal))
	_, err = pool.Exec(ctx, `UPDATE messaging_space_chat_manifests SET sealed=false,sealed_at=NULL WHERE space_id=$1`, space)
	require.NoError(t, err)
	_, err = s.ObserveBotPurgeChatCoverage(ctx, input)
	require.ErrorIs(t, err, ErrSpaceManifestBinding)
	_, err = pool.Exec(ctx, `UPDATE messaging_space_chat_manifests SET sealed=true,sealed_at=$2 WHERE space_id=$1`, space, originalSeal)
	require.NoError(t, err)
	// The observation survives ordinary full-byte retention only as membership.
	// It neither recreates a receipt nor extends its original completion clock.
	purge := &messagingv1.PurgeSpaceRequest{Purge: &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 8, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, Manifest: manifest, PurgeDecidedAt: timestamppb.Now()}}
	_, err = s.CompleteSpaceLifecyclePurge(ctx, purge, uuid.NewString())
	require.NoError(t, err)
	key := SpacePurgeReceiptKey{SpaceID: space, DeletionOperationID: operation, PurgeGeneration: 8, SourceScheduleGeneration: 7, MessagingRequestSHA256: spacePurgeRequestHash(purge)}
	_, err = s.GetSpacePurgeResponse(ctx, key)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE messaging_space_purge_receipts SET completed_at=clock_timestamp()-interval '30 days 1 second' WHERE space_id=$1`, space)
	require.NoError(t, err)
	require.NoError(t, s.CleanupSpaceLifecycleEvidence(ctx))
	_, err = s.GetSpacePurgeResponse(ctx, key)
	require.ErrorIs(t, err, ErrSpacePurgeReceiptNotFound)
	observed, err = s.ObserveBotPurgeChatCoverage(ctx, input)
	require.NoError(t, err)
	require.Equal(t, []bool{true, false}, observed.Covered)
	var fullReceipts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messaging_space_purge_receipts WHERE space_id=$1`, space).Scan(&fullReceipts))
	require.Zero(t, fullReceipts)
	// Wait on an actual table lock, then mutate the caller's arrays. The owned
	// snapshot must still return the authenticated original source/target tuple.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(ctx, `LOCK TABLE messaging_space_lifecycle_fences IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	alias := input
	alias.Manifest = proto.Clone(manifest).(*commonv1.ManifestBinding)
	alias.Chats = append([]uuid.UUID(nil), input.Chats...)
	type result struct {
		value *BotPurgeChatCoverage
		err   error
	}
	done := make(chan result, 1)
	go func() { v, e := s.ObserveBotPurgeChatCoverage(ctx, alias); done <- result{v, e} }()
	require.Eventually(t, func() bool {
		var waiting bool
		e := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT generation,state,deletion_operation_id,source_schedule_generation%')`).Scan(&waiting)
		return e == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	alias.Manifest.ManifestSha256[0] ^= 1
	alias.Chats[0] = uuid.New()
	require.NoError(t, tx.Commit(ctx))
	select {
	case got := <-done:
		require.NoError(t, got.err)
		require.Equal(t, []bool{true, false}, got.value.Covered)
	case <-time.After(5 * time.Second):
		t.Fatal("coverage query did not resume")
	}
}
