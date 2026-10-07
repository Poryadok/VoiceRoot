package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
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

func (r staticSpaceMediaGrants) ResolveVoiceRoomGrants(context.Context, string, string, string) (CanonicalVoiceRoomGrants, error) {
	return r.grants, r.err
}

func TestGetJoinToken_SpaceIssuesShortServerBoundIncarnationIdentity(t *testing.T) {
	spaceID, roomID, profileID, accountID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	svc.SpaceMediaReady = true
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
