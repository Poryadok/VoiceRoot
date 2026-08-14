package grpcsvc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFiltersFromReqEventType(t *testing.T) {
	got := filtersFromReq(map[string]string{"event_type": "api_request"})
	require.Equal(t, "api_request", got.EventType)
}

func TestFiltersFromReqEmpty(t *testing.T) {
	got := filtersFromReq(nil)
	require.Empty(t, got.EventType)
}
