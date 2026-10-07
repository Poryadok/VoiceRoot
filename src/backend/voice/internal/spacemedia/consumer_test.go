package spacemedia

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/voice/internal/grpcsvc"
	"voice/backend/voice/internal/s2s"
)

type invalidationAckMessage struct {
	acked, nacked, termed int
	nakErr                error
}

func (m *invalidationAckMessage) AckSync(...nats.AckOpt) error {
	m.acked++
	return nil
}

func (m *invalidationAckMessage) NakWithDelay(_ time.Duration, _ ...nats.AckOpt) error {
	m.nacked++
	return m.nakErr
}

func (m *invalidationAckMessage) Term(...nats.AckOpt) error {
	m.termed++
	return nil
}

func TestSpaceAccessMessageACKFollowsFullReconciliation(t *testing.T) {
	ctx := context.Background()
	spaceID, roomID, voiceRoomID, profileID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	calls := newRoomStore(t, ctx, spaceID, roomID, voiceRoomID, profileID)
	coordinator := &Coordinator{
		Store:      calls,
		Admissions: fakeRecoveryForCommittedCall(t, ctx, calls, roomID),
		Access: fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{
			profileID: {SpaceID: spaceID, Member: true, Active: true, AccessEpoch: 4},
		}},
		Grants: fakeGrants{byProfile: map[string]s2s.VoiceRoomGrants{
			profileID: {PolicyEpoch: 7, CanJoin: true, CanSubscribe: true, CanPublishAudio: true},
		}},
		Media: &fakeMedia{},
	}
	envelope := &eventsv1.ChatStreamEvent{
		EventId: uuid.NewString(),
		Payload: &eventsv1.ChatStreamEvent_VoiceRoomAccessInvalidated{
			VoiceRoomAccessInvalidated: &eventsv1.VoiceRoomAccessInvalidated{SpaceId: spaceID, AccessEpoch: 4},
		},
	}
	data, err := proto.Marshal(envelope)
	require.NoError(t, err)
	msg := &invalidationAckMessage{}

	err = handleSpaceAccessInvalidation(ctx, spaceAccessSubject, data, msg, coordinator)

	require.NoError(t, err)
	require.Equal(t, 1, msg.acked)
	require.Zero(t, msg.nacked)
	require.Zero(t, msg.termed)
	progress, err := calls.GetSpaceMediaEpochProgress(ctx, spaceID)
	require.NoError(t, err)
	require.Equal(t, progress.Observed, progress.Reconciled)
}

func TestSpaceAccessMessageNAKsUntilEveryEjectionIsComplete(t *testing.T) {
	ctx := context.Background()
	spaceID, roomID, voiceRoomID, profileID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	calls := newRoomStore(t, ctx, spaceID, roomID, voiceRoomID, profileID)
	call, err := calls.GetCall(ctx, roomID)
	require.NoError(t, err)
	participant := call.SpaceMedia[profileID]
	media := &fakeMedia{err: errors.New("remove failed")}
	coordinator := &Coordinator{
		Store:      calls,
		Admissions: fakeRecoveryForCommittedCall(t, ctx, calls, roomID),
		Access: fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{
			profileID: {SpaceID: spaceID, Member: false, Active: true, AccessEpoch: 4},
		}},
		Grants: fakeGrants{byProfile: map[string]s2s.VoiceRoomGrants{}},
		Media:  media,
	}
	envelope := &eventsv1.ChatStreamEvent{
		EventId: uuid.NewString(),
		Payload: &eventsv1.ChatStreamEvent_VoiceRoomAccessInvalidated{
			VoiceRoomAccessInvalidated: &eventsv1.VoiceRoomAccessInvalidated{SpaceId: spaceID, AccessEpoch: 4},
		},
	}
	data, err := proto.Marshal(envelope)
	require.NoError(t, err)
	msg := &invalidationAckMessage{}

	err = handleSpaceAccessInvalidation(ctx, spaceAccessSubject, data, msg, coordinator)

	require.Error(t, err)
	require.Equal(t, []string{participant.Identity}, media.removed)
	current, err := calls.GetCall(ctx, roomID)
	require.NoError(t, err)
	retained, ok := current.SpaceMedia[profileID]
	require.True(t, ok, "failed media ejection must retain the roster target")
	require.Equal(t, participant.Identity, retained.Identity)
	require.Equal(t, participant.Generation, retained.Generation)
	require.True(t, retained.Revoking, "failed media ejection must retain the exact revocation target")
	require.Zero(t, msg.acked)
	require.Equal(t, 1, msg.nacked)
	require.Zero(t, msg.termed)
	progress, err := calls.GetSpaceMediaEpochProgress(ctx, spaceID)
	require.NoError(t, err)
	require.Zero(t, progress.Reconciled.AccessEpoch)
}

func TestMalformedSpaceAccessMessageIsTerminatedWithoutPayloadLogging(t *testing.T) {
	msg := &invalidationAckMessage{}
	err := handleSpaceAccessInvalidation(context.Background(), spaceAccessSubject, []byte("not-protobuf"), msg, &Coordinator{})
	require.Error(t, err)
	require.Zero(t, msg.acked)
	require.Zero(t, msg.nacked)
	require.Equal(t, 1, msg.termed)
}

var _ ackableInvalidation = (*invalidationAckMessage)(nil)
