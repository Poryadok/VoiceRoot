package s2s

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGameTombstoneKeySourceRequiresExplicitKeyAndExposesPublicJWKOnly(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	path := t.TempDir() + "/key.pem"
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600))
	now := time.Now().UTC().Truncate(time.Second)
	id := uuid.New()
	config := GameTombstoneKeyConfig{KeyID: id.String(), PrivateKeyFile: path, NotBefore: now.Add(-time.Minute).Format(time.RFC3339), NotAfter: now.Add(time.Hour).Format(time.RFC3339)}
	_, err = LoadGameTombstoneKeySource(GameTombstoneKeyConfig{})
	require.Error(t, err)
	source, err := LoadGameTombstoneKeySource(config)
	require.NoError(t, err)
	key, err := source.CurrentGameTombstoneKey(context.Background())
	require.NoError(t, err)
	require.Equal(t, id, key.KeyID)
	public, err := source.PublicJWKS()
	require.NoError(t, err)
	require.Contains(t, string(public), `"kid":"`+id.String()+`"`)
	require.NotContains(t, string(public), `"d"`)
}
