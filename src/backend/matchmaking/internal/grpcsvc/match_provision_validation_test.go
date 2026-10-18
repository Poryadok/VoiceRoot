package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/store"
)

type rejectingMatchVoiceClient struct {
	callsv1.MatchSquadVoiceServiceClient
	code  codes.Code
	calls int
}

func (c *rejectingMatchVoiceClient) CreateMatchSquadRoom(context.Context, *callsv1.CreateMatchSquadRoomRequest, ...grpc.CallOption) (*callsv1.CreateMatchSquadRoomResponse, error) {
	c.calls++
	return nil, status.Error(c.code, "protected Voice creation fixture rejection")
}

// Exercise the request path and a fresh recovery worker against durable state:
// validation failures compensate only the resource with a known creation receipt;
// ambiguous failures retain the immutable provisioning intent for retry.
func TestMatchSquadVoiceCreationRejectionCompensation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	for _, code := range []codes.Code{codes.InvalidArgument, codes.FailedPrecondition, codes.Unavailable, codes.DeadlineExceeded} {
		t.Run(code.String(), func(t *testing.T) {
			ctx := context.Background()
			pool := startDB(t, ctx)
			worker := newTestProtectedProvisioner(t, pool)
			voice := &rejectingMatchVoiceClient{code: code}
			worker.Voice = voice
			srv := matchTestServer(t, pool, worker)
			matchText, a, b := seedPendingDuoMatch(t, ctx, srv)
			matchID := uuid.MustParse(matchText)
			_, err := srv.RespondToMatch(ctxWithProfile(a), &matchmakingv1.RespondToMatchRequest{MatchId: matchText, Accept: true})
			require.NoError(t, err)
			_, err = srv.RespondToMatch(ctxWithProfile(b), &matchmakingv1.RespondToMatchRequest{MatchId: matchText, Accept: true})
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Equal(t, 1, voice.calls)
			operation, err := srv.Matches.GetMatchSquadProvisioningOperation(ctx, matchID)
			require.NoError(t, err)
			require.NotNil(t, operation.ChatReceiptID)
			require.Nil(t, operation.VoiceReceiptID)
			terminal := code == codes.InvalidArgument || code == codes.FailedPrecondition
			if terminal {
				require.Equal(t, "compensating", operation.State)
			} else {
				require.Equal(t, "provisioning", operation.State)
			}
			pending, err := srv.Matches.ListPendingMatchSquadTeardownParticipants(ctx, 100)
			require.NoError(t, err)
			if terminal {
				require.Len(t, pending, 1)
				require.Equal(t, "chat", pending[0].Provider, "never teardown an unknown Voice resource")
			} else {
				require.Empty(t, pending, "ambiguous response must not revoke the created Chat")
			}
			resumed := newTestProtectedProvisioner(t, pool)
			resumed.Voice = voice
			_, resumeErr := resumed.RunProvisioningOnce(ctx, 100)
			if terminal {
				require.NoError(t, resumeErr)
				require.Equal(t, 1, voice.calls, "restart must not retry terminally rejected creation")
			} else {
				require.Error(t, resumeErr)
				require.Equal(t, 2, voice.calls)
			}
			after, err := srv.Matches.GetMatchSquadProvisioningOperation(ctx, matchID)
			require.NoError(t, err)
			require.Equal(t, operation.ChatReceiptID, after.ChatReceiptID)
			require.Equal(t, operation.VoiceOperationID, after.VoiceOperationID)
			require.Equal(t, operation.VoiceRequestBytes, after.VoiceRequestBytes)
			match, err := srv.Matches.Get(ctx, matchID)
			require.NoError(t, err)
			require.Equal(t, store.MatchStatusPendingAccept, match.Status)
			events, err := srv.Matches.ListPendingMatchSquadCompletionEvents(ctx, 100)
			require.NoError(t, err)
			require.Empty(t, events, "failed provisioning never emits match completion")
		})
	}
}
