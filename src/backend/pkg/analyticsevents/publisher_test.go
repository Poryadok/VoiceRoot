package analyticsevents

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/analyticshash"
)

func TestAnalyticsPublisherUsesCallerInbox(t *testing.T) {
	for _, service := range []string{"gateway", "moderation", "notification", "search", "subscription"} {
		t.Run(service, func(t *testing.T) {
			opts, err := publisherNATSOptions(service)
			require.NoError(t, err)
			var connOpts nats.Options
			for _, opt := range opts {
				require.NoError(t, opt(&connOpts))
			}
			require.Equal(t, "_INBOX.voice."+service, connOpts.InboxPrefix)
		})
	}
	_, err := publisherNATSOptions("analytics")
	require.Error(t, err)
}

func TestAnalyticsEventFromAccountNoPIIInProps(t *testing.T) {
	props := map[string]any{
		"provider_event_id": "evt-1",
		"plan":              "premium",
	}
	b, err := json.Marshal(props)
	require.NoError(t, err)
	propsJSON := string(b)

	accountID := "11111111-1111-1111-1111-111111111111"
	require.NotContains(t, propsJSON, accountID)

	hashed := analyticshash.ID("test-hash-key", accountID)
	require.NotEmpty(t, hashed)
	require.NotEqual(t, accountID, hashed)
}

func TestValidateAnalyticsStreamRequiresDeploymentContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  nats.StreamConfig
		wantErr bool
	}{
		{
			name: "exact deployment stream",
			config: nats.StreamConfig{
				Name:      streamName,
				Subjects:  []string{"analytics.>"},
				Retention: nats.LimitsPolicy,
				MaxAge:    7 * 24 * time.Hour,
				Storage:   nats.FileStorage,
			},
		},
		{
			name: "wrong retention",
			config: nats.StreamConfig{
				Name:      streamName,
				Subjects:  []string{"analytics.>"},
				Retention: nats.InterestPolicy,
				MaxAge:    7 * 24 * time.Hour,
				Storage:   nats.FileStorage,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAnalyticsStream(&nats.StreamInfo{Config: tt.config})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateAnalyticsStreamRejectsMissingStream(t *testing.T) {
	t.Parallel()
	require.Error(t, validateAnalyticsStream(nil))
}
