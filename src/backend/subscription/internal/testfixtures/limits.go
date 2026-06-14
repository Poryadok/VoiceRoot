// Package testfixtures holds Phase 12 billing limit constants for red-phase tests.
package testfixtures

const (
	FileUploadBytesFree    int64 = 52_428_800  // 50 MiB
	FileUploadBytesPremium int64 = 209_715_200 // 200 MiB
	ProfileCountFree             = 2
	ProfileCountPremium            = 5
	SpaceMemberCountFree           = 50
	SpaceMemberCountSpacePro       = 5000
	SpaceTreeNodesFree             = 50
	SpaceTreeNodesSpacePro         = 500
	VoiceRoomParticipantsFree      = 32
	VoiceRoomParticipantsSpacePro  = 128
	PremiumMonthlyPriceCents       = 500
	YearlyDiscountPercent          = 20
)
