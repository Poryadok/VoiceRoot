package limits

import (
	"encoding/json"

	"voice/backend/subscription/internal/testfixtures"
)

// ForAccount returns entitlement limits JSON for an account subscription tier.
func ForAccount(tier string) string {
	return buildJSON(accountLimits(tier))
}

// ForSpace returns entitlement limits JSON for a space subscription tier.
func ForSpace(hasSpacePro bool) string {
	return buildJSON(spaceLimits(hasSpacePro))
}

func spaceLimits(hasSpacePro bool) map[string]int64 {
	return map[string]int64{
		"space_member_count":      SpaceMemberCap(hasSpacePro),
		"voice_room_participants": int64(VoiceRoomCap(hasSpacePro)),
		"space_tree_nodes":        spaceTreeNodes(hasSpacePro),
		"space_banner":            flag(hasSpacePro),
		"space_custom_emoji":      flag(hasSpacePro),
	}
}

func spaceTreeNodes(hasSpacePro bool) int64 {
	if hasSpacePro {
		return testfixtures.SpaceTreeNodesSpacePro
	}
	return testfixtures.SpaceTreeNodesFree
}

func accountLimits(tier string) map[string]int64 {
	paid := tier == "premium" || tier == "grace_period"
	m := map[string]int64{
		"file_upload_bytes":       testfixtures.FileUploadBytesFree,
		"profile_count":           testfixtures.ProfileCountFree,
		"file_retention_days":     90,
		"e2e_file_retention_days": 90,
		"message_reaction_count":  1,
	}
	if paid {
		m["file_upload_bytes"] = testfixtures.FileUploadBytesPremium
		m["profile_count"] = testfixtures.ProfileCountPremium
		m["file_retention_days"] = 0 // Unlimited for live non-E2E references only.
		m["message_reaction_count"] = 3
	}
	for _, key := range []string{
		"premium_badge", "animated_avatar", "profile_banner", "custom_status",
		"emoji_status", "premium_username_count", "exclusive_stickers",
		"exclusive_emoji", "custom_app_icons", "exclusive_chat_themes",
		"file_download_priority",
	} {
		m[key] = flag(paid)
	}
	return m
}

func flag(enabled bool) int64 {
	if enabled {
		return 1
	}
	return 0
}

// SpaceMemberCap returns max members for a space entitlement.
func SpaceMemberCap(hasSpacePro bool) int64 {
	if hasSpacePro {
		return testfixtures.SpaceMemberCountSpacePro
	}
	return testfixtures.SpaceMemberCountFree
}

// VoiceRoomCap returns max voice room participants for a space.
func VoiceRoomCap(hasSpacePro bool) int {
	if hasSpacePro {
		return testfixtures.VoiceRoomParticipantsSpacePro
	}
	return testfixtures.VoiceRoomParticipantsFree
}

func buildJSON(m map[string]int64) string {
	b, _ := json.Marshal(m)
	return string(b)
}
