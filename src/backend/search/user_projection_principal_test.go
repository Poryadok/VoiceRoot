package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadSignedUserProjectionClientFromEnv_RejectsPartialConfiguration(t *testing.T) {
	t.Setenv("SEARCH_USER_PROJECTION_GRPC_ADDR", "user:9093")
	_, _, err := loadSignedUserProjectionClientFromEnv()
	require.ErrorContains(t, err, "address and principal signing keys are required")
}

func TestLoadSearchProjectionJWKS_RejectsPartialConfiguration(t *testing.T) {
	t.Setenv("SEARCH_PRINCIPAL_JWKS_LISTEN", ":8443")
	_, err := loadSearchProjectionJWKS()
	require.ErrorContains(t, err, "JWKS listen, TLS and signing keys are required")
}
