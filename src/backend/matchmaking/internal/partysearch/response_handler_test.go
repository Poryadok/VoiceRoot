package partysearch

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	"time"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/grpcsvc"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/responseprincipal"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/principal"
)

func signedResponseSubject(t *testing.T, req *matchmakingv1.RespondToMatchRequest, account, profile uuid.UUID) context.Context {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "matchmaking", RPC: matchmakingv1.MatchmakingService_RespondToMatch_FullMethodName, RequestID: req.OperationId, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, ClientExpiresAt: now.Add(time.Minute)})
	require.NoError(t, err)
	v := responseprincipal.Verifier{Clock: func() time.Time { return now }, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(context.Context, string, int64) error { return nil }}
	ctx, err := v.Verify(metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", req.OperationId)), req)
	require.NoError(t, err)
	return ctx
}

// Actual signed response subject + PG + production provider coordinator; Chat,
// Voice source and registration below are explicit component fixtures, not
// installed transport/provider/Voice coverage authority.
func TestNativeResponseHandlerActivatesOnceAndClosedRetryIsReadOnly(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	_, err := pool.Exec(ctx, `UPDATE match_proposals SET response='pending' WHERE match_id=$1`, intent.MatchID)
	require.NoError(t, err)
	probe := &forwardProbe{}
	worker, chat, voice := lifetimeWorker(t, st, p, probe)
	validator := worker.PartyValidator.(partysource.ResponseValidator)
	svc := grpcsvc.NewNativeRouteHandler(grpcsvc.MatchmakingGRPC{Matches: st, Sessions: &store.SessionStore{Pool: pool}, Squad: worker, ResponseValidator: validator, ResponseLifetime: probe})
	var req *matchmakingv1.RespondToMatchRequest
	for i, member := range p.MemberAuthorities {
		req = &matchmakingv1.RespondToMatchRequest{MatchId: intent.MatchID.String(), Accept: true, OperationId: uuid.NewString()}
		resp, err := svc.RespondToMatch(signedResponseSubject(t, req, member.AccountID, member.ProfileID), req)
		require.NoError(t, err)
		if i == 0 {
			require.Equal(t, store.MatchStatusPendingAccept, resp.Match.Status)
			require.Empty(t, resp.Match.GetChatId())
			require.Empty(t, resp.Match.GetVoiceRoomId())
		} else {
			require.Equal(t, store.MatchStatusActive, resp.Match.Status)
			require.NotEmpty(t, resp.Match.GetChatId())
			require.NotEmpty(t, resp.Match.GetVoiceRoomId())
		}
	}
	require.Equal(t, 1, chat.calls)
	require.Equal(t, 1, voice.calls)
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	svc.ResponseValidator = nil
	svc.ResponseLifetime = nil
	last := p.MemberAuthorities[len(p.MemberAuthorities)-1]
	replayed, err := svc.RespondToMatch(signedResponseSubject(t, req, last.AccountID, last.ProfileID), req)
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusActive, replayed.Match.Status)
	require.Equal(t, 1, chat.calls)
	require.Equal(t, 1, voice.calls)
	changed := &matchmakingv1.RespondToMatchRequest{MatchId: req.MatchId, OperationId: req.OperationId, Accept: false}
	_, err = svc.RespondToMatch(signedResponseSubject(t, changed, last.AccountID, last.ProfileID), changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
}

func TestNativeResponseHandlerDeclinesClosedAndRejectsRawContext(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	_, err := pool.Exec(ctx, `UPDATE match_proposals SET response='pending' WHERE match_id=$1`, intent.MatchID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	svc := grpcsvc.NewNativeRouteHandler(grpcsvc.MatchmakingGRPC{Matches: st, Sessions: &store.SessionStore{Pool: pool}})
	req := &matchmakingv1.RespondToMatchRequest{MatchId: intent.MatchID.String(), OperationId: uuid.NewString()}
	_, err = svc.RespondToMatch(context.Background(), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	member := p.MemberAuthorities[0]
	resp, err := svc.RespondToMatch(signedResponseSubject(t, req, member.AccountID, member.ProfileID), req)
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusAbandoned, resp.Match.Status)
}

type responseValidationFunc func(context.Context, store.ResponseCandidate) (partysource.ResponseValidation, error)

func (f responseValidationFunc) ValidateResponseCandidate(ctx context.Context, c store.ResponseCandidate) (partysource.ResponseValidation, error) {
	return f(ctx, c)
}

func TestNativeResponseHandlerUnknownBanAndLocalRaceNeverDispatch(t *testing.T) {
	for _, kind := range []string{"unknown", "changed", "nonactor-ban", "closed-readback", "local-cut", "invalid-changed-set", "generic-ban"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			pool, st, p, intent := admittedProvisionFixture(t)
			_, err := pool.Exec(ctx, `UPDATE match_proposals SET response='pending' WHERE match_id=$1`, intent.MatchID)
			require.NoError(t, err)
			worker, chat, voice := lifetimeWorker(t, st, p, &forwardProbe{})
			validator := worker.PartyValidator.(partysource.ResponseValidator)
			life := &projectionLifetimeFixture{}
			svc := grpcsvc.NewNativeRouteHandler(grpcsvc.MatchmakingGRPC{Matches: st, Sessions: &store.SessionStore{Pool: pool}, Squad: worker, ResponseValidator: validator, ResponseLifetime: life})
			switch kind {
			case "unknown":
				svc.ResponseValidator = responseValidationFunc(func(context.Context, store.ResponseCandidate) (partysource.ResponseValidation, error) {
					return partysource.ResponseValidation{}, partysource.ErrUnknown
				})
			case "changed":
				svc.ResponseValidator = responseValidationFunc(func(_ context.Context, c store.ResponseCandidate) (partysource.ResponseValidation, error) {
					return partysource.ResponseValidation{ChangedPartyIDs: []uuid.UUID{c.Parties()[0].ID}}, nil
				})
			case "nonactor-ban":
				require.NoError(t, (&store.BanStore{Pool: pool}).InsertPlatformMMBan(ctx, store.InsertPlatformMMBanParams{AccountID: p.MemberAuthorities[1].AccountID, BannedByProfileID: uuid.New()}))
			case "closed-readback":
				life.err = partysource.ErrUnknown
			case "local-cut":
				life.enter = func() {
					_, e := pool.Exec(ctx, `UPDATE match_proposals SET response='accepted' WHERE match_id=$1 AND profile_id=$2`, intent.MatchID, p.MemberAuthorities[1].ProfileID)
					require.NoError(t, e)
				}
			case "invalid-changed-set":
				svc.ResponseValidator = responseValidationFunc(func(context.Context, store.ResponseCandidate) (partysource.ResponseValidation, error) {
					return partysource.ResponseValidation{ChangedPartyIDs: []uuid.UUID{uuid.New()}}, nil
				})
			case "generic-ban":
				svc.ResponseValidator = responseValidationFunc(func(context.Context, store.ResponseCandidate) (partysource.ResponseValidation, error) {
					return partysource.ResponseValidation{}, store.ErrPlatformBanDenied
				})
			}
			req := &matchmakingv1.RespondToMatchRequest{MatchId: intent.MatchID.String(), Accept: true, OperationId: uuid.NewString()}
			actor := p.MemberAuthorities[0]
			_, err = svc.RespondToMatch(signedResponseSubject(t, req, actor.AccountID, actor.ProfileID), req)
			require.Error(t, err)
			require.Zero(t, chat.calls)
			require.Zero(t, voice.calls)
			var receipts, pending int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_response_receipts`).Scan(&receipts))
			require.Zero(t, receipts)
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE match_id=$1 AND status='pending_accept'`, intent.MatchID).Scan(&pending))
			if kind == "changed" || kind == "nonactor-ban" {
				require.Zero(t, pending, "definitive loss resets the complete affected party")
			} else {
				require.Equal(t, 2, pending, "Unknown cannot cancel a party")
			}
		})
	}
}
