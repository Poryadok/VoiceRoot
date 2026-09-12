package presence

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	userv1 "voice.app/voice/user/v1"
)

type fakeUserServiceClient struct {
	userv1.UserServiceClient
	response *userv1.GetBulkPresenceResponse
	err      error
}

func (f fakeUserServiceClient) GetBulkPresence(context.Context, *userv1.GetBulkPresenceRequest, ...grpc.CallOption) (*userv1.GetBulkPresenceResponse, error) {
	return f.response, f.err
}

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
			name: "legacy online string remains an active session",
			status: &userv1.PresenceStatus{
				Status: "online",
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
			name: "legacy offline string stays push eligible",
			status: &userv1.PresenceStatus{
				Status: "offline",
			},
			want: false,
		},
		{
			name: "legacy unknown string stays push eligible",
			status: &userv1.PresenceStatus{
				Status: "away",
			},
			want: false,
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
			name: "explicit online enum wins over contradictory legacy invisible string",
			status: &userv1.PresenceStatus{
				StatusEnum: userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE.Enum(),
				Status:     "invisible",
			},
			want: true,
		},
		{
			name: "explicit unspecified enum fails closed despite legacy online string",
			status: &userv1.PresenceStatus{
				StatusEnum: userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_UNSPECIFIED.Enum(),
				Status:     "online",
			},
			want: false,
		},
		{
			name: "explicit unknown enum fails closed despite legacy online string",
			status: &userv1.PresenceStatus{
				StatusEnum: userv1.PresenceOnlineStatus(99).Enum(),
				Status:     "online",
			},
			want: false,
		},
		{
			name: "explicit invisible enum wins over contradictory legacy online string",
			status: &userv1.PresenceStatus{
				StatusEnum: userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_INVISIBLE.Enum(),
				Status:     "online",
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

func TestGRPCCheckerIsOnlineFailsClosedAtPresenceBoundary(t *testing.T) {
	profileID := uuid.New()
	otherProfileID := uuid.New()
	active := func(profile string) *userv1.PresenceStatus {
		return &userv1.PresenceStatus{
			ProfileId:  profile,
			StatusEnum: userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE.Enum(),
		}
	}

	tests := []struct {
		name     string
		response *userv1.GetBulkPresenceResponse
		want     bool
	}{
		{
			name: "exact requested profile active",
			response: &userv1.GetBulkPresenceResponse{ByProfileId: map[string]*userv1.PresenceStatus{
				profileID.String(): active(profileID.String()),
			}},
			want: true,
		},
		{
			name:     "requested map key missing",
			response: &userv1.GetBulkPresenceResponse{ByProfileId: map[string]*userv1.PresenceStatus{}},
		},
		{
			name: "requested status nil",
			response: &userv1.GetBulkPresenceResponse{ByProfileId: map[string]*userv1.PresenceStatus{
				profileID.String(): nil,
			}},
		},
		{
			name: "embedded profile missing",
			response: &userv1.GetBulkPresenceResponse{ByProfileId: map[string]*userv1.PresenceStatus{
				profileID.String(): active(""),
			}},
		},
		{
			name: "embedded profile mismatches requested key",
			response: &userv1.GetBulkPresenceResponse{ByProfileId: map[string]*userv1.PresenceStatus{
				profileID.String(): active(otherProfileID.String()),
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &GRPCChecker{client: fakeUserServiceClient{response: tt.response}}
			got, err := checker.IsOnline(context.Background(), profileID)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}

	t.Run("rpc error propagates", func(t *testing.T) {
		expected := errors.New("user service unavailable")
		checker := &GRPCChecker{client: fakeUserServiceClient{err: expected}}
		got, err := checker.IsOnline(context.Background(), profileID)
		require.False(t, got)
		require.ErrorIs(t, err, expected)
	})
}
