package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestLfpStore_ListingAndJoinRequestRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	lfp := &LfpStore{Pool: pool}
	storyID := uuid.New()
	authorID := uuid.New()
	responderID := uuid.New()
	criteria := `{"game_id":"` + uuid.New().String() + `","mode":"5v5 Ranked","region":"eu"}`

	listing, err := lfp.UpsertListing(ctx, storyID, authorID, criteria)
	require.NoError(t, err)
	require.Equal(t, storyID, listing.StoryID)
	require.Nil(t, listing.InactiveAt)

	req, err := lfp.UpsertRequest(ctx, storyID, authorID, responderID, LfpResponseJoin)
	require.NoError(t, err)
	require.Equal(t, LfpRequestPending, req.Status)
	require.Equal(t, LfpResponseJoin, req.ResponseType)

	got, err := lfp.GetPendingRequest(ctx, storyID, responderID, LfpResponseJoin)
	require.NoError(t, err)
	require.Equal(t, req.ID, got.ID)

	decided, err := lfp.DecideRequest(ctx, req.ID, LfpRequestDeclined, nil)
	require.NoError(t, err)
	require.Equal(t, LfpRequestDeclined, decided.Status)
	require.NotNil(t, decided.DecidedAt)

	_, err = lfp.DecideRequest(ctx, req.ID, LfpRequestAccepted, nil)
	require.ErrorIs(t, err, ErrLfpRequestNotPending)
}

func TestPartyStore_CreatePersistsMembers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	games := &GameStore{Pool: pool}
	list, err := games.List(ctx, ListGamesParams{PageSize: 10, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, list.Games)
	gameID := list.Games[0].ID

	leader := uuid.New()
	member := uuid.New()
	parties := &PartyStore{Pool: pool}
	party, err := parties.Create(ctx, CreatePartyParams{
		LeaderProfileID:  leader,
		MemberProfileIDs: []uuid.UUID{leader, member},
		GameID:           gameID,
		Mode:             "5v5 Ranked",
		Criteria:         `{"region":"eu","self":{"role":"Carry","rank":"Herald"}}`,
	})
	require.NoError(t, err)
	require.Equal(t, leader, party.LeaderProfileID)
	require.Len(t, party.MemberProfileIDs, 2)

	got, err := parties.Get(ctx, party.ID)
	require.NoError(t, err)
	require.Equal(t, party.ID, got.ID)
}
