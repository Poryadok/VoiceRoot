package grpcsvc_test

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"voice/backend/bot/internal/store"
)

func freshSession(t *testing.T, st *store.BotStore, id uuid.UUID) store.Session {
	t.Helper()
	bot, err := st.GetBotByID(context.Background(), id)
	require.NoError(t, err)
	return bot.Session()
}

func sessionForToken(t *testing.T, st *store.BotStore, token string) store.Session {
	t.Helper()
	var session store.Session
	err := st.Pool.QueryRow(context.Background(), `SELECT bot_id,session_epoch FROM bot_event_log WHERE interaction_token=$1`, token).Scan(&session.BotID, &session.Epoch)
	require.NoError(t, err)
	return session
}
