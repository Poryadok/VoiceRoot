package store_test

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
