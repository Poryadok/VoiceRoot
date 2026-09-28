package gameprotocol

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGameMessageTombstoneSignsAndVerifiesExactTerminalEnvelope(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	key := GameMessageTombstoneKey{KeyID: uuid.New(), PublicKey: publicKey, PrivateKey: privateKey, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	input := GameMessageTombstone{
		ApplicationID: uuid.New(), EnvironmentID: uuid.New(), ChatID: uuid.New(), MessageID: uuid.New(),
		Revision: 4, PreviousRevisionHash: strings.Repeat("a", 64), ActionID: uuid.New(),
		ReasonClass: "moderation", IssuedAt: now,
	}
	compact, err := SignGameMessageTombstone(input, key, now)
	require.NoError(t, err)
	verified, err := VerifyGameMessageTombstone(compact, map[string]GameMessageTombstoneKey{key.KeyID.String(): key}, now)
	require.NoError(t, err)
	require.Equal(t, input, verified)

	parts := strings.Split(compact, ".")
	require.Len(t, parts, 3)
	parts[1] = strings.TrimRight(parts[1], "A") + "B"
	_, err = VerifyGameMessageTombstone(strings.Join(parts, "."), map[string]GameMessageTombstoneKey{key.KeyID.String(): key}, now)
	require.Error(t, err)
}

func TestGameMessageTombstoneKeyValidityAndRevocation(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	input := GameMessageTombstone{
		ApplicationID: uuid.New(), EnvironmentID: uuid.New(), ChatID: uuid.New(), MessageID: uuid.New(),
		Revision: 2, PreviousRevisionHash: strings.Repeat("b", 64), ActionID: uuid.New(),
		ReasonClass: "system_retention", IssuedAt: now,
	}
	key := GameMessageTombstoneKey{KeyID: uuid.New(), PublicKey: publicKey, PrivateKey: privateKey, NotBefore: now, NotAfter: now.Add(time.Minute)}
	compact, err := SignGameMessageTombstone(input, key, now)
	require.NoError(t, err)
	key.PrivateKey = nil
	revoked := now.Add(time.Second)
	key.RevokedAt = &revoked
	_, err = VerifyGameMessageTombstone(compact, map[string]GameMessageTombstoneKey{key.KeyID.String(): key}, now.Add(2*time.Second))
	require.NoError(t, err, "signatures issued before revocation remain historically verifiable")
	input.ActionID = uuid.New()
	input.IssuedAt = now.Add(2 * time.Second)
	_, err = SignGameMessageTombstone(input, key, input.IssuedAt)
	require.Error(t, err, "revoked key cannot sign new tombstones")
	_, err = SignGameMessageTombstone(input, GameMessageTombstoneKey{KeyID: uuid.New(), PublicKey: publicKey, PrivateKey: privateKey, NotBefore: now, NotAfter: now.Add(time.Minute)}, now.Add(time.Minute))
	require.Error(t, err, "not_after is exclusive")
}
