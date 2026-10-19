package partysearch

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/matcher"
	"voice/backend/matchmaking/internal/store"
)

// This is the local whole-party selection invariant. These legacy MM parties
// are not adopted as a Voice/LFP source or used as snapshot authority evidence.
func TestMatcherSelectsCompletePartiesAcrossInterleavedQueue(t *testing.T) {
	pool, queue := partyDB(t), projectionRedis(t)
	ctx := context.Background()
	games, sessions := &store.GameStore{Pool: pool}, &store.SessionStore{Pool: pool}
	parties := &store.PartyStore{Pool: pool}
	mode := config.Mode{Name: "whole-three", Slots: 3, PartySizeMin: 1, PartySizeMax: 2}
	game, err := games.Create(ctx, "whole-party local selection", config.GameConfig{Regions: []string{"eu"}, Modes: []config.Mode{mode}}, uuid.New())
	require.NoError(t, err)
	createParty := func() []store.SearchSession {
		profiles := []uuid.UUID{uuid.New(), uuid.New()}
		party, createErr := parties.Create(ctx, store.CreatePartyParams{LeaderProfileID: profiles[0], MemberProfileIDs: profiles, GameID: game.ID, Mode: mode.Name, Criteria: `{"region":"eu"}`})
		require.NoError(t, createErr)
		out := make([]store.SearchSession, 0, 2)
		for _, profile := range profiles {
			session, sessionErr := sessions.Create(ctx, store.CreateSessionParams{ProfileID: profile, PartyID: &party.ID, GameID: game.ID, Mode: mode.Name, Criteria: `{"region":"eu"}`, TimeoutAt: time.Now().Add(time.Hour)})
			require.NoError(t, sessionErr)
			out = append(out, session)
		}
		return out
	}
	a, b := createParty(), createParty()
	solo, err := sessions.Create(ctx, store.CreateSessionParams{ProfileID: uuid.New(), GameID: game.ID, Mode: mode.Name, Criteria: `{"region":"eu"}`, TimeoutAt: time.Now().Add(time.Hour)})
	require.NoError(t, err)
	for i, session := range []store.SearchSession{a[0], b[0], b[1], a[1], solo} {
		require.NoError(t, queue.Enqueue(ctx, game.ID, mode.Name, "eu", session.ID, time.Now().Add(time.Duration(i)*time.Second)))
	}
	worker := &matcher.Worker{Queue: queue, Games: games, Sessions: sessions, Matches: &store.MatchStore{Pool: pool}, Bans: &store.BanStore{Pool: pool}}
	require.NoError(t, worker.RunOnce(ctx))
	var selectedMatch uuid.UUID
	for _, session := range []store.SearchSession{a[0], a[1], solo} {
		updated, getErr := sessions.Get(ctx, session.ID)
		require.NoError(t, getErr)
		require.Equal(t, store.SessionStatusPendingAccept, updated.Status, "select a whole oldest party plus a fitting singleton")
		require.NotNil(t, updated.MatchID)
		if selectedMatch == uuid.Nil {
			selectedMatch = *updated.MatchID
		}
		require.Equal(t, selectedMatch, *updated.MatchID)
	}
	for _, session := range b {
		updated, getErr := sessions.Get(ctx, session.ID)
		require.NoError(t, getErr)
		require.Equal(t, store.SessionStatusSearching, updated.Status, "the unfitting party must remain whole and searching")
		require.Nil(t, updated.MatchID)
	}
	depth, err := queue.QueueDepthScoped(ctx, nil, game.ID, mode.Name, "eu")
	require.NoError(t, err)
	require.EqualValues(t, 2, depth)

	// Queue loss of one member must not make the remaining member a singleton.
	d := createParty()
	secondSolo, err := sessions.Create(ctx, store.CreateSessionParams{ProfileID: uuid.New(), GameID: game.ID, Mode: mode.Name, Criteria: `{"region":"eu"}`, TimeoutAt: time.Now().Add(time.Hour)})
	require.NoError(t, err)
	require.NoError(t, queue.Enqueue(ctx, game.ID, mode.Name, "eu", d[0].ID, time.Now().Add(10*time.Second)))
	require.NoError(t, queue.Enqueue(ctx, game.ID, mode.Name, "eu", secondSolo.ID, time.Now().Add(11*time.Second)))
	require.NoError(t, worker.RunOnce(ctx))
	for _, session := range append(b, secondSolo) {
		updated, getErr := sessions.Get(ctx, session.ID)
		require.NoError(t, getErr)
		require.Equal(t, store.SessionStatusPendingAccept, updated.Status)
		require.NotNil(t, updated.MatchID)
	}
	for _, session := range d {
		updated, getErr := sessions.Get(ctx, session.ID)
		require.NoError(t, getErr)
		require.Equal(t, store.SessionStatusSearching, updated.Status)
		require.Nil(t, updated.MatchID, "incomplete Redis party cannot be silently shrunk")
	}
	depth, err = queue.QueueDepthScoped(ctx, nil, game.ID, mode.Name, "eu")
	require.NoError(t, err)
	require.EqualValues(t, 1, depth)
}
