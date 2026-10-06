package store

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/chat/internal/chatevents"
)

func TestSpacePurgeOwnerEvidenceRequiresExactMessagingAndFileBindings(t *testing.T) {
	spaceID, operationID := uuid.NewString(), uuid.NewString()
	purge := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: spaceID, DeletionOperationId: operationID, Generation: 2,
		PurgeDecidedAt: timestamppb.New(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_CHAT,
		Manifest: &commonv1.ManifestBinding{ManifestId: operationID, ManifestSha256: make([]byte, 32), ItemCount: 4}}
	evidence := spacePurgeEvidenceForTest(t, purge)
	require.NoError(t, validateSpacePurgeOwnerEvidence(purge, evidence))

	changed := proto.Clone(evidence.FileReleaseReceipt).(*filev1.ReleaseSpaceDeletionProducerReferencesReceipt)
	changed.PurgeGeneration++
	evidence.FileReleaseReceipt = changed
	require.ErrorIs(t, validateSpacePurgeOwnerEvidence(purge, evidence), ErrSpaceLifecycleEvidence)
}

func TestSpacePurgeDeletesOnlySavedChatRowsAfterOwnerProofAndReplays(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	applyChatMigrationFile(t, ctx, pool, "000021_chat_deleted_event_outbox.up.sql")
	spaceID, operationID, profileID := uuid.New(), uuid.New(), uuid.New()
	chatID := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO chats(id,type,space_id,name,creator_profile_id) VALUES($1,'channel',$2,'remove-me',$3)`, chatID, spaceID, profileID)
	require.NoError(t, err)
	lifecycle := &SpaceLifecycleStore{Pool: pool}
	prepared, err := lifecycle.PrepareSpaceDeletionManifest(ctx, &chatv1.PrepareSpaceDeletionManifestRequest{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), ScheduleGeneration: 1})
	require.NoError(t, err)
	global := proto.Clone(prepared.GetReceipt().GetChatManifest()).(*commonv1.ManifestBinding)
	for _, fence := range []*commonv1.SpaceLifecycleFenceRequest{
		{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: global},
		{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: 2, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, Manifest: global},
	} {
		_, err = lifecycle.ApplySpaceLifecycleFence(ctx, &chatv1.ApplySpaceLifecycleFenceRequest{Fence: fence})
		require.NoError(t, err)
	}
	purge := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: 2, PurgeDecidedAt: timestamppb.Now(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_CHAT, Manifest: global}
	request := &chatv1.PurgeSpaceRequest{Purge: purge}
	changed := proto.Clone(request).(*chatv1.PurgeSpaceRequest)
	changed.Purge.Manifest.ManifestSha256 = bytes.Repeat([]byte{0x5a}, 32)
	require.ErrorIs(t, lifecycle.ValidateSpacePurgeRequest(ctx, changed), ErrSpaceLifecycleState)
	require.NoError(t, lifecycle.ValidateSpacePurgeRequest(ctx, request))
	evidence := spacePurgeEvidenceForTest(t, purge)
	_, err = lifecycle.PurgeSpace(ctx, changed, evidence)
	require.Error(t, err, "invalid purge evidence must not delete Chat rows or enqueue a terminal event")
	var outboxRows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chat_deleted_event_outbox`).Scan(&outboxRows))
	require.Zero(t, outboxRows)
	var beforePurge int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chats WHERE id=$1`, chatID).Scan(&beforePurge))
	require.Equal(t, 1, beforePurge)
	first, err := lifecycle.PurgeSpace(ctx, request, evidence)
	require.NoError(t, err)
	var eventID uuid.UUID
	var eventBytes []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_id,event_bytes FROM chat_deleted_event_outbox WHERE chat_id=$1`, chatID).Scan(&eventID, &eventBytes))
	require.NotEqual(t, uuid.Nil, eventID)
	require.NotEmpty(t, eventBytes)
	messagingProofBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(evidence.MessagingReceipt)
	require.NoError(t, err)
	fileProofBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(evidence.FileReleaseReceipt)
	require.NoError(t, err)
	var savedMessagingProof, savedFileProof []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT messaging_receipt_bytes,file_release_receipt_bytes FROM chat_space_lifecycle_operations WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=2 AND operation_kind='PURGE'`, spaceID, operationID).Scan(&savedMessagingProof, &savedFileProof))
	require.Equal(t, messagingProofBytes, savedMessagingProof)
	require.Equal(t, fileProofBytes, savedFileProof)
	replay, err := lifecycle.PurgeSpace(ctx, proto.Clone(request).(*chatv1.PurgeSpaceRequest), evidence)
	require.NoError(t, err)
	require.True(t, proto.Equal(first, replay))
	var replayEventID uuid.UUID
	var replayEventBytes []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_id,event_bytes FROM chat_deleted_event_outbox WHERE chat_id=$1`, chatID).Scan(&replayEventID, &replayEventBytes))
	require.Equal(t, eventID, replayEventID)
	require.Equal(t, eventBytes, replayEventBytes)
	bytePublisher := &recordingChatDeletedBytesPublisher{err: errors.New("temporary publish failure")}
	outbox := &chatevents.ChatDeletedOutbox{Pool: pool, Publisher: bytePublisher}
	_, err = pool.Exec(ctx, `UPDATE chat_space_lifecycle_operations SET retain_until=clock_timestamp()-interval '1 second' WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=2 AND operation_kind='PURGE'`, spaceID, operationID)
	require.NoError(t, err, "simulate PubAck delayed beyond the original receipt horizon")
	require.NoError(t, outbox.RunOnce(ctx))
	var attempts int64
	var publishedAt pgtype.Timestamptz
	require.NoError(t, pool.QueryRow(ctx, `SELECT attempt_count,published_at FROM chat_deleted_event_outbox WHERE event_id=$1`, eventID).Scan(&attempts, &publishedAt))
	require.EqualValues(t, 1, attempts)
	require.False(t, publishedAt.Valid)
	var retained int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chat_deleted_event_outbox WHERE event_id=$1`, eventID).Scan(&retained))
	require.Equal(t, 1, retained, "an expired parent horizon cannot prune a pending publish")
	_, err = pool.Exec(ctx, `UPDATE chat_deleted_event_outbox SET next_attempt_at=clock_timestamp() WHERE event_id=$1`, eventID)
	require.NoError(t, err)
	bytePublisher.err = nil
	bytePublisher.onPublish = func() error {
		// Simulate another worker reclaiming the row before this publish returns.
		_, updateErr := pool.Exec(ctx, `UPDATE chat_deleted_event_outbox SET lease_token=$2,lease_until=clock_timestamp()+interval '1 minute' WHERE event_id=$1`, eventID, uuid.New())
		return updateErr
	}
	require.NoError(t, outbox.RunOnce(ctx))
	require.Equal(t, eventID.String(), bytePublisher.eventID)
	require.Equal(t, eventBytes, bytePublisher.eventBytes)
	require.Equal(t, 2, bytePublisher.calls)
	require.NoError(t, pool.QueryRow(ctx, `SELECT attempt_count,published_at FROM chat_deleted_event_outbox WHERE event_id=$1`, eventID).Scan(&attempts, &publishedAt))
	require.EqualValues(t, 2, attempts)
	require.False(t, publishedAt.Valid, "stale PubAck cannot complete a row reclaimed under another token")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chat_deleted_event_outbox WHERE event_id=$1`, eventID).Scan(&retained))
	require.Equal(t, 1, retained, "reclaimed pending bytes remain even after the parent receipt horizon")
	_, err = pool.Exec(ctx, `UPDATE chat_deleted_event_outbox SET lease_until=clock_timestamp()-interval '1 second',next_attempt_at=clock_timestamp() WHERE event_id=$1`, eventID)
	require.NoError(t, err)
	bytePublisher.onPublish = func() error {
		// An expired lease can still acknowledge if no worker has reclaimed its token.
		_, updateErr := pool.Exec(ctx, `UPDATE chat_deleted_event_outbox SET lease_until=clock_timestamp()-interval '1 second' WHERE event_id=$1`, eventID)
		return updateErr
	}
	require.NoError(t, outbox.RunOnce(ctx))
	require.Equal(t, eventID.String(), bytePublisher.eventID)
	require.Equal(t, eventBytes, bytePublisher.eventBytes)
	require.Equal(t, 3, bytePublisher.calls)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chat_deleted_event_outbox WHERE event_id=$1`, eventID).Scan(&retained))
	require.Zero(t, retained, "confirmed events are cleaned only after the parent receipt horizon")
	changedEvidence := evidence
	changedEvidence.FileReleaseReceipt = proto.Clone(evidence.FileReleaseReceipt).(*filev1.ReleaseSpaceDeletionProducerReferencesReceipt)
	changedEvidence.FileReleaseReceipt.ReceiptId = uuid.NewString()
	_, err = lifecycle.PurgeSpace(ctx, request, changedEvidence)
	require.ErrorIs(t, err, ErrSpaceLifecycleConflict)
	var remaining int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chats WHERE id=$1`, chatID).Scan(&remaining))
	require.Zero(t, remaining)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM chat_space_lifecycle_fences WHERE space_id=$1`, spaceID).Scan(&state))
	require.Equal(t, "PURGED", state)
}

type recordingChatDeletedBytesPublisher struct {
	calls      int
	eventID    string
	eventBytes []byte
	err        error
	onPublish  func() error
}

func (p *recordingChatDeletedBytesPublisher) PublishChatDeletedBytes(_ context.Context, eventID string, eventBytes []byte) error {
	p.calls++
	p.eventID = eventID
	p.eventBytes = append([]byte(nil), eventBytes...)
	if p.onPublish != nil {
		if err := p.onPublish(); err != nil {
			return err
		}
	}
	return p.err
}

func spacePurgeEvidenceForTest(t *testing.T, purge *commonv1.SpacePurgeRequest) SpacePurgeOwnerEvidence {
	t.Helper()
	messagingPurge := proto.Clone(purge).(*commonv1.SpacePurgeRequest)
	messagingPurge.ParticipantId = commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING
	messagingWire, err := proto.MarshalOptions{Deterministic: true}.Marshal(&messagingv1.PurgeSpaceRequest{Purge: messagingPurge})
	require.NoError(t, err)
	messagingHash := chatLifecycleDigest("voice.messaging.v1.PurgeSpaceRequest", messagingWire)
	fileHash := emptyChatProducerReferenceHash(purge.GetDeletionOperationId(), purge.GetSpaceId(), purge.GetGeneration()-1)
	fileRequest := &filev1.ReleaseSpaceDeletionProducerReferencesRequest{ProtocolVersion: 1, DeletionOperationId: purge.GetDeletionOperationId(), SpaceId: purge.GetSpaceId(), PurgeGeneration: purge.GetGeneration(), SourceScheduleGeneration: purge.GetGeneration() - 1, ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT, ExpectedReferencesSha256: fileHash[:]}
	fileWire, err := proto.MarshalOptions{Deterministic: true}.Marshal(fileRequest)
	require.NoError(t, err)
	fileRequestHash := chatLifecycleDigest("voice.file.v1.ReleaseSpaceDeletionProducerReferencesRequest", fileWire)
	return SpacePurgeOwnerEvidence{
		MessagingReceipt:   &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: purge.GetSpaceId(), DeletionOperationId: purge.GetDeletionOperationId(), Generation: purge.GetGeneration(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: messagingHash[:], CompletedAt: timestamppb.Now()},
		FileReleaseReceipt: &filev1.ReleaseSpaceDeletionProducerReferencesReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: purge.GetSpaceId(), DeletionOperationId: purge.GetDeletionOperationId(), PurgeGeneration: purge.GetGeneration(), SourceScheduleGeneration: purge.GetGeneration() - 1, ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT, ExpectedReferencesSha256: fileHash[:], RequestSha256: fileRequestHash[:], CompletedAt: timestamppb.Now()},
	}
}
