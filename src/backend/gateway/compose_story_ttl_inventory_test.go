package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStoryTTLInventoryRequiresExactObjects(t *testing.T) {
	keys := []string{"original", "processed/a/full.webp", "processed/a/thumb.webp"}
	present := "{\"status\":\"success\",\"key\":\"original\"}\n{\"status\":\"success\",\"key\":\"processed/a/full.webp\"}\n{\"status\":\"success\",\"key\":\"processed/a/thumb.webp\"}\n"
	ok, err := storyTTLInventoryMatches(strings.NewReader(present), keys, true)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = storyTTLInventoryMatches(strings.NewReader(present), keys, false)
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = storyTTLInventoryMatches(strings.NewReader(""), keys, false)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = storyTTLInventoryMatches(strings.NewReader("{\"status\":\"success\",\"key\":\"original\"}\n"), keys, true)
	require.NoError(t, err)
	require.False(t, ok, "one surviving object is not a complete before inventory")
	ok, err = storyTTLInventoryMatches(strings.NewReader("{\"status\":\"success\",\"key\":\"original\"}\n"), keys, false)
	require.NoError(t, err)
	require.False(t, ok, "one surviving object must prevent physical deletion PASS")
	_, err = storyTTLInventoryMatches(strings.NewReader("{\"status\":\"error\"}\n"), keys, false)
	require.Error(t, err)
	_, err = storyTTLInventoryMatches(strings.NewReader("not json\n"), keys, false)
	require.Error(t, err)
}
