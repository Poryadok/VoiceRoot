package grpcsvc

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/matchmaking/internal/store"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

func TestDecideLfpRequest_JoinAcceptCreatesPartyInQueue(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := searchTestServer(t, pool)
	srv.Parties = &store.PartyStore{Pool: pool}
	srv.Lfp = &store.LfpStore{Pool: pool}

	author := uuid.New()
	responder := uuid.New()
	storyID := uuid.New()
	gameID := dotaGameID(t, srv, ctx)
	criteria := fmt.Sprintf(
		`{"game_id":%q,"mode":"5v5 Ranked","region":"eu","self":{"role":"Carry","rank":"Herald"},"sought":{"rank_min":"Herald","rank_max":"Guardian"}}`,
		gameID,
	)

	_, err := srv.Lfp.UpsertListing(ctx, storyID, author, criteria)
	require.NoError(t, err)
	_, err = srv.Lfp.UpsertRequest(ctx, storyID, author, responder, store.LfpResponseJoin)
	require.NoError(t, err)

	resp, err := srv.DecideLfpRequest(ctxWithProfile(author), &matchmakingv1.DecideLfpRequestRequest{
		StoryId:            storyID.String(),
		ResponderProfileId: responder.String(),
		ResponseType:       "JOIN",
		Decision:           "ACCEPT",
	})
	require.NoError(t, err)
	require.Equal(t, "accepted", resp.GetStatus())
	require.NotEmpty(t, resp.GetPartyId())
	require.Len(t, resp.GetSearchSessions(), 2)

	for _, sess := range resp.GetSearchSessions() {
		require.Equal(t, "searching", sess.GetStatus())
		require.Equal(t, resp.GetPartyId(), sess.GetPartyId())
	}

	authorSess, err := srv.Sessions.GetActiveSearching(ctx, author)
	require.NoError(t, err)
	require.Equal(t, store.SessionStatusSearching, authorSess.Status)
	require.NotNil(t, authorSess.PartyID)

	responderSess, err := srv.Sessions.GetActiveSearching(ctx, responder)
	require.NoError(t, err)
	require.Equal(t, store.SessionStatusSearching, responderSess.Status)
	require.Equal(t, *authorSess.PartyID, *responderSess.PartyID)
}

func TestDecideLfpRequest_DeclineLeavesNoParty(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := searchTestServer(t, pool)
	srv.Parties = &store.PartyStore{Pool: pool}
	srv.Lfp = &store.LfpStore{Pool: pool}

	author := uuid.New()
	responder := uuid.New()
	storyID := uuid.New()
	gameID := dotaGameID(t, srv, ctx)
	criteria := fmt.Sprintf(`{"game_id":%q,"mode":"5v5 Ranked","region":"eu"}`, gameID)

	_, err := srv.Lfp.UpsertListing(ctx, storyID, author, criteria)
	require.NoError(t, err)
	_, err = srv.Lfp.UpsertRequest(ctx, storyID, author, responder, store.LfpResponseJoin)
	require.NoError(t, err)

	resp, err := srv.DecideLfpRequest(ctxWithProfile(author), &matchmakingv1.DecideLfpRequestRequest{
		StoryId:            storyID.String(),
		ResponderProfileId: responder.String(),
		ResponseType:       "JOIN",
		Decision:           "DECLINE",
	})
	require.NoError(t, err)
	require.Equal(t, "declined", resp.GetStatus())
	require.Empty(t, resp.GetPartyId())
	require.Empty(t, resp.GetSearchSessions())

	_, err = srv.Sessions.GetActiveSearching(ctx, author)
	require.ErrorIs(t, err, store.ErrSessionNotFound)
}

func TestDecideLfpRequest_NonAuthorDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := searchTestServer(t, pool)
	srv.Parties = &store.PartyStore{Pool: pool}
	srv.Lfp = &store.LfpStore{Pool: pool}

	author := uuid.New()
	responder := uuid.New()
	storyID := uuid.New()
	_, err := srv.Lfp.UpsertListing(ctx, storyID, author, `{"game_id":"`+uuid.New().String()+`","mode":"5v5 Ranked"}`)
	require.NoError(t, err)
	_, err = srv.Lfp.UpsertRequest(ctx, storyID, author, responder, store.LfpResponseInvite)
	require.NoError(t, err)

	_, err = srv.DecideLfpRequest(ctxWithProfile(responder), &matchmakingv1.DecideLfpRequestRequest{
		StoryId:            storyID.String(),
		ResponderProfileId: responder.String(),
		ResponseType:       "INVITE",
		Decision:           "ACCEPT",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
