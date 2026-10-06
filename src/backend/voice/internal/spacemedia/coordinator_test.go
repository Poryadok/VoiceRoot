package spacemedia

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/grpcsvc"
	"voice/backend/voice/internal/store"
)

type fakeAccess struct {
	byProfile map[string]grpcsvc.CanonicalVoiceRoomAccess
}

func (f fakeAccess) ResolveVoiceRoomAccess(_ context.Context, _, profileID string) (grpcsvc.CanonicalVoiceRoomAccess, error) {
	return f.byProfile[profileID], nil
}

type fakeGrants struct {
	byProfile map[string]grpcsvc.CanonicalVoiceRoomGrants
}

func (f fakeGrants) ResolveVoiceRoomGrants(_ context.Context, _, _, profileID string) (grpcsvc.CanonicalVoiceRoomGrants, error) {
	return f.byProfile[profileID], nil
}

type fakeMedia struct {
	removed []string
	err     error
}

func (f *fakeMedia) RemoveParticipant(_ context.Context, _, identity string) error {
	f.removed = append(f.removed, identity)
	return f.err
}

func TestObserveIgnoresNarrowHintAndReconcilesEveryParticipant(t *testing.T) {
	ctx := context.Background()
	spaceID, roomID, voiceRoomID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	profileA, profileB := uuid.NewString(), uuid.NewString()
	calls := newRoomStore(t, ctx, spaceID, roomID, voiceRoomID, profileA, profileB)
	initial, err := calls.GetCall(ctx, roomID)
	require.NoError(t, err)
	identityB := initial.SpaceMedia[profileB].Identity
	access := fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{
		profileA: {SpaceID: spaceID, Member: true, Active: true, AccessEpoch: 4},
		profileB: {SpaceID: spaceID, Member: false, Active: true, AccessEpoch: 4},
	}}
	grants := fakeGrants{byProfile: map[string]grpcsvc.CanonicalVoiceRoomGrants{
		profileA: {PolicyEpoch: 7, CanJoin: true, CanSubscribe: true, CanPublishAudio: true},
		profileB: {PolicyEpoch: 7, CanJoin: true, CanSubscribe: true, CanPublishAudio: true},
	}}
	media := &fakeMedia{}
	c := &Coordinator{Store: calls, Access: access, Grants: grants, Media: media}

	err := c.Observe(ctx, AuthorityNotice{SpaceID: spaceID, Kind: store.SpaceAccessEpoch, Epoch: 4, ProfileID: profileA})

	require.NoError(t, err)
	require.Equal(t, []string{identityB}, media.removed, "a narrow hint must not hide the other profile")
	updated, err := calls.GetCall(ctx, roomID)
	require.NoError(t, err)
	require.NotContains(t, updated.SpaceMedia, profileB)
	require.Contains(t, updated.SpaceMedia, profileA)
	progress, err := calls.GetSpaceMediaEpochProgress(ctx, spaceID)
	require.NoError(t, err)
	require.Equal(t, store.SpaceMediaEpochFloors{AccessEpoch: 4, PolicyEpoch: 7}, progress.Observed)
	require.Equal(t, progress.Observed, progress.Reconciled)
}

func TestDelayedInvalidationUsesCurrentRestoredRights(t *testing.T) {
	ctx := context.Background()
	spaceID, roomID, voiceRoomID, profile := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	calls := newRoomStore(t, ctx, spaceID, roomID, voiceRoomID, profile)
	media := &fakeMedia{}
	c := &Coordinator{
		Store: calls,
		Access: fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{
			profile: {SpaceID: spaceID, Member: true, Active: true, AccessEpoch: 9},
		}},
		Grants: fakeGrants{byProfile: map[string]grpcsvc.CanonicalVoiceRoomGrants{
			profile: {PolicyEpoch: 12, CanJoin: true, CanSubscribe: true, CanPublishAudio: true},
		}},
		Media: media,
	}

	err := c.Observe(ctx, AuthorityNotice{SpaceID: spaceID, Kind: store.RolePolicyEpoch, Epoch: 10})

	require.NoError(t, err)
	require.Empty(t, media.removed, "a stale denial edge must not override currently restored rights")
	call, err := calls.GetCall(ctx, roomID)
	require.NoError(t, err)
	require.True(t, call.SpaceMedia[profile].Reconciled.CanJoin)
	require.Equal(t, uint64(12), call.SpaceMedia[profile].Reconciled.PolicyEpoch)
}

func TestFailedTargetRemovalLeavesReconciliationIncomplete(t *testing.T) {
	ctx := context.Background()
	spaceID, roomID, voiceRoomID, profile := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	calls := newRoomStore(t, ctx, spaceID, roomID, voiceRoomID, profile)
	media := &fakeMedia{err: errors.New("injected ejection failure")}
	c := &Coordinator{
		Store: calls,
		Access: fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{
			profile: {SpaceID: spaceID, Member: false, Active: true, AccessEpoch: 2},
		}},
		Grants: fakeGrants{byProfile: map[string]grpcsvc.CanonicalVoiceRoomGrants{}},
		Media:  media,
	}

	err := c.Observe(ctx, AuthorityNotice{SpaceID: spaceID, Kind: store.SpaceAccessEpoch, Epoch: 2})

	require.Error(t, err)
	progress, progressErr := calls.GetSpaceMediaEpochProgress(ctx, spaceID)
	require.NoError(t, progressErr)
	require.Equal(t, uint64(2), progress.Observed.AccessEpoch)
	require.Zero(t, progress.Reconciled.AccessEpoch, "failed external effect cannot advance completion watermark")
	call, getErr := calls.GetCall(ctx, roomID)
	require.NoError(t, getErr)
	require.True(t, call.SpaceMedia[profile].Revoking, "retry must retain the exact in-flight revocation")
}

func TestOutOfOrderAuthoritiesKeepIndependentMonotonicFloors(t *testing.T) {
	ctx := context.Background()
	spaceID := uuid.NewString()
	calls := store.NewMemoryCallStore()
	_, err := calls.RaiseSpaceMediaEpochFloor(ctx, spaceID, store.SpaceAccessEpoch, 8)
	require.NoError(t, err)
	_, err = calls.RaiseSpaceMediaEpochFloor(ctx, spaceID, store.RolePolicyEpoch, 3)
	require.NoError(t, err)
	_, err = calls.RaiseSpaceMediaEpochFloor(ctx, spaceID, store.RolePolicyEpoch, 9)
	require.NoError(t, err)
	_, err = calls.RaiseSpaceMediaEpochFloor(ctx, spaceID, store.SpaceAccessEpoch, 2)
	require.NoError(t, err)
	progress, err := calls.GetSpaceMediaEpochProgress(ctx, spaceID)
	require.NoError(t, err)
	require.Equal(t, store.SpaceMediaEpochFloors{AccessEpoch: 8, PolicyEpoch: 9}, progress.Observed)
	require.Zero(t, progress.Reconciled.AccessEpoch)
	require.Zero(t, progress.Reconciled.PolicyEpoch)
}

func TestSpaceMediaLeaveEjectionFailureRetainsRevocationTarget(t *testing.T) {
	ctx := context.Background()
	spaceID, roomID, voiceRoomID, profile := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	calls := newRoomStore(t, ctx, spaceID, roomID, voiceRoomID, profile, uuid.NewString())
	call, err := calls.GetCall(ctx, roomID)
	require.NoError(t, err)
	participant := call.SpaceMedia[profile]
	media := &fakeMedia{err: errors.New("injected ejection failure")}
	c := &Coordinator{Store: calls, Media: media}

	_, removed, err := c.RevokeSpaceMediaParticipant(ctx, call, participant)
	require.Error(t, err)
	require.False(t, removed)
	require.Equal(t, []string{participant.Identity}, media.removed)

	current, err := calls.GetCall(ctx, roomID)
	require.NoError(t, err)
	require.True(t, current.SpaceMedia[profile].Revoking, "a failed ejection must remain discoverable for retry")
	require.Equal(t, participant.Identity, current.SpaceMedia[profile].Identity)
	require.Equal(t, participant.Generation, current.SpaceMedia[profile].Generation)
	require.True(t, current.IsParticipant(profile), "failed ejection must not erase the roster target")
}

func TestSpaceMediaLeaveWithStaleGenerationCannotEjectNewIncarnation(t *testing.T) {
	ctx := context.Background()
	spaceID, roomID, voiceRoomID, profile := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	calls := newRoomStore(t, ctx, spaceID, roomID, voiceRoomID, profile, uuid.NewString())
	oldCall, err := calls.GetCall(ctx, roomID)
	require.NoError(t, err)
	old := oldCall.SpaceMedia[profile]
	_, matched, err := calls.BeginSpaceMediaRevocation(ctx, roomID, profile, old.Identity, old.Generation)
	require.NoError(t, err)
	require.True(t, matched)
	_, matched, err = calls.CompleteSpaceMediaRevocation(ctx, roomID, profile, old.Identity, old.Generation)
	require.NoError(t, err)
	require.True(t, matched)
	newGeneration := uuid.NewString()
	newIdentity := mediaIdentity(profile) + "-next"
	_, err = calls.AdmitSpaceMediaParticipant(ctx, roomID, store.SpaceMediaParticipant{
		ProfileID: profile, Identity: newIdentity, Generation: newGeneration,
		Issued: store.SpaceMediaGrant{SessionEpoch: 1, AccessEpoch: 2, PolicyEpoch: 2, CanJoin: true, CanSubscribe: true},
	}, store.MaxSpaceProVoiceParticipants)
	require.NoError(t, err)
	media := &fakeMedia{}
	c := &Coordinator{Store: calls, Media: media}

	_, removed, err := c.RevokeSpaceMediaParticipant(ctx, oldCall, old)
	require.NoError(t, err)
	require.False(t, removed)
	require.Empty(t, media.removed, "a stale leave must not eject the current incarnation")

	current, err := calls.GetCall(ctx, roomID)
	require.NoError(t, err)
	require.Equal(t, newIdentity, current.SpaceMedia[profile].Identity)
	require.Equal(t, newGeneration, current.SpaceMedia[profile].Generation)
	require.True(t, current.IsParticipant(profile))
}

func newRoomStore(t *testing.T, ctx context.Context, spaceID, roomID, voiceRoomID string, profiles ...string) *store.MemoryCallStore {
	t.Helper()
	calls := store.NewMemoryCallStore()
	_, err := calls.CreateCall(ctx, store.Call{
		RoomID: roomID, LivekitRoomName: "lk-" + roomID, VoiceRoomID: voiceRoomID, SpaceID: spaceID,
		SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		MediaKind:   callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:      callsv1.CallStatus_CALL_STATUS_ACTIVE, States: map[string]store.ParticipantState{},
	})
	require.NoError(t, err)
	for _, profile := range profiles {
		_, err := calls.AdmitSpaceMediaParticipant(ctx, roomID, store.SpaceMediaParticipant{
			ProfileID: profile, Identity: mediaIdentity(profile), Generation: uuid.NewString(),
			Issued:     store.SpaceMediaGrant{SessionEpoch: 1, AccessEpoch: 1, PolicyEpoch: 1, CanJoin: true, CanSubscribe: true, CanPublishAudio: true},
			Reconciled: store.SpaceMediaGrant{SessionEpoch: 1, AccessEpoch: 1, PolicyEpoch: 1, CanJoin: true, CanSubscribe: true, CanPublishAudio: true},
		}, store.MaxSpaceProVoiceParticipants)
		require.NoError(t, err)
	}
	return calls
}

func mediaIdentity(profileID string) string {
	return "profile:" + profileID + ":media:" + uuid.NewString()
}
