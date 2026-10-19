package nativeapp

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNativeCodeContractsRejectOperatorChosenCatalogue(t *testing.T) {
	cfg := nativeConfig{descriptor: descriptorFixture(t)}
	require.ErrorIs(t, checkCodeContracts(cfg), errUnavailable,
		"well-formed descriptor hashes cannot replace the linked native catalogue")
}

func TestNativeCodeContractsBindBothCompleteLinkedPolicies(t *testing.T) {
	cfg := nativeConfig{descriptor: descriptorFixture(t)}
	factories, routes := nativeCodeContracts()
	cfg.descriptor.Binding.SourceFactoryManifestHash = factories
	cfg.descriptor.Binding.FreshRouteManifestHash = routes
	require.NoError(t, checkCodeContracts(cfg))
	cfg.descriptor.Binding.FreshRouteManifestHash[0] ^= 1
	require.ErrorIs(t, checkCodeContracts(cfg), errUnavailable)
	cfg.descriptor.Binding.FreshRouteManifestHash = routes
	cfg.descriptor.Binding.SourceFactoryManifestHash[0] ^= 1
	require.ErrorIs(t, checkCodeContracts(cfg), errUnavailable)
}
