package indexer

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"

	"voice/backend/search/internal/store"
)

type recordingChatStore struct {
	lastChatID uuid.UUID
	lastTitle  string
}

func (r *recordingChatStore) UpsertChat(_ context.Context, chatID uuid.UUID, title string) error {
	r.lastChatID = chatID
	r.lastTitle = title
	return nil
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
