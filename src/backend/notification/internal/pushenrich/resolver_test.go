package pushenrich_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/pushenrich"
)

func TestNoopResolver_ReturnsEmpty(t *testing.T) {
	var r pushenrich.NoopResolver
	preview, err := r.MessagePreview(context.Background(), "msg-1")
	require.NoError(t, err)
	require.Empty(t, preview)
	label, err := r.SenderLabel(context.Background(), "profile-1")
	require.NoError(t, err)
	require.Empty(t, label)
}
