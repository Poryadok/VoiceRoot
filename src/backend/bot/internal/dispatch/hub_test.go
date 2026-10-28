package dispatch_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/bot/internal/dispatch"
	"voice/backend/bot/internal/store"
)

var hubTestSession = store.Session{BotID: uuid.MustParse("00000000-0000-4000-8000-000000000001"), Epoch: 1}

func TestHub_completeWakesWaiter(t *testing.T) {
	hub := dispatch.NewHub()
	ch := hub.Register("tok", hubTestSession)
	go func() {
		time.Sleep(20 * time.Millisecond)
		require.True(t, hub.Complete("tok", hubTestSession, store.InteractionReply{Content: "pong"}))
	}()
	reply, ok := hub.Wait(ch, time.Second)
	require.True(t, ok)
	require.Equal(t, "pong", reply.Content)
}

func TestHub_waitTimeout(t *testing.T) {
	hub := dispatch.NewHub()
	ch := hub.Register("slow", hubTestSession)
	reply, ok := hub.Wait(ch, 30*time.Millisecond)
	require.False(t, ok)
	require.Equal(t, dispatch.ErrTimeout, reply.Err)
}
