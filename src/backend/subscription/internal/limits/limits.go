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
	}
}

func spaceTreeNodes(hasSpacePro bool) int64 {
	if hasSpacePro {
		return testfixtures.SpaceTreeNodesSpacePro
	}
	return testfixtures.SpaceTreeNodesFree
}

func accountLimits(tier string) map[string]int64 {
	if tier == "premium" || tier == "grace_period" {
		return map[string]int64{
			"file_upload_bytes": testfixtures.FileUploadBytesPremium,
			"profile_count":     testfixtures.ProfileCountPremium,
		}
	}
	return map[string]int64{
		"file_upload_bytes": testfixtures.FileUploadBytesFree,
		"profile_count":     testfixtures.ProfileCountFree,
	}
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
