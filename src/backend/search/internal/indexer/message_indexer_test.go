package indexer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"

	"voice/backend/search/internal/store"
)

type recordingMessageStore struct {
	upserts []store.MessageDocument
	deletes []uuid.UUID
}

func (r *recordingMessageStore) Upsert(_ context.Context, doc store.MessageDocument) error {
	r.upserts = append(r.upserts, doc)
	return nil
}

func (r *recordingMessageStore) Delete(_ context.Context, messageID uuid.UUID) error {
	r.deletes = append(r.deletes, messageID)
	return nil
}

type stubMessagingClient struct {
	body    string
	err     error
	fetchID uuid.UUID
}

func (s *stubMessagingClient) GetMessageBody(_ context.Context, chatID, messageID uuid.UUID) (string, time.Time, error) {
	s.fetchID = messageID
	if s.err != nil {
		return "", time.Time{}, s.err
	}
	return s.body, time.Now().UTC(), nil
}

func messageSentEvent(chatID, messageID, senderID uuid.UUID) *eventsv1.MessageStreamEvent {
	return messageSentEventE2E(chatID, messageID, senderID, false)
}

func messageSentEventE2E(chatID, messageID, senderID uuid.UUID, isE2E bool) *eventsv1.MessageStreamEvent {
	return &eventsv1.MessageStreamEvent{
		EventId:     uuid.New().String(),
		OccurredAt:  timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				MessageId:       messageID.String(),
				ChatId:          chatID.String(),
				SenderProfileId: senderID.String(),
				IsE2E:           isE2E,
			},
		},
	}
}

func messageEditedEvent(chatID, messageID uuid.UUID) *eventsv1.MessageStreamEvent {
	return messageEditedEventE2E(chatID, messageID, false)
}

func messageEditedEventE2E(chatID, messageID uuid.UUID, isE2E bool) *eventsv1.MessageStreamEvent {
	return &eventsv1.MessageStreamEvent{
		EventId:    uuid.New().String(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageEdited{
			MessageEdited: &eventsv1.MessageEdited{
				MessageId: messageID.String(),
				ChatId:    chatID.String(),
				IsE2E:     isE2E,
			},
		},
	}
}

func messageDeletedEvent(chatID, messageID uuid.UUID) *eventsv1.MessageStreamEvent {
	return &eventsv1.MessageStreamEvent{
		EventId:    uuid.New().String(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageDeleted{
			MessageDeleted: &eventsv1.MessageDeleted{
				MessageId: messageID.String(),
				ChatId:    chatID.String(),
			},
		},
	}
}

func TestMessageIndexer_MessageSent_UpsertsDocument(t *testing.T) {
	t.Parallel()
	rec := &recordingMessageStore{}
	messaging := &stubMessagingClient{body: "hello world"}
	idx := &MessageIndexer{Store: rec, Messaging: messaging}

	chatID := uuid.New()
	msgID := uuid.New()
	senderID := uuid.New()

	require.NoError(t, idx.Handle(ctxBackground(), messageSentEvent(chatID, msgID, senderID)))
	require.Len(t, rec.upserts, 1)
	require.Equal(t, msgID, rec.upserts[0].MessageID)
	require.Equal(t, chatID, rec.upserts[0].ChatID)
	require.Equal(t, senderID, rec.upserts[0].SenderProfileID)
	require.Equal(t, "hello world", rec.upserts[0].Body)
	require.Equal(t, msgID, messaging.fetchID)
}

func TestMessageIndexer_MessageEdited_UpdatesDocument(t *testing.T) {
	t.Parallel()
	rec := &recordingMessageStore{}
	messaging := &stubMessagingClient{body: "edited body"}
	idx := &MessageIndexer{Store: rec, Messaging: messaging}

	chatID := uuid.New()
	msgID := uuid.New()

	require.NoError(t, idx.Handle(ctxBackground(), messageEditedEvent(chatID, msgID)))
	require.Len(t, rec.upserts, 1)
	require.Equal(t, msgID, rec.upserts[0].MessageID)
	require.Equal(t, "edited body", rec.upserts[0].Body)
}

func TestMessageIndexer_MessageDeleted_RemovesDocument(t *testing.T) {
	t.Parallel()
	rec := &recordingMessageStore{}
	idx := &MessageIndexer{Store: rec, Messaging: &stubMessagingClient{}}

	chatID := uuid.New()
	msgID := uuid.New()

	require.NoError(t, idx.Handle(ctxBackground(), messageDeletedEvent(chatID, msgID)))
	require.Empty(t, rec.upserts)
	require.Equal(t, []uuid.UUID{msgID}, rec.deletes)
}

// TestMessageIndexer_MessageSent_E2E_SkipsUpsert documents Phase 15: E2E ciphertext is not indexed.
func TestMessageIndexer_MessageSent_E2E_SkipsUpsert(t *testing.T) {
	t.Parallel()
	rec := &recordingMessageStore{}
	messaging := &stubMessagingClient{body: "should-not-be-fetched"}
	idx := &MessageIndexer{Store: rec, Messaging: messaging}

	chatID := uuid.New()
	msgID := uuid.New()
	senderID := uuid.New()

	require.NoError(t, idx.Handle(ctxBackground(), messageSentEventE2E(chatID, msgID, senderID, true)))
	require.Empty(t, rec.upserts, "E2E messages must not be upserted into search index")
	require.Equal(t, uuid.Nil, messaging.fetchID, "indexer must not fetch body for E2E messages")
}

// TestMessageIndexer_MessageSent_NonE2E_StillUpserts documents regression: plaintext messages remain indexed.
func TestMessageIndexer_MessageSent_NonE2E_StillUpserts(t *testing.T) {
	t.Parallel()
	rec := &recordingMessageStore{}
	messaging := &stubMessagingClient{body: "indexed plaintext"}
	idx := &MessageIndexer{Store: rec, Messaging: messaging}

	chatID := uuid.New()
	msgID := uuid.New()
	senderID := uuid.New()

	require.NoError(t, idx.Handle(ctxBackground(), messageSentEventE2E(chatID, msgID, senderID, false)))
	require.Len(t, rec.upserts, 1)
	require.Equal(t, "indexed plaintext", rec.upserts[0].Body)
}

// TestMessageIndexer_MessageEdited_E2E_SkipsUpsert documents Phase 15: edited E2E ciphertext is not indexed.
func TestMessageIndexer_MessageEdited_E2E_SkipsUpsert(t *testing.T) {
	t.Parallel()
	rec := &recordingMessageStore{}
	messaging := &stubMessagingClient{body: "should-not-be-fetched"}
	idx := &MessageIndexer{Store: rec, Messaging: messaging}

	chatID := uuid.New()
	msgID := uuid.New()

	require.NoError(t, idx.Handle(ctxBackground(), messageEditedEventE2E(chatID, msgID, true)))
	require.Empty(t, rec.upserts, "E2E message edits must not be upserted into search index")
	require.Equal(t, uuid.Nil, messaging.fetchID, "indexer must not fetch body for E2E edits")
}

func TestMessageIndexer_SkipsUpsertWhenMessagingUnavailable(t *testing.T) {
	t.Parallel()
	rec := &recordingMessageStore{}
	messaging := &stubMessagingClient{err: errors.New("messaging down")}
	idx := &MessageIndexer{Store: rec, Messaging: messaging}

	chatID := uuid.New()
	msgID := uuid.New()
	senderID := uuid.New()

	err := idx.Handle(ctxBackground(), messageSentEvent(chatID, msgID, senderID))
	require.Error(t, err)
	require.Empty(t, rec.upserts)
}

func ctxBackground() context.Context {
	return context.Background()
}
