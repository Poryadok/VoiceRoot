package spacemedia

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/spacelifecycle"
	"voice/backend/voice/internal/store"
)

type observingRetiredDrain struct {
	*Coordinator
	oldOperation string
	afterOld     func()
}

func (d observingRetiredDrain) ResumeRetiredCapturedDrain(ctx context.Context, target spacelifecycle.CapturedTarget, verified retirementproof.VerifiedRetirement, work retirementproof.OwnerWork) error {
	err := d.Coordinator.ResumeRetiredCapturedDrain(ctx, target, verified, work)
	if err == nil && target.OperationID == d.oldOperation {
		d.afterOld()
	}
	return err
}

func TestNativeOwnerRetainedEndedBeforeNewerProjectionDoesNotStall(t *testing.T) {
	ctx := t.Context()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	applyOwnerMigration(t, ctx, pool)
	admissions := NewPostgresAdmissionStore(pool)
	old := admissionFixture()
	old.OperationID = uuid.MustParse("00000000-0000-0000-0000-000000000010")
	old.Identity = "old-retained-identity"
	finalize := func(a *Admission) {
		t.Helper()
		require.NoError(t, prepareAdmission(ctx, admissions, a))
		require.NoError(t, admissions.Fence(ctx, *a))
		require.NoError(t, admissions.MarkCommitted(ctx, a.OperationID, a.Generation))
		require.NoError(t, admissions.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
	}
	finalize(&old)
	joiner := func(op string) Admission {
		a := admissionFixture()
		a.OperationID = uuid.MustParse(op)
		a.VoiceRoomID = old.VoiceRoomID
		a.SpaceID = old.SpaceID
		a.RoomID = old.RoomID
		a.RoomGeneration = old.RoomGeneration
		a.CreatedRoom = false
		a.Events = []AdmissionEvent{{ID: uuid.New(), Subject: "voice.member_joined", Payload: []byte("retained-history-rejoin")}}
		return a
	}
	survivor := joiner("00000000-0000-0000-0000-000000000030")
	finalize(&survivor)
	require.NoError(t, admissions.BeginRoomLeave(ctx, old.OperationID, old.Generation))
	require.NoError(t, admissions.CompleteRoomLeave(ctx, old))
	newer := joiner("00000000-0000-0000-0000-000000000020")
	newer.AccountID = old.AccountID
	newer.ProfileID = old.ProfileID
	newer.Identity = "new-rejoined-identity"
	finalize(&newer)
	calls, client := newP3Redis(t)
	call := store.Call{RoomID: old.RoomID, VoiceRoomID: old.VoiceRoomID, SpaceID: old.SpaceID.String(), LivekitRoomName: "voice-room-" + old.VoiceRoomID, SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE, States: map[string]store.ParticipantState{}, SpaceMedia: map[string]store.SpaceMediaParticipant{}}
	for _, a := range []Admission{newer, survivor} {
		profile := a.ProfileID.String()
		call.States[profile] = store.ParticipantState{ProfileID: profile}
		call.SpaceMedia[profile] = store.SpaceMediaParticipant{AccountID: a.AccountID.String(), AdmissionOperationID: a.OperationID.String(), ProfileID: profile, Identity: a.Identity, Generation: a.Generation, RoomGeneration: a.RoomGeneration}
		require.NoError(t, client.Set(ctx, "p3:session:"+profile, a.RoomID, 0).Err())
	}
	// Explicit saved projection fixture; it grants no serving authorization.
	require.NoError(t, client.Set(ctx, "p3:call:"+call.RoomID, ownerFixtureJSON(t, call), 0).Err())
	media := &p3Media{}
	coordinator := &Coordinator{Calls: calls, Store: calls, Admissions: admissions, ProjectionGates: calls, Media: media}
	oldTarget := spacelifecycle.CapturedTarget{OperationID: old.OperationID.String(), Generation: old.Generation, AccountID: old.AccountID.String(), ProfileID: old.ProfileID.String(), SpaceID: old.SpaceID.String(), RoomID: old.RoomID, VoiceRoomID: old.VoiceRoomID, RoomGeneration: old.RoomGeneration, Identity: old.Identity, ParticipantState: "ENDED", State: "COMMITTED"}
	require.ErrorIs(t, coordinator.ResumeCapturedDrain(ctx, oldTarget), ErrAdmissionConflict, "ordinary unqualified cleanup remains strict")
	resource := newOwnerResourceFixture(t)
	preserved := 0
	drain := observingRetiredDrain{Coordinator: coordinator, oldOperation: old.OperationID.String(), afterOld: func() {
		preserved++
		projected, err := calls.GetCall(ctx, old.RoomID)
		require.NoError(t, err)
		require.Equal(t, newer.OperationID.String(), projected.SpaceMedia[newer.ProfileID.String()].AdmissionOperationID)
		var op uuid.UUID
		require.NoError(t, pool.QueryRow(ctx, `SELECT admission_operation_id FROM voice_account_voice_fences WHERE account_id=$1`, newer.AccountID).Scan(&op))
		require.Equal(t, newer.OperationID, op, "old history must preserve newer fence until its own target")
	}}
	owner := &spacelifecycle.NativeRetirementOwner{Store: spacelifecycle.NewPostgresStore(pool, BeginPartyRoomClose), Authority: ownerMaintenanceFixture{resource.record.Document.Cut}, NativeDrain: drain, ProjectionFence: calls}
	owner.Verifier = retirementproof.TrustedVerifier{Journal: resource, Owner: owner, Installation: resource, Resources: resource}
	head, err := owner.ObserveCutoverHead(ctx, old.SpaceID)
	require.NoError(t, err)
	ref, err := owner.PrepareCutoverDrain(ctx, resource.record.Document.Cut, spacelifecycle.ExpectedOwnerBinding{WorkID: uuid.New(), SpaceID: old.SpaceID, Purpose: retirementproof.ColdCutDrain, Head: head})
	require.NoError(t, err)
	receipt, err := owner.ResumeCutoverDrain(ctx, ref, resource.record.Ref)
	require.NoError(t, err, "old ENDED target must not permanently precede the newer captured owner")
	require.NotNil(t, receipt)
	require.Equal(t, 1, preserved)
	var ended int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_media_admissions WHERE space_id=$1 AND participant_state='ENDED'`, old.SpaceID).Scan(&ended))
	require.Equal(t, 3, ended)
	final, err := calls.GetCall(ctx, old.RoomID)
	require.NoError(t, err)
	require.Empty(t, final.SpaceMedia)
	require.Empty(t, media.removed)
}
