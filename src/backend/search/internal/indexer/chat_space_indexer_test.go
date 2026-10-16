package indexer

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"

	"voice/backend/search/internal/store"
)

type recordingChatStore struct {
	lastChatID       uuid.UUID
	lastTitle        string
	deletedChatID    uuid.UUID
	deletedSpaceID   uuid.UUID
	deletedOperation uuid.UUID
	deletedGen       uint64
	deletedManifest  uuid.UUID
	deletedHash      []byte
	deleteErr        error
}

func (r *recordingChatStore) UpsertChat(_ context.Context, chatID uuid.UUID, title string) error {
	r.lastChatID = chatID
	r.lastTitle = title
	return nil
}

func (r *recordingChatStore) DeleteChat(_ context.Context, chatID, spaceID, operationID uuid.UUID, generation uint64, manifestID uuid.UUID, manifestHash []byte) error {
	r.deletedChatID, r.deletedSpaceID, r.deletedOperation = chatID, spaceID, operationID
	r.deletedGen, r.deletedManifest = generation, manifestID
	r.deletedHash = append([]byte(nil), manifestHash...)
	return r.deleteErr
}

type recordingSpaceStore struct {
	upserts []store.SpaceDocument
	deletes []uuid.UUID
}

func (r *recordingSpaceStore) UpsertSpace(_ context.Context, doc store.SpaceDocument) error {
	r.upserts = append(r.upserts, doc)
	return nil
}

func (r *recordingSpaceStore) DeleteSpace(_ context.Context, spaceID uuid.UUID) error {
	r.deletes = append(r.deletes, spaceID)
	return nil
}

type stubChatHydrator struct {
	title string
}

func (s *stubChatHydrator) LoadChatTitle(_ context.Context, _ uuid.UUID) (string, error) {
	return s.title, nil
}

type stubSpaceHydrator struct {
	name        string
	description string
	visibility  string
	memberCount int
}

func (s *stubSpaceHydrator) LoadSpace(_ context.Context, _ uuid.UUID) (string, string, string, int, error) {
	return s.name, s.description, s.visibility, s.memberCount, nil
}

func TestChatSpaceIndexer_SpaceCreated_PublicUpsert(t *testing.T) {
	t.Parallel()
	spaces := &recordingSpaceStore{}
	spaceID := uuid.New()
	idx := &ChatSpaceIndexer{
		Spaces:   spaces,
		SpaceAPI: &stubSpaceHydrator{name: "Public Guild", visibility: "public", memberCount: 3},
	}
	env := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.ChatStreamEvent_SpaceCreated{
			SpaceCreated: &eventsv1.SpaceCreated{SpaceId: spaceID.String()},
		},
	}
	require.NoError(t, idx.Handle(context.Background(), env))
	require.Len(t, spaces.upserts, 1)
	require.Equal(t, spaceID, spaces.upserts[0].SpaceID)
	require.Equal(t, "public", spaces.upserts[0].Visibility)
}

func TestChatSpaceIndexer_SpaceCreated_PrivateDeletes(t *testing.T) {
	t.Parallel()
	spaces := &recordingSpaceStore{}
	spaceID := uuid.New()
	idx := &ChatSpaceIndexer{
		Spaces:   spaces,
		SpaceAPI: &stubSpaceHydrator{visibility: "private"},
	}
	env := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.ChatStreamEvent_SpaceCreated{
			SpaceCreated: &eventsv1.SpaceCreated{SpaceId: spaceID.String()},
		},
	}
	require.NoError(t, idx.Handle(context.Background(), env))
	require.Equal(t, []uuid.UUID{spaceID}, spaces.deletes)
}

func TestChatSpaceIndexer_ChatCreated_UpsertsTitle(t *testing.T) {
	t.Parallel()
	chats := &recordingChatStore{}
	chatID := uuid.New()
	idx := &ChatSpaceIndexer{
		Chats:   chats,
		ChatAPI: &stubChatHydrator{title: "Raid planning"},
	}
	env := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.ChatStreamEvent_ChatCreated{
			ChatCreated: &eventsv1.ChatCreated{ChatId: chatID.String()},
		},
	}
	require.NoError(t, idx.Handle(context.Background(), env))
	require.Equal(t, chatID, chats.lastChatID)
	require.Equal(t, "Raid planning", chats.lastTitle)
}

func TestChatSpaceIndexer_ChatUpdatedRefreshesAndDeletedRequiresPermanentFence(t *testing.T) {
	t.Parallel()
	chats := &recordingChatStore{}
	chatID, spaceID, operationID, manifestID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	idx := &ChatSpaceIndexer{Chats: chats, ChatAPI: &stubChatHydrator{title: "Updated title"}}
	updated := &eventsv1.ChatStreamEvent{Payload: &eventsv1.ChatStreamEvent_ChatUpdated{ChatUpdated: &eventsv1.ChatUpdated{ChatId: chatID.String(), ChangedFields: []string{"name"}}}}
	require.NoError(t, idx.Handle(context.Background(), updated))
	require.Equal(t, chatID, chats.lastChatID)
	require.Equal(t, "Updated title", chats.lastTitle)

	deleted := &eventsv1.ChatStreamEvent{Payload: &eventsv1.ChatStreamEvent_ChatDeleted{ChatDeleted: &eventsv1.ChatDeleted{
		ChatId: chatID.String(), SpaceId: spaceID.String(), DeletionOperationId: operationID.String(),
		Generation: 3, ManifestId: manifestID.String(), ManifestSha256: make([]byte, 32),
	}}}
	require.NoError(t, idx.Handle(context.Background(), deleted))
	require.Equal(t, chatID, chats.deletedChatID)
	require.Equal(t, spaceID, chats.deletedSpaceID)
	require.Equal(t, operationID, chats.deletedOperation)
	require.EqualValues(t, 3, chats.deletedGen)
	require.Equal(t, manifestID, chats.deletedManifest)
	require.Len(t, chats.deletedHash, 32)

	chats.deleteErr = store.ErrChatDeletedProjectionNotReady
	require.ErrorIs(t, idx.Handle(context.Background(), deleted), store.ErrChatDeletedProjectionNotReady)
	require.False(t, isPermanentConsumeError(idx.Handle(context.Background(), deleted)), "not-ready fences remain retryable")
	chats.deleteErr = store.ErrChatDeletedProjectionConflict
	require.True(t, isPermanentConsumeError(idx.Handle(context.Background(), deleted)), "a contradictory terminal binding is semantic poison")

	malformed := proto.Clone(deleted).(*eventsv1.ChatStreamEvent)
	malformed.GetChatDeleted().ManifestSha256 = []byte{1}
	require.True(t, isPermanentConsumeError(idx.Handle(context.Background(), malformed)))

	unconfigured := &ChatSpaceIndexer{}
	require.Error(t, unconfigured.Handle(context.Background(), deleted))
	require.False(t, isPermanentConsumeError(unconfigured.Handle(context.Background(), deleted)), "missing projection store is retryable")
}
