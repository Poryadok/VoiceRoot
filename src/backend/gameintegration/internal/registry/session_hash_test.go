package registry

import (
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSessionRequestHashUsesScopedRFC8785CanonicalJSON(t *testing.T) {
	appID := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	envID := uuid.MustParse("00000000-0000-4000-8000-000000000002")
	operationID := uuid.MustParse("00000000-0000-4000-8000-000000000003")
	in := CreateSessionInput{OperationID: operationID, Kind: "party", ExternalKey: "raid<&", DisplayName: "Run", RosterRevision: 2, RosterComplete: true, Members: []uuid.UUID{}}
	hash, err := sessionRequestHash(SessionPrincipal{ApplicationID: appID, EnvironmentID: envID}, in)
	require.NoError(t, err)
	canonical := `{"api_version":"v1","request":{"display_name":"Run","external_key":"raid<&","kind":"party","members":[],"operation_id":"00000000-0000-4000-8000-000000000003","roster_complete":true,"roster_revision":2},"scope":{"application_id":"00000000-0000-4000-8000-000000000001","environment_id":"00000000-0000-4000-8000-000000000002"}}`
	want := sha256.Sum256([]byte(canonical))
	require.Equal(t, want[:], hash)
}
