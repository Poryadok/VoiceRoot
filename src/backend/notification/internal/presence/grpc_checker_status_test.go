package presence

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	userv1 "voice.app/voice/user/v1"
)

func TestIsActiveSessionPresence(t *testing.T) {
	tests := []struct {
		name   string
		status *userv1.PresenceStatus
		want   bool
	}{
		{
			name: "online enum",
			status: &userv1.PresenceStatus{
				StatusEnum: userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE.Enum(),
			},
			want: true,
		},
		{
			name: "idle enum remains an active session",
			status: &userv1.PresenceStatus{
				StatusEnum: userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_IDLE.Enum(),
			},
			want: true,
		},
		{
			name: "legacy idle string remains an active session",
			status: &userv1.PresenceStatus{
				Status: "IDLE",
			},
			want: true,
		},
		{
			name: "dnd enum remains an active session",
			status: &userv1.PresenceStatus{
				StatusEnum: userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_DND.Enum(),
			},
			want: true,
		},
		{
			name: "call metadata keeps a live session active",
			status: &userv1.PresenceStatus{
				StatusEnum:   userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_IDLE.Enum(),
				CallInfoJson: proto.String(`{"room_id":"room-1"}`),
			},
			want: true,
		},
		{
			name: "call metadata without a live status is stale",
			status: &userv1.PresenceStatus{
				CallInfoJson: proto.String(`{"room_id":"room-1"}`),
			},
			want: false,
		},
		{
			name: "invisible stays push-eligible even with call metadata",
			status: &userv1.PresenceStatus{
				StatusEnum:   userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_INVISIBLE.Enum(),
				CallInfoJson: proto.String(`{"room_id":"room-1"}`),
			},
			want: false,
		},
		{
			name:   "missing presence is offline",
			status: nil,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isActiveSessionPresence(tt.status))
		})
	}
}
