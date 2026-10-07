package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/authctx"
	"voice/backend/voice/internal/livekit"
	voicestore "voice/backend/voice/internal/store"

	callsv1 "voice.app/voice/calls/v1"
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
	ctx := verifiedVoiceUserContext(t, request, accountID, profileID, 9)
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

func verifiedVoiceUserContext(t *testing.T, request *callsv1.GetJoinTokenRequest, accountID, profileID string, epoch int64) context.Context {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	requestID := uuid.NewString()
	expiresAt := time.Now().Add(45 * time.Second)
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{
		Audience: "voice", RPC: callsv1.VoiceService_GetJoinToken_FullMethodName,
		RequestID: requestID, RequestHash: hash, AccountID: accountID, ProfileID: profileID,
		SessionEpoch: epoch, ClientExpiresAt: expiresAt,
	})
	require.NoError(t, err)
	verified, err := principal.VerifyDelegatedUser(context.Background(), token, principal.VerifyConfig{
		ExpectedIssuer: "gateway", ExpectedAudience: "voice",
		ExpectedRPC:       callsv1.VoiceService_GetJoinToken_FullMethodName,
		ExpectedRequestID: requestID, ExpectedRequestHash: hash,
		KeyResolver: func(_ context.Context, issuerName, keyID string) (*rsa.PublicKey, error) {
			require.Equal(t, "gateway", issuerName)
			require.Equal(t, "test", keyID)
			return &key.PublicKey, nil
		},
		ReplayGuard: func(context.Context, string, string, time.Time) error { return nil },
		SessionEpochChecker: func(_ context.Context, gotAccount string, gotEpoch int64) error {
			require.Equal(t, accountID, gotAccount)
			require.Equal(t, epoch, gotEpoch)
			return nil
		},
	})
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		authctx.HeaderAccountID, accountID,
		authctx.HeaderProfileID, profileID,
		authctx.HeaderSessionEpoch, fmt.Sprint(epoch),
	))
	return principal.WithVerified(ctx, verified)
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
