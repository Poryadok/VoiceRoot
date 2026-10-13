package spacemedia

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	callsv1 "voice.app/voice/calls/v1"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/voice/internal/grpcsvc"
	"voice/backend/voice/internal/store"
)

type recordingAccountFloors struct {
	floors map[string]int64
	reads  map[string]int
	err    error
}

func (f *recordingAccountFloors) CurrentFloor(_ context.Context, accountID string) (int64, error) {
	f.reads[accountID]++
	return f.floors[accountID], f.err
}

func (f *recordingAccountFloors) RequireCurrent(ctx context.Context, accountID string, epoch int64) error {
	floor, err := f.CurrentFloor(ctx, accountID)
	if err != nil {
		return err
	}
	if floor <= 0 {
		return status.Error(codes.Unavailable, "invalid test floor")
	}
	if epoch < floor {
		return status.Error(codes.Unauthenticated, "stale test epoch")
	}
	return nil
}

type floorParticipant struct {
	account string
	epoch   uint64
}

func floorPassFixture(t *testing.T, participants ...floorParticipant) (*Coordinator, *store.MemoryCallStore, store.Call, *recordingAccountFloors, *fakeMedia) {
	t.Helper()
	ctx := t.Context()
	spaceID, roomID, voiceRoomID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	calls := store.NewMemoryCallStore()
	_, err := calls.CreateCall(ctx, store.Call{
		RoomID: roomID, LivekitRoomName: "lk-" + roomID, SpaceID: spaceID, VoiceRoomID: voiceRoomID,
		SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		MediaKind:   callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE,
	})
	require.NoError(t, err)
	access := fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{}}
	grants := fakeGrants{byProfile: map[string]grpcsvc.CanonicalVoiceRoomGrants{}}
	floors := &recordingAccountFloors{floors: map[string]int64{}, reads: map[string]int{}}
	// Sort order places a current profile before a stale same-account profile,
	// so reusing boolean success would incorrectly retain the stale participant.
	profiles := []string{"00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002"}
	for i, participant := range participants {
		profileID := profiles[i]
		_, err = calls.AdmitSpaceMediaParticipant(ctx, roomID, store.SpaceMediaParticipant{
			AccountID: participant.account, ProfileID: profileID, AdmissionOperationID: uuid.NewString(),
			Identity: mediaIdentity(profileID), Generation: uuid.NewString(),
			Issued: store.SpaceMediaGrant{SessionEpoch: participant.epoch, AccessEpoch: 1, PolicyEpoch: 1, CanJoin: true, CanSubscribe: true},
		}, store.MaxVoiceRoomParticipants)
		require.NoError(t, err)
		access.byProfile[profileID] = grpcsvc.CanonicalVoiceRoomAccess{SpaceID: spaceID, Member: true, Active: true, AccessEpoch: 1}
		grants.byProfile[profileID] = grpcsvc.CanonicalVoiceRoomGrants{PolicyEpoch: 1, CanJoin: true, CanSubscribe: true}
		floors.floors[participant.account] = 3
	}
	_, err = calls.RaiseSpaceMediaEpochFloor(ctx, spaceID, store.SpaceAccessEpoch, 1)
	require.NoError(t, err)
	_, err = calls.RaiseSpaceMediaEpochFloor(ctx, spaceID, store.RolePolicyEpoch, 1)
	require.NoError(t, err)
	call, err := calls.GetCall(ctx, roomID)
	require.NoError(t, err)
	// MemoryCallStore returns a Call whose participant map can change during
	// reconciliation. Keep independent evidence of each exact incarnation and
	// of the roster that authority-failure tests require to remain unchanged.
	snapshot := make(map[string]store.SpaceMediaParticipant, len(call.SpaceMedia))
	for profileID, participant := range call.SpaceMedia {
		snapshot[profileID] = participant
	}
	call.SpaceMedia = snapshot
	media := &fakeMedia{}
	c := &Coordinator{Store: calls, Calls: calls, Access: access, Grants: grants, Media: media,
		Admissions: fakeRecoveryForCommittedCall(t, ctx, calls, roomID), SessionEpochChecker: floors}
	return c, calls, call, floors, media
}

type orderedFloorSpacesStore struct {
	Store
	spaces []string
}

func (s orderedFloorSpacesStore) ListActiveSpaceIDs(context.Context) ([]string, error) {
	return append([]string(nil), s.spaces...), nil
}

func floorSweepFixture(t *testing.T, firstEpoch, secondEpoch uint64) (*Coordinator, *store.MemoryCallStore, store.Call, store.Call, *recordingAccountFloors, *fakeMedia) {
	t.Helper()
	accountID := uuid.NewString()
	c, calls, first, floors, media := floorPassFixture(t, floorParticipant{accountID, firstEpoch})
	spaceID, roomID := uuid.NewString(), uuid.NewString()
	_, err := calls.CreateCall(t.Context(), store.Call{
		RoomID: roomID, LivekitRoomName: "lk-" + roomID, SpaceID: spaceID, VoiceRoomID: uuid.NewString(),
		SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		MediaKind:   callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE,
	})
	require.NoError(t, err)
	participant := first.SpaceMedia["00000000-0000-0000-0000-000000000001"]
	participant.ProfileID = "00000000-0000-0000-0000-000000000002"
	participant.Identity, participant.Generation, participant.AdmissionOperationID = mediaIdentity(participant.ProfileID), uuid.NewString(), uuid.NewString()
	participant.Issued.SessionEpoch = secondEpoch
	_, err = calls.AdmitSpaceMediaParticipant(t.Context(), roomID, participant, store.MaxVoiceRoomParticipants)
	require.NoError(t, err)
	access, grants := c.Access.(fakeAccess), c.Grants.(fakeGrants)
	access.byProfile[participant.ProfileID] = grpcsvc.CanonicalVoiceRoomAccess{SpaceID: spaceID, Member: true, Active: true, AccessEpoch: 1}
	grants.byProfile[participant.ProfileID] = grpcsvc.CanonicalVoiceRoomGrants{PolicyEpoch: 1, CanJoin: true, CanSubscribe: true}
	for _, admission := range fakeRecoveryForCommittedCall(t, t.Context(), calls, roomID).confirmed {
		c.Admissions.(*fakeAdmissionRecovery).seedConfirmedProjection(admission)
	}
	_, err = calls.RaiseSpaceMediaEpochFloor(t.Context(), spaceID, store.SpaceAccessEpoch, 1)
	require.NoError(t, err)
	_, err = calls.RaiseSpaceMediaEpochFloor(t.Context(), spaceID, store.RolePolicyEpoch, 1)
	require.NoError(t, err)
	second, err := calls.GetCall(t.Context(), roomID)
	require.NoError(t, err)
	c.Store = orderedFloorSpacesStore{Store: calls, spaces: []string{first.SpaceID, second.SpaceID}}
	return c, calls, first, second, floors, media
}

func TestReconcileAllSharesAccountFloorAcrossSpacesAndChecksEachEpoch(t *testing.T) {
	c, calls, first, second, floors, media := floorSweepFixture(t, 4, 1)
	current := first.SpaceMedia["00000000-0000-0000-0000-000000000001"]
	stale := second.SpaceMedia["00000000-0000-0000-0000-000000000002"]

	require.NoError(t, c.ReconcileAll(t.Context()))

	require.Equal(t, 1, floors.reads[current.AccountID])
	require.Equal(t, []string{stale.Identity}, media.removed)
	after, err := calls.GetCall(t.Context(), first.RoomID)
	require.NoError(t, err)
	require.Equal(t, current.Generation, after.SpaceMedia[current.ProfileID].Generation)
	floors.floors[current.AccountID] = 5
	require.NoError(t, c.ReconcileAll(t.Context()))
	require.Equal(t, 2, floors.reads[current.AccountID], "next full sweep must discard earlier floor values")
	after, err = calls.GetCall(t.Context(), first.RoomID)
	require.NoError(t, err)
	require.Empty(t, after.SpaceMedia)
}

func TestReconcileAllWatermarkRestartDiscardsSharedAccountFloor(t *testing.T) {
	c, calls, first, second, floors, media := floorSweepFixture(t, 4, 4)
	current := first.SpaceMedia["00000000-0000-0000-0000-000000000001"]
	stale := second.SpaceMedia["00000000-0000-0000-0000-000000000002"]
	laterSpace, laterRoom := uuid.NewString(), uuid.NewString()
	_, err := calls.CreateCall(t.Context(), store.Call{
		RoomID: laterRoom, LivekitRoomName: "lk-" + laterRoom, SpaceID: laterSpace, VoiceRoomID: uuid.NewString(),
		SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		MediaKind:   callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE,
	})
	require.NoError(t, err)
	later := current
	later.ProfileID = "00000000-0000-0000-0000-000000000003"
	later.Identity, later.Generation, later.AdmissionOperationID = mediaIdentity(later.ProfileID), uuid.NewString(), uuid.NewString()
	_, err = calls.AdmitSpaceMediaParticipant(t.Context(), laterRoom, later, store.MaxVoiceRoomParticipants)
	require.NoError(t, err)
	for _, admission := range fakeRecoveryForCommittedCall(t, t.Context(), calls, laterRoom).confirmed {
		c.Admissions.(*fakeAdmissionRecovery).seedConfirmedProjection(admission)
	}
	c.Store = orderedFloorSpacesStore{Store: calls, spaces: []string{first.SpaceID, second.SpaceID, laterSpace}}
	access := c.Access.(fakeAccess)
	access.byProfile[later.ProfileID] = grpcsvc.CanonicalVoiceRoomAccess{SpaceID: laterSpace, Member: true, Active: true, AccessEpoch: 1}
	grants := c.Grants.(fakeGrants)
	grants.byProfile[later.ProfileID] = grpcsvc.CanonicalVoiceRoomGrants{PolicyEpoch: 1, CanJoin: true, CanSubscribe: true}
	value := access.byProfile[stale.ProfileID]
	value.AccessEpoch = 2
	access.byProfile[stale.ProfileID] = value
	access.beforeResolve = func() { floors.floors[current.AccountID] = 5 }
	c.Access = access

	require.NoError(t, c.ReconcileAll(t.Context()))

	require.Equal(t, 2, floors.reads[current.AccountID], "Space restart must invalidate the floor shared by the full sweep")
	require.Equal(t, []string{stale.Identity, later.Identity}, media.removed, "later Spaces must use the refreshed floor")
	after, err := calls.GetCall(t.Context(), second.RoomID)
	require.NoError(t, err)
	require.Empty(t, after.SpaceMedia)
	after, err = calls.GetCall(t.Context(), laterRoom)
	require.NoError(t, err)
	require.Empty(t, after.SpaceMedia)
	after, err = calls.GetCall(t.Context(), first.RoomID)
	require.NoError(t, err)
	require.Equal(t, current.Generation, after.SpaceMedia[current.ProfileID].Generation, "earlier completed Spaces are not an atomic snapshot or rescanned on a later restart")
}

func TestReconcileSpaceReadsOneAccountFloorAndChecksEachProfileEpoch(t *testing.T) {
	accountID := uuid.NewString()
	c, calls, before, floors, media := floorPassFixture(t,
		floorParticipant{accountID, 4}, floorParticipant{accountID, 1})

	require.NoError(t, c.ReconcileSpace(t.Context(), before.SpaceID))

	require.Equal(t, 1, floors.reads[accountID])
	current := before.SpaceMedia["00000000-0000-0000-0000-000000000001"]
	stale := before.SpaceMedia["00000000-0000-0000-0000-000000000002"]
	require.Equal(t, []string{stale.Identity}, media.removed)
	after, err := calls.GetCall(t.Context(), before.RoomID)
	require.NoError(t, err)
	require.NotContains(t, after.SpaceMedia, stale.ProfileID)
	require.Equal(t, current.Identity, after.SpaceMedia[current.ProfileID].Identity)
	require.Equal(t, current.Generation, after.SpaceMedia[current.ProfileID].Generation)
	require.Equal(t, current.Issued.SessionEpoch, after.SpaceMedia[current.ProfileID].Issued.SessionEpoch)
	progress, err := calls.GetSpaceMediaEpochProgress(t.Context(), before.SpaceID)
	require.NoError(t, err)
	require.Equal(t, progress.Observed, progress.Reconciled)
}

func TestReconcileSpaceReadsDistinctAccountsSeparately(t *testing.T) {
	first, second := uuid.NewString(), uuid.NewString()
	c, _, before, floors, media := floorPassFixture(t, floorParticipant{first, 4}, floorParticipant{second, 4})
	require.NoError(t, c.ReconcileSpace(t.Context(), before.SpaceID))
	require.Equal(t, map[string]int{first: 1, second: 1}, floors.reads)
	require.Empty(t, media.removed)
}

func TestReconcileSpaceSharesAccountFloorAcrossRoomsInSameScan(t *testing.T) {
	accountID := uuid.NewString()
	c, calls, before, floors, media := floorPassFixture(t, floorParticipant{accountID, 4})
	secondRoom := uuid.NewString()
	_, err := calls.CreateCall(t.Context(), store.Call{
		RoomID: secondRoom, LivekitRoomName: "lk-" + secondRoom, SpaceID: before.SpaceID, VoiceRoomID: uuid.NewString(),
		SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		MediaKind:   callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE,
	})
	require.NoError(t, err)
	participant := before.SpaceMedia["00000000-0000-0000-0000-000000000001"]
	participant.ProfileID = "00000000-0000-0000-0000-000000000002"
	participant.Identity, participant.Generation, participant.AdmissionOperationID = mediaIdentity(participant.ProfileID), uuid.NewString(), uuid.NewString()
	_, err = calls.AdmitSpaceMediaParticipant(t.Context(), secondRoom, participant, store.MaxVoiceRoomParticipants)
	require.NoError(t, err)
	access, grants := c.Access.(fakeAccess), c.Grants.(fakeGrants)
	access.byProfile[participant.ProfileID] = grpcsvc.CanonicalVoiceRoomAccess{SpaceID: before.SpaceID, Member: true, Active: true, AccessEpoch: 1}
	grants.byProfile[participant.ProfileID] = grpcsvc.CanonicalVoiceRoomGrants{PolicyEpoch: 1, CanJoin: true, CanSubscribe: true}
	for _, admission := range fakeRecoveryForCommittedCall(t, t.Context(), calls, secondRoom).confirmed {
		c.Admissions.(*fakeAdmissionRecovery).seedConfirmedProjection(admission)
	}

	require.NoError(t, c.ReconcileSpace(t.Context(), before.SpaceID))

	require.Equal(t, 1, floors.reads[accountID])
	require.Empty(t, media.removed)
	second, err := calls.GetCall(t.Context(), secondRoom)
	require.NoError(t, err)
	require.Equal(t, participant.Generation, second.SpaceMedia[participant.ProfileID].Generation)
}

func TestReconcileSpaceAuthoritativeMissingAccountStillEjectsEachProfile(t *testing.T) {
	accountID := uuid.NewString()
	c, calls, before, floors, media := floorPassFixture(t, floorParticipant{accountID, 4}, floorParticipant{accountID, 4})
	floors.err = status.Error(codes.NotFound, "account does not exist")

	require.NoError(t, c.ReconcileSpace(t.Context(), before.SpaceID))

	require.Equal(t, 1, floors.reads[accountID])
	require.Len(t, media.removed, 2)
	after, err := calls.GetCall(t.Context(), before.RoomID)
	require.NoError(t, err)
	require.Empty(t, after.SpaceMedia)
}

func TestReconcileSpaceNewPassReadsChangedAuthFloor(t *testing.T) {
	accountID := uuid.NewString()
	c, calls, before, floors, media := floorPassFixture(t, floorParticipant{accountID, 4}, floorParticipant{accountID, 4})
	require.NoError(t, c.ReconcileSpace(t.Context(), before.SpaceID))
	require.Equal(t, 1, floors.reads[accountID])
	floors.floors[accountID] = 5

	require.NoError(t, c.ReconcileSpace(t.Context(), before.SpaceID))

	require.Equal(t, 2, floors.reads[accountID])
	require.Len(t, media.removed, 2)
	after, err := calls.GetCall(t.Context(), before.RoomID)
	require.NoError(t, err)
	require.Empty(t, after.SpaceMedia)
}

func TestReconcileSpaceWatermarkRestartReadsChangedAuthFloor(t *testing.T) {
	accountID := uuid.NewString()
	c, calls, before, floors, media := floorPassFixture(t, floorParticipant{accountID, 4}, floorParticipant{accountID, 4})
	access := c.Access.(fakeAccess)
	for profileID, value := range access.byProfile {
		value.AccessEpoch = 2
		access.byProfile[profileID] = value
	}
	access.beforeResolve = func() { floors.floors[accountID] = 5 }
	c.Access = access

	require.NoError(t, c.ReconcileSpace(t.Context(), before.SpaceID))

	require.Equal(t, 2, floors.reads[accountID], "restart must discard the earlier account-floor value")
	require.Len(t, media.removed, 2)
	after, err := calls.GetCall(t.Context(), before.RoomID)
	require.NoError(t, err)
	require.Empty(t, after.SpaceMedia)
}

func TestAuthFloorFailureLeavesInvalidationUnacknowledgedAndRosterUnchanged(t *testing.T) {
	for _, name := range []string{"unavailable", "zero floor", "negative floor", "missing checker"} {
		t.Run(name, func(t *testing.T) {
			accountID := uuid.NewString()
			c, calls, before, floors, media := floorPassFixture(t, floorParticipant{accountID, 4})
			c.ready = true
			switch name {
			case "unavailable":
				floors.err = status.Error(codes.Unavailable, "Auth unavailable")
			case "zero floor":
				floors.floors[accountID] = 0
			case "negative floor":
				floors.floors[accountID] = -1
			case "missing checker":
				c.SessionEpochChecker = nil
			}
			envelope := &eventsv1.ChatStreamEvent{EventId: uuid.NewString(),
				Payload: &eventsv1.ChatStreamEvent_VoiceRoomAccessInvalidated{
					VoiceRoomAccessInvalidated: &eventsv1.VoiceRoomAccessInvalidated{SpaceId: before.SpaceID, AccessEpoch: 1},
				}}
			data, err := proto.Marshal(envelope)
			require.NoError(t, err)
			msg := &invalidationAckMessage{}

			err = handleSpaceAccessInvalidation(t.Context(), spaceAccessSubject, data, msg, c)

			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Zero(t, msg.acked)
			require.Equal(t, 1, msg.nacked)
			require.Zero(t, msg.termed)
			require.False(t, c.ready)
			require.Empty(t, media.removed)
			after, err := calls.GetCall(t.Context(), before.RoomID)
			require.NoError(t, err)
			require.Equal(t, before.SpaceMedia, after.SpaceMedia)
			progress, err := calls.GetSpaceMediaEpochProgress(t.Context(), before.SpaceID)
			require.NoError(t, err)
			require.Zero(t, progress.Reconciled.AccessEpoch)
			require.Zero(t, progress.Reconciled.PolicyEpoch)
		})
	}
}
