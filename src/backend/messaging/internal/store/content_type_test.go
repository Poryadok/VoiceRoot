package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInferLastMessageContentType_fromAttachments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		att   string
		want  string
	}{
		{"photo", `[{"type":"image","file_id":"f1"}]`, "photo"},
		{"voice", `[{"type":"voice_message","file_id":"f1"}]`, "voice"},
		{"gif", `[{"type":"gif","file_id":"f1"}]`, "gif"},
		{"empty", `[]`, "text"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := inferLastMessageContentType("hello", tc.att)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestInferLastMessageContentType_textFallback(t *testing.T) {
	t.Parallel()
	require.Equal(t, "text", inferLastMessageContentType("plain", "[]"))
	require.Equal(t, "", inferLastMessageContentType("", "[]"))
}

func TestEffectiveContentType_prefersColumn(t *testing.T) {
	t.Parallel()
	require.Equal(t, "photo", EffectiveContentType("photo", "ignored", `[{"type":"video"}]`))
	require.Equal(t, "video", EffectiveContentType("", "", `[{"type":"video"}]`))
}
