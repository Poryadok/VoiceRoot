package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/voice/internal/livekit"
	voicestore "voice/backend/voice/internal/store"

	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
)

type canonicalAccessResolver struct {
	result CanonicalVoiceRoomAccess
	err    error
	calls  []canonicalAccessCall
}
type canonicalAccessCall struct{ voiceRoomID, profileID string }

func (r *canonicalAccessResolver) ResolveVoiceRoomAccess(_ context.Context, roomID, profileID string) (CanonicalVoiceRoomAccess, error) {
	r.calls = append(r.calls, canonicalAccessCall{roomID, profileID})
	return r.result, r.err
}

var _ AuthoritativeVoiceRoomAccessResolver = (*canonicalAccessResolver)(nil)

type poisonLegacySpaceMembers struct{ calls int }

func (m *poisonLegacySpaceMembers) EnsureMember(context.Context, string, string) error {
	m.calls++
	return nil
}

type canonicalRolePermissions struct{ joinChecks, speakChecks []voiceRolePermissionCheck }

func (*canonicalRolePermissions) EnsureScreenShare(context.Context, string, string, string) error {
	return nil
}
func (r *canonicalRolePermissions) EnsureVoiceJoin(_ context.Context, spaceID, profileID, roomID string) error {
	r.joinChecks = append(r.joinChecks, voiceRolePermissionCheck{spaceID, profileID, roomID})
	return nil
}
func (r *canonicalRolePermissions) EnsureVoiceSpeak(_ context.Context, spaceID, profileID, roomID string) error {
	r.speakChecks = append(r.speakChecks, voiceRolePermissionCheck{spaceID, profileID, roomID})
	return nil
}
func (*canonicalRolePermissions) EnsureMuteOthers(context.Context, string, string, string) error {
	return nil
}

type canonicalTokenIssuer struct{ joinCalls int }

func (i *canonicalTokenIssuer) JoinToken(string, string, *bool, time.Time) (string, time.Time, error) {
	i.joinCalls++
	return "canonical-token", time.Unix(1700003600, 0).UTC(), nil
}
func (*canonicalTokenIssuer) LivekitURL() string { return "ws://livekit.test" }

var _ livekit.TokenIssuer = (*canonicalTokenIssuer)(nil)

func TestJoinVoiceRoom_RequiresMatchingCanonicalSpaceAssertion(t *testing.T) {
	canonicalSpaceID, forgedSpaceID, voiceRoomID, profileID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, tc := range []struct {
		name, assertion string
		want            codes.Code
	}{{"matching assertion authorizes", canonicalSpaceID, codes.OK}, {"forged A B pairing is denied", forgedSpaceID, codes.PermissionDenied}} {
		t.Run(tc.name, func(t *testing.T) {
			events := &recordingEvents{}
			svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), events)
			resolver := &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{SpaceID: canonicalSpaceID, Member: true, Active: true}}
			roles, legacy := &canonicalRolePermissions{}, &poisonLegacySpaceMembers{}
			svc.VoiceRoomAccessResolver, svc.Roles, svc.SpaceMembers = resolver, roles, legacy
			response, err := svc.JoinVoiceRoom(voiceTestCtx(profileID), &callsv1.JoinVoiceRoomRequest{VoiceRoomId: voiceRoomID, Space: &spacev1.SpaceRef{Id: tc.assertion}})
			require.Equal(t, tc.want, status.Code(err))
			require.Equal(t, []canonicalAccessCall{{voiceRoomID, profileID}}, resolver.calls)
			require.Zero(t, legacy.calls)
			if tc.want != codes.OK {
				require.Nil(t, response)
				require.Empty(t, roles.joinChecks)
				require.Empty(t, events.startedCall)
				_, e := svc.Calls.GetCallByVoiceRoomID(t.Context(), voiceRoomID)
				require.ErrorIs(t, e, voicestore.ErrNotFound)
				return
			}
			require.NoError(t, err)
			requireVoiceRoleCheck(t, roles.joinChecks, canonicalSpaceID, profileID, voiceRoomID)
			call, e := svc.Calls.GetCall(t.Context(), response.GetVoiceSession().GetRoomId())
			require.NoError(t, e)
			require.Equal(t, canonicalSpaceID, call.SpaceID)
		})
	}
}

func TestJoinVoiceRoom_CanonicalResolverFailuresHaveNoSideEffects(t *testing.T) {
	for _, tc := range []struct {
		name     string
		resolver AuthoritativeVoiceRoomAccessResolver
		want     codes.Code
	}{{"missing resolver", nil, codes.FailedPrecondition}, {"nonmember", &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{SpaceID: uuid.NewString(), Active: true}}, codes.PermissionDenied}, {"inactive", &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{SpaceID: uuid.NewString(), Member: true}}, codes.PermissionDenied}, {"malformed", &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{Member: true, Active: true}}, codes.PermissionDenied}, {"unavailable", &canonicalAccessResolver{err: status.Error(codes.Unavailable, "space")}, codes.Unavailable}} {
		t.Run(tc.name, func(t *testing.T) {
			room, profile := uuid.NewString(), uuid.NewString()
			events := &recordingEvents{}
			svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), events)
			roles, legacy := &canonicalRolePermissions{}, &poisonLegacySpaceMembers{}
			svc.VoiceRoomAccessResolver, svc.Roles, svc.SpaceMembers = tc.resolver, roles, legacy
			got, err := svc.JoinVoiceRoom(voiceTestCtx(profile), &callsv1.JoinVoiceRoomRequest{VoiceRoomId: room, Space: &spacev1.SpaceRef{Id: uuid.NewString()}})
			require.Nil(t, got)
			require.Equal(t, tc.want, status.Code(err))
			require.Zero(t, legacy.calls)
			require.Empty(t, roles.joinChecks)
			require.Empty(t, events.startedCall)
			require.Empty(t, events.memberJoined)
			_, callErr := svc.Calls.GetCallByVoiceRoomID(t.Context(), room)
			require.ErrorIs(t, callErr, voicestore.ErrNotFound)
		})
	}
}

func TestJoinVoiceRoom_CanonicalNotFoundHasNoSideEffects(t *testing.T) {
	spaceID, voiceRoomID, profileID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	events := &recordingEvents{}
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), events)
	resolver := &canonicalAccessResolver{err: status.Error(codes.NotFound, "room")}
	roles, legacy := &canonicalRolePermissions{}, &poisonLegacySpaceMembers{}
	svc.VoiceRoomAccessResolver, svc.Roles, svc.SpaceMembers = resolver, roles, legacy

	response, err := svc.JoinVoiceRoom(voiceTestCtx(profileID), &callsv1.JoinVoiceRoomRequest{VoiceRoomId: voiceRoomID, Space: &spacev1.SpaceRef{Id: spaceID}})
	require.Nil(t, response)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Equal(t, []canonicalAccessCall{{voiceRoomID, profileID}}, resolver.calls)
	require.Zero(t, legacy.calls)
	require.Empty(t, roles.joinChecks)
	require.Empty(t, events.startedCall)
	require.Empty(t, events.memberJoined)
	_, callErr := svc.Calls.GetCallByVoiceRoomID(t.Context(), voiceRoomID)
	require.ErrorIs(t, callErr, voicestore.ErrNotFound)
}

func TestJoinVoiceRoom_RejectsStoredCallSpaceMismatchBeforeParticipantOrEvent(t *testing.T) {
	canonicalSpaceID, staleSpaceID, voiceRoomID, profileID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	events := &recordingEvents{}
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), events)
	resolver := &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{SpaceID: canonicalSpaceID, Member: true, Active: true}}
	roles, legacy := &canonicalRolePermissions{}, &poisonLegacySpaceMembers{}
	svc.VoiceRoomAccessResolver, svc.Roles, svc.SpaceMembers = resolver, roles, legacy
	_, err := svc.Calls.CreateCall(t.Context(), voicestore.Call{
		RoomID:             uuid.NewString(),
		LivekitRoomName:    "voice-room-" + voiceRoomID,
		VoiceRoomID:        voiceRoomID,
		SpaceID:            staleSpaceID,
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: uuid.NewString(),
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          time.Now(),
	})
	require.NoError(t, err)

	response, err := svc.JoinVoiceRoom(voiceTestCtx(profileID), &callsv1.JoinVoiceRoomRequest{VoiceRoomId: voiceRoomID, Space: &spacev1.SpaceRef{Id: canonicalSpaceID}})
	require.Nil(t, response)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, []canonicalAccessCall{{voiceRoomID, profileID}}, resolver.calls)
	requireVoiceRoleCheck(t, roles.joinChecks, canonicalSpaceID, profileID, voiceRoomID)
	require.Zero(t, legacy.calls)
	require.Empty(t, events.startedCall)
	require.Empty(t, events.memberJoined)
	call, err := svc.Calls.GetCallByVoiceRoomID(t.Context(), voiceRoomID)
	require.NoError(t, err)
	require.False(t, call.IsParticipant(profileID))
}

func TestJoinVoiceRoom_PreservesInputValidationBeforeResolver(t *testing.T) {
	resolver := &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{SpaceID: uuid.NewString(), Member: true, Active: true}}
	svc := newTestVoiceService(time.Now(), &recordingEvents{})
	svc.VoiceRoomAccessResolver = resolver
	for _, req := range []*callsv1.JoinVoiceRoomRequest{{Space: &spacev1.SpaceRef{Id: uuid.NewString()}}, {VoiceRoomId: "bad", Space: &spacev1.SpaceRef{Id: uuid.NewString()}}, {VoiceRoomId: uuid.NewString()}, {VoiceRoomId: uuid.NewString(), Space: &spacev1.SpaceRef{Id: "bad"}}} {
		_, err := svc.JoinVoiceRoom(voiceTestCtx(uuid.NewString()), req)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	require.Empty(t, resolver.calls)
}

func TestGetJoinToken_ReResolvesCanonicalRoomBeforeRoleAndMint(t *testing.T) {
	canonical, stored, room, profile := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, tc := range []struct {
		name, stored string
		access       CanonicalVoiceRoomAccess
		resolverErr  error
		want         codes.Code
	}{{"mints", canonical, CanonicalVoiceRoomAccess{SpaceID: canonical, Member: true, Active: true}, nil, codes.OK}, {"stored mismatch", stored, CanonicalVoiceRoomAccess{SpaceID: canonical, Member: true, Active: true}, nil, codes.PermissionDenied}, {"nonmember", canonical, CanonicalVoiceRoomAccess{SpaceID: canonical, Active: true}, nil, codes.PermissionDenied}, {"inactive", canonical, CanonicalVoiceRoomAccess{SpaceID: canonical, Member: true}, nil, codes.PermissionDenied}, {"unavailable", canonical, CanonicalVoiceRoomAccess{}, status.Error(codes.Unavailable, "space"), codes.Unavailable}} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
			resolver := &canonicalAccessResolver{result: tc.access, err: tc.resolverErr}
			roles, tokens, legacy := &canonicalRolePermissions{}, &canonicalTokenIssuer{}, &poisonLegacySpaceMembers{}
			svc.VoiceRoomAccessResolver, svc.Roles, svc.Tokens, svc.SpaceMembers = resolver, roles, tokens, legacy
			call, err := svc.Calls.CreateCall(t.Context(), voicestore.Call{RoomID: uuid.NewString(), LivekitRoomName: "voice-room-" + room, VoiceRoomID: room, SpaceID: tc.stored, SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, InitiatorProfileID: profile, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE, StartedAt: time.Now()})
			require.NoError(t, err)
			response, err := svc.GetJoinToken(voiceTestCtx(profile), &callsv1.GetJoinTokenRequest{RoomId: call.RoomID})
			require.Equal(t, tc.want, status.Code(err))
			require.Equal(t, []canonicalAccessCall{{room, profile}}, resolver.calls)
			require.Zero(t, legacy.calls)
			if tc.want == codes.OK {
				require.Equal(t, "canonical-token", response.GetJwt())
				requireVoiceRoleCheck(t, roles.joinChecks, canonical, profile, room)
				requireVoiceRoleCheck(t, roles.speakChecks, canonical, profile, room)
				require.Equal(t, 1, tokens.joinCalls)
			} else {
				require.Nil(t, response)
				require.Empty(t, roles.joinChecks)
				require.Empty(t, roles.speakChecks)
				require.Zero(t, tokens.joinCalls)
			}
		})
	}
}

func TestGetJoinToken_CanonicalNotFoundDoesNotMint(t *testing.T) {
	spaceID, voiceRoomID, profileID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	resolver := &canonicalAccessResolver{err: status.Error(codes.NotFound, "room")}
	roles, tokens, legacy := &canonicalRolePermissions{}, &canonicalTokenIssuer{}, &poisonLegacySpaceMembers{}
	svc.VoiceRoomAccessResolver, svc.Roles, svc.Tokens, svc.SpaceMembers = resolver, roles, tokens, legacy
	call, err := svc.Calls.CreateCall(t.Context(), voicestore.Call{
		RoomID:             uuid.NewString(),
		LivekitRoomName:    "voice-room-" + voiceRoomID,
		VoiceRoomID:        voiceRoomID,
		SpaceID:            spaceID,
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: profileID,
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          time.Now(),
	})
	require.NoError(t, err)

	response, err := svc.GetJoinToken(voiceTestCtx(profileID), &callsv1.GetJoinTokenRequest{RoomId: call.RoomID})
	require.Nil(t, response)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Equal(t, []canonicalAccessCall{{voiceRoomID, profileID}}, resolver.calls)
	require.Empty(t, roles.joinChecks)
	require.Empty(t, roles.speakChecks)
	require.Zero(t, tokens.joinCalls)
	require.Zero(t, legacy.calls)
}
