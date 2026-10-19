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
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/grpcsvc"
	"voice/backend/matchmaking/internal/mmadmission"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/matchmaking/internal/terminalprincipal"
	"voice/backend/pkg/principal"
)

func nativeTerminalContext(t *testing.T, request proto.Message, method string, account, profile uuid.UUID) context.Context {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	id := uuid.NewString()
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "matchmaking", RPC: method, RequestID: id, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 99, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	ctx, err := (terminalprincipal.Verifier{KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(context.Context, string, int64) error { return nil }}).Verify(metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), request, method)
	require.NoError(t, err)
	return ctx
}
func TestNativeTerminalHandlerReadsRequireExactPrivateActor(t *testing.T) {
	pool, st, p, intent := admittedProvisionFixture(t)
	service := grpcsvc.NewNativeRouteHandler(grpcsvc.MatchmakingGRPC{Matches: st, Sessions: &store.SessionStore{Pool: pool}})
	var session uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT id FROM search_sessions WHERE match_id=$1 AND profile_id=$2`, intent.MatchID, p.InitiatorProfileID).Scan(&session))
	matchReq := &matchmakingv1.GetMatchRequest{MatchId: intent.MatchID.String()}
	ctx := nativeTerminalContext(t, matchReq, matchmakingv1.MatchmakingService_GetMatch_FullMethodName, p.InitiatorAccountID, p.InitiatorProfileID)
	result, err := service.GetMatch(ctx, matchReq)
	require.NoError(t, err)
	require.Equal(t, intent.MatchID.String(), result.Match.Id)
	statusReq := &matchmakingv1.GetSearchStatusRequest{SessionId: session.String()}
	ctx = nativeTerminalContext(t, statusReq, matchmakingv1.MatchmakingService_GetSearchStatus_FullMethodName, p.InitiatorAccountID, p.InitiatorProfileID)
	observed, err := service.GetSearchStatus(ctx, statusReq)
	require.NoError(t, err)
	require.Equal(t, session.String(), observed.SearchSession.Id)
	history := &matchmakingv1.GetMatchHistoryRequest{}
	ctx = nativeTerminalContext(t, history, matchmakingv1.MatchmakingService_GetMatchHistory_FullMethodName, p.InitiatorAccountID, p.InitiatorProfileID)
	_, err = service.GetMatchHistory(ctx, history)
	require.NoError(t, err)
	raw := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", p.InitiatorProfileID.String(), "x-voice-user-id", p.InitiatorAccountID.String()))
	_, err = service.GetMatch(raw, matchReq)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = service.GetSearchStatus(raw, statusReq)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = service.GetMatchHistory(raw, history)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
func TestNativeTerminalHandlerCancelsWholePartyWhileClosed(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	binding, _, _ := historyReceiptFixture(t, pool)
	p := partyParams(t, pool)
	party, err := (&store.PartyStore{Pool: pool, NativeAdmission: &binding}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	service := grpcsvc.NewNativeRouteHandler(grpcsvc.MatchmakingGRPC{Matches: &store.MatchStore{Pool: pool, NativeAdmission: &binding}, Sessions: &store.SessionStore{Pool: pool}})
	var session uuid.UUID
	for _, row := range party.Sessions {
		if row.ProfileID == p.InitiatorProfileID {
			session = row.ID
		}
	}
	req := &matchmakingv1.CancelSearchRequest{SessionId: session.String()}
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	raw := metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-profile-id", p.InitiatorProfileID.String()))
	_, err = service.CancelSearch(raw, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	verified := nativeTerminalContext(t, req, matchmakingv1.MatchmakingService_CancelSearch_FullMethodName, p.InitiatorAccountID, p.InitiatorProfileID)
	_, err = service.CancelSearch(verified, req)
	require.NoError(t, err)
	_, err = service.CancelSearch(verified, req)
	require.NoError(t, err, "domain exact retry does not redispatch queue or duplicate events")
	var cancelled int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE party_id=$1 AND status='cancelled'`, party.Party.ID).Scan(&cancelled))
	require.Equal(t, len(p.MemberProfileIDs), cancelled)
}

func TestNativeTerminalReadWithLostBindingCannotExpireMatch(t *testing.T) {
	ctx := context.Background()
	pool, _, p, intent := admittedProvisionFixture(t)
	_, err := pool.Exec(ctx, `UPDATE match_proposals SET response='pending' WHERE match_id=$1`, intent.MatchID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE matches SET created_at=clock_timestamp()-interval '1 hour' WHERE id=$1`, intent.MatchID)
	require.NoError(t, err)
	service := grpcsvc.NewNativeRouteHandler(grpcsvc.MatchmakingGRPC{Matches: &store.MatchStore{Pool: pool}, Sessions: &store.SessionStore{Pool: pool}})
	req := &matchmakingv1.GetMatchRequest{MatchId: intent.MatchID.String()}
	verified := nativeTerminalContext(t, req, matchmakingv1.MatchmakingService_GetMatch_FullMethodName, p.InitiatorAccountID, p.InitiatorProfileID)
	_, err = service.GetMatch(verified, req)
	require.Error(t, err, "lost native binding cannot select legacy expiry mutation")
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM matches WHERE id=$1`, intent.MatchID).Scan(&state))
	require.Equal(t, store.MatchStatusPendingAccept, state)
}

func TestNativeTerminalReadExpiryWaitsForCloseAndRemainsAvailableClosed(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	_, err := pool.Exec(ctx, `UPDATE match_proposals SET response='pending' WHERE match_id=$1`, intent.MatchID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE matches SET created_at=clock_timestamp()-interval '1 hour' WHERE id=$1`, intent.MatchID)
	require.NoError(t, err)
	service := grpcsvc.NewNativeRouteHandler(grpcsvc.MatchmakingGRPC{Matches: st, Sessions: &store.SessionStore{Pool: pool}})
	req := &matchmakingv1.GetMatchRequest{MatchId: intent.MatchID.String()}
	verified := nativeTerminalContext(t, req, matchmakingv1.MatchmakingService_GetMatch_FullMethodName, p.InitiatorAccountID, p.InitiatorProfileID)
	closer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer closer.Rollback(ctx)
	require.NoError(t, mmadmission.AcquireCloseGate(ctx, closer))
	_, err = closer.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	type outcome struct {
		response *matchmakingv1.GetMatchResponse
		err      error
	}
	done := make(chan outcome, 1)
	go func() {
		bounded, cancel := context.WithTimeout(verified, 8*time.Second)
		defer cancel()
		r, e := service.GetMatch(bounded, req)
		done <- outcome{r, e}
	}()
	require.Eventually(t, func() bool {
		select {
		case result := <-done:
			t.Fatalf("read expiry bypassed FIRST terminal admission: %v", result.err)
		default:
		}
		var waiting bool
		return pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%pg_catalog.pg_advisory_xact_lock_shared%')`).Scan(&waiting) == nil && waiting
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, closer.Commit(ctx))
	result := <-done
	require.NoError(t, result.err)
	require.Equal(t, store.MatchStatusAbandoned, result.response.Match.Status)
}

func TestNativeTerminalChangePreservesExplicitPrecutDeadlineRead(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	p := partyParams(t, pool)
	party, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	st := &store.MatchStore{Pool: pool}
	proposal, err := createProvisionProposalFixture(t, pool, st, partyProposal(p, party.Sessions))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE matches SET created_at=clock_timestamp()-interval '1 hour' WHERE id=$1`, proposal.Match.ID)
	require.NoError(t, err)
	// Explicit compatibility composition, never the fixed native factory.
	service := &grpcsvc.MatchmakingGRPC{Matches: st, Sessions: &store.SessionStore{Pool: pool}}
	actor := metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-profile-id", p.InitiatorProfileID.String()))
	response, err := service.GetMatch(actor, &matchmakingv1.GetMatchRequest{MatchId: proposal.Match.ID.String()})
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusAbandoned, response.Match.Status)
	var heads int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM mm_native_admission_head`).Scan(&heads))
	require.Zero(t, heads)
}
