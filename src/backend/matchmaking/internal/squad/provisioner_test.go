package squad_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/matchmaking/internal/squad"
)

type mockChatClient struct {
	lastMatchID    uuid.UUID
	lastProfileIDs []uuid.UUID
	chatID         string
	err            error
}

func (m *mockChatClient) CreateMatchChat(ctx context.Context, matchID uuid.UUID, profileIDs []uuid.UUID) (string, error) {
	m.lastMatchID = matchID
	m.lastProfileIDs = append([]uuid.UUID(nil), profileIDs...)
	if m.chatID == "" {
		m.chatID = "chat-" + matchID.String()
	}
	return m.chatID, m.err
}

type mockVoiceClient struct {
	lastMatchID    uuid.UUID
	lastProfileIDs []uuid.UUID
	lastChatID     string
	roomID         string
	err            error
}

func (m *mockVoiceClient) CreateMatchRoom(ctx context.Context, matchID uuid.UUID, profileIDs []uuid.UUID, chatID string) (string, error) {
	m.lastMatchID = matchID
	m.lastProfileIDs = append([]uuid.UUID(nil), profileIDs...)
	m.lastChatID = chatID
	if m.roomID == "" {
		m.roomID = "voice-" + matchID.String()
	}
	return m.roomID, m.err
}

func TestProvisioner_ProvisionCallsChatAndVoiceClients(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	matchID := uuid.New()
	profileA := uuid.New()
	profileB := uuid.New()
	profileIDs := []uuid.UUID{profileA, profileB}

	chat := &mockChatClient{}
	voice := &mockVoiceClient{}
	p := &squad.Provisioner{Chat: chat, Voice: voice}

	voiceRoomID, chatID, err := p.Provision(ctx, matchID, profileIDs)
	require.NoError(t, err)
	require.Equal(t, voice.roomID, voiceRoomID)
	require.Equal(t, chat.chatID, chatID)
	require.Equal(t, matchID, chat.lastMatchID)
	require.Equal(t, matchID, voice.lastMatchID)
	require.Equal(t, chat.chatID, voice.lastChatID)
	require.ElementsMatch(t, profileIDs, chat.lastProfileIDs)
	require.ElementsMatch(t, profileIDs, voice.lastProfileIDs)
}

func TestProvisioner_PropagatesVoiceError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	voice := &mockVoiceClient{err: context.Canceled}
	p := &squad.Provisioner{
		Chat:  &mockChatClient{},
		Voice: voice,
	}
	_, _, err := p.Provision(ctx, uuid.New(), []uuid.UUID{uuid.New()})
	require.ErrorIs(t, err, context.Canceled)
}
