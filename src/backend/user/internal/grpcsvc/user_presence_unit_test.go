package grpcsvc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

func TestNormalizePresenceInput_statusString(t *testing.T) {
	cases := []struct {
		in       string
		wantSt   string
		wantEnum userv1.PresenceOnlineStatus
	}{
		{"online", "online", userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE},
		{"ONLINE", "online", userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE},
		{"idle", "idle", userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_IDLE},
		{"dnd", "dnd", userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_DND},
		{"invisible", "invisible", userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_INVISIBLE},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			s, e, err := normalizePresenceInput(&userv1.UpdatePresenceRequest{Status: tc.in})
			require.NoError(t, err)
			require.Equal(t, tc.wantSt, s)
			require.Equal(t, int32(tc.wantEnum), e)
		})
	}

	_, _, err := normalizePresenceInput(&userv1.UpdatePresenceRequest{Status: "nope"})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, _, err = normalizePresenceInput(&userv1.UpdatePresenceRequest{})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestNormalizePresenceInput_enumOverridesInvalidString(t *testing.T) {
	s, e, err := normalizePresenceInput(&userv1.UpdatePresenceRequest{
		Status:     "nope",
		StatusEnum: userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_DND.Enum(),
	})
	require.NoError(t, err)
	require.Equal(t, "dnd", s)
	require.Equal(t, int32(userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_DND), e)
}

func TestPresenceSnapshotToProto_live(t *testing.T) {
	pid := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	ts := time.Unix(2000000000, 0).UTC()
	out := presenceSnapshotToProto(pid, &store.PresenceSnapshot{
		Live:           true,
		Status:         "online",
		StatusEnum:     1,
		GameTitle:      "G1",
		CustomStatus:   "cs",
		CallInfoJSON:   `{"k":1}`,
		LastSeenUnix:   ts.Unix(),
		LastActiveUnix: ts.Unix(),
	})
	require.Equal(t, pid.String(), out.GetProfileId())
	require.Equal(t, "online", out.GetStatus())
	require.Equal(t, userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE, out.GetStatusEnum())
	require.Equal(t, "G1", out.GetGameTitle())
	require.Equal(t, "cs", out.GetCustomStatus())
	require.Equal(t, `{"k":1}`, out.GetCallInfoJson())
	require.NotNil(t, out.GetLastSeen())
}

func TestPresenceSnapshotToProto_offlineLastSeenOnly(t *testing.T) {
	pid := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	out := presenceSnapshotToProto(pid, &store.PresenceSnapshot{
		Live:         false,
		LastSeenUnix: 42,
	})
	require.Equal(t, pid.String(), out.GetProfileId())
	require.Empty(t, out.GetStatus())
	require.Empty(t, out.GetGameTitle())
	require.Empty(t, out.GetCustomStatus())
	require.Empty(t, out.GetCallInfoJson())
	require.Equal(t, int64(42), out.GetLastSeen().AsTime().Unix())
}

func TestPresenceSnapshotToProto_nilSnapshot(t *testing.T) {
	pid := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	out := presenceSnapshotToProto(pid, nil)
	require.Equal(t, pid.String(), out.GetProfileId())
	require.Empty(t, out.GetStatus())
}
