package permissions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestThreadPermissionBits_32Through34 documents Phase 10 thread permission bit positions.
func TestThreadPermissionBits_32Through34(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		bit  uint64
	}{
		{TextChatCreateThreads, 1 << 32},
		{TextChatSendInThreads, 1 << 33},
		{TextChatManageThreads, 1 << 34},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := MaskFor(tc.name)
			require.NoError(t, err)
			require.Equal(t, tc.bit, got)
		})
	}
}
