package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMessageViewDeletionRuntimeFailsClosedOnPartialConfig(t *testing.T) {
	lookup := func(key string) (string, bool) {
		if key == "MESSAGING_USER_DELETION_GRPC_ADDR" {
			return "user:9096", true
		}
		return "", false
	}
	checker, conn, err := newMessageViewDeletionClient(lookup, nil)
	require.Error(t, err)
	require.Nil(t, checker)
	require.Nil(t, conn)
	complete := func(string) (string, bool) { return "configured", true }
	checker, conn, err = newMessageViewDeletionClient(complete, nil)
	require.Error(t, err)
	require.Nil(t, checker)
	require.Nil(t, conn)
	absent := func(string) (string, bool) { return "", false }
	checker, conn, err = newMessageViewDeletionClient(absent, nil)
	require.NoError(t, err)
	require.Nil(t, checker)
	require.Nil(t, conn)
}
