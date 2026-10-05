package matchsquad

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/gameprovision"
	"voice/backend/voice/internal/matchsquadprincipal"
	"voice/backend/voice/internal/store"
)

// These tests use the repository's disposable PostgreSQL fixture and apply the
// real Voice migration chain. Only Redis, LiveKit, Auth-current and profile
// lookup boundaries are replaced; all membership, operation, grant, effect,
// receipt, and account-fence assertions observe committed PostgreSQL state.
func TestPostgresMatchSquadMember_JoinGrantAndLostReplyReplay(t *testing.T) {
	f := newMatchSquadPostgresFixture(t)
	member := f.memberService(map[uuid.UUID]uuid.UUID{f.firstProfile: uuid.New()})
	account := member.Profiles.(testProfileAccounts).accounts[f.firstProfile]

	join := f.joinRequest()
	joinCtx := verifiedMemberContext(t, join, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, join.GetOperationId(), account, f.firstProfile)
	first, err := member.Join(joinCtx, join)
	require.NoError(t, err)
	require.Equal(t, callsv1.MatchSquadMembershipState_MATCH_SQUAD_MEMBERSHIP_STATE_JOINED, first.GetMembershipState())

	var operationState, membershipState, effectState string
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT o.state,m.membership_state,e.state
FROM voice_match_squad_member_operations o
JOIN voice_room_memberships m ON m.profile_id=o.profile_id AND m.media_epoch=o.media_epoch
JOIN voice_match_squad_member_effects e ON e.operation_id=o.operation_id AND e.effect_kind='join_projection'
WHERE o.operation_id=$1`, uuid.MustParse(join.GetOperationId())).Scan(&operationState, &membershipState, &effectState))
	require.Equal(t, "complete", operationState)
	require.Equal(t, "JOINED", membershipState)
	require.Equal(t, "confirmed", effectState)

	// Simulate a committed Join whose reply was lost, then recover through a new
	// controller instance. The response bytes and media generation are immutable.
	restarted := f.memberService(map[uuid.UUID]uuid.UUID{f.firstProfile: account})
	replayed, err := restarted.Join(joinCtx, proto.Clone(join).(*callsv1.JoinMatchSquadRoomRequest))
	require.NoError(t, err)
	require.True(t, proto.Equal(first, replayed))
	var operations, memberships int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_match_squad_member_operations WHERE profile_id=$1 AND method='join'`, f.firstProfile).Scan(&operations))
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_room_memberships WHERE profile_id=$1 AND room_id=$2`, f.firstProfile, f.roomID).Scan(&memberships))
	require.Equal(t, 1, operations)
	require.Equal(t, 1, memberships)

	requestID := uuid.New()
	tokenRequest := &callsv1.GetMatchSquadJoinTokenRequest{ProtocolVersion: 1, MatchId: f.matchID.String(), RoomId: f.roomID.String(), MediaEpoch: first.GetMediaEpoch()}
	tokenCtx := verifiedMemberContext(t, tokenRequest, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, requestID.String(), account, f.firstProfile)
	token, err := member.GetJoinToken(tokenCtx, tokenRequest)
	require.NoError(t, err)
	require.NotEmpty(t, token.GetToken().GetJwt())
	var persistedExpiry, grantExpiry time.Time
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT m.latest_grant_expires_at,g.expires_at
FROM voice_room_memberships m JOIN voice_match_squad_member_grants g
ON g.profile_id=m.profile_id AND g.media_epoch=m.media_epoch
WHERE m.profile_id=$1 AND g.request_id=$2`, f.firstProfile, requestID).Scan(&persistedExpiry, &grantExpiry))
	require.Equal(t, grantExpiry.UTC(), persistedExpiry.UTC(), "grant expiry is committed before the token response is returned")
	require.True(t, persistedExpiry.After(time.Now().UTC()))
	var activeFence int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_account_voice_fences WHERE account_id=$1 AND profile_id=$2 AND room_id=$3 AND state='active'`, account, f.firstProfile, f.roomID.String()).Scan(&activeFence))
	require.Equal(t, 1, activeFence)
}

func TestPostgresMatchSquadMember_LeaveRepairWaitsForDatabaseGrantExpiry(t *testing.T) {
	f := newMatchSquadPostgresFixture(t)
	account := uuid.New()
	member := f.memberService(map[uuid.UUID]uuid.UUID{f.firstProfile: account})
	join := f.joinRequest()
	joined, err := member.Join(verifiedMemberContext(t, join, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, join.GetOperationId(), account, f.firstProfile), join)
	require.NoError(t, err)

	tokenRequestID := uuid.New()
	tokenRequest := &callsv1.GetMatchSquadJoinTokenRequest{ProtocolVersion: 1, MatchId: f.matchID.String(), RoomId: f.roomID.String(), MediaEpoch: joined.GetMediaEpoch()}
	tokenCtx := verifiedMemberContext(t, tokenRequest, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, tokenRequestID.String(), account, f.firstProfile)
	_, err = member.GetJoinToken(tokenCtx, tokenRequest)
	require.NoError(t, err)

	f.effects.failNextRemoval()
	leave := f.leaveRequest(joined.GetMediaEpoch())
	leaveCtx := verifiedMemberContext(t, leave, callsv1.MatchSquadMemberService_LeaveMatchSquadRoom_FullMethodName, leave.GetOperationId(), account, f.firstProfile)
	_, err = member.Leave(leaveCtx, leave)
	require.Equal(t, codes.Unavailable, status.Code(err), "failed LiveKit removal leaves durable cleanup pending")
	assertMatchSquadMemberState(t, f, f.firstProfile, "LEAVING", "pending")
	assertMatchSquadFence(t, f, account, f.firstProfile, true)
	require.Error(t, member.ReadyForTeardown(f.ctx, f.matchID, f.roomID), "unconfirmed member effects block aggregate teardown")

	// Repair only confirms the exact pending effects and records a LEAVING
	// receipt; a still-valid bearer keeps the member state and account fence.
	require.NoError(t, member.RepairPendingLeaves(f.ctx, 16))
	assertMatchSquadMemberState(t, f, f.firstProfile, "LEAVING", "confirmed")
	require.NoError(t, member.RepairExpiredLeaves(f.ctx, 16))
	assertMatchSquadMemberState(t, f, f.firstProfile, "LEAVING", "confirmed")
	assertMatchSquadFence(t, f, account, f.firstProfile, true)

	requireDatabaseGrantExpiry(t, f, f.firstProfile, tokenRequestID)
	require.NoError(t, member.RepairExpiredLeaves(f.ctx, 16))
	assertMatchSquadMemberState(t, f, f.firstProfile, "LEFT", "confirmed")
	assertMatchSquadFence(t, f, account, f.firstProfile, false)

	// LEFT is an actor-level terminal state; it does not tear down the shared
	// room or erase the other manifest participant's eligibility.
	call, err := f.calls.GetCall(f.ctx, f.roomID.String())
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ACTIVE, call.Status)
	require.Empty(t, call.States)
	leaveReceipt, err := member.Leave(leaveCtx, leave)
	require.NoError(t, err)
	require.Equal(t, callsv1.MatchSquadMembershipState_MATCH_SQUAD_MEMBERSHIP_STATE_LEAVING, leaveReceipt.GetMembershipState(), "lost reply replays the exact committed leave receipt")
}

func TestPostgresMatchSquadTeardown_WaitsForGrantAndOrdersProjectionBeforeClose(t *testing.T) {
	f := newMatchSquadPostgresFixture(t)
	account := uuid.New()
	member := f.memberService(map[uuid.UUID]uuid.UUID{f.firstProfile: account})
	f.service.Members = member
	join := f.joinRequest()
	joined, err := member.Join(verifiedMemberContext(t, join, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, join.GetOperationId(), account, f.firstProfile), join)
	require.NoError(t, err)

	tokenRequestID := uuid.New()
	tokenRequest := &callsv1.GetMatchSquadJoinTokenRequest{ProtocolVersion: 1, MatchId: f.matchID.String(), RoomId: f.roomID.String(), MediaEpoch: joined.GetMediaEpoch()}
	tokenCtx := verifiedMemberContext(t, tokenRequest, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, tokenRequestID.String(), account, f.firstProfile)
	_, err = member.GetJoinToken(tokenCtx, tokenRequest)
	require.NoError(t, err)

	teardown := f.teardownRequest()
	teardownCtx := verifiedServiceContext(t, teardown, matchsquadprincipal.TeardownMethod, teardown.GetTeardownOperationId())
	_, err = f.service.Teardown(teardownCtx, teardown)
	require.Equal(t, codes.Unavailable, status.Code(err), "the durable close may begin, but a valid bearer prevents projection/media closure")
	require.Equal(t, 0, f.effects.closeCalls())
	call, err := f.calls.GetCall(f.ctx, f.roomID.String())
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ACTIVE, call.Status)

	requireDatabaseGrantExpiry(t, f, f.firstProfile, tokenRequestID)
	f.effects.requireProjectionDrainedBeforeClose = true
	receipt, err := f.service.Teardown(teardownCtx, teardown)
	require.NoError(t, err)
	teardownReceipt := new(callsv1.MatchSquadRoomTeardownReceipt)
	require.NoError(t, proto.Unmarshal(receipt, teardownReceipt))
	require.Equal(t, callsv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED, teardownReceipt.GetStatus())
	require.Equal(t, 1, f.effects.closeCalls())
	var resourceState, operationState string
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT r.state,o.state FROM voice_room_instances r JOIN voice_match_squad_operations o USING(room_id) WHERE r.room_id=$1`, f.roomID).Scan(&resourceState, &operationState))
	require.Equal(t, "closed", resourceState)
	require.Equal(t, "closed", operationState)
	assertMatchSquadMemberState(t, f, f.firstProfile, "EJECTED", "confirmed")
	assertMatchSquadFence(t, f, account, f.firstProfile, false)

	replayed, err := f.service.Teardown(teardownCtx, proto.Clone(teardown).(*callsv1.TeardownMatchSquadRoomRequest))
	require.NoError(t, err, "a lost teardown reply recovers the durable receipt")
	require.Equal(t, receipt, replayed)
	require.Equal(t, 1, f.effects.closeCalls(), "receipt replay does not re-run external close")
}

func TestPostgresMatchSquadMember_ConcurrentProfilesCannotShareAccountFence(t *testing.T) {
	f := newMatchSquadPostgresFixture(t)
	account := uuid.New()
	accounts := map[uuid.UUID]uuid.UUID{f.firstProfile: account, f.secondProfile: account}
	memberA := f.memberService(accounts)
	memberB := f.memberService(accounts)
	requests := []*callsv1.JoinMatchSquadRoomRequest{f.joinRequest(), f.joinRequest()}
	profiles := []uuid.UUID{f.firstProfile, f.secondProfile}
	services := []*MatchSquadMemberService{memberA, memberB}
	contexts := make([]context.Context, len(requests))
	for i, req := range requests {
		contexts[i] = verifiedMemberContext(t, req, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, req.GetOperationId(), account, profiles[i])
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := range requests {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := services[i].Join(contexts[i], requests[i])
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	var successes, conflicts int
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		if status.Code(err) == codes.FailedPrecondition {
			conflicts++
			continue
		}
		require.NoError(t, err)
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	var joined, activeFences int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_room_memberships WHERE room_id=$1 AND membership_state='JOINED'`, f.roomID).Scan(&joined))
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_account_voice_fences WHERE account_id=$1 AND state='active'`, account).Scan(&activeFences))
	require.Equal(t, 1, joined)
	require.Equal(t, 1, activeFences)
}

type matchSquadPostgresFixture struct {
	t               *testing.T
	ctx             context.Context
	pool            *pgxpool.Pool
	calls           *store.MemoryCallStore
	effects         *testMatchSquadEffects
	service         *Service
	matchID, roomID uuid.UUID
	firstProfile    uuid.UUID
	secondProfile   uuid.UUID
	creationReceipt *callsv1.MatchSquadRoomReceipt
	creationRequest *callsv1.CreateMatchSquadRoomRequest
}

func newMatchSquadPostgresFixture(t *testing.T) *matchSquadPostgresFixture {
	t.Helper()
	ctx := context.Background()
	pool := startMatchSquadPostgres(t, ctx)
	calls := store.NewMemoryCallStore()
	effects := &testMatchSquadEffects{calls: calls}
	service := &Service{Pool: pool, Calls: calls, Effects: effects}
	request := validCreateRequest()
	createCtx := verifiedServiceContext(t, request, matchsquadprincipal.CreateMethod, request.GetOperationId())
	receiptBytes, err := service.Create(createCtx, request)
	require.NoError(t, err)
	receipt := new(callsv1.MatchSquadRoomReceipt)
	require.NoError(t, proto.Unmarshal(receiptBytes, receipt))
	first, err := uuid.Parse(request.GetParticipants()[0].GetProfileId())
	require.NoError(t, err)
	second, err := uuid.Parse(request.GetParticipants()[1].GetProfileId())
	require.NoError(t, err)
	return &matchSquadPostgresFixture{t: t, ctx: ctx, pool: pool, calls: calls, effects: effects, service: service,
		matchID: uuid.MustParse(receipt.GetMatchId()), roomID: uuid.MustParse(receipt.GetRoomId()), firstProfile: first, secondProfile: second,
		creationReceipt: receipt, creationRequest: request}
}

func startMatchSquadPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	integrationtest.ConfigureDockerTesting()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	migrations := filepath.Join(root, "src", "backend", "migrations", "voice_db")
	pool := integrationtest.StartPostgres(t, ctx, "voice_match_squad", filepath.Join(migrations, "000001_room_lifecycle.up.sql"))
	for _, name := range []string{
		"000002_redis_divergence", "000003_matchmaking_membership", "000004_game_session_rooms",
		"000005_game_session_close", "000006_game_session_roster_lease", "000007_t17_sdk_conversion_fence",
		"000008_account_voice_fence", "000009_space_lifecycle", "000010_match_squad_operations",
	} {
		body, err := os.ReadFile(filepath.Join(migrations, name+".up.sql"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err)
	}
	return pool
}

func (f *matchSquadPostgresFixture) memberService(accounts map[uuid.UUID]uuid.UUID) *MatchSquadMemberService {
	return &MatchSquadMemberService{
		Pool: f.pool, Calls: f.calls, Fences: gameprovision.NewPostgresAccountVoiceFenceStore(f.pool),
		Profiles: testProfileAccounts{accounts: accounts}, Principal: testCurrentMember{}, Tokens: &testMemberTokens{},
		RoomEffect: f.effects,
	}
}

func (f *matchSquadPostgresFixture) joinRequest() *callsv1.JoinMatchSquadRoomRequest {
	return &callsv1.JoinMatchSquadRoomRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), MatchId: f.matchID.String(), RoomId: f.roomID.String()}
}

func (f *matchSquadPostgresFixture) leaveRequest(epoch string) *callsv1.LeaveMatchSquadRoomRequest {
	return &callsv1.LeaveMatchSquadRoomRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), MatchId: f.matchID.String(), RoomId: f.roomID.String(), ExpectedMediaEpoch: epoch}
}

func (f *matchSquadPostgresFixture) teardownRequest() *callsv1.TeardownMatchSquadRoomRequest {
	return &callsv1.TeardownMatchSquadRoomRequest{
		ProtocolVersion: 1, TeardownOperationId: uuid.NewString(), MatchId: f.matchID.String(), RoomId: f.roomID.String(),
		CreationReceiptId: f.creationReceipt.GetReceiptId(), ParticipantManifestSha256: f.creationReceipt.GetParticipantManifestSha256(),
		CreationRequestSha256: f.creationReceipt.GetRequestSha256(),
	}
}

func verifiedServiceContext(t *testing.T, request proto.Message, method, operation string) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "matchmaking", Subject: "service:matchmaking", Audience: "voice",
		RPC: method, RequestID: operation, RequestHash: hash,
	})
}

func verifiedMemberContext(t *testing.T, request proto.Message, method, requestID string, account, profile uuid.UUID) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer hosted-test-credential", "x-request-id", requestID))
	return principal.WithVerified(ctx, principal.Principal{
		Kind: "delegated_user", Issuer: "gateway", Subject: account.String(), AccountID: account.String(), ProfileID: profile.String(),
		Audience: "voice", RPC: method, RequestID: requestID, RequestHash: hash, SessionEpoch: 7, ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
}

func assertMatchSquadMemberState(t *testing.T, f *matchSquadPostgresFixture, profile uuid.UUID, wantState, wantEffect string) {
	t.Helper()
	var state, effect string
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT m.membership_state,COALESCE((SELECT string_agg(e.state,',' ORDER BY e.effect_kind) FROM voice_match_squad_member_effects e WHERE e.profile_id=m.profile_id AND e.media_epoch=m.media_epoch),'') FROM voice_room_memberships m WHERE m.profile_id=$1 AND m.room_id=$2`, profile, f.roomID).Scan(&state, &effect))
	require.Equal(t, wantState, state)
	if wantEffect == "pending" {
		require.Contains(t, effect, "pending")
	} else if wantEffect == "confirmed" {
		require.NotContains(t, effect, "pending")
	}
}

func assertMatchSquadFence(t *testing.T, f *matchSquadPostgresFixture, account, profile uuid.UUID, active bool) {
	t.Helper()
	var count int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_account_voice_fences WHERE account_id=$1 AND profile_id=$2 AND room_id=$3 AND state='active'`, account, profile, f.roomID.String()).Scan(&count))
	require.Equal(t, active, count == 1)
}

type testProfileAccounts struct{ accounts map[uuid.UUID]uuid.UUID }

func (r testProfileAccounts) AccountIDByProfileID(_ context.Context, profile uuid.UUID) (uuid.UUID, error) {
	account, ok := r.accounts[profile]
	if !ok {
		return uuid.Nil, errors.New("missing hosted test profile mapping")
	}
	return account, nil
}

type testCurrentMember struct{}

func (testCurrentMember) CheckCurrent(_ context.Context, actor principal.Principal, now time.Time) error {
	if !actor.ExpiresAt.After(now) {
		return errors.New("hosted test actor expired")
	}
	return nil
}

type testMemberTokens struct{}

func (*testMemberTokens) MatchSquadJoinToken(_ string, _ string, _ *bool, now, actorExpiry time.Time) (string, time.Time, error) {
	expires := now.Add(45 * time.Second).Truncate(time.Second)
	if actorExpiry.Before(expires) {
		expires = actorExpiry.Truncate(time.Second)
	}
	if !expires.After(now) {
		return "", time.Time{}, errors.New("hosted test actor expires too soon")
	}
	return "hosted-test-jwt", expires, nil
}

func (*testMemberTokens) LivekitURL() string { return "wss://hosted-test.invalid" }

type testMatchSquadEffects struct {
	mu                                  sync.Mutex
	calls                               *store.MemoryCallStore
	removeFailures                      int
	closeCount                          int
	requireProjectionDrainedBeforeClose bool
}

func (e *testMatchSquadEffects) EnsureRoom(context.Context, string) error { return nil }

func (e *testMatchSquadEffects) CloseRoom(ctx context.Context, room string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	call, err := e.calls.GetCall(ctx, strings.TrimPrefix(room, "match-squad-"))
	if err != nil {
		return err
	}
	if e.requireProjectionDrainedBeforeClose && (call.Status != callsv1.CallStatus_CALL_STATUS_ENDED || len(call.States) != 0) {
		return errors.New("MatchSquad Redis projection was not drained and ended before LiveKit close")
	}
	e.closeCount++
	return nil
}

func (e *testMatchSquadEffects) RemoveParticipant(context.Context, string, string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.removeFailures > 0 {
		e.removeFailures--
		return errors.New("injected hosted LiveKit participant removal failure")
	}
	return nil
}

func (e *testMatchSquadEffects) failNextRemoval() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.removeFailures++
}

func (e *testMatchSquadEffects) closeCalls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.closeCount
}

func requireDatabaseGrantExpiry(t *testing.T, f *matchSquadPostgresFixture, profileID, requestID uuid.UUID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.ctx, 70*time.Second)
	defer cancel()
	var grantExpiry, latestExpiry time.Time
	err := f.pool.QueryRow(ctx, `SELECT g.expires_at,m.latest_grant_expires_at
FROM voice_match_squad_member_grants g
JOIN voice_room_memberships m ON m.profile_id=g.profile_id AND m.media_epoch=g.media_epoch
WHERE g.request_id=$1 AND g.profile_id=$2 AND m.room_id=$3`, requestID, profileID, f.roomID).Scan(&grantExpiry, &latestExpiry)
	require.NoError(t, err, "read the durable grant and monotonic membership expiry")
	require.True(t, grantExpiry.Equal(latestExpiry), "the issued grant expiry must be the current durable generation bound")

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		var expired bool
		err := f.pool.QueryRow(ctx, `SELECT latest_grant_expires_at=$3 AND latest_grant_expires_at<=clock_timestamp()
FROM voice_room_memberships WHERE profile_id=$1 AND room_id=$2`, profileID, f.roomID, latestExpiry).Scan(&expired)
		require.NoError(t, err, "check the durable grant against PostgreSQL clock")
		if expired {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("PostgreSQL did not reach the recorded MatchSquad grant expiry within 70 seconds: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}
