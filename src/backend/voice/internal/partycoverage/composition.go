package partycoverage

import (
	"encoding/json"
	"sort"

	"voice/backend/voice/internal/retirementproof"
)

const CompositionReportDomain = "voice.party.sources.composition.v1"

// ContractArtifact retains the exact bytes and hash recipe of an installed
// contract. Protocol identifies its owner; HashDomain is the domain passed to
// retirementproof.Hash, rather than an inferred interpretation of Hash.
type ContractArtifact struct {
	Protocol       string
	HashDomain     string
	CanonicalBytes []byte
	Hash           [32]byte
}

// CompositionReport is diagnostic output from the non-serving executable's
// actual factory/schema checks. It confers no coverage or retirement authority.
// SourceBindings lists only linked, verified capabilities; a partial list must
// never be interpreted as the closed source catalogue required for activation.
// The executor separately binds these bytes to its actual image, deployment,
// credentials and listener inventory. ExecutableSHA256 is not an image digest.
type CompositionReport struct {
	Protocol                           uint32
	DatabaseIdentity, ExecutableSHA256 [32]byte
	SourceBindings                     []retirementproof.SourceBinding
	SourceContracts                    []ContractArtifact
	SchemaContracts                    []ContractArtifact
	FactoryContracts                   []ContractArtifact
}

// CanonicalCompositionReport validates recoverable contract bytes and returns a
// deterministic diagnostic document. It neither invokes factories nor proves
// that any source is drained, any ingress is closed, or credentials are retired.
func CanonicalCompositionReport(report CompositionReport) ([]byte, [32]byte, error) {
	if report.Protocol != 1 || report.DatabaseIdentity == ([32]byte{}) || report.ExecutableSHA256 == ([32]byte{}) {
		return nil, [32]byte{}, ErrUnavailable
	}
	report.SourceBindings = append([]retirementproof.SourceBinding(nil), report.SourceBindings...)
	sort.Slice(report.SourceBindings, func(i, j int) bool { return report.SourceBindings[i].SourceKind < report.SourceBindings[j].SourceKind })
	for i, source := range report.SourceBindings {
		if source.SourceKind == "" || source.OwnerProtocol == "" || source.ContractHash == ([32]byte{}) || (i > 0 && report.SourceBindings[i-1].SourceKind == source.SourceKind) {
			return nil, [32]byte{}, ErrUnavailable
		}
	}
	groups := []*[]ContractArtifact{&report.SourceContracts, &report.SchemaContracts, &report.FactoryContracts}
	for _, group := range groups {
		if len(*group) == 0 {
			return nil, [32]byte{}, ErrUnavailable
		}
		*group = append([]ContractArtifact(nil), (*group)...)
		sort.Slice(*group, func(i, j int) bool { return (*group)[i].Protocol < (*group)[j].Protocol })
		for i, artifact := range *group {
			if artifact.Protocol == "" || artifact.HashDomain == "" || len(artifact.CanonicalBytes) == 0 || artifact.Hash != retirementproof.Hash(artifact.HashDomain, artifact.CanonicalBytes) || (i > 0 && (*group)[i-1].Protocol == artifact.Protocol) {
				return nil, [32]byte{}, ErrUnavailable
			}
		}
	}
	if len(report.SourceBindings) == 0 {
		return nil, [32]byte{}, ErrUnavailable
	}
	for _, source := range report.SourceBindings {
		found := false
		for _, artifact := range report.SourceContracts {
			if artifact.Protocol == source.OwnerProtocol && artifact.Hash == source.ContractHash {
				found = true
			}
		}
		if !found {
			return nil, [32]byte{}, ErrUnavailable
		}
	}
	data, err := json.Marshal(report)
	if err != nil {
		return nil, [32]byte{}, ErrUnavailable
	}
	return data, retirementproof.Hash(CompositionReportDomain, data), nil
}
