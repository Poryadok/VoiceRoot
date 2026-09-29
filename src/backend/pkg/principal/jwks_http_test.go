package principal

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJWKSHandlerPublishesOnlyPublicRS256Key(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	handler := JWKSHandler("gis-key-1", &key.PublicKey)

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/internal/v1/principal/jwks.json", nil))
	require.Equal(t, http.StatusOK, get.Code)
	require.Equal(t, "application/jwk-set+json", get.Header().Get("Content-Type"))
	var document map[string][]map[string]any
	require.NoError(t, json.Unmarshal(get.Body.Bytes(), &document))
	require.Len(t, document["keys"], 1)
	item := document["keys"][0]
	require.Equal(t, "RSA", item["kty"])
	require.Equal(t, "gis-key-1", item["kid"])
	require.Equal(t, "RS256", item["alg"])
	require.NotContains(t, item, "d")
	parsed, err := ParseJWKS(get.Body.Bytes())
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/internal/v1/principal/jwks.json", nil))
	require.Equal(t, http.StatusMethodNotAllowed, post.Code)
}

func TestJWKSHandlerPublishesCurrentAndNextPublicKeys(t *testing.T) {
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	handler := JWKSHandlerKeys([]JWKSKey{{KeyID: "current", PublicKey: &current.PublicKey}, {KeyID: "next", PublicKey: &next.PublicKey}})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/internal/v1/principal/jwks.json", nil))
	require.Equal(t, http.StatusOK, response.Code)
	parsed, err := ParseJWKS(response.Body.Bytes())
	require.NoError(t, err)
	require.Len(t, parsed, 2)
	require.Contains(t, parsed, "current")
	require.Contains(t, parsed, "next")
}

func TestLoadRSAPrivateKeyFileAcceptsPKCS1AndRejectsNonRSA(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "issuer.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600))
	loaded, err := LoadRSAPrivateKeyFile(path)
	require.NoError(t, err)
	require.Equal(t, key.PublicKey, loaded.PublicKey)

	invalid := filepath.Join(t.TempDir(), "invalid.pem")
	require.NoError(t, os.WriteFile(invalid, []byte("not a key"), 0600))
	_, err = LoadRSAPrivateKeyFile(invalid)
	require.Error(t, err)
}
