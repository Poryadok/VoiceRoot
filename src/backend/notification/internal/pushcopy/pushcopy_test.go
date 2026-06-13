package pushcopy_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/pushcopy"
)

func TestMessageBody(t *testing.T) {
	require.Equal(t, "You have a new message", pushcopy.MessageBody(""))
	require.Equal(t, "Hello", pushcopy.MessageBody("Hello"))
}

func TestMentionBody(t *testing.T) {
	require.Equal(t, "You were mentioned", pushcopy.MentionBody(""))
	require.Equal(t, "Mentioned you: ping", pushcopy.MentionBody("ping"))
}
