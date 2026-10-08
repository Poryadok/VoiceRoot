package grpcsvc

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/pkg/privacy"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

func TestScheduledRecipientOnlineRequiresVisibleExactOnlineLiveState(t *testing.T) {
	for name, tc := range map[string]struct {
		snapshot *store.PresenceSnapshot
		want     bool
		wantErr  bool
	}{
		"no snapshot":         {nil, false, false},
		"not live":            {&store.PresenceSnapshot{Live: false, Status: "online", StatusEnum: int32(userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE)}, false, false},
		"enum online":         {&store.PresenceSnapshot{Live: true, StatusEnum: int32(userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE)}, true, false},
		"string online":       {&store.PresenceSnapshot{Live: true, Status: "online"}, true, false},
		"idle":                {&store.PresenceSnapshot{Live: true, StatusEnum: int32(userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_IDLE)}, false, false},
		"dnd":                 {&store.PresenceSnapshot{Live: true, StatusEnum: int32(userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_DND)}, false, false},
		"invisible":           {&store.PresenceSnapshot{Live: true, StatusEnum: int32(userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_INVISIBLE)}, false, false},
		"offline":             {&store.PresenceSnapshot{Live: true, Status: "offline"}, false, false},
		"unknown enum":        {&store.PresenceSnapshot{Live: true, StatusEnum: 77}, false, true},
		"unknown live string": {&store.PresenceSnapshot{Live: true, Status: "connecting"}, false, true},
		"missing live status": {&store.PresenceSnapshot{Live: true}, false, true},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := scheduledRecipientOnline(tc.snapshot)
			require.Equal(t, tc.want, got)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestScheduledAudienceRequiresConfiguredRelationshipAuthorities(t *testing.T) {
	for name, tc := range map[string]struct {
		matcher  privacy.Matcher
		audience privacy.Audience
		want     bool
	}{
		"public audience needs none": {privacy.Matcher{}, privacy.Audience{}, true},
		"friends require Social":     {privacy.Matcher{}, privacy.FriendsOnly(), false},
		"space requires Space":       {privacy.Matcher{}, privacy.SpaceMembersOnly(), false},
		"configured friends":         {privacy.Matcher{Social: alwaysFriendsGraph{}}, privacy.FriendsOnly(), true},
		"configured space":           {privacy.Matcher{Space: stubSpaceCoMembership{}}, privacy.SpaceMembersOnly(), true},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, scheduledAudienceDependenciesReady(tc.matcher, tc.audience))
		})
	}
}
