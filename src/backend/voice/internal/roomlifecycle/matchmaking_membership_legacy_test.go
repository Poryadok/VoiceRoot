package roomlifecycle

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMatchmakingMembershipLegacyWritersAfterAllMigrations(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL legacy writer compatibility requires testcontainers")
	}
	for name, method := range map[string]LifecycleMethod{
		"join": LifecycleMethodJoin, "leave": LifecycleMethodLeave,
		"selfmove": LifecycleMethodSelfMove, "modmove": LifecycleMethodModeratorMove,
	} {
		t.Run(name, func(t *testing.T) {
			fixture := r22NewStoreFixture(t, "mmlegacy"+name)
			_, err := fixture.pool.Exec(fixture.ctx, r223ReadMigration(t, "up"))
			require.NoError(t, err)
			_, err = fixture.pool.Exec(fixture.ctx, mmReadMigration(t, "up"))
			require.NoError(t, err)
			require.NoError(t, fixture.store.CheckSchema(fixture.ctx))
			require.NoError(t, fixture.store.CheckMatchmakingMembershipSchema(fixture.ctx))
			assertUnknown := func() {
				t.Helper()
				var unknown bool
				require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT account_id IS NULL AND session_epoch IS NULL AND membership_state IS NULL
AND reconnect_started_at IS NULL AND reconnect_deadline IS NULL FROM voice_room_memberships WHERE profile_id=$1`, fixture.subjectProfileID).Scan(&unknown))
				require.True(t, unknown, "legacy writers must not invent verified identity")
			}
			if method != LifecycleMethodJoin {
				fixture.seedMembership(t, nil)
				assertUnknown()
			}
			decision := fixture.decision(method)
			operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
			require.NoError(t, err)
			r22ApplyAllEffects(t, fixture, decision)
			claim := r22ClaimOperation(t, fixture)
			sourceRoster, destinationRoster := int64(4), int64(1)
			receipt := r22JoinReceipt(fixture, decision, destinationRoster)
			outbox := []LifecycleOutboxRecord{r22TestOutbox(fixture, operation, 0, "joined", destinationRoster)}
			expectedRooms := make(map[uuid.UUID]int64)
			switch method {
			case LifecycleMethodLeave:
				receipt = LifecycleReceipt{OperationID: decision.OperationID, ActorProfileID: decision.ActorProfileID,
					SubjectProfileID: decision.SubjectProfileID, SpaceID: decision.SpaceID, Method: method, Outcome: LifecycleOutcomeLeft,
					SourceVoiceRoomID: operation.SourceVoiceRoomID, RoomID: operation.SourceRoomID, SourceRosterVersion: &sourceRoster}
				outbox = []LifecycleOutboxRecord{r22TestOutbox(fixture, operation, 0, "left", sourceRoster)}
				expectedRooms[*operation.SourceRoomID] = sourceRoster
			case LifecycleMethodSelfMove, LifecycleMethodModeratorMove:
				receipt.Outcome, receipt.SourceVoiceRoomID, receipt.SourceRosterVersion = LifecycleOutcomeMoved, operation.SourceVoiceRoomID, &sourceRoster
				left := r22TestOutbox(fixture, operation, 0, "left", sourceRoster)
				left.RoomID, left.VoiceRoomID = *operation.SourceRoomID, *operation.SourceVoiceRoomID
				outbox = []LifecycleOutboxRecord{left, r22TestOutbox(fixture, operation, 1, "joined", destinationRoster)}
				expectedRooms[*operation.SourceRoomID] = sourceRoster
			}
			if method != LifecycleMethodLeave {
				expectedRooms[*operation.DestinationRoomID] = destinationRoster
			}
			completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
				WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: receipt, Outbox: outbox,
				CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
			completed, err := fixture.store.CompleteOperation(fixture.ctx, completion)
			require.NoError(t, err)
			r22RequireCompletedReceipt(t, fixture, completed, receipt)
			r22RequireOutboxRows(t, fixture, completion)
			for roomID, version := range expectedRooms {
				room, found, loadErr := fixture.store.LoadRoomSnapshot(fixture.ctx, roomID)
				require.NoError(t, loadErr)
				require.True(t, found)
				require.Equal(t, version, room.RosterVersion)
			}
			membership, found, err := fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
			require.NoError(t, err)
			if method == LifecycleMethodLeave {
				require.False(t, found)
				return
			}
			require.True(t, found)
			require.Equal(t, *operation.DestinationRoomID, membership.RoomID)
			require.Equal(t, *operation.DestinationMediaEpoch, membership.MediaEpoch)
			assertUnknown()
			expiry := r22StoreTime.Add(time.Hour)
			advanced, err := fixture.store.AdvanceLatestGrantExpiry(fixture.ctx, fixture.subjectProfileID, membership.MediaEpoch, expiry)
			require.NoError(t, err)
			require.NotNil(t, advanced.LatestGrantExpiresAt)
			require.True(t, expiry.Equal(*advanced.LatestGrantExpiresAt))
			assertUnknown()
		})
	}
}
