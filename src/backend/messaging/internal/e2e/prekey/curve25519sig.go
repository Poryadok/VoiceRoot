package prekey

import (
	"crypto/ed25519"
	"math/big"
)

// verifyCurve25519Signature checks a libsignal signed-pre-key signature.
// See libsignal Curve.verifySignature / curve_sigs.c curve25519_verify.
func verifyCurve25519Signature(publicKey, message, signature []byte) bool {
	if len(publicKey) != 32 || len(signature) != signatureLen {
		return false
	}
	pub := append([]byte(nil), publicKey...)
	sig := append([]byte(nil), signature...)

	pub[31] &= 0x7F

	edPub, ok := montgomeryPublicKeyToEd25519(pub)
	if !ok {
		return false
	}
	edPub[31] |= sig[63] & 0x80
	sig[63] &= 0x7F

	return ed25519.Verify(ed25519.PublicKey(edPub[:]), message, sig)
}

func montgomeryPublicKeyToEd25519(montPub []byte) ([32]byte, bool) {
	var out [32]byte
	p := curve25519Prime()
	montX := decodeLE255(montPub)

	one := big.NewInt(1)
	num := new(big.Int).Sub(montX, one)
	num.Mod(num, p)
	den := new(big.Int).Add(montX, one)
	den.Mod(den, p)
	if den.Sign() == 0 {
		return out, false
	}
	denInv := new(big.Int).ModInverse(den, p)
	if denInv == nil {
		return out, false
	}
	edY := new(big.Int).Mul(num, denInv)
	edY.Mod(edY, p)
	encodeLE255(edY, out[:])
	return out, true
}

func curve25519Prime() *big.Int {
	// 2^255 - 19
	p := new(big.Int).Lsh(big.NewInt(1), 255)
	p.Sub(p, big.NewInt(19))
	return p
}

func decodeLE255(b []byte) *big.Int {
	reversed := make([]byte, len(b))
	for i := range b {
		reversed[i] = b[len(b)-1-i]
	}
	return new(big.Int).SetBytes(reversed)
}

func encodeLE255(v *big.Int, out []byte) {
	raw := v.Bytes()
	for i := 0; i < len(raw) && i < len(out); i++ {
		out[i] = raw[len(raw)-1-i]
	}
}
