package prekey

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
)

// testBundleB64 builds a wire bundle with a valid Ed25519 signed-pre-key signature for tests.
func TestBundleB64(payload map[string]any) string {
	identityB64, _ := payload["identity_key"].(string)
	signedPubB64, _ := payload["signed_pre_key_public"].(string)
	identityKey, err := base64.StdEncoding.DecodeString(identityB64)
	if err != nil || len(identityKey) != 33 {
		panic("test bundle: invalid identity_key")
	}
	signedPub, err := base64.StdEncoding.DecodeString(signedPubB64)
	if err != nil || len(signedPub) != 33 {
		panic("test bundle: invalid signed_pre_key_public")
	}
	privSeed, ok := payload["_test_identity_seed"].([]byte)
	if !ok || len(privSeed) != ed25519.SeedSize {
		panic("test bundle: missing _test_identity_seed")
	}
	delete(payload, "_test_identity_seed")
	priv := ed25519.NewKeyFromSeed(privSeed)
	sig := ed25519.Sign(priv, signedPub)
	payload["signed_pre_key_signature"] = base64.StdEncoding.EncodeToString(sig)
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// TestIdentitySeed returns a deterministic ed25519 seed for test bundles.
func TestIdentitySeed(tag byte) []byte {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = tag ^ byte(i)
	}
	return seed
}

// TestIdentityKeyWire returns 0x05 || ed25519 public key for [seed].
func TestIdentityKeyWire(seed []byte) []byte {
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	out := make([]byte, 33)
	out[0] = 0x05
	copy(out[1:], pub)
	return out
}
