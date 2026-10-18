package grpcsvc

import (
	"context"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/tap"
	"google.golang.org/grpc/test/bufconn"
	botv1 "voice.app/voice/bot/v1"
)

type stalledPollingServer struct {
	botv1.UnimplementedBotServiceServer
	guard      sync.RWMutex
	secondSend chan struct{}
	returned   chan error
	active     atomic.Bool
}

func (s *stalledPollingServer) PollEvents(_ *botv1.PollEventsRequest, stream botv1.BotService_PollEventsServer) error {
	defer releasePollingTransport(stream.Context())
	s.guard.RLock()
	defer s.guard.RUnlock()
	payload := strings.Repeat("x", 1024*1024)
	for i := 0; i < 100; i++ {
		if i == 1 {
			close(s.secondSend)
		}
		s.active.Store(true)
		err := stream.Send(&botv1.PollEventsResponse{BotEvent: &botv1.BotEvent{PayloadJson: payload}})
		s.active.Store(false)
		if err != nil {
			s.returned <- err
			return err
		}
	}
	s.returned <- nil
	return nil
}

// This uses a real gRPC HTTP/2 transport and no client deadline/Recv. Multiple
// oversized messages exhaust stream credit and the server write quota, making
// the second native Send block. The lock models the store callback lifetime;
// PostgreSQL/actual PollEvents composition remains separate integration proof.
func TestPollingTransportCapTerminatesNativeSendBeforeGuardRelease(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	svc := &stalledPollingServer{secondSend: make(chan struct{}), returned: make(chan error, 1)}
	srv := grpc.NewServer(grpc.InTapHandle(pollEventsTransportTap(150 * time.Millisecond)))
	botv1.RegisterBotServiceServer(srv, svc)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop(); _ = lis.Close() })
	conn, err := grpc.NewClient("passthrough:///poll-stalled", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithInitialWindowSize(64*1024), grpc.WithInitialConnWindowSize(64*1024))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	_, err = botv1.NewBotServiceClient(conn).PollEvents(ctx, &botv1.PollEventsRequest{})
	require.NoError(t, err)
	select {
	case <-svc.secondSend:
	case <-time.After(time.Second):
		t.Fatal("did not reach native second Send")
	}
	fence := make(chan bool, 1)
	go func() { svc.guard.Lock(); defer svc.guard.Unlock(); fence <- svc.active.Load() }()
	select {
	case err := <-svc.returned:
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("server-owned cap did not terminate native flow-control blocked Send")
	}
	select {
	case active := <-fence:
		require.False(t, active, "guard released with an active Send")
	case <-time.After(time.Second):
		t.Fatal("fence stayed blocked after Send returned")
	}
}

func TestPollingTransportScopeEarlierDeadlineAndCleanup(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	tapHandle := pollEventsTransportTap(2 * time.Hour)
	other, err := tapHandle(parent, &tap.Info{FullMethodName: botv1.BotService_GetBot_FullMethodName})
	require.NoError(t, err)
	require.Equal(t, parent, other)
	bounded, err := tapHandle(parent, &tap.Info{FullMethodName: botv1.BotService_PollEvents_FullMethodName})
	require.NoError(t, err)
	want, _ := parent.Deadline()
	got, ok := bounded.Deadline()
	require.True(t, ok)
	require.Equal(t, want, got)
	releasePollingTransport(bounded)
	require.ErrorIs(t, bounded.Err(), context.Canceled)
	require.NoError(t, parent.Err(), "cleanup must not cancel the parent stream context")
}
