package analyticsevents

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/pkg/analyticshash"
)

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
