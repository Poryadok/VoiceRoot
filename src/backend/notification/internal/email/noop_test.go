package email_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/email"
)

func TestNoopSender_DoesNotError(t *testing.T) {
	var sender email.Sender = &email.NoopSender{}
	require.NoError(t, sender.Send(context.Background(), email.Message{
		To:      "user@example.com",
		Subject: "Verify",
		Body:    "code",
	}))
}
