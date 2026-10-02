package store

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/principal"
)

func TestSpaceFileProducerSnapshotSurvivesPayloadDeletionAndRejectsUnfencedCapture(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	space, operation, chat, message, file := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,attachments,created_at) VALUES($1,$2,'dm',$3,'fixture',$4::jsonb,clock_timestamp()-interval '1 hour')`, message, chat, uuid.New(), fmt.Sprintf(`[{"type":"image","file_id":"%s"}]`, file))
	require.NoError(t, err)
	_, err = s.SpaceFileProducerReferences(ctx, space, operation, 7)
	require.Error(t, err)
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ItemCount: 1, ManifestSha256: messagingChatManifestSHA(space, operation, 7, []uuid.UUID{chat})}
	_, err = s.ImportSpacePurgeManifestPage(ctx, SpacePurgeManifestPageInput{SpaceID: space, DeletionOperationID: operation, ScheduleGeneration: 7, Page: &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: manifest, ItemIds: []string{chat.String()}, PageSha256: make([]byte, 32)}, SealsManifest: true, RequestBytes: []byte("page"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("page-receipt")})
	require.NoError(t, err)
	_, err = s.ApplySpaceLifecycleFence(ctx, SpaceLifecycleFenceInput{SpaceID: space, DeletionOperationID: operation, Generation: 7, State: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest, RequestBytes: []byte("freeze"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("fence-receipt")})
	require.NoError(t, err)
	first, err := s.SpaceFileProducerReferences(ctx, space, operation, 7)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, space.String(), first[0].GetScopeSpaceId())
	require.Equal(t, file.String(), first[0].FileId)
	_, err = s.ApplySpaceLifecycleFence(ctx, SpaceLifecycleFenceInput{SpaceID: space, DeletionOperationID: operation, Generation: 8, State: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, Manifest: manifest, RequestBytes: []byte("purge"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("purge-receipt")})
	require.NoError(t, err)
	child := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("voice.messaging.v1.SpaceChatPurge\x00%s\x00%s\x00%s", space, operation, chat)))
	_, err = s.StartManagedChatPurge(ctx, child, chat, time.Now().UTC().Add(-time.Second), make([]byte, 32))
	require.NoError(t, err)
	_, err = s.CompleteManagedChatPurge(ctx, child, make([]byte, 32), make([]byte, 32))
	require.Error(t, err, "stable child UUID is not permission to bypass the Space fence")
	parent := &messagingv1.PurgeSpaceRequest{Purge: &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 8, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, Manifest: manifest, PurgeDecidedAt: timestamppb.Now()}}
	parentHash, err := principal.RequestHash(parent)
	require.NoError(t, err)
	authorized := principal.WithVerified(ctx, principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "messaging", RPC: messagingv1.MessagingService_PurgeSpace_FullMethodName, RequestID: uuid.NewString(), RequestHash: parentHash})
	authorized, err = SpacePurgeContext(authorized, parent)
	require.NoError(t, err)
	_, err = s.CompleteManagedChatPurge(authorized, child, make([]byte, 32), make([]byte, 32))
	require.NoError(t, err)
	replay, err := s.SpaceFileProducerReferences(ctx, space, operation, 7)
	require.NoError(t, err)
	require.Len(t, replay, 1)
	require.True(t, proto.Equal(first[0], replay[0]))
	fullReceipt, err := s.CompleteSpaceLifecyclePurge(ctx, parent, uuid.NewString())
	require.NoError(t, err)
	require.NoError(t, s.CleanupSpaceLifecycleEvidence(ctx))
	replayReceipt, err := s.CompleteSpaceLifecyclePurge(ctx, parent, uuid.NewString())
	require.NoError(t, err)
	require.Equal(t, fullReceipt, replayReceipt)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messaging_space_file_producers WHERE space_id=$1`, space).Scan(&count))
	require.Equal(t, 1, count)
	_, err = pool.Exec(ctx, `UPDATE messaging_space_purge_receipts SET completed_at=clock_timestamp()-interval '30 days 1 second' WHERE space_id=$1`, space)
	require.NoError(t, err)
	require.NoError(t, s.CleanupSpaceLifecycleEvidence(ctx))
	require.NoError(t, s.CleanupSpaceLifecycleEvidence(ctx))
	for _, table := range []string{"messaging_space_file_producers", "messaging_space_purge_receipts", "managed_chat_purge_operations", "managed_chat_purge_messages", "messaging_space_lifecycle_operations"} {
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
		require.Zero(t, count, table)
	}
	var state string
	var privateBytes int
	require.NoError(t, pool.QueryRow(ctx, `SELECT state,octet_length(request_bytes)+octet_length(receipt_bytes) FROM messaging_space_lifecycle_fences WHERE space_id=$1`, space).Scan(&state, &privateBytes))
	require.Equal(t, "PURGED", state)
	require.Zero(t, privateBytes)
	require.NoError(t, pool.QueryRow(ctx, `SELECT octet_length(page_bytes)+octet_length(request_bytes)+octet_length(receipt_bytes)+length(next_page_token) FROM messaging_space_chat_manifest_pages WHERE space_id=$1`, space).Scan(&privateBytes))
	require.Zero(t, privateBytes)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messaging_space_chat_manifest_items WHERE space_id=$1 AND chat_id=$2`, space, chat).Scan(&count))
	require.Equal(t, 1, count, "minimal chat fence must survive private evidence expiry")
	_, err = s.CompleteSpaceLifecyclePurge(ctx, parent, uuid.NewString())
	require.Error(t, err, "expired full evidence cannot fabricate a new exact receipt")
	_, err = s.SpaceFileProducerReferences(ctx, space, operation, 7)
	require.Error(t, err, "expired private producer bytes must not be recaptured after PURGED")
	_, err = s.StartManagedChatPurge(ctx, child, chat, parent.Purge.PurgeDecidedAt.AsTime(), make([]byte, 32))
	require.ErrorIs(t, err, ErrManagedChatPurgeOperationConflict, "ordinary replay cannot fabricate a new empty child receipt after private evidence expiry")
}
