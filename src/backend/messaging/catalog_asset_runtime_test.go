package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/grpcsvc"
)

func TestCatalogRuntimeCannotActivateWithoutReferenceAcquisition(t *testing.T) {
	f := newCatalogRuntimeFixture(t)
	service := &grpcsvc.MessagingGRPC{}
	require.ErrorContains(t, attachCatalogAssetRuntime(service, f.runtime), "reference acquisition")
	require.Nil(t, service.CatalogAssets)
	require.NoError(t, attachCatalogAssetRuntime(service, nil))
	service.AttachmentReferences = &grpcsvc.AttachmentReferenceCoordinator{Issuer: f.parent.Issuer}
	require.ErrorContains(t, attachCatalogAssetRuntime(service, f.runtime), "reference acquisition")
	service.AttachmentReferences.Files = filev1.NewFileServiceClient(f.runtime.connections[0])
	service.AttachmentReferences.Issuer = nil
	require.ErrorContains(t, attachCatalogAssetRuntime(service, f.runtime), "reference acquisition")
	service.AttachmentReferences.Issuer = f.parent.Issuer
	require.NoError(t, attachCatalogAssetRuntime(service, f.runtime))
	require.Same(t, f.runtime.Coordinator, service.CatalogAssets)
}

var catalogRuntimeSuffixes = []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "TLS_CLIENT_CERT_FILE", "TLS_CLIENT_KEY_FILE"}

func catalogRuntimeEnv(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) { value, ok := values[key]; return value, ok }
}

func completeCatalogRuntimeEnv() map[string]string {
	values := map[string]string{}
	for _, owner := range []string{"FILE", "CHAT"} {
		for _, suffix := range catalogRuntimeSuffixes {
			values["MESSAGING_CATALOG_ASSET_"+owner+"_"+suffix] = "configured"
		}
	}
	return values
}

func TestCatalogRuntimeAbsentAndPartialConfiguration(t *testing.T) {
	runtime, err := newCatalogAssetRuntime(catalogRuntimeEnv(nil), nil)
	require.NoError(t, err)
	require.Nil(t, runtime)
	for _, owner := range []string{"FILE", "CHAT"} {
		for _, suffix := range catalogRuntimeSuffixes {
			key := "MESSAGING_CATALOG_ASSET_" + owner + "_" + suffix
			t.Run(key, func(t *testing.T) {
				for _, value := range []string{"configured", "", " "} {
					runtime, err := newCatalogAssetRuntime(catalogRuntimeEnv(map[string]string{key: value}), nil)
					require.Error(t, err)
					require.Nil(t, runtime)
				}
				values := completeCatalogRuntimeEnv()
				delete(values, key)
				runtime, err := newCatalogAssetRuntime(catalogRuntimeEnv(values), nil)
				require.Error(t, err)
				require.Nil(t, runtime)
			})
		}
	}
}

func TestCatalogRuntimeRequiresExistingMessagingSignerAndJWKS(t *testing.T) {
	for _, parent := range []*managedChatPurgeRuntime{nil, {}, {JWKS: &http.Server{}}} {
		runtime, err := newCatalogAssetRuntime(catalogRuntimeEnv(completeCatalogRuntimeEnv()), parent)
		require.ErrorContains(t, err, "signing keys and protected JWKS")
		require.Nil(t, runtime)
	}
}
