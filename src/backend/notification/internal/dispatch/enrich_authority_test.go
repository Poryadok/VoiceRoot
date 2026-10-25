package dispatch_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/presence"
)

type failedRoutingAuthority struct{ err error }

func (p failedRoutingAuthority) IsOnline(context.Context, uuid.UUID) (bool, error) {
	return false, p.err
}

func TestEnrichDecisions_RequiresRoutingAuthority(t *testing.T) {
	failure := errors.New("authority unavailable")
	for _, tc := range []struct {
		name    string
		checker presence.Checker
	}{
		{"missing", nil}, {"unavailable", failedRoutingAuthority{failure}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recipient := uuid.New()
			got, err := dispatch.EnrichDecisions(t.Context(), tc.checker, nil, map[string]delivery.DeliveryDecision{recipient.String(): {}}, uuid.New(), "", delivery.TypeFriendReq)
			require.Error(t, err, "unknown presence must retry instead of allowing push")
			require.Nil(t, got, "no partial dispatch on authority failure")
			if tc.checker != nil {
				require.ErrorIs(t, err, failure)
			}
		})
	}
}
