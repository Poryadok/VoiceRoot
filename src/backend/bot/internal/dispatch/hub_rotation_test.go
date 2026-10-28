package dispatch

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
	"voice/backend/bot/internal/store"
)

func TestHubDeferredNotificationOnlyOnce(t *testing.T) {
	hub := NewHub()
	session := store.Session{BotID: uuid.New(), Epoch: 1}
	ch := hub.Register("token", session)
	require.True(t, hub.Complete("token", session, store.InteractionReply{Deferred: true}))
	<-ch
	require.False(t, hub.Complete("token", session, store.InteractionReply{Deferred: true}), "already notified deferred token is not another pending response")
}

func TestHubEpochFloorAndFullChannelNeverResurrectOldAuthority(t *testing.T) {
	hub := NewHub()
	a, b := uuid.New(), uuid.New()
	old := store.Session{BotID: a, Epoch: 1}
	other := store.Session{BotID: b, Epoch: 1}
	current := store.Session{BotID: a, Epoch: 2}
	ch := hub.Register("old", old)
	ch <- store.InteractionReply{Content: "already queued"}
	hub.RegisterDeferred("other", other)
	done := make(chan struct{})
	go func() {
		hub.Complete("old", old, store.InteractionReply{Content: "late"})
		hub.PurgeBot(a, 2)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("full/orphaned channel blocked completion or purge")
	}
	require.False(t, hub.IsPending("old"))
	require.True(t, hub.IsDeferred("other"))
	hub.RegisterDeferred("old", old)
	require.False(t, hub.IsPending("old"))
	hub.RegisterDeferred("new", current)
	require.True(t, hub.IsDeferred("new"))
	require.False(t, hub.Complete("old", old, store.InteractionReply{}))
}

func TestHubAutocompleteIDsDoNotAliasAfterRestartOrAcrossHubs(t *testing.T) {
	seen := map[string]bool{}
	for range 128 {
		id := NewHub().NextAutocompleteRequestID()
		_, err := uuid.Parse(id)
		require.NoError(t, err)
		require.False(t, seen[id])
		seen[id] = true
	}
}

func TestHubConcurrentCompletionPurgeAndRehydrate(t *testing.T) {
	hub := NewHub()
	bot := uuid.New()
	old := store.Session{BotID: bot, Epoch: 1}
	current := store.Session{BotID: bot, Epoch: 2}
	hub.Register("token", old)
	var group sync.WaitGroup
	for range 32 {
		group.Add(3)
		go func() { defer group.Done(); hub.Complete("token", old, store.InteractionReply{Deferred: true}) }()
		go func() { defer group.Done(); hub.PurgeBot(bot, 2) }()
		go func() { defer group.Done(); hub.RegisterDeferred("token", old) }()
	}
	done := make(chan struct{})
	go func() { group.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Hub operation blocked")
	}
	require.False(t, hub.IsPending("token"))
	hub.RegisterDeferred("new", current)
	require.True(t, hub.IsDeferred("new"))
}
