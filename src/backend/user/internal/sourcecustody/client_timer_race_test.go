package sourcecustody

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Delaying processing after frame selection makes the timer expire before
// the selected frame is validated, independent of Go select scheduling.
func TestUserCustodyClientRejectsFirstFrameProcessedAfterTimeout(t *testing.T) {
	client, _, binding := adversarialClient(t, "valid-heartbeats")
	entered, resume := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var enteredOnce sync.Once
	release := func() { once.Do(func() { close(resume) }) }
	defer release()
	client.afterFrame = func() { enteredOnce.Do(func() { close(entered); <-resume }) }
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	type result struct {
		grant *Grant
		err   error
	}
	resultCh := make(chan result, 1)
	go func() { grant, err := client.Acquire(ctx, binding); resultCh <- result{grant, err} }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first frame not received")
	}
	<-time.After(400 * time.Millisecond)
	release()
	select {
	case got := <-resultCh:
		if got.grant != nil {
			got.grant.Release()
		}
		require.Error(t, got.err, "first frame past the configured deadline cannot establish a grant")
		require.NoError(t, ctx.Err())
	case <-time.After(2 * time.Second):
		t.Fatal("first-frame acquisition did not resolve")
	}
}

func TestUserCustodyClientRejectsHeartbeatProcessedAfterTimeout(t *testing.T) {
	client, peer, binding := adversarialClient(t, "valid-heartbeats")
	entered, resume := make(chan struct{}), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(resume) }) }
	defer release()
	var received atomic.Int32
	client.afterFrame = func() {
		if received.Add(1) == 2 {
			close(entered)
			<-resume
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	grant, err := client.Acquire(ctx, binding)
	require.NoError(t, err)
	defer grant.Release()
	close(peer.proceed)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("heartbeat not received")
	}
	<-time.After(400 * time.Millisecond)
	release()
	select {
	case <-grant.Context().Done():
		require.NoError(t, ctx.Err())
	case <-time.After(time.Second):
		t.Fatal("late heartbeat incorrectly kept grant alive")
	}
}
