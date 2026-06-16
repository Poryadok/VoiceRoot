package prekey_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/messaging/internal/e2e/prekey"
)

func TestVerifyCurve25519Signature_RejectsGarbage(t *testing.T) {
	wire, err := prekey.ParseWire(loadLibsignalGoldenPreKeyBundleB64(t))
	require.NoError(t, err)
	sig := append([]byte(nil), wire.SignedPreKeySignature...)
	sig[0] ^= 0xff
	require.Error(t, prekey.VerifySignedPreKeySignature(&prekey.Wire{
		RegistrationID:        wire.RegistrationID,
		DeviceID:              wire.DeviceID,
		SignedPreKeyID:        wire.SignedPreKeyID,
		SignedPreKeyPublic:    wire.SignedPreKeyPublic,
		SignedPreKeySignature: sig,
		IdentityKey:           wire.IdentityKey,
	}))
}
