package permissions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModeratorDefaultMask_includesPinMessages(t *testing.T) {
	modMask, err := moderatorDefaultMask()
	require.NoError(t, err)
	ok, err := HasPermission(modMask, TextChatPinMessages)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestMemberDefaultMask_excludesPinMessages(t *testing.T) {
	memberMask, err := memberDefaultMask()
	require.NoError(t, err)
	ok, err := HasPermission(memberMask, TextChatPinMessages)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestMaskFor_TextChatPinMessages_bit25(t *testing.T) {
	bit, err := MaskFor(TextChatPinMessages)
	require.NoError(t, err)
	require.Equal(t, uint64(1<<25), bit)
}
