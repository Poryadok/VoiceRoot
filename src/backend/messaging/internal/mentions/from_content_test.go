package mentions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEntriesJSONFromContent(t *testing.T) {
	raw := EntriesJSONFromContent("**hi** @everyone ping @01020304-0506-0708-090a-0b0c0d0e0f10")
	require.JSONEq(t, `[{"type":"everyone"},{"type":"user","target_id":"01020304-0506-0708-090a-0b0c0d0e0f10"}]`, raw)
	require.Equal(t, "[]", EntriesJSONFromContent("plain text"))
}
