package terminalrouting

import (
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCoverageBindsCompleteOriginalSchedule(t *testing.T) {
	c := Coverage{Schedule: fixtureSchedule(), Revision: 17, Complete: true, Profiles: []uuid.UUID{
		uuid.MustParse("00000000-0000-7000-8000-000000000001"),
		uuid.MustParse("00000000-0000-7000-8000-000000000002"),
	}}
	digest, err := c.Digest()
	require.NoError(t, err)
	require.Equal(t, "9df245471267980e6786e5008cd1a8660ca65c800d4a7a9bb5d8a4b70eb7b991", hex.EncodeToString(digest[:]))
	c.SHA256 = digest
	require.NoError(t, c.Validate())
	c.Complete = false
	require.ErrorIs(t, c.Validate(), ErrCoverageIncomplete)
	c.Complete = true
	c.Schedule.Revision = 42 // restore decision is not the immutable schedule revision
	require.ErrorIs(t, c.Validate(), ErrAuthorityConflict)
	c.Schedule.Revision = 41
	c.Profiles = append(c.Profiles, c.Profiles[0])
	require.ErrorIs(t, c.Validate(), ErrAuthorityInvalid)
}
