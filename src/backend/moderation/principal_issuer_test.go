package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestModerationIssuer_RequiresCompleteDistinctRotationRing(t *testing.T) {
	root := t.TempDir()
	write := func(name string, key *rsa.PrivateKey) {
		raw, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(root, name+".pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw}), 0600))
	}
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	write("current", current)
	_, _, err = loadModerationIssuer(root, "current")
	require.Error(t, err, "partial key ring must fail startup")
	write("next", current)
	_, _, err = loadModerationIssuer(root, "current")
	require.Error(t, err, "duplicate public keys cannot pretend to be a rotation pair")
	write("next", next)
	issuer, keys, err := loadModerationIssuer(root, "current")
	require.NoError(t, err)
	require.NotNil(t, issuer)
	require.Len(t, keys, 2)
	_, _, err = loadModerationIssuer(root, "missing")
	require.Error(t, err)
}
