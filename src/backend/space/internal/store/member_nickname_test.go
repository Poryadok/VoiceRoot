package store

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanonicalMemberNickname(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		invalid           bool
	}{
		{"trim", "\u2003  Alice  \u00a0", "Alice", false},
		{"clear", "\u2003\u00a0 ", "", false},
		{"cyrillic", "Игрок", "Игрок", false},
		{"interior", "A  B", "A  B", false},
		{"combining", "e\u0301", "e\u0301", false},
		{"zwj", "👩\u200d💻", "👩\u200d💻", false},
		{"scalar64", strings.Repeat("😀", 64), strings.Repeat("😀", 64), false},
		{"scalar65", strings.Repeat("😀", 65), "", true},
		{"newlineBeforeTrim", "\nAlice", "", true},
		{"tab", "A\tB", "", true},
		{"nul", "A\x00", "", true},
		{"separator", "A\u2028B", "", true},
		{"paragraph", "A\u2029B", "", true},
		{"bidi", "A\u202eB", "", true},
		{"isolate", "A\u2066B", "", true},
		{"invisibleOnly", "\u200d\u0301", "", true},
		{"invalidUTF8", string([]byte{0xff}), "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CanonicalMemberNickname(tc.input)
			if tc.invalid {
				require.ErrorIs(t, err, ErrInvalidMemberNickname)
				return
			}
			require.NoError(t, err)
			if tc.want == "" {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				require.Equal(t, tc.want, *got)
			}
		})
	}
}
