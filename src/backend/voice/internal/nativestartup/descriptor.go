// Package nativestartup defines the protected installer input for the fixed
// native executable. Parsing this input does not establish installer custody,
// qualify factories, issue coverage, or authorize serving.
package nativestartup

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/google/uuid"
	"voice/backend/voice/internal/partycoverage"
	"voice/backend/voice/internal/retirementproof"
)

const DescriptorPath = "/run/voice/installation/native-serving-v1.json"
const Entrypoint = "/service-native"

var ErrInvalidInstallation = errors.New("native installation unavailable")

type Digest [32]byte

func (d Digest) MarshalText() ([]byte, error) { return []byte(hex.EncodeToString(d[:])), nil }
func (d *Digest) UnmarshalText(text []byte) error {
	if d == nil || len(text) != 64 || string(text) != strings.ToLower(string(text)) {
		return ErrInvalidInstallation
	}
	decoded, err := hex.DecodeString(string(text))
	if err != nil {
		return ErrInvalidInstallation
	}
	copy(d[:], decoded)
	return nil
}

type FactoryContract struct {
	Protocol       string `json:"protocol"`
	HashDomain     string `json:"hash_domain"`
	CanonicalBytes []byte `json:"canonical_bytes"`
	SHA256         Digest `json:"sha256"`
}

type Descriptor struct {
	ProtocolVersion                uint32            `json:"protocol_version"`
	Purpose                        string            `json:"purpose"`
	InstallationID                 uuid.UUID         `json:"installation_id"`
	LaunchManifestID               uuid.UUID         `json:"launch_manifest_id"`
	LaunchManifestSHA256           Digest            `json:"launch_manifest_sha256"`
	EnvironmentID                  uuid.UUID         `json:"environment_id"`
	AuthorityIncarnation           uuid.UUID         `json:"authority_incarnation"`
	CoverageEpoch                  uint64            `json:"coverage_epoch"`
	VoiceDatabaseIdentity          Digest            `json:"voice_database_identity"`
	SourceCatalogSHA256            Digest            `json:"source_catalog_sha256"`
	SchemaContractSHA256           Digest            `json:"schema_contract_sha256"`
	WriterManifestSHA256           Digest            `json:"writer_manifest_sha256"`
	ServingCredentialCohortID      uuid.UUID         `json:"serving_credential_cohort_id"`
	ExpectedDatabaseLoginRoleOID   uint32            `json:"expected_database_login_role_oid"`
	ExpectedDatabaseLoginRoleName  string            `json:"expected_database_login_role_name"`
	AdmittedNativeExecutableSHA256 Digest            `json:"admitted_native_executable_sha256"`
	AdmittedEntrypoint             string            `json:"admitted_entrypoint"`
	ProtectedDatabaseCredentialRef string            `json:"protected_database_credential_ref"`
	NativeFactoryContracts         []FactoryContract `json:"native_factory_contracts"`
}

func ParseDescriptor(input io.Reader) (Descriptor, error) {
	if input == nil {
		return Descriptor{}, ErrInvalidInstallation
	}
	body, err := io.ReadAll(io.LimitReader(input, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return Descriptor{}, ErrInvalidInstallation
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var d Descriptor
	if err := decoder.Decode(&d); err != nil {
		return Descriptor{}, ErrInvalidInstallation
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Descriptor{}, ErrInvalidInstallation
	}
	if d.ProtocolVersion != 1 || d.Purpose != "NATIVE_SERVING" || d.InstallationID == uuid.Nil || d.LaunchManifestID == uuid.Nil ||
		d.EnvironmentID == uuid.Nil || d.AuthorityIncarnation == uuid.Nil || d.CoverageEpoch == 0 || d.ServingCredentialCohortID == uuid.Nil ||
		d.ExpectedDatabaseLoginRoleOID == 0 || !nonempty(d.ExpectedDatabaseLoginRoleName) || d.AdmittedEntrypoint != Entrypoint ||
		!nonempty(d.ProtectedDatabaseCredentialRef) || len(d.NativeFactoryContracts) == 0 {
		return Descriptor{}, ErrInvalidInstallation
	}
	for _, digest := range []Digest{d.LaunchManifestSHA256, d.VoiceDatabaseIdentity, d.SourceCatalogSHA256, d.SchemaContractSHA256, d.WriterManifestSHA256, d.AdmittedNativeExecutableSHA256} {
		if digest == (Digest{}) {
			return Descriptor{}, ErrInvalidInstallation
		}
	}
	seen := make(map[string]bool)
	for _, contract := range d.NativeFactoryContracts {
		if !nonempty(contract.Protocol) || !nonempty(contract.HashDomain) || len(contract.CanonicalBytes) == 0 || seen[contract.Protocol] ||
			retirementproof.Hash(contract.HashDomain, contract.CanonicalBytes) != [32]byte(contract.SHA256) {
			return Descriptor{}, ErrInvalidInstallation
		}
		seen[contract.Protocol] = true
	}
	return d, nil
}

func nonempty(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}

func (d Descriptor) RuntimeBinding() partycoverage.RuntimeBinding {
	return partycoverage.RuntimeBinding{EnvironmentID: d.EnvironmentID, AuthorityIncarnation: d.AuthorityIncarnation, CoverageEpoch: d.CoverageEpoch,
		DatabaseIdentity: [32]byte(d.VoiceDatabaseIdentity), SourceCatalogHash: [32]byte(d.SourceCatalogSHA256), SchemaContractHash: [32]byte(d.SchemaContractSHA256), WriterManifestHash: [32]byte(d.WriterManifestSHA256)}
}
