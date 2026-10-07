package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/livekit"
	voicestore "voice/backend/voice/internal/store"
	"voice/backend/voice/internal/voiceuserprincipalruntime"

	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
)

type staticSpaceMediaGrants struct {
	grants CanonicalVoiceRoomGrants
	err    error
}

type readySpaceMediaAdmission struct {
	err           error
	projectionErr error
	headErr       error
}

type currentSessionCheckerFunc func(context.Context, string, int64) error

func (f currentSessionCheckerFunc) RequireCurrent(ctx context.Context, accountID string, epoch int64) error {
	return f(ctx, accountID, epoch)
}

func (r readySpaceMediaAdmission) CheckSchema(context.Context) error { return r.err }
func (r readySpaceMediaAdmission) ClaimRoomHead(_ context.Context, voiceRoomID, spaceID, roomID string, operationID uuid.UUID, allowCreate bool) (voicestore.SpaceMediaRoomHead, error) {
	head := voicestore.SpaceMediaRoomHead{VoiceRoomID: voiceRoomID, SpaceID: spaceID, RoomID: roomID, RoomGeneration: 1, State: "OPEN", Ready: !allowCreate}
	if allowCreate {
		head.CreatorOperationID = operationID
	} else {
		head.CreatorOperationID = uuid.New()
	}
	return head, r.err
}
func (r readySpaceMediaAdmission) AbandonRoomClaim(context.Context, string, uint64, uuid.UUID) error {
	return r.err
}
func (r readySpaceMediaAdmission) Prepare(context.Context, voicestore.SpaceMediaAdmission) error {
	return r.err
}
func (r readySpaceMediaAdmission) Fence(context.Context, voicestore.SpaceMediaAdmission) error {
	return r.err
}
func (r readySpaceMediaAdmission) MarkCommitted(context.Context, uuid.UUID, string) error {
	return r.err
}
func (r readySpaceMediaAdmission) MarkProjectionApplied(context.Context, uuid.UUID, string) error {
	return r.err
}
func (r readySpaceMediaAdmission) ConfirmProjection(context.Context, uuid.UUID, string) error {
	if r.projectionErr != nil {
		return r.projectionErr
	}
	return r.err
}
func (r readySpaceMediaAdmission) ConfirmRoomHeadOpen(context.Context, voicestore.SpaceMediaAdmission) error {
	if r.headErr != nil {
		return r.headErr
	}
	return r.err
}
func (r readySpaceMediaAdmission) MarkAborting(context.Context, uuid.UUID, string) error {
	return r.err
}
func (r readySpaceMediaAdmission) ReleaseFence(context.Context, voicestore.SpaceMediaAdmission) error {
	return r.err
}
func (r readySpaceMediaAdmission) MarkCleanupCompleted(context.Context, uuid.UUID, string) error {
	return r.err
}
func (r readySpaceMediaAdmission) BeginRoomLeave(context.Context, uuid.UUID, string) error {
	return r.err
}
func (r readySpaceMediaAdmission) CompleteRoomLeave(context.Context, voicestore.SpaceMediaAdmission) error {
	return r.err
}

func (r staticSpaceMediaGrants) ResolveVoiceRoomGrants(context.Context, string, string, string) (CanonicalVoiceRoomGrants, error) {
	return r.grants, r.err
}

type fixtureSpaceMediaAccessResolver struct {
	rooms       map[string]string
	members     map[string]map[string]bool
	accessEpoch uint64
}

func (r fixtureSpaceMediaAccessResolver) ResolveVoiceRoomAccess(_ context.Context, voiceRoomID, profileID string) (CanonicalVoiceRoomAccess, error) {
	spaceID, active := r.rooms[voiceRoomID]
	return CanonicalVoiceRoomAccess{
		SpaceID: spaceID, Active: active, Member: r.members[spaceID][profileID], AccessEpoch: r.accessEpoch,
	}, nil
}

func configureReadySpaceMediaFixture(svc *VoiceGRPC, resolver AuthoritativeVoiceRoomAccessResolver) {
	svc.SpaceMediaReady = true
	svc.SpaceMediaAdmissions = readySpaceMediaAdmission{}
	svc.SpaceTokens = livekit.NewSpaceTokenIssuer("test-key", "test-secret", "ws://livekit.test", time.Minute)
	svc.VoiceRoomAccessResolver = resolver
	svc.SpaceVoiceRoomGrants = staticSpaceMediaGrants{grants: CanonicalVoiceRoomGrants{
		PolicyEpoch: 7, CanJoin: true, CanPublishAudio: false, CanSubscribe: true,
	}}
	svc.SessionEpochChecker = currentSessionCheckerFunc(func(_ context.Context, accountID string, epoch int64) error {
		account, err := uuid.Parse(accountID)
		if err != nil || account == uuid.Nil || account.String() != accountID || epoch != 9 {
			return errors.New("unexpected test session authority")
		}
		return nil
	})
}

func TestGetJoinToken_SpaceIssuesShortServerBoundIncarnationIdentity(t *testing.T) {
	spaceID, roomID, profileID, accountID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	svc.SpaceMediaReady = true
	svc.SpaceMediaAdmissions = readySpaceMediaAdmission{}
	svc.VoiceRoomAccessResolver = &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{
		SpaceID: spaceID, Member: true, Active: true, AccessEpoch: 11,
	}}
	svc.SpaceVoiceRoomGrants = staticSpaceMediaGrants{grants: CanonicalVoiceRoomGrants{
		PolicyEpoch: 7, CanJoin: true, CanPublishAudio: false, CanSubscribe: true,
	}}
	svc.SpaceTokens = livekit.NewSpaceTokenIssuer("test-key", "test-secret", "ws://livekit.test", time.Minute)
	call, err := svc.Calls.CreateCall(t.Context(), voicestore.Call{
		RoomID: uuid.NewString(), LivekitRoomName: "voice-room-" + roomID,
		VoiceRoomID: roomID, SpaceID: spaceID,
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: profileID, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status: callsv1.CallStatus_CALL_STATUS_ACTIVE, StartedAt: time.Unix(1700000000, 0).UTC(),
	})
	require.NoError(t, err)
	generation := uuid.NewString()
	identity, err := livekit.SpaceParticipantIdentity(profileID, generation)
	require.NoError(t, err)
	_, err = svc.Calls.AdmitSpaceMediaParticipant(t.Context(), call.RoomID, voicestore.SpaceMediaParticipant{
		AccountID: accountID, AdmissionOperationID: uuid.NewString(), ProfileID: profileID,
		Identity: identity, Generation: generation,
		Issued: voicestore.SpaceMediaGrant{SessionEpoch: 9, AccessEpoch: 11, PolicyEpoch: 7, CanJoin: true, CanSubscribe: true},
	}, voicestore.MaxVoiceRoomParticipants)
	require.NoError(t, err)
	svc.SessionEpochChecker = currentSessionCheckerFunc(func(_ context.Context, gotAccount string, gotEpoch int64) error {
		require.Equal(t, accountID, gotAccount)
		require.Equal(t, int64(9), gotEpoch)
		return nil
	})
	request := &callsv1.GetJoinTokenRequest{RoomId: call.RoomID}
	ctx := verifiedVoiceUserContext(t, callsv1.VoiceService_GetJoinToken_FullMethodName, request, accountID, profileID, 9)
	response, err := svc.GetJoinToken(ctx, request)
	require.NoError(t, err)
	require.NotEmpty(t, response.GetJwt())
	require.Equal(t, "ws://livekit.test", response.GetLivekitUrl())
	require.LessOrEqual(t, response.GetExpiresAt().AsTime().Sub(time.Unix(1700000000, 0)).Seconds(), 60.0)

	parts := strings.Split(response.GetJwt(), ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims struct {
		Subject   string `json:"sub"`
		ProfileID string `json:"profile_id"`
		Video     struct {
			CanPublish bool `json:"canPublish"`
		} `json:"video"`
	}
	require.NoError(t, json.Unmarshal(payload, &claims))
	stored, err := svc.Calls.GetCall(t.Context(), call.RoomID)
	require.NoError(t, err)
	participant := stored.SpaceMedia[profileID]
	wantIdentity, err := livekit.SpaceParticipantIdentity(profileID, participant.Generation)
	require.NoError(t, err)
	require.Equal(t, wantIdentity, claims.Subject)
	require.Equal(t, profileID, claims.ProfileID)
	require.False(t, claims.Video.CanPublish)
	require.Equal(t, participant.Identity, claims.Subject)
	floors, err := svc.Calls.GetSpaceMediaEpochFloors(t.Context(), spaceID)
	require.NoError(t, err)
	require.Equal(t, voicestore.SpaceMediaEpochFloors{AccessEpoch: 11, PolicyEpoch: 7}, floors)
}

func verifiedVoiceUserContext(t *testing.T, method string, request proto.Message, accountID, profileID string, epoch int64) context.Context {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	requestID := uuid.NewString()
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{
		Audience: "voice", RPC: method, RequestID: requestID, RequestHash: hash,
		AccountID: accountID, ProfileID: profileID, SessionEpoch: epoch,
		ClientExpiresAt: time.Now().Add(45 * time.Second),
	})
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer "+token, "x-request-id", requestID,
	))
	var verifiedContext context.Context
	interceptor := voiceuserprincipalruntime.StrictUnaryInterceptor(strictTestVoiceUserVerifier{
		key: &key.PublicKey, accountID: accountID, epoch: epoch,
	})
	_, err = interceptor(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
		verifiedContext = ctx
		return nil, nil
	})
	require.NoError(t, err)
	require.NotNil(t, verifiedContext)
	return verifiedContext
}

type strictTestVoiceUserVerifier struct {
	key       *rsa.PublicKey
	accountID string
	epoch     int64
}

func (v strictTestVoiceUserVerifier) Verify(ctx context.Context, token, method, requestID, requestHash string) (principal.Principal, error) {
	return principal.VerifyDelegatedUser(ctx, token, principal.VerifyConfig{
		ExpectedIssuer: "gateway", ExpectedAudience: "voice", ExpectedRPC: method,
		ExpectedRequestID: requestID, ExpectedRequestHash: requestHash,
		KeyResolver: func(_ context.Context, issuerName, keyID string) (*rsa.PublicKey, error) {
			if issuerName != "gateway" || keyID != "test" {
				return nil, errors.New("unexpected test signing key")
			}
			return v.key, nil
		},
		ReplayGuard: func(context.Context, string, string, time.Time) error { return nil },
		SessionEpochChecker: func(_ context.Context, accountID string, epoch int64) error {
			if accountID != v.accountID || epoch != v.epoch {
				return errors.New("unexpected test session epoch")
			}
			return nil
		},
	})
}

func joinSpaceVoiceUser(t *testing.T, svc *VoiceGRPC, profileID string, request *callsv1.JoinVoiceRoomRequest) (*callsv1.JoinVoiceRoomResponse, error) {
	t.Helper()
	return joinSpaceVoiceUserAs(t, svc, fixtureAccountID(profileID), profileID, request)
}

func fixtureAccountID(profileID string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("voice-test-account:"+profileID)).String()
}

func joinSpaceVoiceUserAs(t *testing.T, svc *VoiceGRPC, accountID, profileID string, request *callsv1.JoinVoiceRoomRequest) (*callsv1.JoinVoiceRoomResponse, error) {
	t.Helper()
	return svc.JoinVoiceRoom(verifiedVoiceUserContext(t, callsv1.VoiceService_JoinVoiceRoom_FullMethodName, request, accountID, profileID, 9), request)
}

func TestJoinVoiceRoom_SpaceMediaAdmissionReturnsConfirmedSpaceSession(t *testing.T) {
	spaceID, voiceRoomID, accountID, profileID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	events := &recordingEvents{}
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), events)
	svc.SpaceMediaReady = true
	svc.SpaceMediaAdmissions = readySpaceMediaAdmission{}
	svc.VoiceRoomAccessResolver = &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{
		SpaceID: spaceID, Member: true, Active: true, AccessEpoch: 11,
	}}
	svc.Roles = &canonicalRolePermissions{}
	svc.SpaceVoiceRoomGrants = staticSpaceMediaGrants{grants: CanonicalVoiceRoomGrants{
		PolicyEpoch: 7, CanJoin: true, CanPublishAudio: false, CanSubscribe: true,
	}}
	svc.SessionEpochChecker = currentSessionCheckerFunc(func(_ context.Context, gotAccount string, gotEpoch int64) error {
		if gotAccount != accountID || gotEpoch != 9 {
			return errors.New("unexpected test session")
		}
		return nil
	})
	request := &callsv1.JoinVoiceRoomRequest{VoiceRoomId: voiceRoomID, Space: &spacev1.SpaceRef{Id: spaceID}}
	ctx := verifiedVoiceUserContext(t, callsv1.VoiceService_JoinVoiceRoom_FullMethodName, request, accountID, profileID, 9)
	response, err := svc.JoinVoiceRoom(ctx, request)
	require.NoError(t, err)
	require.Equal(t, voiceRoomID, response.GetVoiceSession().GetVoiceRoomId())
	require.Equal(t, spaceID, response.GetVoiceSession().GetSpaceId())
	call, err := svc.Calls.GetCall(t.Context(), response.GetVoiceSession().GetRoomId())
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ACTIVE, call.Status)
	participant, ok := call.SpaceMedia[profileID]
	require.True(t, ok)
	require.Equal(t, accountID, participant.AccountID)
	require.True(t, participant.Issued.CanJoin)
	require.True(t, participant.Issued.CanSubscribe)
	require.Equal(t, uint64(9), participant.Issued.SessionEpoch)
	require.Len(t, events.startedCall, 1)
	require.Len(t, events.memberJoined, 1)
}

func TestGetJoinToken_SpaceMediaReadinessFailsClosed(t *testing.T) {
	svc := &VoiceGRPC{}
	_, err := svc.getSpaceMediaJoinToken(context.Background(), voicestore.Call{}, "profile", CanonicalVoiceRoomAccess{})
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestGetJoinToken_SpaceMediaAdmissionSchemaFailsClosed(t *testing.T) {
	svc := &VoiceGRPC{SpaceMediaReady: true, SpaceMediaAdmissions: readySpaceMediaAdmission{err: fmt.Errorf("schema missing")}}
	_, err := svc.getSpaceMediaJoinToken(context.Background(), voicestore.Call{}, "profile", CanonicalVoiceRoomAccess{})
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestSpaceMediaPublicReadsFailClosedUntilReconciliationIsReady(t *testing.T) {
	profileID, spaceID, voiceRoomID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	calls := voicestore.NewMemoryCallStore()
	generation := uuid.NewString()
	identity, err := livekit.SpaceParticipantIdentity(profileID, generation)
	require.NoError(t, err)
	call, err := calls.CreateCall(t.Context(), voicestore.Call{
		RoomID: uuid.NewString(), VoiceRoomID: voiceRoomID, SpaceID: spaceID,
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: profileID, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status: callsv1.CallStatus_CALL_STATUS_ACTIVE,
		States: map[string]voicestore.ParticipantState{profileID: {}},
	})
	require.NoError(t, err)
	_, err = calls.AdmitSpaceMediaParticipant(t.Context(), call.RoomID, voicestore.SpaceMediaParticipant{
		AccountID: uuid.NewString(), AdmissionOperationID: uuid.NewString(), ProfileID: profileID,
		Identity: identity, Generation: generation, RoomGeneration: 1, CreatedRoom: true,
		Issued: voicestore.SpaceMediaGrant{SessionEpoch: 1, AccessEpoch: 1, PolicyEpoch: 1, CanJoin: true, CanSubscribe: true},
	}, voicestore.MaxVoiceRoomParticipants)
	require.NoError(t, err)
	ready := &atomic.Bool{}
	svc := &VoiceGRPC{Calls: calls, SpaceMediaReady: true, SpaceMediaServing: ready, SpaceMediaAdmissions: readySpaceMediaAdmission{}}

	_, activeErr := svc.GetActiveCall(voiceTestCtx(profileID), &callsv1.GetActiveCallRequest{})
	_, statesErr := svc.GetVoiceStates(voiceTestCtx(profileID), &callsv1.GetVoiceStatesRequest{RoomId: call.RoomID})
	require.Equal(t, codes.Unavailable, status.Code(activeErr))
	require.Equal(t, codes.Unavailable, status.Code(statesErr))

	ready.Store(true)
	active, activeErr := svc.GetActiveCall(voiceTestCtx(profileID), &callsv1.GetActiveCallRequest{})
	states, statesErr := svc.GetVoiceStates(voiceTestCtx(profileID), &callsv1.GetVoiceStatesRequest{RoomId: call.RoomID})
	require.NoError(t, activeErr)
	require.Equal(t, call.RoomID, active.GetCallSession().GetRoomId())
	require.NoError(t, statesErr)
	require.Len(t, states.GetParticipants(), 1)

	svc.SpaceMediaAdmissions = readySpaceMediaAdmission{projectionErr: fmt.Errorf("projection is still pending")}
	_, statesErr = svc.GetVoiceStates(voiceTestCtx(profileID), &callsv1.GetVoiceStatesRequest{RoomId: call.RoomID})
	require.Equal(t, codes.Unavailable, status.Code(statesErr), "Redis membership alone cannot authorize a public Space roster read")
	muted := true
	_, updateErr := svc.UpdateVoiceState(voiceTestCtx(profileID), &callsv1.UpdateVoiceStateRequest{RoomId: call.RoomID, IsMuted: &muted})
	require.Equal(t, codes.Unavailable, status.Code(updateErr), "an unconfirmed projection cannot authorize Space participant mutation")
	unchanged, err := calls.GetCall(t.Context(), call.RoomID)
	require.NoError(t, err)
	require.False(t, unchanged.States[profileID].IsMuted)
}

func TestLeaveSpaceVoiceRoomFailsClosedWhenParticipantHasNoAdmission(t *testing.T) {
	profileID, spaceID, voiceRoomID, roomID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	calls := voicestore.NewMemoryCallStore()
	call, err := calls.CreateCall(t.Context(), voicestore.Call{
		RoomID: roomID, VoiceRoomID: voiceRoomID, SpaceID: spaceID,
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: profileID, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status: callsv1.CallStatus_CALL_STATUS_ACTIVE,
		States: map[string]voicestore.ParticipantState{profileID: {}},
	})
	require.NoError(t, err)
	svc := &VoiceGRPC{Calls: calls}
	_, err = svc.LeaveVoiceRoom(voiceTestCtx(profileID), &callsv1.LeaveVoiceRoomRequest{VoiceRoomId: voiceRoomID})
	require.Equal(t, codes.Unavailable, status.Code(err))
	current, err := calls.GetCall(t.Context(), call.RoomID)
	require.NoError(t, err)
	require.True(t, current.IsParticipant(profileID), "an unjournaled Space projection cannot use legacy leave mutation")
}
