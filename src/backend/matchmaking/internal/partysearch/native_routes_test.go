package partysearch

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/authctx"
	"voice/backend/matchmaking/internal/grpcsvc"
	"voice/backend/matchmaking/internal/store"
)

type deniedNativeSpaceProbe struct{ calls int }

func (p *deniedNativeSpaceProbe) EnsureMemberAndMMEnabled(context.Context, uuid.UUID) error {
	p.calls++
	return nil
}

func TestNativeFreshRoutesPreserveNewerSearchAndHistoricalLfp(t *testing.T) {
	ctx := context.Background()
	pool, q := partyDB(t), projectionRedis(t)
	p := partyParams(t, pool)
	parties := &store.PartyStore{Pool: pool}
	created, err := parties.CreateSourceParty(ctx, p)
	require.NoError(t, err)
	did, err := (&Projector{Store: parties, Queue: q}).RunOnce(ctx)
	require.NoError(t, err)
	require.True(t, did)
	lfp := &store.LfpStore{Pool: pool}
	storyID := uuid.New()
	_, err = lfp.UpsertListing(ctx, storyID, p.InitiatorProfileID, `{}`)
	require.NoError(t, err)
	_, err = lfp.UpsertRequest(ctx, storyID, p.InitiatorProfileID, p.MemberProfileIDs[1], store.LfpResponseJoin)
	require.NoError(t, err)
	probe := &deniedNativeSpaceProbe{}
	svc := grpcsvc.NewNativeRouteHandler(grpcsvc.MatchmakingGRPC{Parties: parties, Sessions: &store.SessionStore{Pool: pool}, Games: &store.GameStore{Pool: pool}, Lfp: lfp, Bans: &store.BanStore{Pool: pool}, Queue: q, SpaceQueue: probe})
	actor := metadata.NewIncomingContext(ctx, metadata.Pairs(authctx.HeaderProfileID, p.InitiatorProfileID.String(), authctx.HeaderAccountID, p.InitiatorAccountID.String()))
	_, err = svc.StartSpaceQueue(actor, &matchmakingv1.StartSpaceQueueRequest{SpaceId: uuid.NewString(), GameId: p.GameID.String(), Mode: p.Mode, CriteriaJson: p.Criteria})
	require.Equal(t, codes.Unavailable, status.Code(err))
	for _, decision := range []string{"ACCEPT", "DECLINE"} {
		_, err = svc.DecideLfpRequest(actor, &matchmakingv1.DecideLfpRequestRequest{StoryId: storyID.String(), ResponderProfileId: p.MemberProfileIDs[1].String(), ResponseType: "JOIN", Decision: decision})
		require.Equal(t, codes.Unavailable, status.Code(err))
	}
	require.Zero(t, probe.calls)
	request, err := lfp.GetPendingRequest(ctx, storyID, p.MemberProfileIDs[1], store.LfpResponseJoin)
	require.NoError(t, err)
	require.Equal(t, store.LfpRequestPending, request.Status)
	for _, session := range created.Sessions {
		actual, err := svc.Sessions.Get(ctx, session.ID)
		require.NoError(t, err)
		require.Equal(t, store.SessionStatusSearching, actual.Status)
		require.Equal(t, session.RecoveryGeneration, actual.RecoveryGeneration)
	}
	var partyCount, sessionCount, intentCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM parties),(SELECT count(*) FROM search_sessions),(SELECT count(*) FROM matchmaking_party_search_queue_intents)`).Scan(&partyCount, &sessionCount, &intentCount))
	require.Equal(t, 1, partyCount)
	require.Equal(t, 2, sessionCount)
	require.Equal(t, 1, intentCount)
	depth, err := q.QueueDepth(ctx, p.GameID, p.Mode, "eu")
	require.NoError(t, err)
	require.EqualValues(t, 2, depth)
}
