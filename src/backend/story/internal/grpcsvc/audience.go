package grpcsvc

import (
	"encoding/json"
	"strings"

	"voice/backend/pkg/privacy"

	storyv1 "voice.app/voice/story/v1"
)

func audienceFromStoryRow(visibility string, audienceJSON *string) privacy.Audience {
	if audienceJSON != nil && strings.TrimSpace(*audienceJSON) != "" {
		var a privacy.Audience
		if err := json.Unmarshal([]byte(*audienceJSON), &a); err == nil {
			return a
		}
	}
	switch strings.TrimSpace(visibility) {
	case "everyone":
		return privacy.EveryoneWithGuests()
	case "friends":
		return privacy.FriendsOnly()
	case "close_friends":
		return privacy.FriendsAndFoF()
	case "custom":
		return privacy.Nobody()
	default:
		return privacy.FriendsOnly()
	}
}

func audienceToStoryVisibility(a privacy.Audience) (visibility string, audienceJSON *string) {
	if a.IsEveryoneShortcut() {
		return "everyone", nil
	}
	if a.IsNobody() {
		return "custom", audienceJSONPtr(a)
	}
	if a.Friends && a.FriendsOfFriends && !a.SpaceMembers && !a.IncludeGuests && len(a.SpaceIDs) == 0 {
		return "close_friends", nil
	}
	if a.Friends && !a.FriendsOfFriends && !a.SpaceMembers && !a.IncludeGuests && len(a.SpaceIDs) == 0 {
		return "friends", nil
	}
	return "custom", audienceJSONPtr(a)
}

func audienceJSONPtr(a privacy.Audience) *string {
	b, err := json.Marshal(a)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}

func visibilityFromRequest(visibility string, enum storyv1.StoryAudience) (string, *string) {
	v := strings.TrimSpace(visibility)
	if v == "" {
		v = storyAudienceString(enum)
	}
	switch v {
	case "everyone", "public":
		return "everyone", nil
	case "friends":
		return "friends", nil
	case "close_friends":
		return "close_friends", nil
	case "custom":
		return "custom", audienceJSONPtr(privacy.Nobody())
	default:
		return "", nil
	}
}

func storyAudienceString(v storyv1.StoryAudience) string {
	switch v {
	case storyv1.StoryAudience_STORY_AUDIENCE_PUBLIC:
		return "everyone"
	case storyv1.StoryAudience_STORY_AUDIENCE_FRIENDS:
		return "friends"
	case storyv1.StoryAudience_STORY_AUDIENCE_CLOSE_FRIENDS:
		return "close_friends"
	case storyv1.StoryAudience_STORY_AUDIENCE_CUSTOM:
		return "custom"
	default:
		return ""
	}
}
