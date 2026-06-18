package dispatch_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/push"
)

func TestStoryPusher_skipsWhenNoPush(t *testing.T) {
	p := &dispatch.StoryPusher{Tokens: nil, Pusher: nil}
	err := p.SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		uuid.NewString(): {Push: false},
	}, push.Payload{})
	require.NoError(t, err)
}
