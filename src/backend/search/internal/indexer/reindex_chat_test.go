package indexer_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/search/internal/deps"
	"voice/backend/search/internal/indexer"
	storepkg "voice/backend/search/internal/store"
)

type stubMessageLister struct {
	pages [][]deps.MessageRow
	page  int
}

func (s *stubMessageLister) ListChatMessages(
	_ context.Context,
	_ uuid.UUID,
	_ string,
	_ int32,
) ([]deps.MessageRow, string, error) {
	if s.page >= len(s.pages) {
		return nil, "", nil
	}
	rows := s.pages[s.page]
	s.page++
	next := ""
	if s.page < len(s.pages) {
		next = "cursor-next"
	}
	return rows, next, nil
}

type recordingMessageStore struct {
	docs []storepkg.MessageDocument
}

func (s *recordingMessageStore) Upsert(_ context.Context, doc storepkg.MessageDocument) error {
	s.docs = append(s.docs, doc)
	return nil
}

func (s *recordingMessageStore) Delete(_ context.Context, _ uuid.UUID) error {
	return nil
}

func TestReindexChat_PagesAndUpserts(t *testing.T) {
	chatID := uuid.New()
	msg1 := uuid.New()
	msg2 := uuid.New()
	sender := uuid.New()
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	lister := &stubMessageLister{
		pages: [][]deps.MessageRow{
			{
				{ID: msg1, SenderProfileID: sender, Body: "hello", CreatedAt: created},
			},
			{
				{ID: msg2, SenderProfileID: sender, Body: "world", CreatedAt: created},
			},
		},
	}
	store := &recordingMessageStore{}

	err := indexer.ReindexChat(context.Background(), chatID, lister, store)
	require.NoError(t, err)
	require.Len(t, store.docs, 2)
	require.Equal(t, msg1, store.docs[0].MessageID)
	require.Equal(t, "hello", store.docs[0].Body)
	require.Equal(t, msg2, store.docs[1].MessageID)
}
