package nativeapp

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	w "voice/backend/pkg/mmadmissionwire"
)

func descriptorFixture(t *testing.T) w.DescriptorV1 {
	b, s, _ := gateFixture(t)
	d := w.DescriptorV1{ProtocolVersion: 1, Purpose: w.NativePurpose, AuthorityScope: w.LocalOwned, ScopeID: w.ID(uuid.New()), ControllerID: w.ID(uuid.New()), Binding: b, DatabaseLineageID: s.DatabaseLineageID, ReplicaID: w.ID(uuid.New()), ProcessLaunchID: w.ID(uuid.New()), AdmittedImageDigest: "sha256:" + w.Hash{1}.String(), AdmittedExecutableHash: w.Hash{2}, AdmittedArgv: []string{"/service-native"}, ConfigContractHash: w.Hash{3}, CredentialRefsManifestHash: w.Hash{4}, UpstreamContractManifestHash: w.Hash{5}, BrokerContractManifestHash: w.Hash{6}, ReadbackProtocol: w.Protocol}
	require.NoError(t, d.Validate())
	return d
}

func TestFixedNativeRequiredInputMatrixAndDefensiveConfig(t *testing.T) {
	d := descriptorFixture(t)
	raw, err := json.Marshal(d)
	require.NoError(t, err)
	for bits := 0; bits < 8; bits++ {
		t.Run(fmt.Sprintf("D%d_I%d_P%d", bits&1, (bits>>1)&1, (bits>>2)&1), func(t *testing.T) {
			env := map[string]string{}
			if bits&1 != 0 {
				env["DATABASE_URL"] = "postgres://mm:fixture@private-db:5432/matchmaking?sslmode=verify-full"
			}
			if bits&4 != 0 {
				for _, k := range requiredProfileKeys {
					env[k] = "fixture-value"
				}
			}
			body := append([]byte(nil), raw...)
			cfg, err := resolveConfig(func(k string) string { return env[k] }, func() ([]byte, error) {
				if bits&2 == 0 {
					return nil, errUnavailable
				}
				return body, nil
			})
			if bits != 7 {
				require.ErrorIs(t, err, errUnavailable)
				return
			}
			require.NoError(t, err, "all inputs only proceed to actual checks, never imply Ready")
			body[0] = 'x'
			env["DATABASE_URL"] = "changed"
			require.Equal(t, raw, cfg.rawDescriptor)
			require.NotEqual(t, "changed", cfg.values["DATABASE_URL"])
		})
	}
}

func TestFixedNativeMissingEachProfileOrMalformedDescriptorRefuses(t *testing.T) {
	d := descriptorFixture(t)
	raw, err := json.Marshal(d)
	require.NoError(t, err)
	for _, missing := range requiredProfileKeys {
		t.Run(missing, func(t *testing.T) {
			_, err := resolveConfig(func(k string) string {
				if k == missing {
					return ""
				}
				return "fixture"
			}, func() ([]byte, error) { return raw, nil })
			require.ErrorIs(t, err, errUnavailable)
		})
	}
	for _, body := range [][]byte{nil, []byte(`{}`), append(append([]byte(nil), raw...), []byte(` {}`)...)} {
		_, err := resolveConfig(func(string) string { return "fixture" }, func() ([]byte, error) { return body, nil })
		require.ErrorIs(t, err, errUnavailable)
	}
}
