package grpcsvc

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidateUpdateScopes_canonicalizesEqualSet(t *testing.T) {
	got, err := validateUpdateScopes(
		`["DM_SEND","TEXT_CHAT_SEND_MESSAGES"]`,
		`["TEXT_CHAT_SEND_MESSAGES","DM_SEND"]`,
	)

	require.NoError(t, err)
	require.Equal(t, `["DM_SEND","TEXT_CHAT_SEND_MESSAGES"]`, got)
}

func TestValidateUpdateScopes_allowsRemovingScopes(t *testing.T) {
	got, err := validateUpdateScopes(
		`["DM_SEND","TEXT_CHAT_SEND_MESSAGES"]`,
		`["TEXT_CHAT_SEND_MESSAGES"]`,
	)

	require.NoError(t, err)
	require.Equal(t, `["TEXT_CHAT_SEND_MESSAGES"]`, got)
}

func TestValidateUpdateScopes_rejectsEscalation(t *testing.T) {
	_, err := validateUpdateScopes(
		`["TEXT_CHAT_SEND_MESSAGES"]`,
		`["TEXT_CHAT_SEND_MESSAGES","DM_SEND"]`,
	)

	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestValidateUpdateScopes_rejectsInvalidScopeSets(t *testing.T) {
	for _, requested := range []string{
		`not-json`,
		`{"scope":"DM_SEND"}`,
		`["UNKNOWN_SCOPE"]`,
		`["DM_SEND","DM_SEND"]`,
		`[" DM_SEND"]`,
	} {
		_, err := validateUpdateScopes(`["DM_SEND"]`, requested)
		require.Equal(t, codes.InvalidArgument, status.Code(err), requested)
	}
}
