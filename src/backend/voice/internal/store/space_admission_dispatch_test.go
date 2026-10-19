package store

import (
	"context"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// Pause the real Redis pipeline immediately before EXEC. The competing client
// advances only the admission gate; the Space lifecycle and call key stay fixed.
// Omitting the admission key from WATCH therefore makes this test fail.
type admissionDispatchPipelineHook struct {
	once   sync.Once
	before func()
}

func (h *admissionDispatchPipelineHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *admissionDispatchPipelineHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return next
}
func (h *admissionDispatchPipelineHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, commands []redis.Cmder) error {
		for _, command := range commands {
			if command.Name() == "exec" {
				h.once.Do(h.before)
				break
			}
		}
		return next(ctx, commands)
	}
}

func TestRedisAdmissionDispatchTakeoverInvalidatesPendingExec(t *testing.T) {
	ctx := t.Context()
	s, client := newSpaceGateStoreForTest(t, "admission-watch:")
	call := projectionShell()
	require.NoError(t, s.EnsureSpaceLifecycle(ctx, call.SpaceID, liveFixture(0)))
	ticket, err := fixtureTicket(ctx, s, call)
	require.NoError(t, err)
	call.SpaceProjectionTicket = ticket
	competitorClient := redis.NewClient(client.Options())
	defer competitorClient.Close()
	competitor := NewRedisCallStore(competitorClient, "admission-watch:")
	takenOver := false
	client.AddHook(&admissionDispatchPipelineHook{before: func() {
		_, err := competitor.EnsureAdmissionDispatch(ctx, ticket.OperationID, denyFixture(2))
		require.NoError(t, err)
		takenOver = true
	}})
	_, err = s.CreateCall(ctx, call)
	require.True(t, takenOver, "the takeover must occur after the old gate read and before actual EXEC")
	require.ErrorIs(t, err, ErrSpaceMediaStaleGrant)
	require.Zero(t, client.Exists(ctx, s.callKey(call.RoomID), s.activeVoiceRoomKey(call.VoiceRoomID)).Val())
}

func TestRedisAdmissionTakeoverInvalidatesPendingCleanupExec(t *testing.T) {
	for _, complete := range []bool{false, true} {
		t.Run(map[bool]string{false: "begin", true: "complete"}[complete], func(t *testing.T) {
			ctx := t.Context()
			s, client := newSpaceGateStoreForTest(t, "cleanup-watch:")
			call := projectionShell()
			_, err := createLifecycleCallForTest(ctx, s, call)
			require.NoError(t, err)
			p := testSpaceMediaParticipant("member", "identity", "generation", 1, 1)
			_, err = admitLifecycleParticipantForTest(ctx, s, call.RoomID, p, 32)
			require.NoError(t, err)
			cleanup := fixtureCleanupContext(t, ctx, s, call.RoomID, p)
			if complete {
				_, matched, err := s.BeginSpaceMediaRevocation(cleanup, call.RoomID, p.ProfileID, p.Identity, p.Generation)
				require.NoError(t, err)
				require.True(t, matched)
			}
			before, err := client.Get(ctx, s.callKey(call.RoomID)).Result()
			require.NoError(t, err)
			competitorClient := redis.NewClient(client.Options())
			defer competitorClient.Close()
			competitor := NewRedisCallStore(competitorClient, "cleanup-watch:")
			taken := false
			client.AddHook(&admissionDispatchPipelineHook{before: func() {
				_, err := competitor.EnsureAdmissionDispatch(ctx, p.AdmissionOperationID, denyFixture(2))
				require.NoError(t, err)
				taken = true
			}})
			if complete {
				_, _, err = s.CompleteSpaceMediaRevocation(cleanup, call.RoomID, p.ProfileID, p.Identity, p.Generation)
			} else {
				_, _, err = s.BeginSpaceMediaRevocation(cleanup, call.RoomID, p.ProfileID, p.Identity, p.Generation)
			}
			require.True(t, taken)
			require.ErrorIs(t, err, ErrSpaceMediaStaleGrant)
			after, err := client.Get(ctx, s.callKey(call.RoomID)).Result()
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}
