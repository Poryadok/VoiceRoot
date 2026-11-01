package sourcecustody

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/pkg/principal"
)

func TestUserJWKSHandlerServesOnlyPublicKeys(t *testing.T) {
	dir := t.TempDir()
	_, current := testPKCS8Key(t, 2048)
	_, next := testPKCS8Key(t, 2048)
	writeCustodyKey(t, dir, "current.pem", current)
	writeCustodyKey(t, dir, "next.pem", next)
	_, public, err := LoadUserSigner(dir, "current")
	require.NoError(t, err)
	handler, err := NewUserJWKSHandler(public)
	require.NoError(t, err)
	server := httptest.NewTLSServer(handler)
	defer server.Close()
	response, err := server.Client().Get(server.URL + "/.well-known/jwks.json")
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	keys, err := principal.ParseJWKS(body)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.Contains(t, keys, "current")
	require.Contains(t, keys, "next")
	require.NotContains(t, strings.ToUpper(string(body)), "PRIVATE")
	require.NotContains(t, string(body), `"d"`)
	var input map[string][]map[string]any
	require.NoError(t, json.Unmarshal(public, &input))
	input["keys"][0]["d"] = "must-not-be-served"
	withPrivate, err := json.Marshal(input)
	require.NoError(t, err)
	safeHandler, err := NewUserJWKSHandler(withPrivate)
	if err == nil {
		result := httptest.NewRecorder()
		safeHandler.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
		require.NotContains(t, result.Body.String(), "must-not-be-served")
		require.NotContains(t, result.Body.String(), `"d"`)
	}
	delete(input["keys"][0], "d")
	input["keys"][1]["n"] = input["keys"][0]["n"]
	input["keys"][1]["e"] = input["keys"][0]["e"]
	duplicateKeys, err := json.Marshal(input)
	require.NoError(t, err)
	_, err = NewUserJWKSHandler(duplicateKeys)
	require.Error(t, err)
	post, err := server.Client().Post(server.URL+"/.well-known/jwks.json", "application/json", strings.NewReader("{}"))
	require.NoError(t, err)
	defer post.Body.Close()
	require.Equal(t, http.StatusMethodNotAllowed, post.StatusCode)
	missing, err := server.Client().Get(server.URL + "/other")
	require.NoError(t, err)
	defer missing.Body.Close()
	require.Equal(t, http.StatusNotFound, missing.StatusCode)
}
