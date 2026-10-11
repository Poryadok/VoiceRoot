package registry

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func validSessionInputForRevision(revision int64) CreateSessionInput {
	return CreateSessionInput{
		OperationID: uuid.MustParse("00000000-0000-4000-8000-000000000003"),
		Kind: "match", ExternalKey: "match-1", DisplayName: "Match",
		RosterRevision: revision, RosterComplete: true, Members: []uuid.UUID{},
	}
}

func TestValidateCreateRejectsUnsafeRosterRevisions(t *testing.T) {
	for _, revision := range []int64{9007199254740992, 9007199254740993} {
		require.Error(t, ValidateCreateSessionInput(validSessionInputForRevision(revision)))
	}
	require.NoError(t, ValidateCreateSessionInput(validSessionInputForRevision(9007199254740991)))
}

func TestValidateCreateEnforcesUTF8AndDisplayNameScalarLimits(t *testing.T) {
	input := validSessionInputForRevision(1)
	input.DisplayName = strings.Repeat("ø", 128)
	require.NoError(t, ValidateCreateSessionInput(input))
	input.DisplayName += "ø"
	require.Error(t, ValidateCreateSessionInput(input))
	input = validSessionInputForRevision(1)
	input.ExternalKey = string([]byte{0xff})
	require.Error(t, ValidateCreateSessionInput(input))
	input = validSessionInputForRevision(1)
	input.DisplayName = string([]byte{0xff})
	require.Error(t, ValidateCreateSessionInput(input))
}

func TestSessionRequestHashRejectsUnsafeRosterRevisionBeforeCanonicalization(t *testing.T) {
	principal := SessionPrincipal{
		ApplicationID: uuid.MustParse("00000000-0000-4000-8000-000000000001"),
		EnvironmentID: uuid.MustParse("00000000-0000-4000-8000-000000000002"),
	}
	for _, revision := range []int64{9007199254740992, 9007199254740993} {
		_, err := sessionRequestHash(principal, validSessionInputForRevision(revision))
		require.Error(t, err)
	}
}
