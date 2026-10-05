package matchsquad

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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
	"voice/backend/voice/internal/livekit"
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
	assertMatchSquadTerminalGeneration(t, f, f.firstProfile, account, joined.GetMediaEpoch(), tokenRequestID, "LEFT")
	assertMatchSquadFence(t, f, account, f.firstProfile, false)
	deniedTokenID := uuid.New()
	deniedToken := &callsv1.GetMatchSquadJoinTokenRequest{ProtocolVersion: 1, MatchId: f.matchID.String(), RoomId: f.roomID.String(), MediaEpoch: joined.GetMediaEpoch()}
	_, err = member.GetJoinToken(verifiedMemberContext(t, deniedToken, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, deniedTokenID.String(), account, f.firstProfile), deniedToken)
	require.Error(t, err, "terminal LEFT membership must not receive another token")
	var grants int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_match_squad_member_grants WHERE profile_id=$1 AND media_epoch=$2`, f.firstProfile, uuid.MustParse(joined.GetMediaEpoch())).Scan(&grants))
	require.Equal(t, 1, grants, "a denied terminal token request does not persist a bearer grant")

	// LEFT is an actor-level terminal state; it does not tear down the shared
	// room or erase the other manifest participant's eligibility.
	call, err := f.calls.GetCall(f.ctx, f.roomID.String())
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ACTIVE, call.Status)
	require.Empty(t, call.States)
	historicalJoin, err := member.Join(verifiedMemberContext(t, join, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, join.GetOperationId(), account, f.firstProfile), proto.Clone(join).(*callsv1.JoinMatchSquadRoomRequest))
	require.NoError(t, err, "the original Join operation may replay its immutable receipt after LEFT")
	require.True(t, proto.Equal(joined, historicalJoin))
	call, err = f.calls.GetCall(f.ctx, f.roomID.String())
	require.NoError(t, err)
	require.Empty(t, call.States, "historical Join replay cannot re-add the terminal member projection")
	leaveReceipt, err := member.Leave(leaveCtx, leave)
	require.NoError(t, err)
	require.Equal(t, callsv1.MatchSquadMembershipState_MATCH_SQUAD_MEMBERSHIP_STATE_LEAVING, leaveReceipt.GetMembershipState(), "lost reply replays the exact committed leave receipt")
}

func TestPostgresMatchSquadMember_ImmediateLeaveWithoutGrantPreservesGeneration(t *testing.T) {
	f := newMatchSquadPostgresFixture(t)
	account := uuid.New()
	member := f.memberService(map[uuid.UUID]uuid.UUID{f.firstProfile: account})
	join := f.joinRequest()
	joined, err := member.Join(verifiedMemberContext(t, join, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, join.GetOperationId(), account, f.firstProfile), join)
	require.NoError(t, err)

	leave := f.leaveRequest(joined.GetMediaEpoch())
	left, err := member.Leave(verifiedMemberContext(t, leave, callsv1.MatchSquadMemberService_LeaveMatchSquadRoom_FullMethodName, leave.GetOperationId(), account, f.firstProfile), leave)
	require.NoError(t, err)
	require.Equal(t, callsv1.MatchSquadMembershipState_MATCH_SQUAD_MEMBERSHIP_STATE_LEFT, left.GetMembershipState())
	assertMatchSquadMemberState(t, f, f.firstProfile, "LEFT", "confirmed")
	assertMatchSquadUnissuedTerminalGeneration(t, f, f.firstProfile, account, joined.GetMediaEpoch(), "LEFT")
	assertMatchSquadFence(t, f, account, f.firstProfile, false)
	oldTokenID := uuid.New()
	oldToken := &callsv1.GetMatchSquadJoinTokenRequest{ProtocolVersion: 1, MatchId: f.matchID.String(), RoomId: f.roomID.String(), MediaEpoch: joined.GetMediaEpoch()}
	_, err = member.GetJoinToken(verifiedMemberContext(t, oldToken, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, oldTokenID.String(), account, f.firstProfile), oldToken)
	require.Error(t, err, "the LEFT media epoch cannot issue a token")

	newJoin := f.joinRequest()
	rejoined, err := member.Join(verifiedMemberContext(t, newJoin, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, newJoin.GetOperationId(), account, f.firstProfile), newJoin)
	require.NoError(t, err, "a drained LEFT generation may rejoin the still-active owned resource")
	require.NotEqual(t, joined.GetMediaEpoch(), rejoined.GetMediaEpoch())
	require.Equal(t, callsv1.MatchSquadMembershipState_MATCH_SQUAD_MEMBERSHIP_STATE_JOINED, rejoined.GetMembershipState())
	assertMatchSquadFence(t, f, account, f.firstProfile, true)
	staleEpochTokenID := uuid.New()
	_, err = member.GetJoinToken(verifiedMemberContext(t, oldToken, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, staleEpochTokenID.String(), account, f.firstProfile), oldToken)
	require.Error(t, err, "the prior media epoch cannot receive a token after a new epoch joins")
	var staleGrants int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_match_squad_member_grants WHERE profile_id=$1 AND media_epoch=$2`, f.firstProfile, uuid.MustParse(joined.GetMediaEpoch())).Scan(&staleGrants))
	require.Zero(t, staleGrants)
}

func TestPostgresMatchSquadMember_PendingLeaveRepairWithoutGrantPreservesGeneration(t *testing.T) {
	f := newMatchSquadPostgresFixture(t)
	account := uuid.New()
	member := f.memberService(map[uuid.UUID]uuid.UUID{f.firstProfile: account})
	join := f.joinRequest()
	joined, err := member.Join(verifiedMemberContext(t, join, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, join.GetOperationId(), account, f.firstProfile), join)
	require.NoError(t, err)
	leave := f.leaveRequest(joined.GetMediaEpoch())
	leaveCtx := verifiedMemberContext(t, leave, callsv1.MatchSquadMemberService_LeaveMatchSquadRoom_FullMethodName, leave.GetOperationId(), account, f.firstProfile)
	f.effects.failNextRemoval()
	_, err = member.Leave(leaveCtx, leave)
	require.Equal(t, codes.Unavailable, status.Code(err))
	assertMatchSquadMemberState(t, f, f.firstProfile, "LEAVING", "pending")
	require.NoError(t, member.RepairPendingLeaves(f.ctx, 16))
	assertMatchSquadMemberState(t, f, f.firstProfile, "LEFT", "confirmed")
	assertMatchSquadUnissuedTerminalGeneration(t, f, f.firstProfile, account, joined.GetMediaEpoch(), "LEFT")
	assertMatchSquadFence(t, f, account, f.firstProfile, false)
}

func TestPostgresMatchSquadMember_JoinRollbackAndRepairPreserveGeneration(t *testing.T) {
	for _, tc := range []struct {
		name         string
		failRemoval  bool
		wantJoinCode codes.Code
	}{
		{name: "abort join", wantJoinCode: codes.Unauthenticated},
		{name: "repair pending join", failRemoval: true, wantJoinCode: codes.Unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newMatchSquadPostgresFixture(t)
			account := uuid.New()
			member := f.memberService(map[uuid.UUID]uuid.UUID{f.firstProfile: account})
			member.Principal = &failCurrentMemberCheck{failAt: 5}
			if tc.failRemoval {
				f.effects.failNextRemoval()
			}
			join := f.joinRequest()
			_, err := member.Join(verifiedMemberContext(t, join, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, join.GetOperationId(), account, f.firstProfile), join)
			require.Equal(t, tc.wantJoinCode, status.Code(err))
			if tc.failRemoval {
				assertMatchSquadMemberState(t, f, f.firstProfile, "JOINING", "pending")
				require.NoError(t, member.RepairPendingJoins(f.ctx, 16))
			}
			assertMatchSquadMemberState(t, f, f.firstProfile, "EJECTED", "confirmed")
			assertMatchSquadUnissuedTerminalGeneration(t, f, f.firstProfile, account, f.memberEpoch(t, f.firstProfile), "EJECTED")
			assertMatchSquadFence(t, f, account, f.firstProfile, false)
		})
	}
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
	assertMatchSquadTerminalGeneration(t, f, f.firstProfile, account, joined.GetMediaEpoch(), tokenRequestID, "EJECTED")
	assertMatchSquadFence(t, f, account, f.firstProfile, false)
	deniedTokenID := uuid.New()
	deniedToken := &callsv1.GetMatchSquadJoinTokenRequest{ProtocolVersion: 1, MatchId: f.matchID.String(), RoomId: f.roomID.String(), MediaEpoch: joined.GetMediaEpoch()}
	_, err = member.GetJoinToken(verifiedMemberContext(t, deniedToken, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, deniedTokenID.String(), account, f.firstProfile), deniedToken)
	require.Error(t, err, "terminal EJECTED membership must not receive another token")
	var grants int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_match_squad_member_grants WHERE profile_id=$1 AND media_epoch=$2`, f.firstProfile, uuid.MustParse(joined.GetMediaEpoch())).Scan(&grants))
	require.Equal(t, 1, grants, "a denied terminal token request does not persist a bearer grant")

	replayed, err := f.service.Teardown(teardownCtx, proto.Clone(teardown).(*callsv1.TeardownMatchSquadRoomRequest))
	require.NoError(t, err, "a lost teardown reply recovers the durable receipt")
	require.Equal(t, receipt, replayed)
	require.Equal(t, 1, f.effects.closeCalls(), "receipt replay does not re-run external close")
}

func TestPostgresMatchSquadMember_ClosingBeforeReservationDoesNotGrant(t *testing.T) {
	f := newMatchSquadPostgresFixture(t)
	account := uuid.New()
	member := f.memberService(map[uuid.UUID]uuid.UUID{f.firstProfile: account})
	f.service.Members = member
	join := f.joinRequest()
	joined, err := member.Join(verifiedMemberContext(t, join, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, join.GetOperationId(), account, f.firstProfile), join)
	require.NoError(t, err)

	f.effects.closeEntered = make(chan struct{})
	f.effects.closeRelease = make(chan struct{})
	defer f.effects.unblockClose()
	teardown := f.teardownRequest()
	teardownCtx := verifiedServiceContext(t, teardown, matchsquadprincipal.TeardownMethod, teardown.GetTeardownOperationId())
	type teardownResult struct {
		receipt []byte
		err     error
	}
	teardownDone := make(chan teardownResult, 1)
	go func() {
		receipt, teardownErr := f.service.Teardown(teardownCtx, teardown)
		teardownDone <- teardownResult{receipt: receipt, err: teardownErr}
	}()
	select {
	case <-f.effects.closeEntered:
	case <-time.After(10 * time.Second):
		t.Fatal("teardown did not reach the blocked LiveKit close after committing closing")
	}
	requireMatchSquadResourceState(t, f, "closing")

	requestID := uuid.New()
	tokenRequest := &callsv1.GetMatchSquadJoinTokenRequest{ProtocolVersion: 1, MatchId: f.matchID.String(), RoomId: f.roomID.String(), MediaEpoch: joined.GetMediaEpoch()}
	_, err = member.GetJoinToken(verifiedMemberContext(t, tokenRequest, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, requestID.String(), account, f.firstProfile), tokenRequest)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "closing denies before creating a grant reservation")
	var grants int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_match_squad_member_grants WHERE request_id=$1`, requestID).Scan(&grants))
	require.Zero(t, grants)

	f.effects.unblockClose()
	select {
	case result := <-teardownDone:
		require.NoError(t, result.err)
		require.NotEmpty(t, result.receipt)
	case <-time.After(10 * time.Second):
		t.Fatal("teardown did not finish after LiveKit close resumed")
	}
}
func TestPostgresMatchSquadMember_JWTMatchesDurableReservation(t *testing.T) {
	f := newMatchSquadPostgresFixture(t)
	account := uuid.New()
	member := f.memberService(map[uuid.UUID]uuid.UUID{f.firstProfile: account})
	member.Tokens = livekit.NewHS256TokenIssuer("hosted-test-key", "synthetic-test-secret", "wss://hosted-test.invalid", time.Hour)
	join := f.joinRequest()
	joined, err := member.Join(verifiedMemberContext(t, join, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, join.GetOperationId(), account, f.firstProfile), join)
	require.NoError(t, err)

	requestID := uuid.New()
	tokenRequest := &callsv1.GetMatchSquadJoinTokenRequest{ProtocolVersion: 1, MatchId: f.matchID.String(), RoomId: f.roomID.String(), MediaEpoch: joined.GetMediaEpoch()}
	response, err := member.GetJoinToken(verifiedMemberContext(t, tokenRequest, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, requestID.String(), account, f.firstProfile), tokenRequest)
	require.NoError(t, err)
	require.NotEmpty(t, response.GetToken().GetJwt())
	replay, replayErr := member.GetJoinToken(verifiedMemberContext(t, tokenRequest, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, requestID.String(), account, f.firstProfile), tokenRequest)
	require.Equal(t, codes.AlreadyExists, status.Code(replayErr), "a used request ID cannot sign a second bearer")
	require.Nil(t, replay)

	var issuedAt, grantExpiry, latestExpiry time.Time
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT g.issued_at,g.expires_at,m.latest_grant_expires_at
FROM voice_match_squad_member_grants g JOIN voice_room_memberships m
ON m.profile_id=g.profile_id AND m.media_epoch=g.media_epoch
WHERE g.request_id=$1 AND g.profile_id=$2 AND g.account_id=$3 AND g.session_epoch=7`, requestID, f.firstProfile, account).Scan(&issuedAt, &grantExpiry, &latestExpiry))
	require.True(t, response.GetToken().GetExpiresAt().AsTime().Equal(grantExpiry))
	require.True(t, latestExpiry.Equal(grantExpiry))
	var livekitRoom string
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT livekit_room_name FROM voice_room_instances WHERE room_id=$1`, f.roomID).Scan(&livekitRoom))
	claims := decodeHostedJWTClaims(t, response.GetToken().GetJwt())
	identity, err := livekit.MatchSquadIdentity(f.firstProfile.String(), joined.GetMediaEpoch())
	require.NoError(t, err)
	require.Equal(t, identity, claims["sub"])
	require.Equal(t, issuedAt.Unix(), int64(claims["iat"].(float64)))
	require.Equal(t, grantExpiry.Unix(), int64(claims["exp"].(float64)))
	require.LessOrEqual(t, int64(claims["exp"].(float64)-claims["iat"].(float64)), int64(60))
	video := claims["video"].(map[string]any)
	require.Equal(t, livekitRoom, video["room"])
	require.Equal(t, true, video["canPublish"])
}

func decodeHostedJWTClaims(t *testing.T, jwt string) map[string]any {
	t.Helper()
	parts := strings.Split(jwt, ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	return claims
}
func TestPostgresMatchSquadMember_TokenIssuanceRacesTeardownClosing(t *testing.T) {
	f := newMatchSquadPostgresFixture(t)
	account := uuid.New()
	member := f.memberService(map[uuid.UUID]uuid.UUID{f.firstProfile: account})
	f.service.Members = member
	join := f.joinRequest()
	joined, err := member.Join(verifiedMemberContext(t, join, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, join.GetOperationId(), account, f.firstProfile), join)
	require.NoError(t, err)

	issuer := &blockingMemberTokens{entered: make(chan struct{}), release: make(chan struct{})}
	defer issuer.unblock()
	member.Tokens = issuer
	tokenRequestID := uuid.New()
	tokenRequest := &callsv1.GetMatchSquadJoinTokenRequest{ProtocolVersion: 1, MatchId: f.matchID.String(), RoomId: f.roomID.String(), MediaEpoch: joined.GetMediaEpoch()}
	tokenCtx := verifiedMemberContext(t, tokenRequest, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, tokenRequestID.String(), account, f.firstProfile)
	type tokenResult struct {
		response *callsv1.GetMatchSquadJoinTokenResponse
		err      error
	}
	tokenDone := make(chan tokenResult, 1)
	go func() {
		response, tokenErr := member.GetJoinToken(tokenCtx, tokenRequest)
		tokenDone <- tokenResult{response: response, err: tokenErr}
	}()
	select {
	case <-issuer.entered: // the durable grant reservation has committed before signing
	case <-time.After(5 * time.Second):
		t.Fatal("token signer was not reached")
	}
	var grants int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_match_squad_member_grants WHERE request_id=$1`, tokenRequestID).Scan(&grants))
	require.Equal(t, 1, grants, "the exact bearer expiry is durable before signing starts")

	teardown := f.teardownRequest()
	teardownCtx := verifiedServiceContext(t, teardown, matchsquadprincipal.TeardownMethod, teardown.GetTeardownOperationId())
	teardownDone := make(chan error, 1)
	go func() {
		_, teardownErr := f.service.Teardown(teardownCtx, teardown)
		teardownDone <- teardownErr
	}()
	requireMatchSquadResourceState(t, f, "closing")
	issuer.unblock()

	var token tokenResult
	select {
	case token = <-tokenDone:
	case <-time.After(10 * time.Second):
		t.Fatal("token issuance did not settle")
	}
	require.Equal(t, codes.FailedPrecondition, status.Code(token.err), "closing before the final locked gate must discard the signed JWT")
	require.Nil(t, token.response)
	var teardownErr error
	select {
	case teardownErr = <-teardownDone:
	case <-time.After(10 * time.Second):
		t.Fatal("teardown did not settle")
	}
	require.Equal(t, codes.Unavailable, status.Code(teardownErr), "teardown must remain pending while the reserved bearer is live")
	require.Equal(t, 0, f.effects.closeCalls(), "LiveKit must remain open while a grant is outstanding")
	var persistedExpiry, grantExpiry time.Time
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT m.latest_grant_expires_at,g.expires_at
FROM voice_room_memberships m JOIN voice_match_squad_member_grants g
ON g.profile_id=m.profile_id AND g.media_epoch=m.media_epoch
WHERE m.profile_id=$1 AND g.request_id=$2`, f.firstProfile, tokenRequestID).Scan(&persistedExpiry, &grantExpiry))
	require.True(t, persistedExpiry.Equal(grantExpiry))
	require.True(t, persistedExpiry.After(time.Now().UTC()), "a denied token response does not shorten its durable grant")
}
func TestPostgresMatchSquadMember_TokenIssuanceDeniedWhenLeaveWinsFinalGate(t *testing.T) {
	f := newMatchSquadPostgresFixture(t)
	account := uuid.New()
	member := f.memberService(map[uuid.UUID]uuid.UUID{f.firstProfile: account})
	join := f.joinRequest()
	joined, err := member.Join(verifiedMemberContext(t, join, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, join.GetOperationId(), account, f.firstProfile), join)
	require.NoError(t, err)

	issuer := &blockingMemberTokens{entered: make(chan struct{}), release: make(chan struct{})}
	defer issuer.unblock()
	member.Tokens = issuer
	tokenRequestID := uuid.New()
	tokenRequest := &callsv1.GetMatchSquadJoinTokenRequest{ProtocolVersion: 1, MatchId: f.matchID.String(), RoomId: f.roomID.String(), MediaEpoch: joined.GetMediaEpoch()}
	tokenCtx := verifiedMemberContext(t, tokenRequest, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, tokenRequestID.String(), account, f.firstProfile)
	type tokenResult struct {
		response *callsv1.GetMatchSquadJoinTokenResponse
		err      error
	}
	tokenDone := make(chan tokenResult, 1)
	go func() {
		response, tokenErr := member.GetJoinToken(tokenCtx, tokenRequest)
		tokenDone <- tokenResult{response: response, err: tokenErr}
	}()
	select {
	case <-issuer.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("token signer was not reached after reservation")
	}

	leave := f.leaveRequest(joined.GetMediaEpoch())
	leaveCtx := verifiedMemberContext(t, leave, callsv1.MatchSquadMemberService_LeaveMatchSquadRoom_FullMethodName, leave.GetOperationId(), account, f.firstProfile)
	leaveDone := make(chan error, 1)
	go func() {
		_, leaveErr := member.Leave(leaveCtx, leave)
		leaveDone <- leaveErr
	}()
	select {
	case err = <-leaveDone:
	case <-time.After(10 * time.Second):
		t.Fatal("leave did not reach its durable pending state")
	}
	require.Equal(t, codes.Unavailable, status.Code(err), "Leave must remain pending while the reservation is live")
	assertMatchSquadMemberState(t, f, f.firstProfile, "LEAVING", "confirmed")

	issuer.unblock()
	var token tokenResult
	select {
	case token = <-tokenDone:
	case <-time.After(10 * time.Second):
		t.Fatal("token issuance did not settle")
	}
	require.Equal(t, codes.FailedPrecondition, status.Code(token.err), "Leave before the final gate must discard the signed JWT")
	require.Nil(t, token.response)
	var latestExpiry, grantExpiry time.Time
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT m.latest_grant_expires_at,g.expires_at
FROM voice_room_memberships m JOIN voice_match_squad_member_grants g
ON g.profile_id=m.profile_id AND g.media_epoch=m.media_epoch
WHERE m.profile_id=$1 AND g.request_id=$2`, f.firstProfile, tokenRequestID).Scan(&latestExpiry, &grantExpiry))
	require.True(t, latestExpiry.Equal(grantExpiry))
	require.True(t, latestExpiry.After(time.Now().UTC()), "denied issuance does not shorten the pending leave fence")
}
func TestPostgresMatchSquadMember_SignerFailureRetainsReservation(t *testing.T) {
	f := newMatchSquadPostgresFixture(t)
	account := uuid.New()
	member := f.memberService(map[uuid.UUID]uuid.UUID{f.firstProfile: account})
	join := f.joinRequest()
	joined, err := member.Join(verifiedMemberContext(t, join, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, join.GetOperationId(), account, f.firstProfile), join)
	require.NoError(t, err)
	member.Tokens = failingMemberTokens{testMemberTokens: &testMemberTokens{}}

	requestID := uuid.New()
	tokenRequest := &callsv1.GetMatchSquadJoinTokenRequest{ProtocolVersion: 1, MatchId: f.matchID.String(), RoomId: f.roomID.String(), MediaEpoch: joined.GetMediaEpoch()}
	response, err := member.GetJoinToken(verifiedMemberContext(t, tokenRequest, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, requestID.String(), account, f.firstProfile), tokenRequest)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Nil(t, response)
	var grantCount int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_match_squad_member_grants WHERE request_id=$1`, requestID).Scan(&grantCount))
	require.Equal(t, 1, grantCount, "signer failure retains its committed reservation through expiry")
}
func TestPostgresMatchSquadMember_CanceledSignerRetainsReservation(t *testing.T) {
	f := newMatchSquadPostgresFixture(t)
	account := uuid.New()
	member := f.memberService(map[uuid.UUID]uuid.UUID{f.firstProfile: account})
	join := f.joinRequest()
	joined, err := member.Join(verifiedMemberContext(t, join, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, join.GetOperationId(), account, f.firstProfile), join)
	require.NoError(t, err)

	issuer := &blockingMemberTokens{entered: make(chan struct{}), release: make(chan struct{})}
	defer issuer.unblock()
	member.Tokens = issuer
	requestID := uuid.New()
	tokenRequest := &callsv1.GetMatchSquadJoinTokenRequest{ProtocolVersion: 1, MatchId: f.matchID.String(), RoomId: f.roomID.String(), MediaEpoch: joined.GetMediaEpoch()}
	baseCtx := verifiedMemberContext(t, tokenRequest, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, requestID.String(), account, f.firstProfile)
	tokenCtx, cancel := context.WithCancel(baseCtx)
	defer cancel()
	type tokenResult struct {
		response *callsv1.GetMatchSquadJoinTokenResponse
		err      error
	}
	done := make(chan tokenResult, 1)
	go func() {
		response, tokenErr := member.GetJoinToken(tokenCtx, tokenRequest)
		done <- tokenResult{response: response, err: tokenErr}
	}()
	select {
	case <-issuer.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("token signer was not reached after reservation")
	}
	cancel()
	issuer.unblock() // a late signer result must be discarded after caller cancellation
	select {
	case result := <-done:
		require.Equal(t, codes.Unavailable, status.Code(result.err))
		require.Nil(t, result.response)
	case <-time.After(10 * time.Second):
		t.Fatal("canceled token request did not settle")
	}
	var grantCount int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_match_squad_member_grants WHERE request_id=$1`, requestID).Scan(&grantCount))
	require.Equal(t, 1, grantCount, "caller cancellation retains the committed reservation")
}

func TestPostgresMatchSquadMember_SignerFinishesAfterActorExpiryRetainsReservation(t *testing.T) {
	f := newMatchSquadPostgresFixture(t)
	account := uuid.New()
	member := f.memberService(map[uuid.UUID]uuid.UUID{f.firstProfile: account})
	join := f.joinRequest()
	joined, err := member.Join(verifiedMemberContext(t, join, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, join.GetOperationId(), account, f.firstProfile), join)
	require.NoError(t, err)

	issuer := &blockingMemberTokens{entered: make(chan struct{}), release: make(chan struct{})}
	defer issuer.unblock()
	member.Tokens = issuer
	requestID := uuid.New()
	tokenRequest := &callsv1.GetMatchSquadJoinTokenRequest{ProtocolVersion: 1, MatchId: f.matchID.String(), RoomId: f.roomID.String(), MediaEpoch: joined.GetMediaEpoch()}
	actorExpiry := time.Now().UTC().Add(15 * time.Second)
	tokenCtx := verifiedMemberContextUntil(t, tokenRequest, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, requestID.String(), account, f.firstProfile, actorExpiry)
	type tokenResult struct {
		response *callsv1.GetMatchSquadJoinTokenResponse
		err      error
	}
	done := make(chan tokenResult, 1)
	go func() {
		response, tokenErr := member.GetJoinToken(tokenCtx, tokenRequest)
		done <- tokenResult{response: response, err: tokenErr}
	}()
	select {
	case <-issuer.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("token signer was not reached after reservation")
	}
	leave := f.leaveRequest(joined.GetMediaEpoch())
	leaveCtx := verifiedMemberContext(t, leave, callsv1.MatchSquadMemberService_LeaveMatchSquadRoom_FullMethodName, leave.GetOperationId(), account, f.firstProfile)
	_, err = member.Leave(leaveCtx, leave)
	require.NoError(t, err, "Leave records the durable cleanup while the old signer remains blocked")
	assertMatchSquadMemberState(t, f, f.firstProfile, "LEAVING", "confirmed")
	requireDatabaseTimeAtLeast(t, f, actorExpiry)
	require.NoError(t, member.RepairExpiredLeaves(f.ctx, 16))
	assertMatchSquadMemberState(t, f, f.firstProfile, "LEFT", "confirmed")
	assertMatchSquadFence(t, f, account, f.firstProfile, false)

	// A new authorized media generation may start only after the old grant and
	// removal effects have drained. The old signer must not affect this epoch.
	newJoin := f.joinRequest()
	newJoined, err := member.Join(verifiedMemberContext(t, newJoin, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, newJoin.GetOperationId(), account, f.firstProfile), newJoin)
	require.NoError(t, err)
	require.NotEqual(t, joined.GetMediaEpoch(), newJoined.GetMediaEpoch())
	newTokenRequest := &callsv1.GetMatchSquadJoinTokenRequest{ProtocolVersion: 1, MatchId: f.matchID.String(), RoomId: f.roomID.String(), MediaEpoch: newJoined.GetMediaEpoch()}
	newToken, err := member.GetJoinToken(verifiedMemberContext(t, newTokenRequest, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, uuid.NewString(), account, f.firstProfile), newTokenRequest)
	require.NoError(t, err)
	require.NotEmpty(t, newToken.GetToken().GetJwt())

	issuer.unblock()
	select {
	case result := <-done:
		require.Equal(t, codes.FailedPrecondition, status.Code(result.err), "the final DB/Auth gate rejects a signer result after the old epoch is terminal")
		require.Nil(t, result.response)
	case <-time.After(10 * time.Second):
		t.Fatal("expired token request did not settle")
	}
	var grantCount int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_match_squad_member_grants WHERE request_id=$1`, requestID).Scan(&grantCount))
	require.Equal(t, 1, grantCount, "expired signing never deletes or rewrites its reservation")
	assertMatchSquadMemberState(t, f, f.firstProfile, "JOINED", "confirmed")
	assertMatchSquadFence(t, f, account, f.firstProfile, true)
	require.Equal(t, newJoined.GetMediaEpoch(), f.memberEpoch(t, f.firstProfile))
}

func TestPostgresMatchSquadMember_LateOldRemovalCannotAffectNewGeneration(t *testing.T) {
	f := newMatchSquadPostgresFixture(t)
	account := uuid.New()
	member := f.memberService(map[uuid.UUID]uuid.UUID{f.firstProfile: account})
	join := f.joinRequest()
	joined, err := member.Join(verifiedMemberContext(t, join, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, join.GetOperationId(), account, f.firstProfile), join)
	require.NoError(t, err)
	oldIdentity, err := livekit.MatchSquadIdentity(f.firstProfile.String(), joined.GetMediaEpoch())
	require.NoError(t, err)
	var livekitRoom string
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT livekit_room_name FROM voice_room_instances WHERE room_id=$1`, f.roomID).Scan(&livekitRoom))
	f.effects.seedParticipant(livekitRoom, oldIdentity)
	f.effects.blockNextRemoval()
	defer f.effects.releaseRemoval()

	leave := f.leaveRequest(joined.GetMediaEpoch())
	leaveCtx, cancel := context.WithCancel(verifiedMemberContext(t, leave, callsv1.MatchSquadMemberService_LeaveMatchSquadRoom_FullMethodName, leave.GetOperationId(), account, f.firstProfile))
	leaveDone := make(chan error, 1)
	go func() {
		_, leaveErr := member.Leave(leaveCtx, leave)
		leaveDone <- leaveErr
	}()
	select {
	case <-f.effects.removeEntered:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("old-generation LiveKit removal did not reach the downstream barrier")
	}
	// The dispatched request deliberately ignores caller cancellation: canceling
	// a handler cannot prove that a remote effect stopped executing.
	cancel()
	require.NoError(t, member.RepairPendingLeaves(f.ctx, 16), "a replacement worker confirms the persisted old-epoch effect")
	assertMatchSquadMemberState(t, f, f.firstProfile, "LEFT", "confirmed")
	assertMatchSquadFence(t, f, account, f.firstProfile, false)

	newJoin := f.joinRequest()
	newJoined, err := member.Join(verifiedMemberContext(t, newJoin, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, newJoin.GetOperationId(), account, f.firstProfile), newJoin)
	require.NoError(t, err, "a new media epoch is admitted only after old effects are durably confirmed")
	require.NotEqual(t, joined.GetMediaEpoch(), newJoined.GetMediaEpoch())
	newIdentity, err := livekit.MatchSquadIdentity(f.firstProfile.String(), newJoined.GetMediaEpoch())
	require.NoError(t, err)
	f.effects.seedParticipant(livekitRoom, newIdentity)
	f.effects.releaseRemoval()
	select {
	case leaveErr := <-leaveDone:
		require.Error(t, leaveErr, "canceled old handler cannot turn a late media reply into a successful current receipt")
	case <-time.After(5 * time.Second):
		t.Fatal("canceled old handler did not settle after its dispatched effect returned")
	}

	f.effects.mu.Lock()
	_, newParticipantPresent := f.effects.participants[livekitRoom][newIdentity]
	oldRemovalCount := 0
	for _, target := range f.effects.removalTargets {
		if target == oldIdentity {
			oldRemovalCount++
		}
	}
	f.effects.mu.Unlock()
	require.True(t, newParticipantPresent, "late removal by Eold must not remove the distinct Enew target")
	require.Equal(t, 2, oldRemovalCount, "the original worker and repairer both target only the persisted old generation")
	require.Equal(t, newJoined.GetMediaEpoch(), f.memberEpoch(t, f.firstProfile))
	assertMatchSquadMemberState(t, f, f.firstProfile, "JOINED", "confirmed")
	assertMatchSquadFence(t, f, account, f.firstProfile, true)
}

func requireDatabaseTimeAtLeast(t *testing.T, f *matchSquadPostgresFixture, target time.Time) {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.ctx, 25*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		var reached bool
		err := f.pool.QueryRow(ctx, `SELECT clock_timestamp() >= $1`, target).Scan(&reached)
		require.NoError(t, err, "read the database clock while waiting for actor expiry")
		if reached {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("database clock did not reach the actor expiry before the bounded timeout")
		case <-ticker.C:
		}
	}
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

func TestCreateInsertErrorClassificationIsBoundToVerifiedConstraints(t *testing.T) {
	f := newMatchSquadPostgresFixture(t)
	operation, match, _, manifest, chatReceipt, requestBytes, err := validateCreate(f.creationRequest)
	require.NoError(t, err)
	requestHash, err := principalHash(f.creationRequest)
	require.NoError(t, err)
	chatReceiptBytes, err := marshal(chatReceipt)
	require.NoError(t, err)
	chatReceiptHash := sha256.Sum256(chatReceiptBytes)
	recoveredRoom, recoveredReceipt, found, err := f.service.recoverCreateBinding(f.ctx, operation, match, match, chatReceipt, manifest, chatReceiptHash, requestHash, requestBytes)
	require.NoError(t, err)
	require.True(t, found, "exact durable operation recovers its original committed binding")
	require.Equal(t, f.roomID, recoveredRoom)
	recovered := new(callsv1.MatchSquadRoomReceipt)
	require.NoError(t, proto.Unmarshal(recoveredReceipt, recovered))
	require.Equal(t, f.creationReceipt, recovered)

	conflict := proto.Clone(f.creationRequest).(*callsv1.CreateMatchSquadRoomRequest)
	conflict.OperationId = uuid.NewString()
	conflict.ChatCreationReceipt.OperationId = conflict.OperationId
	conflictCtx := verifiedServiceContext(t, conflict, matchsquadprincipal.CreateMethod, conflict.GetOperationId())
	_, err = f.service.Create(conflictCtx, conflict)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "the verified current owner index is a terminal binding conflict")
	var conflicts int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_match_squad_operations WHERE operation_id=$1`, uuid.MustParse(conflict.GetOperationId())).Scan(&conflicts))
	require.Zero(t, conflicts)
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_room_instances`).Scan(&conflicts))
	require.Equal(t, 1, conflicts, "failed create transaction leaves no partial owned room")

	_, err = f.pool.Exec(f.ctx, `CREATE FUNCTION test_match_squad_unexpected_unique() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'injected private fixture fault' USING ERRCODE='23505', CONSTRAINT='unexpected_voice_constraint'; END; $$;
CREATE TRIGGER test_match_squad_unexpected_unique BEFORE INSERT ON voice_match_squad_operations
FOR EACH ROW EXECUTE FUNCTION test_match_squad_unexpected_unique()`)
	require.NoError(t, err)
	unrecognized := proto.Clone(validCreateRequest()).(*callsv1.CreateMatchSquadRoomRequest)
	unrecognized.OperationId = uuid.NewString()
	unrecognized.MatchId = uuid.NewString()
	unrecognized.ChatCreationReceipt.OperationId = unrecognized.OperationId
	unrecognized.ChatCreationReceipt.MatchId = unrecognized.MatchId
	unrecognizedCtx := verifiedServiceContext(t, unrecognized, matchsquadprincipal.CreateMethod, unrecognized.GetOperationId())
	_, err = f.service.Create(unrecognizedCtx, unrecognized)
	require.Equal(t, codes.Unavailable, status.Code(err), "an unrecognized 23505 is a persistence fault, not proof of a binding conflict")
	require.NotContains(t, status.Convert(err).Message(), "injected private fixture fault")
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_match_squad_operations WHERE operation_id=$1`, uuid.MustParse(unrecognized.GetOperationId())).Scan(&conflicts))
	require.Zero(t, conflicts)
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_room_instances WHERE creation_operation_id=$1`, uuid.MustParse(unrecognized.GetOperationId())).Scan(&conflicts))
	require.Zero(t, conflicts, "unknown unique violation rolls the complete transaction back")
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
	return verifiedMemberContextUntil(t, request, method, requestID, account, profile, time.Now().UTC().Add(time.Hour))
}

func verifiedMemberContextUntil(t *testing.T, request proto.Message, method, requestID string, account, profile uuid.UUID, expiresAt time.Time) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer hosted-test-credential", "x-request-id", requestID))
	return principal.WithVerified(ctx, principal.Principal{
		Kind: "delegated_user", Issuer: "gateway", Subject: account.String(), AccountID: account.String(), ProfileID: profile.String(),
		Audience: "voice", RPC: method, RequestID: requestID, RequestHash: hash, SessionEpoch: 7, ExpiresAt: expiresAt.UTC(),
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

func assertMatchSquadTerminalGeneration(t *testing.T, f *matchSquadPostgresFixture, profile, account uuid.UUID, epoch string, grantRequest uuid.UUID, wantState string) {
	t.Helper()
	var state string
	var storedAccount uuid.UUID
	var sessionEpoch int64
	var storedEpoch uuid.UUID
	var canJoin, canPublishAudio, canPublishVideo, canPublishScreenShare, canSubscribe bool
	var recordedExpiry, grantExpiry time.Time
	err := f.pool.QueryRow(f.ctx, `SELECT m.membership_state,m.account_id,m.session_epoch,m.media_epoch,
m.can_join,m.can_publish_audio,m.can_publish_video,m.can_publish_screen_share,m.can_subscribe,
m.latest_grant_expires_at,g.expires_at
FROM voice_room_memberships m JOIN voice_match_squad_member_grants g
ON g.profile_id=m.profile_id AND g.media_epoch=m.media_epoch
WHERE m.profile_id=$1 AND m.room_id=$2 AND m.media_epoch=$3 AND g.request_id=$4`, profile, f.roomID, uuid.MustParse(epoch), grantRequest).Scan(
		&state, &storedAccount, &sessionEpoch, &storedEpoch, &canJoin, &canPublishAudio, &canPublishVideo, &canPublishScreenShare, &canSubscribe, &recordedExpiry, &grantExpiry)
	require.NoError(t, err)
	require.Equal(t, wantState, state)
	require.Equal(t, account, storedAccount)
	require.EqualValues(t, 7, sessionEpoch)
	require.Equal(t, uuid.MustParse(epoch), storedEpoch)
	require.True(t, canJoin, "the immutable media-generation capability tuple is retained as historical data")
	require.True(t, canPublishAudio)
	require.False(t, canPublishVideo)
	require.False(t, canPublishScreenShare)
	require.True(t, canSubscribe)
	require.True(t, recordedExpiry.Equal(grantExpiry), "terminal transitions preserve the monotonic grant-expiry fence")
}

func assertMatchSquadUnissuedTerminalGeneration(t *testing.T, f *matchSquadPostgresFixture, profile, account uuid.UUID, epoch, wantState string) {
	t.Helper()
	var state string
	var storedAccount, storedEpoch uuid.UUID
	var sessionEpoch int64
	var canJoin, canPublishAudio, canPublishVideo, canPublishScreenShare, canSubscribe, expiryIsNull bool
	err := f.pool.QueryRow(f.ctx, `SELECT membership_state,account_id,session_epoch,media_epoch,
can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,latest_grant_expires_at IS NULL
FROM voice_room_memberships WHERE profile_id=$1 AND room_id=$2 AND media_epoch=$3`, profile, f.roomID, uuid.MustParse(epoch)).Scan(
		&state, &storedAccount, &sessionEpoch, &storedEpoch, &canJoin, &canPublishAudio, &canPublishVideo, &canPublishScreenShare, &canSubscribe, &expiryIsNull)
	require.NoError(t, err)
	require.Equal(t, wantState, state)
	require.Equal(t, account, storedAccount)
	require.EqualValues(t, 7, sessionEpoch)
	require.Equal(t, uuid.MustParse(epoch), storedEpoch)
	require.True(t, canJoin)
	require.True(t, canPublishAudio)
	require.False(t, canPublishVideo)
	require.False(t, canPublishScreenShare)
	require.True(t, canSubscribe)
	require.True(t, expiryIsNull, "a generation with no issued bearer keeps a null expiry")
	var grants int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM voice_match_squad_member_grants WHERE profile_id=$1 AND media_epoch=$2`, profile, uuid.MustParse(epoch)).Scan(&grants))
	require.Zero(t, grants)
}

func requireMatchSquadResourceState(t *testing.T, f *matchSquadPostgresFixture, want string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		var state string
		err := f.pool.QueryRow(ctx, `SELECT state FROM voice_room_instances WHERE room_id=$1`, f.roomID).Scan(&state)
		require.NoError(t, err)
		if state == want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("MatchSquad resource did not reach %q: %v", want, ctx.Err())
		case <-ticker.C:
		}
	}
}

func (f *matchSquadPostgresFixture) memberEpoch(t *testing.T, profile uuid.UUID) string {
	t.Helper()
	var epoch uuid.UUID
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT media_epoch FROM voice_room_memberships WHERE profile_id=$1 AND room_id=$2`, profile, f.roomID).Scan(&epoch))
	return epoch.String()
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

type failCurrentMemberCheck struct {
	calls  int
	failAt int
}

func (p *failCurrentMemberCheck) CheckCurrent(ctx context.Context, actor principal.Principal, now time.Time) error {
	p.calls++
	if p.calls == p.failAt {
		return status.Error(codes.Unauthenticated, "injected hosted principal expiry at join commit")
	}
	return testCurrentMember{}.CheckCurrent(ctx, actor, now)
}

func (testCurrentMember) CheckCurrent(_ context.Context, actor principal.Principal, now time.Time) error {
	if !actor.ExpiresAt.After(now) {
		return errors.New("hosted test actor expired")
	}
	return nil
}

type testMemberTokens struct{}

type failingMemberTokens struct{ *testMemberTokens }

func (failingMemberTokens) MatchSquadJoinTokenUntil(string, string, *bool, time.Time, time.Time) (string, error) {
	return "", errors.New("injected signing failure")
}

type blockingMemberTokens struct {
	entered     chan struct{}
	release     chan struct{}
	once        sync.Once
	releaseOnce sync.Once
}

func (*testMemberTokens) MatchSquadTokenWindow(now, actorExpiry time.Time) (time.Time, time.Time, error) {
	if !actorExpiry.After(now) {
		return time.Time{}, time.Time{}, errors.New("hosted test actor expired")
	}
	issuedAt := time.Unix(now.UTC().Unix(), 0).UTC()
	expiresAt := issuedAt.Add(45 * time.Second)
	actorExpiry = time.Unix(actorExpiry.UTC().Unix(), 0).UTC()
	if actorExpiry.Before(expiresAt) {
		expiresAt = actorExpiry
	}
	if !expiresAt.After(now) || !expiresAt.After(issuedAt) {
		return time.Time{}, time.Time{}, errors.New("hosted test actor expires too soon")
	}
	return issuedAt, expiresAt, nil
}

func (b *blockingMemberTokens) MatchSquadTokenWindow(now, actorExpiry time.Time) (time.Time, time.Time, error) {
	return (*testMemberTokens)(nil).MatchSquadTokenWindow(now, actorExpiry)
}

func (b *blockingMemberTokens) MatchSquadJoinTokenUntil(identity, room string, canPublish *bool, issuedAt, expiresAt time.Time) (string, error) {
	block := false
	b.once.Do(func() { close(b.entered); block = true })
	if block {
		<-b.release
	}
	return (*testMemberTokens)(nil).MatchSquadJoinTokenUntil(identity, room, canPublish, issuedAt, expiresAt)
}

func (*blockingMemberTokens) LivekitURL() string { return "wss://hosted-test.invalid" }

func (b *blockingMemberTokens) unblock() { b.releaseOnce.Do(func() { close(b.release) }) }

func (*testMemberTokens) MatchSquadJoinTokenUntil(_ string, _ string, _ *bool, issuedAt, expiresAt time.Time) (string, error) {
	if !expiresAt.After(issuedAt) || expiresAt.Sub(issuedAt) > time.Minute {
		return "", errors.New("invalid hosted test token window")
	}
	return "hosted-test-jwt", nil
}

func (*testMemberTokens) LivekitURL() string { return "wss://hosted-test.invalid" }

type testMatchSquadEffects struct {
	mu                                  sync.Mutex
	calls                               *store.MemoryCallStore
	removeFailures                      int
	closeCount                          int
	requireProjectionDrainedBeforeClose bool
	closeEntered                        chan struct{}
	closeRelease                        chan struct{}
	closeOnce                           sync.Once
	releaseOnce                         sync.Once
	removeEntered                       chan struct{}
	removeRelease                       chan struct{}
	removeOnce                          sync.Once
	removeReleaseOnce                   sync.Once
	removalTargets                      []string
	participants                        map[string]map[string]struct{}
}

func (e *testMatchSquadEffects) EnsureRoom(context.Context, string) error { return nil }

func (e *testMatchSquadEffects) CloseRoom(ctx context.Context, room string) error {
	if e.closeEntered != nil {
		e.closeOnce.Do(func() { close(e.closeEntered) })
		select {
		case <-e.closeRelease:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
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

func (e *testMatchSquadEffects) RemoveParticipant(ctx context.Context, room, identity string) error {
	block := false
	if e.removeEntered != nil {
		e.removeOnce.Do(func() { close(e.removeEntered); block = true })
	}
	if block {
		<-e.removeRelease
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.removalTargets = append(e.removalTargets, identity)
	if e.removeFailures > 0 {
		e.removeFailures--
		return errors.New("injected hosted LiveKit participant removal failure")
	}
	if e.participants != nil {
		delete(e.participants[room], identity)
	}
	_ = ctx
	return nil
}

func (e *testMatchSquadEffects) blockNextRemoval() {
	e.mu.Lock()
	e.removeEntered = make(chan struct{})
	e.removeRelease = make(chan struct{})
	e.participants = make(map[string]map[string]struct{})
	e.mu.Unlock()
}

func (e *testMatchSquadEffects) seedParticipant(room, identity string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.participants[room] == nil {
		e.participants[room] = make(map[string]struct{})
	}
	e.participants[room][identity] = struct{}{}
}

func (e *testMatchSquadEffects) releaseRemoval() {
	if e.removeRelease != nil {
		e.removeReleaseOnce.Do(func() { close(e.removeRelease) })
	}
}

func (e *testMatchSquadEffects) failNextRemoval() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.removeFailures++
}

func (e *testMatchSquadEffects) unblockClose() {
	if e.closeRelease != nil {
		e.releaseOnce.Do(func() { close(e.closeRelease) })
	}
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
