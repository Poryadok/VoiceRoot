package grpcsvc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseHandle_ok(t *testing.T) {
	u, d, err := parseHandle("  alice#0001  ")
	require.NoError(t, err)
	require.Equal(t, "alice", u)
	require.Equal(t, "0001", d)
}

func TestParseHandle_errors(t *testing.T) {
	for _, tc := range []struct {
		in string
	}{
		{""},
		{"alice"},
		{"alice#"},
		{"#0001"},
		{"alice#000"},
		{"alice#00001"},
		{"alice#abcd"},
		{"alice#12a4"},
	} {
		t.Run(tc.in, func(t *testing.T) {
			_, _, err := parseHandle(tc.in)
			require.Error(t, err)
		})
	}
}
