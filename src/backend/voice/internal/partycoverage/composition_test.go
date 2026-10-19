package partycoverage

import (
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/retirementproof"
)

func TestCompositionReportRetainsExactContractsWithoutClaimingCompleteness(t *testing.T) {
	artifact := func(protocol string) ContractArtifact {
		b := []byte("installed contract for " + protocol)
		return ContractArtifact{Protocol: protocol, HashDomain: protocol + ".contract", CanonicalBytes: b, Hash: retirementproof.Hash(protocol+".contract", b)}
	}
	a, b := artifact("native.ordinary"), artifact("native.space")
	base := CompositionReport{Protocol: 1, DatabaseIdentity: [32]byte{1}, ExecutableSHA256: [32]byte{2},
		SourceBindings:  []retirementproof.SourceBinding{{SourceKind: "NATIVE_SPACE", OwnerProtocol: b.Protocol, ContractHash: b.Hash}, {SourceKind: "NATIVE_ORDINARY", OwnerProtocol: a.Protocol, ContractHash: a.Hash}},
		SourceContracts: []ContractArtifact{b, a}, SchemaContracts: []ContractArtifact{artifact("schema")}, FactoryContracts: []ContractArtifact{artifact("factory")}}
	data, hash, err := CanonicalCompositionReport(base)
	require.NoError(t, err) // A partial diagnostic report is not an activation proof.
	require.Equal(t, retirementproof.Hash(CompositionReportDomain, data), hash)
	require.Equal(t, "NATIVE_SPACE", base.SourceBindings[0].SourceKind) // No caller mutation.
	base.SourceBindings[0], base.SourceBindings[1] = base.SourceBindings[1], base.SourceBindings[0]
	base.SourceContracts[0], base.SourceContracts[1] = base.SourceContracts[1], base.SourceContracts[0]
	other, otherHash, err := CanonicalCompositionReport(base)
	require.NoError(t, err)
	require.Equal(t, data, other)
	require.Equal(t, hash, otherHash)
	t.Run("changed executable", func(t *testing.T) {
		r := base
		r.ExecutableSHA256[0]++
		_, changed, err := CanonicalCompositionReport(r)
		require.NoError(t, err)
		require.NotEqual(t, hash, changed)
	})
	t.Run("altered contract bytes", func(t *testing.T) {
		r := base
		r.FactoryContracts = append([]ContractArtifact(nil), base.FactoryContracts...)
		r.FactoryContracts[0].CanonicalBytes = []byte("substituted")
		_, _, err := CanonicalCompositionReport(r)
		require.ErrorIs(t, err, ErrUnavailable)
	})
	t.Run("missing matching source artifact", func(t *testing.T) {
		r := base
		r.SourceContracts = r.SourceContracts[:1]
		_, _, err := CanonicalCompositionReport(r)
		require.ErrorIs(t, err, ErrUnavailable)
	})
	t.Run("duplicate source", func(t *testing.T) {
		r := base
		r.SourceBindings = append([]retirementproof.SourceBinding{base.SourceBindings[0]}, base.SourceBindings...)
		_, _, err := CanonicalCompositionReport(r)
		require.ErrorIs(t, err, ErrUnavailable)
	})
}
