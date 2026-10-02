package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
)

func attachmentIntentFixture(t *testing.T) (*MessagesStore, MessageRow, *filev1.AcquireFileReferencesRequest) {
	t.Helper()
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	client := uuid.New()
	row := MessageRow{ID: uuid.New(), ChatID: uuid.New(), SenderProfileID: uuid.New(), ChatType: "group", Content: "attachment", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]", ClientMessageID: &client}
	file := uuid.NewString()
	row.AttachmentsJSON = fmt.Sprintf(`[{"type":"file","file_id":"%s"}]`, file)
	request := &filev1.AcquireFileReferencesRequest{ProtocolVersion: 1, ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{{FileId: file, OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE}}}
	return &MessagesStore{Pool: pool}, row, request
}

func TestAttachmentIntentFreezeCapturesNonvisibleAcquisitionAndBlocksOrdinaryRecovery(t *testing.T) {
	s, row, request := attachmentIntentFixture(t)
	ctx := context.Background()
	space, operation := uuid.New(), uuid.New()
	_, err := s.InsertMessageWithReferences(ctx, row, &space, request, func(context.Context, *filev1.AcquireFileReferencesRequest) error {
		return errors.New("lost File response")
	})
	require.Error(t, err)
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ItemCount: 1, ManifestSha256: messagingChatManifestSHA(space, operation, 1, []uuid.UUID{row.ChatID})}
	_, err = s.ImportSpacePurgeManifestPage(ctx, SpacePurgeManifestPageInput{SpaceID: space, DeletionOperationID: operation, ScheduleGeneration: 1, Page: &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: manifest, ItemIds: []string{row.ChatID.String()}, PageSha256: make([]byte, 32)}, SealsManifest: true, RequestBytes: []byte("page"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("receipt")})
	require.NoError(t, err)
	_, err = s.ApplySpaceLifecycleFence(ctx, SpaceLifecycleFenceInput{SpaceID: space, DeletionOperationID: operation, Generation: 1, State: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest, RequestBytes: []byte("freeze"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("receipt")})
	require.NoError(t, err)
	refs, err := s.SpaceFileProducerReferences(ctx, space, operation, 1)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, row.ID.String(), refs[0].OwnerId)
	require.Equal(t, space.String(), refs[0].GetScopeSpaceId())
	_, err = s.Pool.Exec(ctx, `UPDATE messaging_attachment_send_intents SET expires_at=clock_timestamp()-interval '1 second' WHERE message_id=$1`, row.ID)
	require.NoError(t, err)
	require.NoError(t, s.ReconcileAttachmentIntents(ctx, func(context.Context, *filev1.ReleaseFileReferencesRequest) error {
		t.Fatal("frozen manifest owner was bypassed")
		return nil
	}))
	_, err = s.InsertMessageWithReferences(ctx, row, &space, request, func(context.Context, *filev1.AcquireFileReferencesRequest) error {
		t.Fatal("frozen expired intent reacquired")
		return nil
	})
	require.ErrorIs(t, err, ErrAttachmentIntentExpired)
}

func TestAttachmentIntentRetryIsStableAndChangedBodyConflicts(t *testing.T) {
	s, row, request := attachmentIntentFixture(t)
	ctx := context.Background()
	lost := errors.New("lost acquire response")
	var first *filev1.AcquireFileReferencesRequest
	_, err := s.InsertMessageWithReferences(ctx, row, nil, request, func(_ context.Context, r *filev1.AcquireFileReferencesRequest) error {
		first = proto.Clone(r).(*filev1.AcquireFileReferencesRequest)
		return lost
	})
	require.ErrorIs(t, err, lost)
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&count))
	require.Zero(t, count, "failed acquisition cannot become visible")
	changed := row
	changed.Content = "different"
	_, err = s.InsertMessageWithReferences(ctx, changed, nil, request, func(context.Context, *filev1.AcquireFileReferencesRequest) error {
		t.Fatal("changed body reached File")
		return nil
	})
	require.ErrorIs(t, err, ErrAttachmentIntentConflict)
	row.ID = uuid.New()
	saved, err := s.InsertMessageWithReferences(ctx, row, nil, request, func(_ context.Context, r *filev1.AcquireFileReferencesRequest) error {
		require.True(t, proto.Equal(first, r))
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, first.References[0].OwnerId, saved.ID.String())
	_, err = s.InsertMessageWithReferences(ctx, row, nil, request, func(context.Context, *filev1.AcquireFileReferencesRequest) error {
		t.Fatal("committed retry reacquired")
		return nil
	})
	require.NoError(t, err)
}

func TestAttachmentIntentConcurrentRetryAcquiresOneMessage(t *testing.T) {
	s, row, request := attachmentIntentFixture(t)
	// Retries must complete without borrowing a second connection while holding
	// the operation lock, even when the pool has no spare capacity.
	config := s.Pool.Config()
	config.MaxConns = 1
	config.MinConns = 0
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	s.Pool = pool
	var calls atomic.Int32
	var wg sync.WaitGroup
	results := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.InsertMessageWithReferences(ctx, row, nil, request, func(context.Context, *filev1.AcquireFileReferencesRequest) error { calls.Add(1); return nil })
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, calls.Load())
	var count int
	require.NoError(t, s.Pool.QueryRow(context.Background(), `SELECT count(*) FROM messages`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestAttachmentIntentLegacyDedupeDoesNotAcquireOrphanReference(t *testing.T) {
	s, row, request := attachmentIntentFixture(t)
	ctx := context.Background()
	existing, err := s.InsertMessage(ctx, row)
	require.NoError(t, err)
	row.ID = uuid.New()
	saved, err := s.InsertMessageWithReferences(ctx, row, nil, request, func(context.Context, *filev1.AcquireFileReferencesRequest) error {
		t.Fatal("legacy retry acquired for discarded ID")
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, existing.ID, saved.ID)
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM messaging_attachment_send_intents`).Scan(&count))
	require.Zero(t, count)
}

func TestAttachmentIntentFreezeDuringAcquisitionCannotMakeMessageVisible(t *testing.T) {
	s, row, request := attachmentIntentFixture(t)
	ctx := context.Background()
	space, operation := uuid.New(), uuid.New()
	_, err := s.InsertMessageWithReferences(ctx, row, &space, request, func(context.Context, *filev1.AcquireFileReferencesRequest) error {
		manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ItemCount: 1, ManifestSha256: messagingChatManifestSHA(space, operation, 1, []uuid.UUID{row.ChatID})}
		_, freezeErr := s.ImportSpacePurgeManifestPage(ctx, SpacePurgeManifestPageInput{SpaceID: space, DeletionOperationID: operation, ScheduleGeneration: 1, Page: &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: manifest, ItemIds: []string{row.ChatID.String()}, PageSha256: make([]byte, 32)}, SealsManifest: true, RequestBytes: []byte("page"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("receipt")})
		require.NoError(t, freezeErr)
		_, freezeErr = s.ApplySpaceLifecycleFence(ctx, SpaceLifecycleFenceInput{SpaceID: space, DeletionOperationID: operation, Generation: 1, State: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest, RequestBytes: []byte("freeze"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("receipt")})
		require.NoError(t, freezeErr)
		return nil
	})
	require.ErrorIs(t, err, ErrSpaceLifecycleOrder)
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&count))
	require.Zero(t, count)
	refs, err := s.SpaceFileProducerReferences(ctx, space, operation, 1)
	require.NoError(t, err)
	require.Len(t, refs, 1, "sealed purge includes acquired but invisible owner tuple")
}

func TestAttachmentIntentAbandonedAcquisitionIsReleasedAndCannotCommit(t *testing.T) {
	s, row, request := attachmentIntentFixture(t)
	ctx := context.Background()
	var acquired *filev1.AcquireFileReferencesRequest
	_, err := s.InsertMessageWithReferences(ctx, row, nil, request, func(_ context.Context, r *filev1.AcquireFileReferencesRequest) error {
		acquired = proto.Clone(r).(*filev1.AcquireFileReferencesRequest)
		return errors.New("crash after File effect")
	})
	require.Error(t, err)
	_, err = s.Pool.Exec(ctx, `UPDATE messaging_attachment_send_intents SET expires_at=clock_timestamp()-interval '1 second' WHERE message_id=$1`, row.ID)
	require.NoError(t, err)
	releases := 0
	require.NoError(t, s.ReconcileAttachmentIntents(ctx, func(_ context.Context, r *filev1.ReleaseFileReferencesRequest) error {
		releases++
		require.True(t, proto.Equal(acquired.References[0], r.References[0]))
		return nil
	}))
	require.Equal(t, 1, releases)
	require.NoError(t, s.ReconcileAttachmentIntents(ctx, func(context.Context, *filev1.ReleaseFileReferencesRequest) error {
		t.Fatal("released twice")
		return nil
	}))
	_, err = s.InsertMessageWithReferences(ctx, row, nil, request, func(context.Context, *filev1.AcquireFileReferencesRequest) error {
		t.Fatal("abandoned retry reacquired")
		return nil
	})
	require.ErrorIs(t, err, ErrAttachmentIntentExpired)
}
