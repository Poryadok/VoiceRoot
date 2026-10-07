package spacemedia

import (
	"context"
	"errors"
	"testing"
	"time"

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

type currentEpochChecker struct{ err error }

func (c currentEpochChecker) RequireCurrent(context.Context, string, int64) error { return c.err }

type fakeAdmissionRecovery struct {
	released        int
	cleaned         int
	projected       int
	reads           int
	pages           [][]store.SpaceMediaAdmission
	projectionReads int
	projectionPages [][]store.SpaceMediaAdmission
	roomCreator     store.SpaceMediaAdmission
	confirmed       map[string]store.SpaceMediaAdmission
	confirmCalls    int
	recoveryRows    []store.SpaceMediaAdmission
	markAbortFails  int
	markAbortCalls  int
	releaseFails    int
	releaseCalls    int
	releasedRows    []store.SpaceMediaAdmission
}

func (f *fakeAdmissionRecovery) RecoveryRows(context.Context, int) ([]store.SpaceMediaAdmission, error) {
	f.reads++
	if f.recoveryRows != nil {
		return append([]store.SpaceMediaAdmission(nil), f.recoveryRows...), nil
	}
	if len(f.pages) == 0 {
		return nil, nil
	}
	page := f.pages[0]
	f.pages = f.pages[1:]
	return page, nil
}
func (f *fakeAdmissionRecovery) ProjectionRows(context.Context, uuid.UUID, int, bool) ([]store.SpaceMediaAdmission, error) {
	f.projectionReads++
	if len(f.projectionPages) == 0 {
		return nil, nil
	}
	page := f.projectionPages[0]
	f.projectionPages = f.projectionPages[1:]
	for _, admission := range page {
		if admission.State == AdmissionCommitted && admission.ProjectionApplied {
			f.seedConfirmedProjection(admission)
		}
	}
	return page, nil
}
func (f *fakeAdmissionRecovery) seedConfirmedProjection(admission store.SpaceMediaAdmission) {
	if f.confirmed == nil {
		f.confirmed = make(map[string]store.SpaceMediaAdmission)
	}
	f.confirmed[admission.OperationID.String()] = admission
}
func (f *fakeAdmissionRecovery) ConfirmProjection(_ context.Context, operationID uuid.UUID, generation string) error {
	f.confirmCalls++
	admission, ok := f.confirmed[operationID.String()]
	if !ok || admission.Generation != generation || admission.State != AdmissionCommitted || !admission.ProjectionApplied {
		return ErrAdmissionConflict
	}
	return nil
}
func (f *fakeAdmissionRecovery) RoomCreator(context.Context, string, uint64) (store.SpaceMediaAdmission, error) {
	if f.roomCreator.OperationID == uuid.Nil {
		return store.SpaceMediaAdmission{}, store.ErrNotFound
	}
	return f.roomCreator, nil
}
func (f *fakeAdmissionRecovery) MarkAborting(_ context.Context, operationID uuid.UUID, generation string) error {
	f.markAbortCalls++
	if f.markAbortFails > 0 {
		f.markAbortFails--
		return errors.New("injected abort transition failure")
	}
	for i := range f.recoveryRows {
		if f.recoveryRows[i].OperationID == operationID && f.recoveryRows[i].Generation == generation {
			f.recoveryRows[i].State = AdmissionAborting
		}
	}
	return nil
}
func (f *fakeAdmissionRecovery) ReleaseFence(_ context.Context, admission store.SpaceMediaAdmission) error {
	f.released++
	f.releaseCalls++
	f.releasedRows = append(f.releasedRows, admission)
	if f.releaseFails > 0 {
		f.releaseFails--
		return errors.New("injected fence release acknowledgement failure")
	}
	return nil
}
func (f *fakeAdmissionRecovery) MarkCleanupCompleted(_ context.Context, operationID uuid.UUID, generation string) error {
	f.cleaned++
	for i := range f.recoveryRows {
		if f.recoveryRows[i].OperationID == operationID && f.recoveryRows[i].Generation == generation {
			f.recoveryRows = append(f.recoveryRows[:i], f.recoveryRows[i+1:]...)
			break
		}
	}
	return nil
}
func (f *fakeAdmissionRecovery) MarkProjectionApplied(context.Context, uuid.UUID, string) error {
	f.projected++
	return nil
}
func (f *fakeAdmissionRecovery) BeginRoomLeave(context.Context, uuid.UUID, string) error { return nil }
func (f *fakeAdmissionRecovery) ConfirmRoomHeadOpen(context.Context, store.SpaceMediaAdmission) error {
	return nil
}
func (f *fakeAdmissionRecovery) RecoverOrphanRoomHeads(context.Context) error { return nil }
func (f *fakeAdmissionRecovery) CompleteRoomLeave(context.Context, store.SpaceMediaAdmission) error {
	f.released++
	return nil
}

func TestCoordinatorRetriesCommittedRoomFullAbortThroughRecovery(t *testing.T) {
	ctx := context.Background()
	spaceID, voiceRoomID, roomID := uuid.New(), uuid.NewString(), uuid.NewString()
	profileID, accountID := uuid.New(), uuid.New()
	admission := store.SpaceMediaAdmission{
		OperationID: uuid.New(), Generation: uuid.NewString(), AccountID: accountID, ProfileID: profileID,
		SpaceID: spaceID, RoomID: roomID, VoiceRoomID: voiceRoomID, RoomGeneration: 1,
		Identity: "cap-rejected-identity", CreatedRoom: false, MaxParticipants: store.MaxVoiceRoomParticipants,
		SessionEpoch: 3, AccessEpoch: 4, PolicyEpoch: 5, CanJoin: true, CanSubscribe: true,
		State: AdmissionCommitted, ParticipantState: "PREPARED",
	}
	calls := store.NewMemoryCallStore()
	_, err := calls.CreateCall(ctx, store.Call{
		RoomID: roomID, LivekitRoomName: "voice-room-" + voiceRoomID, VoiceRoomID: voiceRoomID,
		SpaceID: spaceID.String(), SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: uuid.NewString(), MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status: callsv1.CallStatus_CALL_STATUS_ACTIVE, States: map[string]store.ParticipantState{},
		SpaceMedia: map[string]store.SpaceMediaParticipant{},
	})
	require.NoError(t, err)
	for i := 0; i < store.MaxVoiceRoomParticipants; i++ {
		profile := uuid.NewString()
		_, err = calls.AdmitSpaceMediaParticipant(ctx, roomID, store.SpaceMediaParticipant{
			AccountID: uuid.NewString(), AdmissionOperationID: uuid.NewString(), ProfileID: profile,
			Identity: "existing-" + profile, Generation: uuid.NewString(),
			Issued: store.SpaceMediaGrant{SessionEpoch: 1, AccessEpoch: 4, PolicyEpoch: 5, CanJoin: true, CanSubscribe: true},
		}, store.MaxVoiceRoomParticipants)
		require.NoError(t, err)
	}
	admissions := &fakeAdmissionRecovery{recoveryRows: []store.SpaceMediaAdmission{admission}, markAbortFails: 1, releaseFails: 1}
	c := &Coordinator{
		Store: calls, Calls: calls, Admissions: admissions,
		Access: fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{
			profileID.String(): {SpaceID: spaceID.String(), Active: true, Member: true, AccessEpoch: 4},
		}},
		Grants: fakeGrants{byProfile: map[string]grpcsvc.CanonicalVoiceRoomGrants{
			profileID.String(): {PolicyEpoch: 5, CanJoin: true, CanSubscribe: true},
		}},
		SessionEpochChecker: currentEpochChecker{},
	}
	firstErr := c.reconcileAdmissions(ctx)
	require.ErrorContains(t, firstErr, "injected abort transition failure")
	require.Len(t, admissions.recoveryRows, 1, "a failed transition leaves the committed row available for retry")
	require.Equal(t, AdmissionCommitted, admissions.recoveryRows[0].State)
	require.Zero(t, admissions.released)
	require.Zero(t, admissions.cleaned)

	releaseErr := c.reconcileAdmissions(ctx)
	require.ErrorContains(t, releaseErr, "injected fence release acknowledgement failure")
	require.Len(t, admissions.recoveryRows, 1)
	require.Equal(t, AdmissionAborting, admissions.recoveryRows[0].State)
	require.Zero(t, admissions.cleaned)

	require.NoError(t, c.reconcileAdmissions(ctx), "recovery retries an ambiguous exact-fence release")
	require.Empty(t, admissions.recoveryRows)
	require.Equal(t, 2, admissions.markAbortCalls)
	require.Equal(t, 2, admissions.releaseCalls)
	require.Len(t, admissions.releasedRows, 2)
	for _, released := range admissions.releasedRows {
		require.Equal(t, admission.OperationID, released.OperationID)
		require.Equal(t, admission.Generation, released.Generation)
	}
	require.Equal(t, 1, admissions.cleaned)
	call, err := calls.GetCall(ctx, roomID)
	require.NoError(t, err)
	require.Len(t, call.SpaceMedia, store.MaxVoiceRoomParticipants)
	require.NotContains(t, call.SpaceMedia, profileID.String())
}

func fakeRecoveryForCommittedCall(t *testing.T, ctx context.Context, calls *store.MemoryCallStore, roomID string) *fakeAdmissionRecovery {
	t.Helper()
	call, err := calls.GetCall(ctx, roomID)
	require.NoError(t, err)
	fake := &fakeAdmissionRecovery{}
	for _, participant := range call.SpaceMedia {
		operationID, parseErr := uuid.Parse(participant.AdmissionOperationID)
		require.NoError(t, parseErr)
		fake.seedConfirmedProjection(store.SpaceMediaAdmission{
			OperationID: operationID, Generation: participant.Generation,
			State: AdmissionCommitted, ProjectionApplied: true,
		})
	}
	return fake
}

func TestFakeAdmissionRecoveryConfirmProjectionRequiresExactCommittedBinding(t *testing.T) {
	operationID, generation := uuid.New(), uuid.NewString()
	fake := &fakeAdmissionRecovery{}
	fake.seedConfirmedProjection(store.SpaceMediaAdmission{
		OperationID: operationID, Generation: generation,
		State: AdmissionCommitted, ProjectionApplied: true,
	})

	require.NoError(t, fake.ConfirmProjection(context.Background(), operationID, generation))
	require.ErrorIs(t, fake.ConfirmProjection(context.Background(), operationID, generation+"-stale"), ErrAdmissionConflict)
	require.ErrorIs(t, fake.ConfirmProjection(context.Background(), uuid.New(), generation), ErrAdmissionConflict)
	require.Equal(t, 3, fake.confirmCalls)
}

func TestCoordinatorDrainsEveryAdmissionRecoveryPageBeforeReady(t *testing.T) {
	row := store.SpaceMediaAdmission{State: AdmissionCommitted, ProjectionApplied: true, ParticipantState: "ACTIVE"}
	admissions := &fakeAdmissionRecovery{pages: [][]store.SpaceMediaAdmission{{row}, nil}}
	c := &Coordinator{
		Store: store.NewMemoryCallStore(), Calls: store.NewMemoryCallStore(),
		Access: fakeAccess{}, Grants: fakeGrants{}, Media: &fakeMedia{}, Admissions: admissions,
		SessionEpochChecker: currentEpochChecker{},
	}
	require.NoError(t, c.reconcileAdmissions(context.Background()))
	require.Equal(t, 2, admissions.reads, "readiness recovery must drain every bounded page")
}

func TestCoordinatorRebuildsLostCallProjectionFromDurableAdmission(t *testing.T) {
	spaceID, voiceRoomID := uuid.NewString(), uuid.NewString()
	profileID, accountID, operationID := uuid.New(), uuid.New(), uuid.New()
	row := store.SpaceMediaAdmission{
		OperationID: operationID, Generation: uuid.NewString(), AccountID: accountID, ProfileID: profileID,
		SpaceID: uuid.MustParse(spaceID), RoomID: uuid.NewString(), VoiceRoomID: voiceRoomID, RoomGeneration: 1,
		Identity: "space-media-recovery-identity", CreatedRoom: true, CallStartedAt: time.Now().UTC(),
		MaxParticipants: store.MaxVoiceRoomParticipants, SessionEpoch: 5, AccessEpoch: 11, PolicyEpoch: 7,
		CanJoin: true, CanSubscribe: true, State: AdmissionCommitted, ParticipantState: "ACTIVE", ProjectionApplied: true,
	}
	admissions := &fakeAdmissionRecovery{projectionPages: [][]store.SpaceMediaAdmission{{row}, nil}, roomCreator: row}
	calls := store.NewMemoryCallStore()
	c := &Coordinator{
		Store: calls, Calls: calls,
		Access: fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{
			profileID.String(): {SpaceID: spaceID, Active: true, Member: true, AccessEpoch: 11},
		}},
		Grants: fakeGrants{byProfile: map[string]grpcsvc.CanonicalVoiceRoomGrants{
			profileID.String(): {PolicyEpoch: 7, CanJoin: true, CanSubscribe: true},
		}},
		Media: &fakeMedia{}, Admissions: admissions, SessionEpochChecker: currentEpochChecker{},
	}
	require.NoError(t, c.reconcileAdmissions(context.Background()))
	call, err := calls.GetCall(context.Background(), row.RoomID)
	require.NoError(t, err)
	require.True(t, call.IsParticipant(profileID.String()))
	participant := call.SpaceMedia[profileID.String()]
	require.Equal(t, operationID.String(), participant.AdmissionOperationID)
	require.Equal(t, row.RoomGeneration, participant.RoomGeneration)
	require.Equal(t, 3, admissions.projectionReads, "creator and joiner pages are drained before readiness")
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
	_, floorErr := calls.RaiseSpaceMediaEpochFloor(ctx, spaceID, store.SpaceAccessEpoch, 4)
	require.NoError(t, floorErr)
	_, floorErr = calls.RaiseSpaceMediaEpochFloor(ctx, spaceID, store.RolePolicyEpoch, 7)
	require.NoError(t, floorErr)
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
	admissions := fakeRecoveryForCommittedCall(t, ctx, calls, roomID)
	c := &Coordinator{Store: calls, Access: access, Grants: grants, Media: media, Admissions: admissions, SessionEpochChecker: currentEpochChecker{}}

	err = c.Observe(ctx, AuthorityNotice{SpaceID: spaceID, Kind: store.SpaceAccessEpoch, Epoch: 4, ProfileID: profileA})

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
	require.Equal(t, 2, admissions.confirmCalls, "both committed participant bindings are confirmed before reconciliation")
}

func TestDelayedInvalidationUsesCurrentRestoredRights(t *testing.T) {
	ctx := context.Background()
	spaceID, roomID, voiceRoomID, profile := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	calls := newRoomStore(t, ctx, spaceID, roomID, voiceRoomID, profile)
	admissions := fakeRecoveryForCommittedCall(t, ctx, calls, roomID)
	media := &fakeMedia{}
	c := &Coordinator{
		Store: calls, Admissions: admissions,
		Access: fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{
			profile: {SpaceID: spaceID, Member: true, Active: true, AccessEpoch: 9},
		}},
		Grants: fakeGrants{byProfile: map[string]grpcsvc.CanonicalVoiceRoomGrants{
			profile: {PolicyEpoch: 12, CanJoin: true, CanSubscribe: true, CanPublishAudio: true},
		}},
		Media: media, SessionEpochChecker: currentEpochChecker{},
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
	admissions := fakeRecoveryForCommittedCall(t, ctx, calls, roomID)
	media := &fakeMedia{err: errors.New("injected ejection failure")}
	c := &Coordinator{
		Store: calls, Admissions: admissions,
		Access: fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{
			profile: {SpaceID: spaceID, Member: false, Active: true, AccessEpoch: 2},
		}},
		Grants: fakeGrants{byProfile: map[string]grpcsvc.CanonicalVoiceRoomGrants{}},
		Media:  media, SessionEpochChecker: currentEpochChecker{},
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

func TestReconcileAllOpensReadinessOnlyAfterCompletePass(t *testing.T) {
	ctx := context.Background()
	spaceID, roomID, voiceRoomID, profileID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	calls := newRoomStore(t, ctx, spaceID, roomID, voiceRoomID, profileID)
	_, floorErr := calls.RaiseSpaceMediaEpochFloor(ctx, spaceID, store.SpaceAccessEpoch, 1)
	require.NoError(t, floorErr)
	_, floorErr = calls.RaiseSpaceMediaEpochFloor(ctx, spaceID, store.RolePolicyEpoch, 1)
	require.NoError(t, floorErr)
	media := &fakeMedia{}
	readiness := []bool{}
	admissions := fakeRecoveryForCommittedCall(t, ctx, calls, roomID)
	c := &Coordinator{
		Store: calls, Calls: calls, Admissions: admissions, Media: media,
		Access: fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{
			profileID: {SpaceID: spaceID, Member: true, Active: true, AccessEpoch: 1},
		}},
		Grants: fakeGrants{byProfile: map[string]grpcsvc.CanonicalVoiceRoomGrants{
			profileID: {PolicyEpoch: 1, CanJoin: true, CanSubscribe: true},
		}},
		SessionEpochChecker: currentEpochChecker{}, OnReadiness: func(ready bool) { readiness = append(readiness, ready) },
	}

	require.NoError(t, c.ReconcileAll(ctx))
	require.Equal(t, []bool{false, true}, readiness)
	require.Equal(t, 1, admissions.confirmCalls, "the committed participant must be confirmed before readiness opens")

	readiness = nil
	media.err = errors.New("authority-driven ejection did not complete")
	c.Access = fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{
		profileID: {SpaceID: spaceID, Member: false, Active: true, AccessEpoch: 2},
	}}
	err := c.ReconcileAll(ctx)
	require.Error(t, err)
	require.Equal(t, []bool{false}, readiness, "failed convergence must keep serving closed")
}

func TestAbortRetryDrainsExactCommittedProjectionBeforeReleasingFence(t *testing.T) {
	ctx := context.Background()
	spaceID, roomID, voiceRoomID, profileID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	calls := newRoomStore(t, ctx, spaceID, roomID, voiceRoomID, profileID)
	call, err := calls.GetCall(ctx, roomID)
	require.NoError(t, err)
	participant := call.SpaceMedia[profileID]
	admissions := fakeRecoveryForCommittedCall(t, ctx, calls, roomID)
	media := &fakeMedia{}
	c := &Coordinator{Store: calls, Calls: calls, Admissions: admissions, Media: media}
	operationID, err := uuid.Parse(participant.AdmissionOperationID)
	require.NoError(t, err)
	row := store.SpaceMediaAdmission{
		OperationID: operationID, Generation: participant.Generation, AccountID: uuid.MustParse(participant.AccountID),
		ProfileID: uuid.MustParse(profileID), SpaceID: uuid.MustParse(spaceID), RoomID: roomID,
		VoiceRoomID: voiceRoomID, Identity: participant.Identity, State: AdmissionAborting,
	}

	err = c.abortAdmission(ctx, row)

	require.NoError(t, err)
	current, err := calls.GetCall(ctx, roomID)
	require.NoError(t, err)
	require.NotContains(t, current.SpaceMedia, profileID, "a retry must drain an operation-owned participant before fence release")
	require.Equal(t, []string{participant.Identity}, media.removed)
	require.Equal(t, 2, admissions.released, "projection drain and final cleanup both release only the exact operation")
	require.Equal(t, 1, admissions.cleaned)
}

func TestCommittedDuplicateRoomAdmissionRecoversByAbortingHiddenLoser(t *testing.T) {
	ctx := context.Background()
	spaceID, voiceRoomID := uuid.NewString(), uuid.NewString()
	activeRoomID, pendingRoomID := uuid.NewString(), uuid.NewString()
	activeProfile, pendingProfile := uuid.NewString(), uuid.NewString()
	calls := store.NewMemoryCallStore()
	_, err := calls.CreateCall(ctx, store.Call{
		RoomID: activeRoomID, LivekitRoomName: "lk-active", VoiceRoomID: voiceRoomID, SpaceID: spaceID,
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: activeProfile, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status: callsv1.CallStatus_CALL_STATUS_ACTIVE,
	})
	require.NoError(t, err)
	_, err = calls.AdmitSpaceMediaParticipant(ctx, activeRoomID, store.SpaceMediaParticipant{
		AccountID: uuid.NewString(), AdmissionOperationID: uuid.NewString(), ProfileID: activeProfile,
		Identity: mediaIdentity(activeProfile), Generation: uuid.NewString(),
		Issued: store.SpaceMediaGrant{SessionEpoch: 1, AccessEpoch: 1, PolicyEpoch: 1, CanJoin: true, CanSubscribe: true},
	}, store.MaxVoiceRoomParticipants)
	require.NoError(t, err)
	_, err = calls.CreateCall(ctx, store.Call{
		RoomID: pendingRoomID, LivekitRoomName: "lk-pending", VoiceRoomID: voiceRoomID, SpaceID: spaceID,
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: pendingProfile, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status: callsv1.CallStatus_CALL_STATUS_UNSPECIFIED,
	})
	require.NoError(t, err)
	access := fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{
		pendingProfile: {SpaceID: spaceID, Member: true, Active: true, AccessEpoch: 1},
	}}
	grants := fakeGrants{byProfile: map[string]grpcsvc.CanonicalVoiceRoomGrants{
		pendingProfile: {PolicyEpoch: 1, CanJoin: true, CanSubscribe: true},
	}}
	admissions := fakeRecoveryForCommittedCall(t, ctx, calls, activeRoomID)
	c := &Coordinator{Store: calls, Calls: calls, Admissions: admissions, Access: access, Grants: grants,
		SessionEpochChecker: currentEpochChecker{}, Media: &fakeMedia{}}
	row := store.SpaceMediaAdmission{
		OperationID: uuid.New(), Generation: uuid.NewString(), AccountID: uuid.New(), ProfileID: uuid.MustParse(pendingProfile),
		SpaceID: uuid.MustParse(spaceID), RoomID: pendingRoomID, VoiceRoomID: voiceRoomID,
		Identity: mediaIdentity(pendingProfile), CreatedRoom: true, CallStartedAt: time.Now().UTC(),
		MaxParticipants: store.MaxVoiceRoomParticipants, SessionEpoch: 1, AccessEpoch: 1, PolicyEpoch: 1,
		CanJoin: true, CanSubscribe: true, State: AdmissionCommitted,
	}

	err = c.recoverCommittedAdmission(ctx, row)

	require.NoError(t, err)
	loser, err := calls.GetCall(ctx, pendingRoomID)
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ENDED, loser.Status)
	require.Equal(t, 1, admissions.released)
	require.Equal(t, 1, admissions.cleaned)
	require.Zero(t, admissions.projected, "a losing operation must never become outbox-claimable")
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
	c := &Coordinator{Store: calls, Admissions: fakeRecoveryForCommittedCall(t, ctx, calls, roomID), Media: media}

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
			AccountID: uuid.NewString(), AdmissionOperationID: uuid.NewString(), ProfileID: profile, Identity: mediaIdentity(profile), Generation: uuid.NewString(),
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
