package grpcsvc

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"voice/backend/user/internal/store"

	eventsv1 "voice.app/voice/events/v1"
	userv1 "voice.app/voice/user/v1"
)

func TestPresenceTransitionForSnapshot_PublishesOnlyCanonicalEnumTransitions(t *testing.T) {
	online := int32(userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE)

	tests := []struct {
		name        string
		previous    *store.PresenceSnapshot
		request     *userv1.UpdatePresenceRequest
		wantOld     string
		wantNew     string
		wantPublish bool
	}{
		{
			name:        "first live observation publishes empty old status",
			previous:    &store.PresenceSnapshot{},
			request:     &userv1.UpdatePresenceRequest{Status: "ONLINE"},
			wantOld:     "",
			wantNew:     "online",
			wantPublish: true,
		},
		{
			name: "same enum heartbeat with ancillary changes does not publish",
			previous: &store.PresenceSnapshot{
				Live:           true,
				Status:         "online",
				StatusEnum:     online,
				GameTitle:      "Dota 2",
				CustomStatus:   "first",
				CallInfoJSON:   `{"room":"one"}`,
				LastActiveUnix: 1,
			},
			request: &userv1.UpdatePresenceRequest{
				Status:       "online",
				GameTitle:    proto.String("Counter-Strike 2"),
				CustomStatus: proto.String("second"),
				CallInfoJson: proto.String(`{"room":"two"}`),
			},
			wantPublish: false,
		},
		{
			name: "status enum transition publishes canonical online to dnd",
			previous: &store.PresenceSnapshot{
				Live:       true,
				Status:     "ONLINE",
				StatusEnum: online,
			},
			request: &userv1.UpdatePresenceRequest{
				Status:     "ONLINE",
				StatusEnum: userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_DND.Enum(),
			},
			wantOld:     "online",
			wantNew:     "dnd",
			wantPublish: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentStatus, currentEnum, err := normalizePresenceInput(tt.request)
			require.NoError(t, err)
			oldStatus, newStatus, publish := presenceTransitionForSnapshot(tt.previous, currentStatus, currentEnum)
			require.Equal(t, tt.wantPublish, publish)
			if !tt.wantPublish {
				return
			}
			require.Equal(t, tt.wantOld, oldStatus)
			require.Equal(t, tt.wantNew, newStatus)
		})
	}
}

func TestPresenceChangeProto_DeltaFieldsRoundTripWithLegacyCurrentStatus(t *testing.T) {
	want := &eventsv1.PresenceChange{
		ProfileId: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Status:    "dnd", // legacy consumers continue to see current/new status.
		OldStatus: "online",
		NewStatus: "dnd",
	}

	b, err := proto.Marshal(want)
	require.NoError(t, err)
	var got eventsv1.PresenceChange
	require.NoError(t, proto.Unmarshal(b, &got))
	require.Equal(t, want.GetProfileId(), got.GetProfileId())
	require.Equal(t, "dnd", got.GetStatus())
	require.Equal(t, "online", got.GetOldStatus())
	require.Equal(t, "dnd", got.GetNewStatus())
}

func TestPresenceChangeProto_DeltaFieldsKeepLegacyWireShapeAdditive(t *testing.T) {
	fields := (&eventsv1.PresenceChange{}).ProtoReflect().Descriptor().Fields()
	status := fields.ByName("status")
	oldStatus := fields.ByName("old_status")
	newStatus := fields.ByName("new_status")

	require.NotNil(t, status, "legacy status must remain in the event contract")
	require.NotNil(t, oldStatus, "old_status must be an additive delta field")
	require.NotNil(t, newStatus, "new_status must be an additive delta field")
	require.Equal(t, protoreflect.FieldNumber(2), status.Number())
	require.Equal(t, protoreflect.FieldNumber(3), oldStatus.Number())
	require.Equal(t, protoreflect.FieldNumber(4), newStatus.Number())
	require.NotEqual(t, status.Name(), oldStatus.Name())
	require.NotEqual(t, status.Name(), newStatus.Name())
	require.NotEqual(t, oldStatus.Name(), newStatus.Name())
	require.NotEqual(t, status.Number(), oldStatus.Number())
	require.NotEqual(t, status.Number(), newStatus.Number())
	require.NotEqual(t, oldStatus.Number(), newStatus.Number())
}
