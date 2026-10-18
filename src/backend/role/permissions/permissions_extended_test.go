package permissions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAllPermissionNames_Count43 includes the accepted dedicated admission permission.
func TestAllPermissionNames_Count43(t *testing.T) {
	t.Parallel()
	all, err := AllMask()
	require.NoError(t, err)
	names := NamesFor(all)
	require.Len(t, names, 43)
}

func TestNamesFor_ReturnsCanonicalBitOrder(t *testing.T) {
	t.Parallel()
	all, err := AllMask()
	require.NoError(t, err)

	require.Equal(t, []string{
		SpaceView,
		SpaceManageSettings,
		SpaceManageRoles,
		SpaceManageInvites,
		SpaceViewAuditLog,
		SpaceManageCustomEmojis,
		SpaceManageBots,
		SpaceManageMatchmaking,
		SpaceViewMemberList,
		MemberKick,
		MemberBan,
		MemberManageNicknames,
		MemberAssignRoles,
		TextChatCreateInSpace,
		TextChatView,
		TextChatSendMessages,
		TextChatManageMessages,
		VoiceJoin,
		VoiceSpeak,
		VoiceMuteOthers,
		TextChatManageSettings,
		TextChatSetSlowMode,
		ModerationTimeoutMembers,
		TextChatMentionAllOnline,
		TextChatMentionAllInChat,
		TextChatPinMessages,
		TextChatSendMedia,
		TextChatEmbedLinks,
		TextChatAttachFiles,
		TextChatAddReactions,
		TextChatUseExternalEmojis,
		TextChatReadHistory,
		TextChatCreateThreads,
		TextChatSendInThreads,
		TextChatManageThreads,
		VoiceVideo,
		VoiceScreenShare,
		VoiceDeafenOthers,
		VoiceMoveOthers,
		VoiceUsePTT,
		VoicePrioritySpeaker,
		ModerationManageReports,
		SpaceApproveJoinRequests,
	}, NamesFor(all))
}

// TestExtendedPermissionBits_RoundTrip documents bits 26–41 from role-service.md.
func TestExtendedPermissionBits_RoundTrip(t *testing.T) {
	t.Parallel()
	extended := []string{
		TextChatSendMedia, TextChatEmbedLinks, TextChatAttachFiles, TextChatAddReactions,
		TextChatUseExternalEmojis, TextChatReadHistory, TextChatCreateThreads,
		TextChatSendInThreads, TextChatManageThreads,
		VoiceVideo, VoiceScreenShare, VoiceDeafenOthers, VoiceMoveOthers,
		VoiceUsePTT, VoicePrioritySpeaker, ModerationManageReports,
	}
	for _, name := range extended {
		bit, err := MaskFor(name)
		require.NoError(t, err, name)
		require.NotZero(t, bit, name)
	}
}
