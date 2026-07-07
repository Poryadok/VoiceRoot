package permissions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAllPermissionNames_Count42 documents app stack0 canonical permission set.
func TestAllPermissionNames_Count42(t *testing.T) {
	t.Parallel()
	all, err := AllMask()
	require.NoError(t, err)
	names := NamesFor(all)
	require.Len(t, names, 42)
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
