package partysearch

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/authctx"
	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/grpcsvc"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/searchprincipal"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/principal"
)

func verifiedHandlerSearch(t *testing.T, req *matchmakingv1.StartSearchRequest, p store.CreateSourcePartyParams) context.Context {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "fixture", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "matchmaking", RPC: matchmakingv1.MatchmakingService_StartSearch_FullMethodName, RequestID: p.SearchOperationID.String(), RequestHash: hash, AccountID: p.InitiatorAccountID.String(), ProfileID: p.InitiatorProfileID.String(), SessionEpoch: p.InitiatorSessionEpoch, ClientExpiresAt: now.Add(time.Minute)})
	require.NoError(t, err)
	v := searchprincipal.Verifier{Clock: func() time.Time { return now }, KeyResolver: func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) {
		require.Equal(t, "gateway", issuer)
		require.Equal(t, "fixture", kid)
		return &key.PublicKey, nil
	}, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(_ context.Context, account string, epoch int64) error {
		require.Equal(t, p.InitiatorAccountID.String(), account)
		require.Equal(t, p.InitiatorSessionEpoch, epoch)
		return nil
	}}
	ctx, err := v.Verify(metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", p.SearchOperationID.String())), req)
	require.NoError(t, err)
	return ctx
}

func TestProtectedSearchHandlerCapturesWholePartyAndRefusesLegacyAuthority(t *testing.T) {
	for _, scenario := range []string{"raw metadata", "missing reader", "unknown source", "client party", "noninitiator ban", "party exceeds mode", "whole party replay"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			pool, q := partyDB(t), projectionRedis(t)
			games, parties := &store.GameStore{Pool: pool}, &store.PartyStore{Pool: pool}
			maximum := 2
			if scenario == "party exceeds mode" {
				maximum = 1
			}
			mode := config.Mode{Name: "protected-duo", Slots: 2, PartySizeMin: 1, PartySizeMax: maximum}
			game, err := games.Create(ctx, "protected source enqueue", config.GameConfig{Regions: []string{"eu"}, Modes: []config.Mode{mode}}, uuid.New())
			require.NoError(t, err)
			p := partyParams(t, pool)
			p.GameID, p.Mode = game.ID, mode.Name
			req := &matchmakingv1.StartSearchRequest{GameId: game.ID.String(), Mode: mode.Name, CriteriaJson: `{"region":"eu"}`}
			if scenario == "client party" {
				value := uuid.NewString()
				req.PartyId = &value
			}
			verified := verifiedHandlerSearch(t, req, p)
			calls := 0
			reader := proposalSnapshotFunc(func(_ context.Context, subject partysource.Subject) (partysource.Snapshot, error) {
				calls++
				require.Zero(t, pool.Stat().AcquiredConns())
				require.Equal(t, p.SearchOperationID, subject.SearchOperationID)
				require.Equal(t, p.InitiatorAccountID, subject.AccountID)
				require.Equal(t, p.InitiatorProfileID, subject.ProfileID)
				require.Equal(t, p.InitiatorSessionEpoch, subject.SessionEpoch)
				if scenario == "unknown source" {
					return partysource.Snapshot{}, errors.New(p.InitiatorAccountID.String())
				}
				return partysource.Snapshot{ProtocolVersion: p.ProtocolVersion, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion, MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}, nil
			})
			svc := &grpcsvc.MatchmakingGRPC{Games: games, Parties: parties, Sessions: &store.SessionStore{Pool: pool}, Bans: &store.BanStore{Pool: pool}, Queue: q, PartySnapshots: reader}
			if scenario == "missing reader" {
				svc.PartySnapshots = nil
			}
			if scenario == "raw metadata" {
				verified = metadata.NewIncomingContext(ctx, metadata.Pairs(authctx.HeaderProfileID, p.InitiatorProfileID.String(), authctx.HeaderAccountID, p.InitiatorAccountID.String()))
			}
			if scenario == "noninitiator ban" {
				require.NoError(t, svc.Bans.InsertPlatformMMBan(ctx, store.InsertPlatformMMBanParams{AccountID: p.MemberAuthorities[1].AccountID, BannedByProfileID: uuid.New()}))
			}
			response, callErr := svc.StartSearch(verified, req)
			if scenario != "whole party replay" {
				require.Error(t, callErr)
				require.Nil(t, response)
				require.NotContains(t, callErr.Error(), p.InitiatorAccountID.String())
				if scenario == "raw metadata" {
					require.Equal(t, codes.Unauthenticated, status.Code(callErr))
					require.Zero(t, calls)
				}
				if scenario == "unknown source" || scenario == "noninitiator ban" || scenario == "party exceeds mode" {
					require.Equal(t, 1, calls)
				}
				var count int
				require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM parties)+(SELECT count(*) FROM search_sessions)+(SELECT count(*) FROM matchmaking_party_search_queue_intents)`).Scan(&count))
				require.Zero(t, count)
				return
			}
			require.NoError(t, callErr)
			require.NotNil(t, response.GetSearchSession())
			require.Equal(t, p.InitiatorProfileID.String(), response.GetSearchSession().GetProfileId())
			var partyID uuid.UUID
			require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM parties WHERE search_operation_id=$1`, p.SearchOperationID).Scan(&partyID))
			captured, err := parties.GetSourceParty(ctx, partyID)
			require.NoError(t, err)
			require.Equal(t, p.MemberAuthorities, captured.MemberAuthorities)
			replayed, err := svc.StartSearch(verifiedHandlerSearch(t, req, p), req)
			require.NoError(t, err)
			require.Equal(t, response, replayed)
			require.Equal(t, 2, calls)
			var partyCount, sessionCount, intentCount int
			require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM parties),(SELECT count(*) FROM search_sessions),(SELECT count(*) FROM matchmaking_party_search_queue_intents)`).Scan(&partyCount, &sessionCount, &intentCount))
			require.Equal(t, 1, partyCount)
			require.Equal(t, 2, sessionCount)
			require.Equal(t, 1, intentCount)
			did, err := (&Projector{Store: parties, Queue: q}).RunOnce(ctx)
			require.NoError(t, err)
			require.True(t, did)
			depth, err := q.QueueDepth(ctx, game.ID, mode.Name, "eu")
			require.NoError(t, err)
			require.EqualValues(t, 2, depth)
		})
	}
}
