package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileJWKSContainsActiveSigningKey(t *testing.T) {
	active, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	document, err := json.Marshal(fileJWKSFromKeys(map[string]*rsa.PrivateKey{"current": active, "next": next}))
	require.NoError(t, err)
	require.NoError(t, assertFileJWKSActiveKey(document, "current", active))
	require.Error(t, assertFileJWKSActiveKey(document, "missing", active))
}
