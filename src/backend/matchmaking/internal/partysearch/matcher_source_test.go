package partysearch

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/matcher"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/store"
)

type matcherProposalFunc func(context.Context, store.ProposalCandidate) (partysource.ProposalValidation, error)

func (f matcherProposalFunc) ValidateProposalCandidate(ctx context.Context, c store.ProposalCandidate) (partysource.ProposalValidation, error) {
	return f(ctx, c)
}

type matcherEvents struct{ found []matcher.MatchFoundEvent }

func (e *matcherEvents) PublishMatchFound(_ context.Context, ev matcher.MatchFoundEvent) error {
	e.found = append(e.found, ev)
	return nil
}

func TestMatcherSourceGateUsesFreshAuthorityAndAffectedWholePartyReset(t *testing.T) {
	for _, scenario := range []string{"missing validator", "partial change then unknown", "exact changed party", "noninitiating account ban", "post RPC local change then fresh success"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			pool, q := partyDB(t), projectionRedis(t)
			games := &store.GameStore{Pool: pool}
			mode := config.Mode{Name: "native-four", Slots: 4, PartySizeMin: 1, PartySizeMax: 2}
			game, err := games.Create(ctx, "native matcher source journey", config.GameConfig{Regions: []string{"eu"}, Modes: []config.Mode{mode}}, uuid.New())
			require.NoError(t, err)
			parties := &store.PartyStore{Pool: pool}
			var searches []store.SourcePartySearch
			for i := 0; i < 2; i++ {
				p := partyParams(t, pool)
				p.GameID, p.Mode = game.ID, mode.Name
				created, createErr := parties.CreateSourceParty(ctx, p)
				require.NoError(t, createErr)
				searches = append(searches, created)
			}
			projector := &Projector{Store: parties, Queue: q}
			for i := 0; i < 2; i++ {
				did, projectErr := projector.RunOnce(ctx)
				require.NoError(t, projectErr)
				require.True(t, did)
			}
			depth, err := q.QueueDepth(ctx, game.ID, mode.Name, "eu")
			require.NoError(t, err)
			require.EqualValues(t, 4, depth)
			// Candidate validation visits canonical party ID order. Put the known
			// change first so the following Unknown must suppress partial reset.
			if searches[0].Party.ID.String() > searches[1].Party.ID.String() {
				searches[0], searches[1] = searches[1], searches[0]
			}
			calls := 0
			mutated := false
			reader := proposalSnapshotFunc(func(_ context.Context, subject partysource.Subject) (partysource.Snapshot, error) {
				calls++
				require.Zero(t, pool.Stat().AcquiredConns(), "snapshot RPC must not retain SQL locks/connections")
				if scenario == "post RPC local change then fresh success" && !mutated {
					mutated = true
					_, updateErr := pool.Exec(ctx, `UPDATE search_sessions SET updated_at=updated_at+interval '1 millisecond' WHERE id=$1`, searches[0].Sessions[0].ID)
					require.NoError(t, updateErr)
				}
				for i, search := range searches {
					p := search.Party
					if subject.ProfileID != p.LeaderProfileID {
						continue
					}
					require.Equal(t, p.InitiatorAccountID, subject.AccountID)
					require.Equal(t, p.SearchOperationID, subject.SearchOperationID)
					require.Equal(t, p.InitiatorSessionEpoch, subject.SessionEpoch)
					if scenario == "partial change then unknown" && i == 1 {
						return partysource.Snapshot{}, partysource.ErrUnknown
					}
					s := partysource.Snapshot{ProtocolVersion: p.ProtocolVersion, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion, MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}
					if i == 0 && (scenario == "partial change then unknown" || scenario == "exact changed party") {
						s.SourceAuthorityHash[0]++
					}
					return s, nil
				}
				return partysource.Snapshot{}, partysource.ErrUnknown
			})
			events := &matcherEvents{}
			w := &matcher.Worker{Queue: q, Games: games, Sessions: &store.SessionStore{Pool: pool}, Matches: &store.MatchStore{Pool: pool}, Bans: &store.BanStore{Pool: pool}, Events: events, ProposalValidator: &partysource.Revalidator{Snapshots: reader}}
			if scenario == "missing validator" {
				w.ProposalValidator = nil
			}
			if scenario == "noninitiating account ban" {
				require.NoError(t, (&store.BanStore{Pool: pool}).InsertPlatformMMBan(ctx, store.InsertPlatformMMBanParams{AccountID: searches[0].Party.MemberAuthorities[1].AccountID, BannedByProfileID: uuid.New()}))
			}
			require.NoError(t, w.RunOnce(ctx))
			require.Empty(t, events.found)
			if scenario == "missing validator" {
				require.Zero(t, calls)
			} else {
				require.GreaterOrEqual(t, calls, 2)
			}
			reset := scenario == "exact changed party" || scenario == "noninitiating account ban"
			for i, search := range searches {
				for _, original := range search.Sessions {
					s, getErr := w.Sessions.Get(ctx, original.ID)
					require.NoError(t, getErr)
					require.Nil(t, s.MatchID)
					if reset && i == 0 {
						require.Equal(t, store.SessionStatusCancelled, s.Status)
						require.Equal(t, original.RecoveryGeneration+1, s.RecoveryGeneration)
					} else {
						require.Equal(t, store.SessionStatusSearching, s.Status)
						require.Equal(t, original.RecoveryGeneration, s.RecoveryGeneration)
					}
				}
			}
			if reset {
				did, projectErr := projector.RunOnce(ctx)
				require.NoError(t, projectErr)
				require.True(t, did)
				depth, err = q.QueueDepth(ctx, game.ID, mode.Name, "eu")
				require.NoError(t, err)
				require.EqualValues(t, 2, depth, "durable RELEASE removes only affected whole party")
			}
			if scenario == "partial change then unknown" {
				// A generic denial has no trusted affected-account evidence.
				w.ProposalValidator = matcherProposalFunc(func(context.Context, store.ProposalCandidate) (partysource.ProposalValidation, error) {
					return partysource.ProposalValidation{}, errors.Join(store.ErrPlatformBanDenied, partysource.ErrUnknown)
				})
				require.NoError(t, w.RunOnce(ctx))
				for _, s := range searches {
					requireProposalStillSearching(t, pool, s)
				}
				w.ProposalValidator = matcherProposalFunc(func(context.Context, store.ProposalCandidate) (partysource.ProposalValidation, error) {
					return partysource.ProposalValidation{ChangedPartyIDs: []uuid.UUID{uuid.New()}}, nil
				})
				require.NoError(t, w.RunOnce(ctx))
				for _, s := range searches {
					requireProposalStillSearching(t, pool, s)
				}
			}
			if scenario == "post RPC local change then fresh success" {
				before := calls
				require.NoError(t, w.RunOnce(ctx))
				require.Equal(t, before+2, calls, "new pass needs fresh source observations")
				require.Len(t, events.found, 1)
				for _, search := range searches {
					for _, original := range search.Sessions {
						s, getErr := w.Sessions.Get(ctx, original.ID)
						require.NoError(t, getErr)
						require.Equal(t, store.SessionStatusPendingAccept, s.Status)
						require.Equal(t, &events.found[0].MatchID, s.MatchID)
						require.Equal(t, original.RecoveryGeneration+1, s.RecoveryGeneration)
					}
				}
				require.NoError(t, w.RunOnce(ctx))
				require.Len(t, events.found, 1)
				require.Equal(t, before+2, calls)
				depth, err = q.QueueDepth(ctx, game.ID, mode.Name, "eu")
				require.NoError(t, err)
				require.Zero(t, depth)
			}
		})
	}
}
