package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/criteria"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/squad"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/principal"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

func duoGameConfig() config.GameConfig {
	return config.GameConfig{
		Regions: []string{"eu"},
		Modes: []config.Mode{{
			Name:          "Duo",
			Slots:         2,
			PartySizeMin:  1,
			PartySizeMax:  1,
			RolesRequired: false,
			RankRequired:  false,
		}},
	}
}

type squadProvisioner interface {
	Provision(context.Context, uuid.UUID, []uuid.UUID) (voiceRoomID, chatID string, err error)
}

type stubSquadProvisioner struct {
	voiceRoomID string
	chatID      string
}

func (s *stubSquadProvisioner) Provision(_ context.Context, _ uuid.UUID, _ []uuid.UUID) (string, string, error) {
	if s.voiceRoomID == "" {
		s.voiceRoomID = "voice-room-1"
	}
	if s.chatID == "" {
		s.chatID = "chat-1"
	}
	return s.voiceRoomID, s.chatID, nil
}

type testMatchSquadChatClient struct {
	chatv1.MatchSquadChatServiceClient
}

func (testMatchSquadChatClient) CreateMatchSquadChat(_ context.Context, req *chatv1.CreateMatchSquadChatRequest, _ ...grpc.CallOption) (*chatv1.CreateMatchSquadChatResponse, error) {
	requestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(req)
	if err != nil {
		return nil, err
	}
	requestHash := sha256.Sum256(requestBytes)
	return &chatv1.CreateMatchSquadChatResponse{Receipt: &chatv1.MatchSquadChatReceipt{
		ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: req.GetOperationId(), MatchId: req.GetMatchId(),
		ChatId: uuid.NewString(), ParticipantManifestSha256: append([]byte(nil), req.GetParticipantManifestSha256()...),
		RequestSha256: requestHash[:], CreatedAt: timestamppb.Now(),
	}}, nil
}

type testMatchSquadVoiceClient struct {
	callsv1.MatchSquadVoiceServiceClient
}

func (testMatchSquadVoiceClient) CreateMatchSquadRoom(_ context.Context, req *callsv1.CreateMatchSquadRoomRequest, _ ...grpc.CallOption) (*callsv1.CreateMatchSquadRoomResponse, error) {
	requestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(req)
	if err != nil {
		return nil, err
	}
	requestHash := sha256.Sum256(requestBytes)
	chatReceiptBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(req.GetChatCreationReceipt())
	if err != nil {
		return nil, err
	}
	chatReceiptHash := sha256.Sum256(chatReceiptBytes)
	return &callsv1.CreateMatchSquadRoomResponse{Receipt: &callsv1.MatchSquadRoomReceipt{
		ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: req.GetOperationId(), MatchId: req.GetMatchId(),
		RoomId: uuid.NewString(), ChatId: req.GetChatCreationReceipt().GetChatId(),
		ChatCreationReceiptId: req.GetChatCreationReceipt().GetReceiptId(), ChatCreationReceiptSha256: chatReceiptHash[:],
		ParticipantManifestSha256: append([]byte(nil), req.GetParticipantManifestSha256()...), RequestSha256: requestHash[:],
		CreatedAt: timestamppb.Now(),
	}}, nil
}

func newTestProtectedProvisioner(t *testing.T, pool *pgxpool.Pool) *squad.MatchSquadProviderWorker {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "matchmaking", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	return &squad.MatchSquadProviderWorker{
		Store:  &store.MatchStore{Pool: pool},
		Chat:   testMatchSquadChatClient{},
		Voice:  testMatchSquadVoiceClient{},
		Issuer: issuer,
	}
}

type errSquadProvisioner struct{ err error }

func (s errSquadProvisioner) Provision(context.Context, uuid.UUID, []uuid.UUID) (string, string, error) {
	return "", "", s.err
}

func matchTestServer(t *testing.T, pool *pgxpool.Pool, provisioner squadProvisioner) *MatchmakingGRPC {
	t.Helper()
	if _, ok := provisioner.(*stubSquadProvisioner); ok {
		provisioner = newTestProtectedProvisioner(t, pool)
	}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
		mr.Close()
	})
	return &MatchmakingGRPC{
		Games:    &store.GameStore{Pool: pool},
		Sessions: &store.SessionStore{Pool: pool},
		Matches:  &store.MatchStore{Pool: pool},
		Queue:    &queue.RedisQueue{Client: rdb, Prefix: "match-test"},
		Events:   mmevents.NoopPublisher{},
		Squad:    provisioner,
	}
}

func seedPendingDuoMatch(t *testing.T, ctx context.Context, srv *MatchmakingGRPC) (matchID string, profileA, profileB uuid.UUID) {
	t.Helper()
	game, err := srv.Games.Create(ctx, "Respond test", duoGameConfig(), uuid.New())
	require.NoError(t, err)
	timeout := time.Now().UTC().Add(30 * time.Minute)
	crit := criteria.MustMarshal(criteria.SearchCriteria{Region: "eu"})
	profileA = uuid.New()
	profileB = uuid.New()
	sessA, err := srv.Sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: profileA,
		GameID:    game.ID,
		Mode:      "Duo",
		Criteria:  crit,
		TimeoutAt: timeout,
	})
	require.NoError(t, err)
	sessB, err := srv.Sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: profileB,
		GameID:    game.ID,
		Mode:      "Duo",
		Criteria:  crit,
		TimeoutAt: timeout,
	})
	require.NoError(t, err)
	result, err := srv.Matches.CreateProposal(ctx, store.CreateProposalParams{
		GameID: game.ID,
		Mode:   "Duo",
		Region: "eu",
		Sessions: []store.ProposalSession{
			{SessionID: sessA.ID, ProfileID: profileA},
			{SessionID: sessB.ID, ProfileID: profileB},
		},
	})
	require.NoError(t, err)
	return result.Match.ID.String(), profileA, profileB
}

func TestRespondToMatch_AcceptAllActivatesMatch(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	provisioner := &stubSquadProvisioner{}
	srv := matchTestServer(t, pool, provisioner)
	matchID, profileA, profileB := seedPendingDuoMatch(t, ctx, srv)

	ctxA := ctxWithProfile(profileA)
	ctxB := ctxWithProfile(profileB)

	respA, err := srv.RespondToMatch(ctxA, &matchmakingv1.RespondToMatchRequest{
		MatchId: matchID,
		Accept:  true,
	})
	require.NoError(t, err)
	require.Equal(t, "pending_accept", respA.GetMatch().GetStatus())

	respB, err := srv.RespondToMatch(ctxB, &matchmakingv1.RespondToMatchRequest{
		MatchId: matchID,
		Accept:  true,
	})
	require.NoError(t, err)
	require.Equal(t, "active", respB.GetMatch().GetStatus())
	require.NotEmpty(t, respB.GetMatch().GetVoiceRoomId())
	require.NotEmpty(t, respB.GetMatch().GetChatId())
	require.Equal(t, "matched", respB.GetSearchSession().GetStatus())

	got, err := srv.GetMatch(ctxA, &matchmakingv1.GetMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	require.Equal(t, "active", got.GetMatch().GetStatus())
	require.Len(t, got.GetMatch().GetProfileIds(), 2)
	require.Equal(t, store.ProposalResponseAccepted, got.GetOwnProposalResponse())
	require.Equal(t, store.SessionStatusMatched, got.GetOwnSearchSession().GetStatus())
	require.Equal(t, got.GetMatch().GetCreatedAt().AsTime().Add(store.MatchAcceptWindow), got.GetAcceptanceDeadlineAt().AsTime())
	require.True(t, got.GetServerNow().IsValid())
}

func TestGetMatch_AtDeadlineReturnsServerRecoveryState(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, _, profileB := seedPendingDuoMatch(t, ctx, srv)
	_, err := pool.Exec(ctx, `UPDATE matches SET created_at = clock_timestamp() - interval '31 seconds' WHERE id = $1`, uuid.MustParse(matchID))
	require.NoError(t, err)

	resp, err := srv.GetMatch(ctxWithProfile(profileB), &matchmakingv1.GetMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusAbandoned, resp.GetMatch().GetStatus())
	require.Equal(t, store.ProposalResponseDeclined, resp.GetOwnProposalResponse())
	require.Equal(t, store.SessionStatusCancelled, resp.GetOwnSearchSession().GetStatus())
	require.True(t, resp.GetServerNow().AsTime().After(resp.GetAcceptanceDeadlineAt().AsTime()))
}

func TestRespondToMatch_RejectsNewAcceptAtOrAfterMatchDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, _, profileB := seedPendingDuoMatch(t, ctx, srv)

	// Seed an already-expired proposal using database time so this request is
	// unambiguously late without relying on the periodic sweeper or host clock.
	// now() is the transaction start time; the RPC's later clock_timestamp()
	// read is therefore at or just after the acceptance boundary.
	_, err := pool.Exec(ctx, `UPDATE matches SET created_at = now() - ($2 * interval '1 millisecond') WHERE id = $1`, uuid.MustParse(matchID), store.MatchAcceptWindow.Milliseconds())
	require.NoError(t, err)

	_, err = srv.RespondToMatch(ctxWithProfile(profileB), &matchmakingv1.RespondToMatchRequest{
		MatchId: matchID,
		Accept:  true,
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "a pending proposal cannot be newly accepted after the server deadline")

	match, err := srv.Matches.Get(ctx, uuid.MustParse(matchID))
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusAbandoned, match.Status)
	proposal, err := srv.Matches.GetProposalForProfile(ctx, uuid.MustParse(matchID), profileB)
	require.NoError(t, err)
	require.Equal(t, store.ProposalResponseDeclined, proposal.Response)
	searchSession, err := srv.Sessions.Get(ctx, proposal.SearchSessionID)
	require.NoError(t, err)
	require.Equal(t, store.SessionStatusCancelled, searchSession.Status)
	queued, err := srv.Queue.ListSessionIDs(ctx, searchSession.GameID, searchSession.Mode, "eu", 0)
	require.NoError(t, err)
	require.NotContains(t, queued, searchSession.ID)
}

func TestRespondToMatch_AcceptsBeforeMatchDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, _, profileB := seedPendingDuoMatch(t, ctx, srv)

	// Leave a one-second server-clock margin so the request is timely without
	// relying on a host-side timing assertion at a sub-millisecond boundary.
	_, err := pool.Exec(ctx, `UPDATE matches SET created_at = now() - (($2 - 1000) * interval '1 millisecond') WHERE id = $1`, uuid.MustParse(matchID), store.MatchAcceptWindow.Milliseconds())
	require.NoError(t, err)

	resp, err := srv.RespondToMatch(ctxWithProfile(profileB), &matchmakingv1.RespondToMatchRequest{
		MatchId: matchID,
		Accept:  true,
	})
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusPendingAccept, resp.GetMatch().GetStatus())
	proposal, err := srv.Matches.GetProposalForProfile(ctx, uuid.MustParse(matchID), profileB)
	require.NoError(t, err)
	require.Equal(t, store.ProposalResponseAccepted, proposal.Response)
}

func TestRespondToMatch_AcceptBlockedAcrossDeadlineIsRejectedAfterLocks(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, _, profileB := seedPendingDuoMatch(t, ctx, srv)
	parsedMatchID := uuid.MustParse(matchID)

	locker, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer locker.Release()
	tx, err := locker.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	var lockedID uuid.UUID
	require.NoError(t, tx.QueryRow(ctx, `SELECT id FROM matches WHERE id = $1 FOR UPDATE`, parsedMatchID).Scan(&lockedID))
	require.Equal(t, parsedMatchID, lockedID)

	var deadline time.Time
	err = tx.QueryRow(ctx, `
		UPDATE matches
		SET created_at = clock_timestamp() - ($2 * interval '1 millisecond') + interval '500 milliseconds'
		WHERE id = $1
		RETURNING created_at + ($2 * interval '1 millisecond')
	`, parsedMatchID, store.MatchAcceptWindow.Milliseconds()).Scan(&deadline)
	require.NoError(t, err)

	type response struct{ err error }
	resultCh := make(chan response, 1)
	callCtx, cancelCall := context.WithTimeout(ctxWithProfile(profileB), 10*time.Second)
	defer cancelCall()
	go func() {
		_, callErr := srv.RespondToMatch(callCtx, &matchmakingv1.RespondToMatchRequest{
			MatchId: matchID,
			Accept:  true,
		})
		resultCh <- response{err: callErr}
	}()

	// Observe that the RPC has reached the row-locking acceptance transaction
	// before letting the shared database deadline pass.
	blocked := false
	blockWaitUntil := time.Now().Add(5 * time.Second)
	for time.Now().Before(blockWaitUntil) {
		var waiting bool
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE pid <> pg_backend_pid()
				  AND wait_event_type = 'Lock' AND state = 'active'
				  AND query LIKE '%FROM matches WHERE id = $1 FOR UPDATE%'
			)
		`).Scan(&waiting)
		require.NoError(t, err)
		if waiting {
			blocked = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.True(t, blocked, "acceptance request must be waiting behind the held match row lock")

	deadlinePassed := false
	deadlineWaitUntil := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadlineWaitUntil) {
		err = tx.QueryRow(ctx, `SELECT clock_timestamp() >= $1`, deadline).Scan(&deadlinePassed)
		require.NoError(t, err)
		if deadlinePassed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, deadlinePassed, "PostgreSQL deadline must pass while acceptance waits on the match lock")
	require.NoError(t, tx.Commit(ctx))

	select {
	case result := <-resultCh:
		require.Equal(t, codes.FailedPrecondition, status.Code(result.err))
	case <-time.After(5 * time.Second):
		t.Fatal("acceptance request did not finish after releasing the match lock")
	}

	match, err := srv.Matches.Get(ctx, parsedMatchID)
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusAbandoned, match.Status)
	proposal, err := srv.Matches.GetProposalForProfile(ctx, parsedMatchID, profileB)
	require.NoError(t, err)
	require.NotEqual(t, store.ProposalResponseAccepted, proposal.Response)
}

func TestMatchDeadline_ConcurrentAcceptDeclineAndSweeperConverge(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, profileA, profileB := seedPendingDuoMatch(t, ctx, srv)
	parsedMatchID := uuid.MustParse(matchID)
	_, err := pool.Exec(ctx, `UPDATE matches SET created_at = now() - ($2 * interval '1 millisecond') WHERE id = $1`, parsedMatchID, store.MatchAcceptWindow.Milliseconds())
	require.NoError(t, err)

	start := make(chan struct{})
	type operationResult struct {
		name string
		err  error
	}
	results := make(chan operationResult, 3)
	go func() {
		<-start
		_, callErr := srv.RespondToMatch(ctxWithProfile(profileA), &matchmakingv1.RespondToMatchRequest{
			MatchId: matchID,
			Accept:  true,
		})
		results <- operationResult{name: "accept", err: callErr}
	}()
	go func() {
		<-start
		_, callErr := srv.RespondToMatch(ctxWithProfile(profileB), &matchmakingv1.RespondToMatchRequest{
			MatchId: matchID,
			Accept:  false,
		})
		results <- operationResult{name: "decline", err: callErr}
	}()
	go func() {
		<-start
		_, _, sweepErr := srv.Matches.ExpirePendingMatchAtDeadline(ctx, parsedMatchID)
		results <- operationResult{name: "sweep", err: sweepErr}
	}()
	close(start)

	for range 3 {
		select {
		case result := <-results:
			switch result.name {
			case "accept":
				require.Equal(t, codes.FailedPrecondition, status.Code(result.err))
			case "decline":
				require.True(t, result.err == nil || status.Code(result.err) == codes.FailedPrecondition)
			case "sweep":
				require.NoError(t, result.err)
			default:
				t.Fatalf("unexpected operation result %q", result.name)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent deadline decisions did not converge")
		}
	}

	match, err := srv.Matches.Get(ctx, parsedMatchID)
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusAbandoned, match.Status)
	for _, profileID := range []uuid.UUID{profileA, profileB} {
		proposal, err := srv.Matches.GetProposalForProfile(ctx, parsedMatchID, profileID)
		require.NoError(t, err)
		require.NotEqual(t, store.ProposalResponseAccepted, proposal.Response)
	}
}

func TestRespondToMatch_AcceptRetryBeforeOtherResponsesIsIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, profileA, _ := seedPendingDuoMatch(t, ctx, srv)
	req := &matchmakingv1.RespondToMatchRequest{MatchId: matchID, Accept: true}

	first, err := srv.RespondToMatch(ctxWithProfile(profileA), req)
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusPendingAccept, first.GetMatch().GetStatus())

	retry, err := srv.RespondToMatch(ctxWithProfile(profileA), req)
	require.NoError(t, err, "retrying an accepted terminal proposal response must preserve its successful outcome")
	require.Equal(t, store.MatchStatusPendingAccept, retry.GetMatch().GetStatus())
	proposal, err := srv.Matches.GetProposalForProfile(ctx, uuid.MustParse(matchID), profileA)
	require.NoError(t, err)
	require.Equal(t, store.ProposalResponseAccepted, proposal.Response)
}

func TestRespondToMatch_AcceptRetryAfterActivationIsIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, profileA, profileB := seedPendingDuoMatch(t, ctx, srv)
	req := &matchmakingv1.RespondToMatchRequest{MatchId: matchID, Accept: true}

	first, err := srv.RespondToMatch(ctxWithProfile(profileA), req)
	require.NoError(t, err)
	_, err = srv.RespondToMatch(ctxWithProfile(profileB), req)
	require.NoError(t, err)

	retry, err := srv.RespondToMatch(ctxWithProfile(profileA), req)
	require.NoError(t, err, "retrying an accepted response after activation must preserve its successful outcome")
	require.Equal(t, store.MatchStatusActive, retry.GetMatch().GetStatus())

	match, err := srv.Matches.Get(ctx, uuid.MustParse(matchID))
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusActive, match.Status)
	session, err := srv.Sessions.Get(ctx, uuid.MustParse(first.GetSearchSession().GetId()))
	require.NoError(t, err)
	require.Equal(t, store.SessionStatusMatched, session.Status)
}

func TestRespondToMatch_DeclineAfterActivationIsRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, profileA, profileB := seedPendingDuoMatch(t, ctx, srv)

	_, err := srv.RespondToMatch(ctxWithProfile(profileA), &matchmakingv1.RespondToMatchRequest{MatchId: matchID, Accept: true})
	require.NoError(t, err)
	_, err = srv.RespondToMatch(ctxWithProfile(profileB), &matchmakingv1.RespondToMatchRequest{MatchId: matchID, Accept: true})
	require.NoError(t, err)

	_, err = srv.RespondToMatch(ctxWithProfile(profileA), &matchmakingv1.RespondToMatchRequest{MatchId: matchID, Accept: false})
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "only an accepted response can be retried after activation")
}

func TestRespondToMatch_ProvisionErrorUnavailableIncludesCause(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, errSquadProvisioner{err: errors.New("chat client unavailable")})
	matchID, profileA, profileB := seedPendingDuoMatch(t, ctx, srv)

	_, err := srv.RespondToMatch(ctxWithProfile(profileA), &matchmakingv1.RespondToMatchRequest{
		MatchId: matchID,
		Accept:  true,
	})
	require.NoError(t, err)

	_, err = srv.RespondToMatch(ctxWithProfile(profileB), &matchmakingv1.RespondToMatchRequest{
		MatchId: matchID,
		Accept:  true,
	})
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Contains(t, err.Error(), "squad provisioning unavailable")
	require.Contains(t, err.Error(), "chat client unavailable")
}

func TestRespondToMatch_DeclineCancelsDeclinerContinuesOtherSolo(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, profileA, profileB := seedPendingDuoMatch(t, ctx, srv)

	resp, err := srv.RespondToMatch(ctxWithProfile(profileA), &matchmakingv1.RespondToMatchRequest{
		MatchId: matchID,
		Accept:  false,
	})
	require.NoError(t, err)
	require.Equal(t, "cancelled", resp.GetSearchSession().GetStatus())
	require.Equal(t, "abandoned", resp.GetMatch().GetStatus())

	declinerSess, err := srv.Sessions.Get(ctx, uuid.MustParse(resp.GetSearchSession().GetId()))
	require.NoError(t, err)
	require.Equal(t, store.SessionStatusCancelled, declinerSess.Status)

	// Other solo continues searching and is re-queued.
	proposals, err := srv.Matches.ListProposals(ctx, uuid.MustParse(matchID))
	require.NoError(t, err)
	var otherSessionID uuid.UUID
	for _, p := range proposals {
		if p.ProfileID == profileB {
			otherSessionID = p.SearchSessionID
			break
		}
	}
	require.NotEqual(t, uuid.Nil, otherSessionID)
	otherSess, err := srv.Sessions.Get(ctx, otherSessionID)
	require.NoError(t, err)
	require.Equal(t, store.SessionStatusSearching, otherSess.Status)
	require.Nil(t, otherSess.MatchID)

	queued, err := srv.Queue.ListSessionIDs(ctx, otherSess.GameID, otherSess.Mode, "eu", 0)
	require.NoError(t, err)
	require.Contains(t, queued, otherSessionID)
	require.NotContains(t, queued, declinerSess.ID)
}

func seedPendingCrossPartyMatch(t *testing.T, ctx context.Context, srv *MatchmakingGRPC) (
	matchID string,
	partyA1, partyA2, partyB1, partyB2 uuid.UUID,
	partyA, partyB uuid.UUID,
) {
	t.Helper()
	game, err := srv.Games.Create(ctx, "Cross-party decline", config.GameConfig{
		Regions: []string{"eu"},
		Modes: []config.Mode{{
			Name:          "2v2",
			Slots:         4,
			PartySizeMin:  1,
			PartySizeMax:  2,
			RolesRequired: false,
			RankRequired:  false,
		}},
	}, uuid.New())
	require.NoError(t, err)

	partyA = uuid.New()
	partyB = uuid.New()
	partyA1 = uuid.New()
	partyA2 = uuid.New()
	partyB1 = uuid.New()
	partyB2 = uuid.New()
	timeout := time.Now().UTC().Add(30 * time.Minute)
	crit := criteria.MustMarshal(criteria.SearchCriteria{Region: "eu"})

	membersA, err := json.Marshal([]string{partyA1.String(), partyA2.String()})
	require.NoError(t, err)
	membersB, err := json.Marshal([]string{partyB1.String(), partyB2.String()})
	require.NoError(t, err)
	_, err = srv.Sessions.Pool.Exec(ctx, `
		INSERT INTO parties (id, leader_profile_id, member_profile_ids, game_id, mode, criteria)
		VALUES
			($1, $2, $3::jsonb, $4, '2v2', $5::jsonb),
			($6, $7, $8::jsonb, $4, '2v2', $5::jsonb)
	`, partyA, partyA1, string(membersA), game.ID, crit,
		partyB, partyB1, string(membersB))
	require.NoError(t, err)

	mkSess := func(profileID, partyID uuid.UUID) store.SearchSession {
		t.Helper()
		pid := partyID
		sess, err := srv.Sessions.Create(ctx, store.CreateSessionParams{
			ProfileID: profileID,
			PartyID:   &pid,
			GameID:    game.ID,
			Mode:      "2v2",
			Criteria:  crit,
			TimeoutAt: timeout,
		})
		require.NoError(t, err)
		return sess
	}

	sessA1 := mkSess(partyA1, partyA)
	sessA2 := mkSess(partyA2, partyA)
	sessB1 := mkSess(partyB1, partyB)
	sessB2 := mkSess(partyB2, partyB)

	result, err := srv.Matches.CreateProposal(ctx, store.CreateProposalParams{
		GameID: game.ID,
		Mode:   "2v2",
		Region: "eu",
		Sessions: []store.ProposalSession{
			{SessionID: sessA1.ID, ProfileID: partyA1, PartyID: &partyA},
			{SessionID: sessA2.ID, ProfileID: partyA2, PartyID: &partyA},
			{SessionID: sessB1.ID, ProfileID: partyB1, PartyID: &partyB},
			{SessionID: sessB2.ID, ProfileID: partyB2, PartyID: &partyB},
		},
	})
	require.NoError(t, err)
	return result.Match.ID.String(), partyA1, partyA2, partyB1, partyB2, partyA, partyB
}

func sessionStatusByProfile(t *testing.T, ctx context.Context, srv *MatchmakingGRPC, matchID string, profileID uuid.UUID) store.SearchSession {
	t.Helper()
	proposals, err := srv.Matches.ListProposals(ctx, uuid.MustParse(matchID))
	require.NoError(t, err)
	for _, p := range proposals {
		if p.ProfileID == profileID {
			sess, err := srv.Sessions.Get(ctx, p.SearchSessionID)
			require.NoError(t, err)
			return sess
		}
	}
	t.Fatalf("no proposal for profile %s", profileID)
	return store.SearchSession{}
}

func TestRespondToMatch_ForeignPartyDecline_AcceptorsContinue(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, a1, a2, b1, b2, _, _ := seedPendingCrossPartyMatch(t, ctx, srv)

	// Party A fully accepts first.
	_, err := srv.RespondToMatch(ctxWithProfile(a1), &matchmakingv1.RespondToMatchRequest{MatchId: matchID, Accept: true})
	require.NoError(t, err)
	_, err = srv.RespondToMatch(ctxWithProfile(a2), &matchmakingv1.RespondToMatchRequest{MatchId: matchID, Accept: true})
	require.NoError(t, err)

	// Foreign party B declines.
	resp, err := srv.RespondToMatch(ctxWithProfile(b1), &matchmakingv1.RespondToMatchRequest{MatchId: matchID, Accept: false})
	require.NoError(t, err)
	require.Equal(t, "cancelled", resp.GetSearchSession().GetStatus())
	require.Equal(t, "abandoned", resp.GetMatch().GetStatus())

	require.Equal(t, store.SessionStatusSearching, sessionStatusByProfile(t, ctx, srv, matchID, a1).Status)
	require.Equal(t, store.SessionStatusSearching, sessionStatusByProfile(t, ctx, srv, matchID, a2).Status)
	require.Equal(t, store.SessionStatusCancelled, sessionStatusByProfile(t, ctx, srv, matchID, b1).Status)
	require.Equal(t, store.SessionStatusCancelled, sessionStatusByProfile(t, ctx, srv, matchID, b2).Status)

	a1Sess := sessionStatusByProfile(t, ctx, srv, matchID, a1)
	queued, err := srv.Queue.ListSessionIDs(ctx, a1Sess.GameID, a1Sess.Mode, "eu", 0)
	require.NoError(t, err)
	require.Contains(t, queued, a1Sess.ID)
	require.Contains(t, queued, sessionStatusByProfile(t, ctx, srv, matchID, a2).ID)
	require.NotContains(t, queued, sessionStatusByProfile(t, ctx, srv, matchID, b1).ID)
	require.NotContains(t, queued, sessionStatusByProfile(t, ctx, srv, matchID, b2).ID)
}

func TestRespondToMatch_OwnPartyDecline_ResetsWholeParty(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, a1, a2, b1, b2, _, _ := seedPendingCrossPartyMatch(t, ctx, srv)

	// One member of party A declines → whole party A cancelled; party B continues.
	resp, err := srv.RespondToMatch(ctxWithProfile(a1), &matchmakingv1.RespondToMatchRequest{MatchId: matchID, Accept: false})
	require.NoError(t, err)
	require.Equal(t, "cancelled", resp.GetSearchSession().GetStatus())
	require.Equal(t, "abandoned", resp.GetMatch().GetStatus())

	require.Equal(t, store.SessionStatusCancelled, sessionStatusByProfile(t, ctx, srv, matchID, a1).Status)
	require.Equal(t, store.SessionStatusCancelled, sessionStatusByProfile(t, ctx, srv, matchID, a2).Status)
	require.Equal(t, store.SessionStatusSearching, sessionStatusByProfile(t, ctx, srv, matchID, b1).Status)
	require.Equal(t, store.SessionStatusSearching, sessionStatusByProfile(t, ctx, srv, matchID, b2).Status)

	b1Sess := sessionStatusByProfile(t, ctx, srv, matchID, b1)
	queued, err := srv.Queue.ListSessionIDs(ctx, b1Sess.GameID, b1Sess.Mode, "eu", 0)
	require.NoError(t, err)
	require.Contains(t, queued, b1Sess.ID)
	require.Contains(t, queued, sessionStatusByProfile(t, ctx, srv, matchID, b2).ID)
	require.NotContains(t, queued, sessionStatusByProfile(t, ctx, srv, matchID, a1).ID)
	require.NotContains(t, queued, sessionStatusByProfile(t, ctx, srv, matchID, a2).ID)
}

func TestGetMatch_NotParticipantDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, _, _ := seedPendingDuoMatch(t, ctx, srv)

	_, err := srv.GetMatch(ctxWithProfile(uuid.New()), &matchmakingv1.GetMatchRequest{MatchId: matchID})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
