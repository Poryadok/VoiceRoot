package dispatch_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"voice/backend/bot/internal/dispatch"
	"voice/backend/bot/internal/store"
)

func TestHub_deferredStaysPendingUntilFinish(t *testing.T) {
	hub := dispatch.NewHub()
	ch := hub.Register("tok")
	go func() {
		time.Sleep(10 * time.Millisecond)
		require.True(t, hub.Complete("tok", store.InteractionReply{Deferred: true}))
	}()
	reply, ok := hub.Wait(ch, time.Second)
	require.True(t, ok)
	require.True(t, reply.Deferred)
	require.True(t, hub.IsPending("tok"))
	require.True(t, hub.IsDeferred("tok"))

	hub.FinishDeferred("tok")
	require.False(t, hub.IsPending("tok"))
	require.False(t, hub.IsDeferred("tok"))
}
