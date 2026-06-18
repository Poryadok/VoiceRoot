package privacy

// Preset defaults aligned with docs/features/privacy.md table.
func PresetSettings(preset string) (showOnline, showGameStatus, showMmRating, showPhone, showStories,
	allowPhoneSearch, allowDM, allowCalls, allowInvites, allowFiles, allowVoice, allowFriendRequests Audience) {
	switch preset {
	case "personal":
		return FriendsOnly(), FriendsOnly(), FriendsAndFoF(), Nobody(), FriendsAndFoF(),
			FriendsOnly(), FriendsAndFoF(), FriendsOnly(), FriendsOnly(), FriendsAndFoF(), FriendsOnly(), EveryoneWithGuests()
	case "work":
		return SpaceMembersOnly(), Nobody(), Nobody(), Nobody(), Nobody(),
			Nobody(), SpaceMembersAndFriends(), SpaceMembersAndFriends(), Nobody(), SpaceMembersAndFriends(), SpaceMembersAndFriends(), SpaceMembersOnly()
	default: // gaming
		return EveryoneWithGuests(), EveryoneWithGuests(), EveryoneWithGuests(), Nobody(), EveryoneWithGuests(),
			FriendsOnly(), EveryoneWithGuests(), FriendsAndFoF(), FriendsAndFoF(), FriendsAndFoF(), FriendsAndFoF(), EveryoneWithGuests()
	}
}

// Settings holds all privacy audience fields for a profile row.
type Settings struct {
	Preset               string
	ShowOnline           Audience
	ShowGameStatus       Audience
	ShowMmRating         Audience
	ShowPhone            Audience
	ShowStories          Audience
	AllowPhoneSearch     Audience
	AllowDM              Audience
	AllowCalls           Audience
	AllowChatSpaceInvites Audience
	AllowFiles           Audience
	AllowVoiceMessages   Audience
	AllowFriendRequests  Audience
	AllowGuestDM         bool
}

func SettingsForPreset(preset string) Settings {
	showOnline, showGameStatus, showMmRating, showPhone, showStories,
		allowPhoneSearch, allowDM, allowCalls, allowInvites, allowFiles, allowVoice, allowFriendRequests := PresetSettings(preset)
	return Settings{
		Preset:               preset,
		ShowOnline:           showOnline,
		ShowGameStatus:       showGameStatus,
		ShowMmRating:         showMmRating,
		ShowPhone:            showPhone,
		ShowStories:          showStories,
		AllowPhoneSearch:     allowPhoneSearch,
		AllowDM:              allowDM,
		AllowCalls:           allowCalls,
		AllowChatSpaceInvites: allowInvites,
		AllowFiles:           allowFiles,
		AllowVoiceMessages:   allowVoice,
		AllowFriendRequests:  allowFriendRequests,
		AllowGuestDM:         preset == "gaming",
	}
}
