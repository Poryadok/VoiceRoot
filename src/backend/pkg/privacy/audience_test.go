package privacy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAudienceNobody(t *testing.T) {
	require.True(t, Nobody().IsNobody())
	require.False(t, FriendsOnly().IsNobody())
}

func TestAudienceEveryoneShortcut(t *testing.T) {
	e := EveryoneWithGuests()
	require.True(t, e.IsEveryoneShortcut())
	require.False(t, FriendsOnly().IsEveryoneShortcut())
}

func TestValidateAudience_spaceIDsRequireSpaceMembers(t *testing.T) {
	err := ValidateAudience("show_online", Audience{SpaceIDs: []string{"s1"}})
	require.Error(t, err)
	require.NoError(t, ValidateAudience("show_online", SpaceMembersOnly()))
}

func TestPresetWork_showOnlineSpaceMembers(t *testing.T) {
	s := SettingsForPreset("work")
	require.True(t, s.ShowOnline.SpaceMembers)
	require.False(t, s.ShowOnline.Friends)
	require.True(t, s.AllowDM.SpaceMembers)
	require.True(t, s.AllowDM.Friends)
}

func TestPresetPersonal_allowDmFriendsAndFoF(t *testing.T) {
	s := SettingsForPreset("personal")
	require.True(t, s.AllowDM.Friends)
	require.True(t, s.AllowDM.FriendsOfFriends)
}

func TestPresetGaming_everyoneWithGuests(t *testing.T) {
	s := SettingsForPreset("gaming")
	require.True(t, s.ShowOnline.IsEveryoneShortcut())
	require.True(t, s.AllowGuestDM)
}

func TestFromProto_nil(t *testing.T) {
	require.Equal(t, Audience{}, FromProto(nil))
}

func TestToProto_roundTrip(t *testing.T) {
	src := FriendsAndFoF()
	p := ToProto(src)
	back := FromProto(p)
	require.Equal(t, src, back)
}

func TestJSON_roundTrip(t *testing.T) {
	src := Audience{Friends: true, SpaceMembers: true, SpaceIDs: []string{"s1"}}
	data, err := MarshalJSON(src)
	require.NoError(t, err)
	got, err := UnmarshalJSON(data)
	require.NoError(t, err)
	require.Equal(t, src, got)
}

func TestUnmarshalJSON_empty(t *testing.T) {
	got, err := UnmarshalJSON(nil)
	require.NoError(t, err)
	require.Equal(t, Audience{}, got)
}
