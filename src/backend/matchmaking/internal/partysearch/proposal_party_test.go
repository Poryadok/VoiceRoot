package partysearch

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/store"
)

func partyProposal(p store.CreateSourcePartyParams, sessions []store.SearchSession) store.CreateProposalParams {
	out := store.CreateProposalParams{GameID: p.GameID, Mode: p.Mode, Region: "eu"}
	for _, session := range sessions {
		out.Sessions = append(out.Sessions, store.ProposalSession{SessionID: session.ID, ProfileID: session.ProfileID, PartyID: session.PartyID})
	}
	return out
}

func TestProposalPreservesCompleteCapturedParty(t *testing.T) {
	pool := partyDB(t)
	ctx := context.Background()
	p := partyParams(t, pool)
	created, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	matches := &store.MatchStore{Pool: pool}
	_, err = matches.CreateProposal(ctx, partyProposal(p, created.Sessions[:1]))
	require.ErrorIs(t, err, store.ErrSessionNotSearchable, "a queue prefix cannot split an immutable party")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matches`).Scan(&count))
	require.Zero(t, count)
	for _, session := range created.Sessions {
		actual, err := (&store.SessionStore{Pool: pool}).Get(ctx, session.ID)
		require.NoError(t, err)
		require.Equal(t, store.SessionStatusSearching, actual.Status)
		require.Equal(t, session.RecoveryGeneration, actual.RecoveryGeneration)
	}
	wrong := partyProposal(p, created.Sessions)
	wrong.GameID = uuid.New()
	_, err = matches.CreateProposal(ctx, wrong)
	require.ErrorIs(t, err, store.ErrSessionNotSearchable)
	result, err := matches.CreateProposal(ctx, partyProposal(p, created.Sessions))
	require.NoError(t, err)
	require.Len(t, result.Proposals, len(created.Sessions))
	for _, session := range created.Sessions {
		actual, err := (&store.SessionStore{Pool: pool}).Get(ctx, session.ID)
		require.NoError(t, err)
		require.Equal(t, store.SessionStatusPendingAccept, actual.Status)
		require.Equal(t, session.RecoveryGeneration+1, actual.RecoveryGeneration)
	}
}

func TestProposalWaitsForPartyThenRejectsCommittedDisband(t *testing.T) {
	pool := partyDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	p := partyParams(t, pool)
	created, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer writer.Rollback(context.Background())
	_, err = writer.Exec(ctx, `UPDATE parties SET disbanded_at=clock_timestamp() WHERE id=$1`, created.Party.ID)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		_, err := (&store.MatchStore{Pool: pool}).CreateProposal(ctx, partyProposal(p, created.Sessions))
		done <- err
	}()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-done:
			t.Fatalf("proposal bypassed held party lock: %v", err)
		case <-deadline.C:
			t.Fatal("proposal did not reach the party lock")
		case <-tick.C:
			var waiting bool
			require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity
 WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%parties%' AND query LIKE '%FOR UPDATE%')`).Scan(&waiting))
			if !waiting {
				continue
			}
			require.NoError(t, writer.Commit(ctx))
			require.ErrorIs(t, <-done, store.ErrSessionNotSearchable)
			var count int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matches`).Scan(&count))
			require.Zero(t, count)
			return
		}
	}
}

func TestProposalRejectsChangedScopeAndExpiredMemberAtomically(t *testing.T) {
	pool := partyDB(t)
	ctx := context.Background()
	p := partyParams(t, pool)
	created, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	matches := &store.MatchStore{Pool: pool}
	proposal := partyProposal(p, created.Sessions)
	member := created.Sessions[1].ID
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET space_id=$2 WHERE id=$1`, member, uuid.New())
	require.NoError(t, err)
	_, err = matches.CreateProposal(ctx, proposal)
	require.ErrorIs(t, err, store.ErrSessionNotSearchable)
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET space_id=NULL,criteria='{"region":"us"}' WHERE id=$1`, member)
	require.NoError(t, err)
	_, err = matches.CreateProposal(ctx, proposal)
	require.ErrorIs(t, err, store.ErrSessionNotSearchable)
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET criteria=$2::jsonb,timeout_at=clock_timestamp()-interval '1 second' WHERE id=$1`, member, p.Criteria)
	require.NoError(t, err)
	_, err = matches.CreateProposal(ctx, proposal)
	require.ErrorIs(t, err, store.ErrSessionNotSearchable)
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET timeout_at=NULL WHERE id=$1`, member)
	require.NoError(t, err)
	_, err = matches.CreateProposal(ctx, proposal)
	require.ErrorIs(t, err, store.ErrSessionNotSearchable)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matches`).Scan(&count))
	require.Zero(t, count)
	for _, session := range created.Sessions {
		actual, err := (&store.SessionStore{Pool: pool}).Get(ctx, session.ID)
		require.NoError(t, err)
		require.Equal(t, store.SessionStatusSearching, actual.Status)
		require.Equal(t, session.RecoveryGeneration, actual.RecoveryGeneration)
	}
}
