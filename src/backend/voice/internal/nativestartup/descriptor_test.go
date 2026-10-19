package nativestartup

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/retirementproof"
)

func completeDescriptor() Descriptor {
	digest := Digest{1}
	return Descriptor{ProtocolVersion: 1, Purpose: "NATIVE_SERVING", InstallationID: uuid.New(), LaunchManifestID: uuid.New(), LaunchManifestSHA256: digest,
		EnvironmentID: uuid.New(), AuthorityIncarnation: uuid.New(), CoverageEpoch: 1, VoiceDatabaseIdentity: digest, SourceCatalogSHA256: digest,
		SchemaContractSHA256: digest, WriterManifestSHA256: digest, ServingCredentialCohortID: uuid.New(), ExpectedDatabaseLoginRoleOID: 1234,
		ExpectedDatabaseLoginRoleName: "voice_native", AdmittedNativeExecutableSHA256: digest, AdmittedEntrypoint: Entrypoint,
		ProtectedDatabaseCredentialRef: "voice-native-db-v1", NativeFactoryContracts: []FactoryContract{{Protocol: "native-test-v1", HashDomain: "test", CanonicalBytes: []byte("contract"), SHA256: Digest(retirementproof.Hash("test", []byte("contract")))}}}
}

func TestDescriptorRequiresCompleteImmutableNativeIdentity(t *testing.T) {
	d := completeDescriptor()
	encoded, err := json.Marshal(d)
	require.NoError(t, err)
	got, err := ParseDescriptor(bytes.NewReader(encoded))
	require.NoError(t, err)
	require.Equal(t, d, got)
	require.Equal(t, d.EnvironmentID, got.RuntimeBinding().EnvironmentID)
	require.Equal(t, [32]byte(d.WriterManifestSHA256), got.RuntimeBinding().WriterManifestHash)
	for name, change := range map[string]func(*Descriptor){
		"purpose":      func(d *Descriptor) { d.Purpose = "LEGACY" },
		"epoch":        func(d *Descriptor) { d.CoverageEpoch = 0 },
		"cohort":       func(d *Descriptor) { d.ServingCredentialCohortID = uuid.Nil },
		"manifest":     func(d *Descriptor) { d.LaunchManifestSHA256 = Digest{} },
		"database":     func(d *Descriptor) { d.VoiceDatabaseIdentity = Digest{} },
		"role":         func(d *Descriptor) { d.ExpectedDatabaseLoginRoleOID = 0 },
		"entrypoint":   func(d *Descriptor) { d.AdmittedEntrypoint = "/service" },
		"factories":    func(d *Descriptor) { d.NativeFactoryContracts = nil },
		"factory hash": func(d *Descriptor) { d.NativeFactoryContracts[0].SHA256 = Digest{2} },
		"duplicate factory": func(d *Descriptor) {
			d.NativeFactoryContracts = append(d.NativeFactoryContracts, d.NativeFactoryContracts[0])
		},
	} {
		t.Run(name, func(t *testing.T) {
			bad := completeDescriptor()
			change(&bad)
			body, marshalErr := json.Marshal(bad)
			require.NoError(t, marshalErr)
			_, parseErr := ParseDescriptor(bytes.NewReader(body))
			require.ErrorIs(t, parseErr, ErrInvalidInstallation)
		})
	}
	_, err = ParseDescriptor(bytes.NewBufferString(string(encoded) + `{}`))
	require.ErrorIs(t, err, ErrInvalidInstallation)
	_, err = ParseDescriptor(bytes.NewBufferString(`{"protocol_version":1,"ready":true}`))
	require.ErrorIs(t, err, ErrInvalidInstallation)
	_, err = ParseDescriptor(strings.NewReader(string(encoded) + strings.Repeat(" ", 1<<20)))
	require.ErrorIs(t, err, ErrInvalidInstallation, "the size boundary must not truncate trailing input into a valid descriptor")
}
