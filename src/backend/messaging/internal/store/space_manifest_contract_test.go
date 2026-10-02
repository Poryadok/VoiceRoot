package store

import (
	"encoding/hex"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSpaceManifestRootMatchesCanonicalChatOwnerGolden(t *testing.T) {
	// Golden from Chat's documented raw UUID/domain/count encoding, generated
	// independently with Python hashlib, not the Messaging test fixture helper.
	expected, err := hex.DecodeString("09e184464f01e9fd66422cd60767fd8751f7420168101052fa1a3a23b37bc6d1")
	require.NoError(t, err)
	actual := messagingChatManifestSHA(uuid.MustParse("20000000-0000-4000-8000-000000000301"), uuid.MustParse("20000000-0000-4000-8000-000000000302"), 7, []uuid.UUID{uuid.MustParse("20000000-0000-4000-8000-000000000303")})
	require.Equal(t, expected, actual, "Messaging must import Chat's owner manifest, never an incompatible local root")
}
