package ordinarycall

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var testNow = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

func TestLastGroupLeaveFreezesFinalAudienceBeforeRemoval(t *testing.T) {
	r, owner := activeGroup(t)
	require.NoError(t, r.DecideLeave(owner.ProfileID, testNow))
	require.Equal(t, StageClosing, r.Stage)
	var ended []Event
	for _, e := range r.Events {
		if e.Kind == EventEnded {
			ended = append(ended, e)
		}
	}
	require.Len(t, ended, 1)
	require.Equal(t, []uuid.UUID{owner.ProfileID}, ended[0].AudienceIDs)
}

func identity() Identity { return Identity{AccountID: uuid.New(), ProfileID: uuid.New()} }

func pendingGroup(t *testing.T) (*Room, Identity, uuid.UUID) {
	t.Helper()
	member := identity()
	op := uuid.New()
	room, err := NewPendingRoom(CreateIntent{
		OperationID: op, RoomID: uuid.New(), ChatID: uuid.New(), Group: true,
		Initiator: member, MediaKind: 1,
	}, testNow)
	require.NoError(t, err)
	return room, member, op
}

func activeGroup(t *testing.T) (*Room, Identity) {
	t.Helper()
	room, member, op := pendingGroup(t)
	require.NoError(t, room.RecordEnsureResult(op, true, testNow))
	require.NoError(t, room.CommitAdmission(op, testNow))
	return room, member
}

func TestRosterVersionTracksDistinctProfileDeltasNotJournalOrCleanup(t *testing.T) {
	caller, callee := identity(), identity()
	op := uuid.New()
	r, err := NewPendingRoom(CreateIntent{OperationID: op, RoomID: uuid.New(), ChatID: uuid.New(),
		Initiator: caller, Callee: &callee, MediaKind: 1}, testNow)
	require.NoError(t, err)
	require.NoError(t, r.RecordEnsureResult(op, true, testNow))
	require.NoError(t, r.CommitAdmission(op, testNow))
	require.Zero(t, r.RosterVersion, "ringing reservations are not a roster")
	accept := uuid.New()
	require.NoError(t, r.BeginAccept(accept, callee.ProfileID, testNow.Add(time.Second)))
	require.NoError(t, r.CommitAdmission(accept, testNow.Add(time.Second)))
	require.Equal(t, uint64(2), r.RosterVersion)
	var versions []uint64
	var profiles []string
	for _, e := range r.Events {
		if e.Kind == EventParticipantJoined {
			versions = append(versions, e.RosterVersion)
			profiles = append(profiles, e.ProfileID.String())
		}
	}
	require.Equal(t, []uint64{1, 2}, versions)
	require.IsIncreasing(t, profiles)
	require.NoError(t, r.CommitAdmission(accept, testNow.Add(2*time.Second)))
	require.Equal(t, uint64(2), r.RosterVersion, "receipt replay must not produce another delta")
	require.NoError(t, r.DecideEnd(caller.ProfileID, "hangup", testNow.Add(3*time.Second)))
	require.Equal(t, uint64(4), r.RosterVersion, "both departing profiles get distinct versions atomically")
	closeEffect, ok := r.PendingClose()
	require.True(t, ok)
	require.NoError(t, r.CompleteRoomCleanup(closeEffect.ID, closeEffect.Revision, testNow.Add(4*time.Second)))
	require.Equal(t, uint64(4), r.RosterVersion, "cleanup changes only the private journal revision")
}

func TestDMVerifiedEpochComesFromEachActorsOwnAdmission(t *testing.T) {
	caller, callee := identity(), identity()
	caller.SessionEpoch = 7
	op := uuid.New()
	r, err := NewPendingRoom(CreateIntent{OperationID: op, RoomID: uuid.New(), ChatID: uuid.New(),
		Initiator: caller, Callee: &callee, MediaKind: 1}, testNow)
	require.NoError(t, err)
	require.Equal(t, int64(7), r.Members[caller.ProfileID.String()].SessionEpoch)
	require.Zero(t, r.Members[callee.ProfileID.String()].SessionEpoch, "invitation cannot authenticate the callee epoch")
	require.NoError(t, r.RecordEnsureResult(op, true, testNow))
	require.NoError(t, r.CommitAdmission(op, testNow))
	callee.SessionEpoch = 13
	accept := uuid.New()
	require.NoError(t, r.BeginVerifiedAccept(accept, callee, testNow.Add(time.Second)))
	require.NoError(t, r.CommitAdmission(accept, testNow.Add(time.Second)))
	require.Equal(t, int64(7), r.Members[caller.ProfileID.String()].SessionEpoch)
	require.Equal(t, int64(13), r.Members[callee.ProfileID.String()].SessionEpoch)
	callee.SessionEpoch = 14
	require.ErrorIs(t, r.BeginVerifiedAccept(accept, callee, testNow.Add(2*time.Second)), ErrConflict)
	require.Equal(t, int64(13), r.Members[callee.ProfileID.String()].SessionEpoch, "replay must never upgrade admitted device epoch")
}

func TestNoopJoinFromAnotherCurrentDeviceDoesNotUpgradeMediaEpoch(t *testing.T) {
	r, owner, op := pendingGroup(t)
	// Fixtures with epoch0 represent unknown legacy provenance, not a floor.
	require.NoError(t, r.RecordEnsureResult(op, true, testNow))
	require.NoError(t, r.CommitAdmission(op, testNow))
	owner.SessionEpoch = 12
	require.NoError(t, r.BeginJoin(uuid.New(), owner, testNow))
	require.Zero(t, r.Members[owner.ProfileID.String()].SessionEpoch)
	require.Equal(t, uint64(1), r.RosterVersion)
}

func TestFormerGroupMemberCannotEndRemainingCallButTerminalWinnerCanReplay(t *testing.T) {
	r, owner := activeGroup(t)
	other := identity()
	op := uuid.New()
	require.NoError(t, r.BeginJoin(op, other, testNow))
	require.NoError(t, r.CommitAdmission(op, testNow))
	old := r.Members[owner.ProfileID.String()]
	require.NoError(t, r.DecideLeave(owner.ProfileID, testNow))
	require.NoError(t, r.CompleteMemberCleanup(owner.ProfileID, old.Generation, testNow))
	require.ErrorIs(t, r.DecideEnd(owner.ProfileID, "hangup", testNow), ErrNotMember, "initiator privilege must not outlive current group membership")
	require.Equal(t, StageActive, r.Stage)
	require.NoError(t, r.DecideEnd(other.ProfileID, "hangup", testNow))
	effect, ok := r.PendingClose()
	require.True(t, ok)
	require.NoError(t, r.CompleteRoomCleanup(effect.ID, effect.Revision, testNow))
	require.NoError(t, r.DecideEnd(other.ProfileID, "hangup", testNow), "authorized winner retains terminal replay")
	require.ErrorIs(t, r.DecideEnd(owner.ProfileID, "hangup", testNow), ErrNotMember)
}

func TestPreparationCannotIssueTokenOrCommitBeforeSDK(t *testing.T) {
	room, member, op := pendingGroup(t)
	require.False(t, room.CanIssueToken(member.ProfileID))
	require.ErrorIs(t, room.CommitAdmission(op, testNow), ErrEffectPending)
	require.Empty(t, room.CommittedProfileIDs())
	require.Empty(t, room.Events)

	require.NoError(t, room.RecordEnsureResult(op, true, testNow))
	require.NoError(t, room.CommitAdmission(op, testNow))
	require.True(t, room.CanIssueToken(member.ProfileID))
	require.Equal(t, []uuid.UUID{member.ProfileID}, room.CommittedProfileIDs())
	require.Len(t, room.Events, 3) // legacy accepted/started plus committed joined
}

func TestEndWinsAgainstPendingJoinWithoutInventedLeft(t *testing.T) {
	room, first := activeGroup(t)
	joiner := identity()
	op := uuid.New()
	require.NoError(t, room.BeginJoin(op, joiner, testNow))
	require.False(t, room.CanIssueToken(joiner.ProfileID))
	before := len(room.Events)
	require.NoError(t, room.DecideEnd(first.ProfileID, "hangup", testNow))
	require.ErrorIs(t, room.CommitAdmission(op, testNow), ErrAdmissionAborted)
	require.NoError(t, room.AbortAdmission(op, testNow))
	require.False(t, room.CanIssueToken(first.ProfileID))
	require.False(t, room.CanIssueToken(joiner.ProfileID))
	require.Len(t, room.Events, before+2) // first left + terminal legacy event
	for _, event := range room.Events {
		if event.Kind == EventParticipantLeft {
			require.Equal(t, first.ProfileID, event.ProfileID)
		}
	}
}

func TestLastLeaveAbortsPendingJoinButCommittedJoinKeepsRoomOpen(t *testing.T) {
	for _, joinWins := range []bool{false, true} {
		t.Run(map[bool]string{false: "leave_first", true: "join_first"}[joinWins], func(t *testing.T) {
			room, first := activeGroup(t)
			joiner := identity()
			op := uuid.New()
			require.NoError(t, room.BeginJoin(op, joiner, testNow))
			if joinWins {
				require.NoError(t, room.CommitAdmission(op, testNow))
			}
			require.NoError(t, room.DecideLeave(first.ProfileID, testNow))
			if joinWins {
				require.True(t, room.CanIssueToken(joiner.ProfileID))
				require.Equal(t, StageActive, room.Stage)
			} else {
				require.ErrorIs(t, room.CommitAdmission(op, testNow), ErrAdmissionAborted)
				require.Equal(t, StageClosing, room.Stage)
			}
		})
	}
}

func TestLateEnsureAfterAbortNeverRevivesRoom(t *testing.T) {
	room, member, op := pendingGroup(t)
	require.NoError(t, room.AbortAdmission(op, testNow))
	require.NoError(t, room.RecordEnsureResult(op, true, testNow.Add(time.Second)))
	require.False(t, room.CanIssueToken(member.ProfileID))
	require.ErrorIs(t, room.CommitAdmission(op, testNow), ErrAdmissionAborted)
	require.Empty(t, room.Events)
	require.Equal(t, StageAborting, room.Stage)
	require.True(t, room.HasPendingClose())
}

func TestCommittedAdmissionReplayRetainsEventAndMediaIdentity(t *testing.T) {
	room, first := activeGroup(t)
	joiner := identity()
	op := uuid.New()
	require.NoError(t, room.BeginJoin(op, joiner, testNow))
	require.NoError(t, room.CommitAdmission(op, testNow))
	member := room.Members[joiner.ProfileID.String()]
	eventCount := len(room.Events)
	lastEventID := room.Events[eventCount-1].ID
	require.NoError(t, room.CommitAdmission(op, testNow.Add(time.Second)))
	require.Len(t, room.Events, eventCount)
	require.Equal(t, lastEventID, room.Events[eventCount-1].ID)
	require.Equal(t, member.MediaIdentity, room.Members[joiner.ProfileID.String()].MediaIdentity)

	require.NoError(t, room.DecideLeave(joiner.ProfileID, testNow))
	require.False(t, room.CanIssueToken(joiner.ProfileID))
	// Rejoin is forbidden until old removal is confirmed, then gets a new identity.
	require.ErrorIs(t, room.BeginJoin(uuid.New(), joiner, testNow), ErrCleanupPending)
	require.NoError(t, room.CompleteMemberCleanup(joiner.ProfileID, member.Generation, testNow))
	newOp := uuid.New()
	require.NoError(t, room.BeginJoin(newOp, joiner, testNow))
	require.NoError(t, room.CommitAdmission(newOp, testNow))
	current := room.Members[joiner.ProfileID.String()]
	require.NotEqual(t, member.Generation, current.Generation)
	require.NotEqual(t, member.MediaIdentity, current.MediaIdentity)
	require.ErrorIs(t, room.CompleteMemberCleanup(joiner.ProfileID, member.Generation, testNow), ErrStaleGeneration)
	require.True(t, room.CanIssueToken(joiner.ProfileID))
	require.True(t, room.CanIssueToken(first.ProfileID))
}

func TestRingingHasNoCommittedParticipantsAndEqualityExpires(t *testing.T) {
	caller, callee := identity(), identity()
	op := uuid.New()
	room, err := NewPendingRoom(CreateIntent{
		OperationID: op, RoomID: uuid.New(), ChatID: uuid.New(),
		Initiator: caller, Callee: &callee, MediaKind: 1,
	}, testNow)
	require.NoError(t, err)
	require.NoError(t, room.RecordEnsureResult(op, true, testNow))
	require.NoError(t, room.CommitAdmission(op, testNow))
	require.Equal(t, StageRinging, room.Stage)
	require.Equal(t, testNow.Add(30*time.Second), room.ExpiresAt)
	require.Empty(t, room.CommittedProfileIDs())
	require.False(t, room.CanIssueToken(caller.ProfileID))
	require.False(t, room.CanIssueToken(callee.ProfileID))
	require.ErrorIs(t, room.BeginAccept(uuid.New(), callee.ProfileID, room.ExpiresAt), ErrExpired)
	require.NoError(t, room.DecideTimeout(room.ExpiresAt))
	for _, event := range room.Events {
		require.NotEqual(t, EventParticipantJoined, event.Kind)
		require.NotEqual(t, EventParticipantLeft, event.Kind)
	}
}

func TestAcceptAndTerminalHaveOneWinner(t *testing.T) {
	caller, callee := identity(), identity()
	start := uuid.New()
	room, err := NewPendingRoom(CreateIntent{
		OperationID: start, RoomID: uuid.New(), ChatID: uuid.New(),
		Initiator: caller, Callee: &callee, MediaKind: 1,
	}, testNow)
	require.NoError(t, err)
	require.NoError(t, room.RecordEnsureResult(start, true, testNow))
	require.NoError(t, room.CommitAdmission(start, testNow))
	accept := uuid.New()
	require.NoError(t, room.BeginAccept(accept, callee.ProfileID, testNow))
	require.NoError(t, room.CommitAdmission(accept, testNow))
	require.ErrorIs(t, room.DecideDecline(callee.ProfileID, testNow), ErrNotRinging)
	require.ErrorIs(t, room.DecideTimeout(room.ExpiresAt), ErrNotRinging)
	require.Len(t, room.CommittedProfileIDs(), 2)
	before := len(room.Events)
	require.NoError(t, room.DecideEnd(caller.ProfileID, "hangup", testNow))
	require.NoError(t, room.DecideEnd(caller.ProfileID, "hangup", testNow))
	require.Len(t, room.Events, before+3) // two left, one ended
}

func TestMissedTimeoutRetainsLegacyMissedBeforeEnded(t *testing.T) {
	caller, callee := identity(), identity()
	op := uuid.New()
	room, err := NewPendingRoom(CreateIntent{OperationID: op, RoomID: uuid.New(),
		ChatID: uuid.New(), Initiator: caller, Callee: &callee, MediaKind: 1}, testNow)
	require.NoError(t, err)
	require.NoError(t, room.RecordEnsureResult(op, true, testNow))
	require.NoError(t, room.CommitAdmission(op, testNow))
	require.NoError(t, room.DecideTimeout(room.ExpiresAt))
	require.Equal(t, []EventKind{EventIncoming, EventKind("missed"), EventEnded}, []EventKind{
		room.Events[0].Kind, room.Events[1].Kind, room.Events[len(room.Events)-1].Kind,
	})
}

func TestJoinOperationReplayRejectsChangedTrustedAccount(t *testing.T) {
	room, _ := activeGroup(t)
	joiner := identity()
	op := uuid.New()
	require.NoError(t, room.BeginJoin(op, joiner, testNow))
	require.NoError(t, room.CommitAdmission(op, testNow))
	joiner.AccountID = uuid.New()
	require.ErrorIs(t, room.BeginJoin(op, joiner, testNow), ErrConflict)
}

func TestRejectedRejoinDoesNotAppendRetiredMembership(t *testing.T) {
	room, _ := activeGroup(t)
	joiner := identity()
	op := uuid.New()
	require.NoError(t, room.BeginJoin(op, joiner, testNow))
	require.NoError(t, room.CommitAdmission(op, testNow))
	old := room.Members[joiner.ProfileID.String()]
	require.NoError(t, room.DecideLeave(joiner.ProfileID, testNow))
	require.NoError(t, room.CompleteMemberCleanup(joiner.ProfileID, old.Generation, testNow))
	for range 31 {
		id, operation := identity(), uuid.New()
		require.NoError(t, room.BeginJoin(operation, id, testNow))
		require.NoError(t, room.CommitAdmission(operation, testNow))
	}
	retired := len(room.RetiredMembers)
	require.ErrorIs(t, room.BeginJoin(uuid.New(), joiner, testNow), ErrCapacity)
	require.Len(t, room.RetiredMembers, retired)
}

func TestGroupStartRetainsExistingAcceptedThenStartedSignaling(t *testing.T) {
	room, _ := activeGroup(t)
	var kinds []EventKind
	for _, event := range room.Events {
		kinds = append(kinds, event.Kind)
	}
	require.Equal(t, []EventKind{EventAccepted, EventCallStarted, EventParticipantJoined}, kinds)
}

func TestTerminalOutboxAudienceIsTheCommittedPreTerminalSnapshot(t *testing.T) {
	room, first := activeGroup(t)
	second, op := identity(), uuid.New()
	require.NoError(t, room.BeginJoin(op, second, testNow))
	require.NoError(t, room.CommitAdmission(op, testNow))
	expected := room.CommittedProfileIDs()
	require.NoError(t, room.DecideEnd(first.ProfileID, "hangup", testNow))
	require.Empty(t, room.CommittedProfileIDs())
	require.Equal(t, EventEnded, room.Events[len(room.Events)-1].Kind)
	require.Equal(t, expected, room.Events[len(room.Events)-1].AudienceIDs)
}

func TestOldDeleteReceiptCannotClearCompensationForALaterCreate(t *testing.T) {
	room, member, operation := pendingGroup(t)
	require.NoError(t, room.AbortAdmission(operation, testNow))
	deleteBeforeLateCreate, ok := room.PendingClose()
	require.True(t, ok)
	// The old Delete executed before this late Create. Its later acknowledgement
	// is not absence proof for the newly observed effect.
	require.NoError(t, room.RecordEnsureResult(operation, true, testNow.Add(time.Second)))
	require.ErrorIs(t, room.CompleteRoomCleanup(deleteBeforeLateCreate.ID,
		deleteBeforeLateCreate.Revision, testNow), ErrStaleGeneration)
	require.True(t, room.HasPendingClose())
	current, ok := room.PendingClose()
	require.True(t, ok)
	require.Greater(t, current.Revision, deleteBeforeLateCreate.Revision)
	require.NoError(t, room.CompleteRoomCleanup(current.ID, current.Revision, testNow))
	require.Equal(t, StageClosed, room.Stage)
	require.False(t, room.CanIssueToken(member.ProfileID))
	require.False(t, room.HasPendingClose())
	// A success observed even after confirmed cleanup requeues exact-name close.
	require.NoError(t, room.RecordEnsureResult(operation, true, testNow))
	require.True(t, room.HasPendingClose())
	require.Equal(t, StageClosed, room.Stage)
}
