package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/federation/mediaauthority"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/federationmedia"
	"voice/backend/voice/internal/livekit"
	"voice/backend/voice/internal/principalgrpc"
	voicestore "voice/backend/voice/internal/store"
	"voice/backend/voice/internal/voiceuserprincipalruntime"
)

type routedMediaFixture struct {
	requests []mediaauthority.RouteRequest
	err      error
}

func (f *routedMediaFixture) JoinToken(_ context.Context, request mediaauthority.RouteRequest) (mediaauthority.ExchangeResult, error) {
	f.requests = append(f.requests, request)
	return mediaauthority.ExchangeResult{JWT: "node-token", LivekitURL: "wss://node.test", ExpiresAt: time.Now().Add(20 * time.Second).UnixMilli()}, f.err
}

func TestGetJoinTokenUsesFederatedEdgeAfterCanonicalChecksAndNeverFallsBackOnFailure(t *testing.T) {
	account, profile, space, room := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	svc := newTestVoiceService(time.Now(), &recordingEvents{})
	resolver := &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{SpaceID: space, Active: true, Member: true, AccessEpoch: 11}}
	svc.VoiceRoomAccessResolver = resolver
	svc.Roles = &canonicalRolePermissions{}
	svc.SpaceMediaReady = true
	svc.SpaceMediaAdmissions = readySpaceMediaAdmission{}
	svc.SessionEpochChecker = currentSessionCheckerFunc(func(context.Context, string, int64) error { return nil })
	svc.SpaceVoiceRoomGrants = staticSpaceMediaGrants{grants: CanonicalVoiceRoomGrants{PolicyEpoch: 7, CanJoin: true, CanPublishAudio: true, CanSubscribe: true}}
	svc.SpaceTokens = livekit.NewSpaceTokenIssuer("test-key", "test-secret", "wss://local.test", time.Minute)
	joinRequest := &callsv1.JoinVoiceRoomRequest{VoiceRoomId: room, Space: &spacev1.SpaceRef{Id: space}}
	joined, err := svc.JoinVoiceRoom(verifiedVoiceUserContext(t, &callsv1.GetJoinTokenRequest{}, account, profile, 5), joinRequest)
	require.NoError(t, err)
	edge := &routedMediaFixture{}
	svc.FederatedMedia = edge
	request := &callsv1.GetJoinTokenRequest{RoomId: joined.VoiceSession.RoomId}
	ctx := verifiedVoiceUserContext(t, request, account, profile, 5)
	for _, failedAdmission := range []readySpaceMediaAdmission{
		{projectionErr: fmt.Errorf("projection not confirmed")},
		{headErr: fmt.Errorf("room generation is closing")},
	} {
		svc.SpaceMediaAdmissions = failedAdmission
		_, gateErr := svc.GetJoinToken(ctx, request)
		require.Equal(t, codes.Unavailable, status.Code(gateErr))
		require.Empty(t, edge.requests, "unconfirmed PG projection or room head must precede federation")
	}
	svc.SpaceMediaAdmissions = readySpaceMediaAdmission{}
	hosted, err := svc.GetJoinToken(ctx, request)
	require.NoError(t, err)
	require.Equal(t, "wss://node.test", hosted.GetLivekitUrl())
	require.Len(t, edge.requests, 1)
	call, err := svc.Calls.GetCall(ctx, request.RoomId)
	require.NoError(t, err)
	require.Equal(t, mediaauthority.RouteRequest{AccountID: account, ProfileID: profile, SpaceID: space, ResourceID: room, RoomName: call.LivekitRoomName, SessionEpoch: 5, CanPublish: true}, edge.requests[0])
	stored, err := svc.Calls.GetCall(ctx, request.RoomId)
	require.NoError(t, err)
	require.Contains(t, stored.SpaceMedia, profile, "federation is reached only after confirmed canonical Space admission")
	floors, err := svc.Calls.GetSpaceMediaEpochFloors(ctx, space)
	require.NoError(t, err)
	require.Equal(t, voicestore.SpaceMediaEpochFloors{AccessEpoch: 11, PolicyEpoch: 7}, floors)

	for _, failure := range []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "denied", err: federationmedia.ErrDenied, code: codes.PermissionDenied},
		{name: "unavailable", err: federationmedia.ErrUnavailable, code: codes.Unavailable},
		{name: "unexpected", err: fmt.Errorf("unexpected route failure"), code: codes.Unavailable},
		{name: "wrapped not hosted is not the exact fallback signal", err: fmt.Errorf("route error: %w", federationmedia.ErrNotHosted), code: codes.Unavailable},
	} {
		t.Run(failure.name, func(t *testing.T) {
			edge.err = failure.err
			before := len(edge.requests)
			_, err := svc.GetJoinToken(ctx, request)
			require.Equal(t, failure.code, status.Code(err))
			require.Len(t, edge.requests, before+1)
			unchanged, getErr := svc.Calls.GetCall(ctx, request.RoomId)
			require.NoError(t, getErr)
			require.Contains(t, unchanged.SpaceMedia, profile)
			unchangedFloors, floorErr := svc.Calls.GetSpaceMediaEpochFloors(ctx, space)
			require.NoError(t, floorErr)
			require.Equal(t, voicestore.SpaceMediaEpochFloors{AccessEpoch: 11, PolicyEpoch: 7}, unchangedFloors)
		})
	}

	edge.err = federationmedia.ErrNotHosted
	result, err := svc.GetJoinToken(ctx, request)
	require.NoError(t, err)
	require.Equal(t, "wss://local.test", result.GetLivekitUrl())
	stored, err = svc.Calls.GetCall(ctx, request.RoomId)
	require.NoError(t, err)
	require.Contains(t, stored.SpaceMedia, profile, "only ErrNotHosted may admit through the local Space issuer")
	floors, err = svc.Calls.GetSpaceMediaEpochFloors(ctx, space)
	require.NoError(t, err)
	require.Equal(t, voicestore.SpaceMediaEpochFloors{AccessEpoch: 11, PolicyEpoch: 7}, floors)

	edge.err = nil
	resolver.result.Member = false
	before := len(edge.requests)
	_, err = svc.GetJoinToken(ctx, request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Len(t, edge.requests, before)
	resolver.result.Member = true
	for _, roleCase := range []struct {
		name string
		role RolePermissionChecker
		code codes.Code
	}{
		{name: "Role denial", role: &recordingVoiceRolePermissions{voiceJoinErr: ErrVoiceJoinDenied}, code: codes.PermissionDenied},
		{name: "Role unavailable", role: &recordingVoiceRolePermissions{voiceJoinErr: status.Error(codes.Unavailable, "Role unavailable")}, code: codes.Unavailable},
	} {
		svc.Roles = roleCase.role
		before = len(edge.requests)
		_, err = svc.GetJoinToken(ctx, request)
		require.Equal(t, roleCase.code, status.Code(err), roleCase.name)
		require.Len(t, edge.requests, before, "Role preflight must precede federation")
	}
	svc.Roles = nil
	before = len(edge.requests)
	_, err = svc.GetJoinToken(ctx, request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Len(t, edge.requests, before, "missing Role dependency must fail closed before federation")
	svc.Roles = &canonicalRolePermissions{}

	_, err = svc.GetJoinToken(voiceTestCtx(profile), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Len(t, edge.requests, before)
}

type currentFloorDelegatedVerifier struct {
	key        *rsa.PublicKey
	accountID  string
	floor      int64
	floorError error
	floorCalls int
}

func (v *currentFloorDelegatedVerifier) Verify(ctx context.Context, token, method, requestID, requestHash string) (principal.Principal, error) {
	return principal.VerifyDelegatedUser(ctx, token, principal.VerifyConfig{
		ExpectedIssuer: "gateway", ExpectedAudience: "voice", ExpectedRPC: method,
		ExpectedRequestID: requestID, ExpectedRequestHash: requestHash,
		KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return v.key, nil },
		ReplayGuard: func(context.Context, string, string, time.Time) error { return nil },
		SessionEpochChecker: func(_ context.Context, accountID string, epoch int64) error {
			v.floorCalls++
			if accountID != v.accountID {
				return status.Error(codes.Unauthenticated, "account mismatch")
			}
			if v.floorError != nil {
				return principalgrpc.Unavailable(v.floorError)
			}
			if epoch < v.floor {
				return status.Error(codes.Unauthenticated, "stale session epoch")
			}
			return nil
		},
	})
}

func TestGetJoinTokenFederationRequiresCurrentAuthFloorBeforeHandler(t *testing.T) {
	account, profile, space, room := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, tc := range []struct {
		name       string
		floor      int64
		floorError error
		wantCode   codes.Code
	}{
		{name: "valid signed principal with stale Auth epoch", floor: 6, wantCode: codes.Unauthenticated},
		{name: "Auth floor lookup failure", floorError: status.Error(codes.Unavailable, "floor unavailable"), wantCode: codes.Unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestVoiceService(time.Now(), &recordingEvents{})
			resolver := &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{SpaceID: space, Active: true, Member: true, AccessEpoch: 11}}
			svc.VoiceRoomAccessResolver = resolver
			svc.Roles = &canonicalRolePermissions{}
			svc.SpaceMediaReady = true
			svc.SpaceMediaAdmissions = readySpaceMediaAdmission{}
			svc.SessionEpochChecker = currentSessionCheckerFunc(func(context.Context, string, int64) error { return nil })
			svc.SpaceVoiceRoomGrants = staticSpaceMediaGrants{grants: CanonicalVoiceRoomGrants{PolicyEpoch: 7, CanJoin: true, CanPublishAudio: true, CanSubscribe: true}}
			svc.SpaceTokens = livekit.NewSpaceTokenIssuer("test-key", "test-secret", "wss://local.test", time.Minute)
			joinRequest := &callsv1.JoinVoiceRoomRequest{VoiceRoomId: room, Space: &spacev1.SpaceRef{Id: space}}
			joined, err := svc.JoinVoiceRoom(verifiedVoiceUserContext(t, &callsv1.GetJoinTokenRequest{}, account, profile, 5), joinRequest)
			require.NoError(t, err)
			edge := &routedMediaFixture{}
			svc.FederatedMedia = edge
			request := &callsv1.GetJoinTokenRequest{RoomId: joined.VoiceSession.RoomId}
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(t, err)
			issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key})
			require.NoError(t, err)
			hash, err := principal.RequestHash(request)
			require.NoError(t, err)
			requestID := uuid.NewString()
			token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{
				Audience: "voice", RPC: callsv1.VoiceService_GetJoinToken_FullMethodName,
				RequestID: requestID, RequestHash: hash, AccountID: account, ProfileID: profile,
				SessionEpoch: 5, ClientExpiresAt: time.Now().Add(30 * time.Second),
			})
			require.NoError(t, err)
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				"authorization", "Bearer "+token, "x-request-id", requestID,
			))
			verifier := &currentFloorDelegatedVerifier{key: &key.PublicKey, accountID: account, floor: tc.floor, floorError: tc.floorError}
			intercept := voiceuserprincipalruntime.StrictUnaryInterceptor(verifier)
			_, err = intercept(ctx, request, &grpc.UnaryServerInfo{FullMethod: callsv1.VoiceService_GetJoinToken_FullMethodName}, func(ctx context.Context, req any) (any, error) {
				return svc.GetJoinToken(ctx, req.(*callsv1.GetJoinTokenRequest))
			})
			require.Equal(t, tc.wantCode, status.Code(err))
			require.Equal(t, 1, verifier.floorCalls, "the signed epoch must be checked against current Auth state")
			require.Empty(t, edge.requests, "Auth admission failure must precede Federation")
			stored, getErr := svc.Calls.GetCall(context.Background(), request.GetRoomId())
			require.NoError(t, getErr)
			require.Contains(t, stored.SpaceMedia, profile, "a later Auth failure must preserve the already confirmed admission")
			floors, floorErr := svc.Calls.GetSpaceMediaEpochFloors(context.Background(), space)
			require.NoError(t, floorErr)
			require.Equal(t, voicestore.SpaceMediaEpochFloors{AccessEpoch: 11, PolicyEpoch: 7}, floors)
		})
	}
}
