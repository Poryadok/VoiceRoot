package limits

import (
	"encoding/json"

	"voice/backend/subscription/internal/testfixtures"
)

// ForAccount returns entitlement limits JSON for an account subscription tier.
func ForAccount(tier string) string {
	return buildJSON(accountLimits(tier))
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
