package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func testAuthority(t *testing.T, st *BotStore, id uuid.UUID) Authority {
	t.Helper()
	bot, err := st.GetBotByID(context.Background(), id)
	require.NoError(t, err)
	return bot.Authority()
}
