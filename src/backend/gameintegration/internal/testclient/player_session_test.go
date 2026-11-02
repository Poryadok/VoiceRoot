package testclient

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type sessionReaderFunc func(context.Context, uuid.UUID) (SessionSnapshot, error)

func (f sessionReaderFunc) GetPlayerSessionSnapshot(ctx context.Context, id uuid.UUID) (SessionSnapshot, error) {
	return f(ctx, id)
}

type chatReaderFunc func(context.Context, uuid.UUID, string) (MessagePage, error)

func (f chatReaderFunc) ListMessages(ctx context.Context, chatID uuid.UUID, cursor string) (MessagePage, error) {
	return f(ctx, chatID, cursor)
}

type recordingSender struct {
	ids      []uuid.UUID
	accepted map[uuid.UUID]json.RawMessage
}

func (s *recordingSender) SendMessage(_ context.Context, chatID, clientID uuid.UUID, body string) (json.RawMessage, error) {
	if chatID == uuid.Nil {
		return nil, errors.New("chat required")
	}
	s.ids = append(s.ids, clientID)
	if prior, ok := s.accepted[clientID]; ok {
		return prior, nil
	}
	message := json.RawMessage(`{"id":"` + uuid.NewString() + `","body":"` + body + `"}`)
	s.accepted[clientID] = message
	return message, nil
}

func TestReconnectUsesCurrentSnapshotSelectedChatCursorAndScopedCache(t *testing.T) {
	sessionID, chatID := uuid.New(), uuid.New()
	cache := NewPageCache()
	var gotChat uuid.UUID
	var gotCursor string
	client := PlayerSessionClient{
		Sessions: sessionReaderFunc(func(_ context.Context, got uuid.UUID) (SessionSnapshot, error) {
			require.Equal(t, sessionID, got)
			return SessionSnapshot{SessionID: sessionID, Kind: "match", RosterRevision: 12, ChatID: &chatID}, nil
		}),
		Chats: chatReaderFunc(func(_ context.Context, id uuid.UUID, cursor string) (MessagePage, error) {
			gotChat, gotCursor = id, cursor
			return MessagePage{Messages: []json.RawMessage{json.RawMessage(`{"id":"m1"}`)}, NextCursor: "message-cursor-2"}, nil
		}),
		Cache: cache,
	}
	identity := Scope{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), AccountID: uuid.New(), ProfileID: uuid.New(), SessionID: sessionID}
	staleScope := identity
	staleScope.ChatID, staleScope.RosterRevision = uuid.New(), 11
	cache.Put(staleScope, CachedPage{Cursor: "stale-roster"})
	page, err := client.Reconnect(context.Background(), identity, "message-cursor-1")
	require.NoError(t, err)
	require.Equal(t, chatID, gotChat)
	require.Equal(t, "message-cursor-1", gotCursor)
	require.Equal(t, "message-cursor-2", page.Cursor)

	identity.ChatID, identity.RosterRevision = chatID, 12
	cached, ok := cache.Get(identity)
	require.True(t, ok)
	require.Equal(t, page, cached)
	_, ok = cache.Get(staleScope)
	require.False(t, ok, "a fresh roster revision evicts stale session cache")
	otherProfile := identity
	otherProfile.ProfileID = uuid.New()
	_, ok = cache.Get(otherProfile)
	require.False(t, ok)
	otherRevision := identity
	otherRevision.RosterRevision++
	_, ok = cache.Get(otherRevision)
	require.False(t, ok)
}

func TestSendUsesSameClientMessageIDAfterLostAcknowledgement(t *testing.T) {
	sender := &recordingSender{accepted: make(map[uuid.UUID]json.RawMessage)}
	client := PlayerSessionClient{Sender: sender}
	chatID, clientMessageID := uuid.New(), uuid.New()
	first, err := client.Send(context.Background(), chatID, clientMessageID, "hello")
	require.NoError(t, err)
	second, err := client.Send(context.Background(), chatID, clientMessageID, "hello")
	require.NoError(t, err)
	require.JSONEq(t, string(first), string(second))
	require.Equal(t, []uuid.UUID{clientMessageID, clientMessageID}, sender.ids)
}

func TestReconnectPreservesCachedPageWhenCursorFetchFails(t *testing.T) {
	sessionID, chatID := uuid.New(), uuid.New()
	cache := NewPageCache()
	scope := Scope{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), AccountID: uuid.New(), ProfileID: uuid.New(), SessionID: sessionID, ChatID: chatID, RosterRevision: 3}
	prior := CachedPage{Page: MessagePage{Messages: []json.RawMessage{json.RawMessage(`{"id":"old"}`)}}, Cursor: "prior"}
	cache.Put(scope, prior)
	client := PlayerSessionClient{
		Sessions: sessionReaderFunc(func(context.Context, uuid.UUID) (SessionSnapshot, error) {
			return SessionSnapshot{SessionID: sessionID, RosterRevision: 3, ChatID: &chatID}, nil
		}),
		Chats: chatReaderFunc(func(context.Context, uuid.UUID, string) (MessagePage, error) {
			return MessagePage{}, errors.New("messaging unavailable")
		}),
		Cache: cache,
	}
	_, err := client.Reconnect(context.Background(), scope, MessageCursor("prior"))
	require.Error(t, err)
	cached, ok := cache.Get(scope)
	require.True(t, ok)
	require.Equal(t, prior, cached)
}
