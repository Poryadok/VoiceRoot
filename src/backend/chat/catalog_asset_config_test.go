package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogActorKeyDocumentAndRuntimePrerequisites(t *testing.T) {
	secret := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	path := filepath.Join(t.TempDir(), "actor-keys.json")
	for _, tc := range []struct {
		document string
		valid    bool
	}{
		{`{"active_key_id":"one","keys":[{"id":"one","secret_base64":"` + secret + `"}]}`, true},
		{`{"active_key_id":"other","keys":[{"id":"one","secret_base64":"` + secret + `"}]}`, false},
		{`{"active_key_id":"one","keys":[{"id":"one","secret_base64":"` + secret + `"},{"id":"one","secret_base64":"` + secret + `"}]}`, false},
		{`{"active_key_id":"one","keys":[{"id":"one","secret_base64":"short"}]}`, false},
		{`{"active_key_id":"one","keys":[],"unknown":true}`, false},
		{`{} {}`, false},
	} {
		require.NoError(t, os.WriteFile(path, []byte(tc.document), 0600))
		keys, err := readCatalogActorKeys(path)
		if tc.valid {
			require.NoError(t, err)
			require.NotNil(t, keys)
		} else {
			require.Error(t, err)
		}
	}
	_, err := readCatalogActorKeys(filepath.Dir(path))
	require.Error(t, err)
	runtime, err := startCatalogAssets(context.Background(), catalogAssetConfig{}, false, catalogRuntimeDeps{})
	require.NoError(t, err)
	require.Nil(t, runtime)
	runtime, err = startCatalogAssets(context.Background(), catalogAssetConfig{}, true, catalogRuntimeDeps{})
	require.Error(t, err)
	require.Nil(t, runtime)
}

func TestCatalogAssetConfigRequiresCompleteProtectedTransport(t *testing.T) {
	empty := func(string) (string, bool) { return "", false }
	_, enabled, err := loadCatalogAssetConfig(empty)
	require.NoError(t, err)
	require.False(t, enabled)
	values := map[string]string{}
	for _, suffix := range catalogRequiredSuffixes() {
		values["CHAT_CATALOG_ASSET_"+suffix] = "fixture"
	}
	values["CHAT_CATALOG_ASSET_GRPC_LISTEN"] = "127.0.0.1:0"
	for _, issuer := range []string{"GATEWAY", "MESSAGING", "FILE"} {
		values["CHAT_CATALOG_ASSET_"+issuer+"_JWKS_URL"] = "https://issuer.test/jwks"
	}
	lookup := func(name string) (string, bool) { value, ok := values[name]; return value, ok }
	_, enabled, err = loadCatalogAssetConfig(lookup)
	require.NoError(t, err)
	require.True(t, enabled)
	for _, suffix := range catalogRequiredSuffixes() {
		key := "CHAT_CATALOG_ASSET_" + suffix
		saved := values[key]
		delete(values, key)
		_, enabled, err = loadCatalogAssetConfig(lookup)
		require.Error(t, err, suffix)
		require.True(t, enabled)
		values[key] = saved
	}
}
