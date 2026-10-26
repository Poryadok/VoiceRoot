package catalogassets

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

type selectorAlias struct {
	keyID  string
	digest [32]byte
}

// selectorAliases never stores the provider tuple or raw actor identifiers in an
// empty cache slot. Length framing and a distinct purpose bind every component.
func (k *ActorKeyring) selectorAliases(actor Actor, capture ActorCapture, selection Selection) ([]selectorAlias, error) {
	if k == nil || !canonicalID(actor.AccountID.String()) || !canonicalID(actor.ProfileID.String()) || !validCapture(capture) || !validSelection(selection.Provider, selection.ProviderID) {
		return nil, errors.New("invalid catalog selector key lookup")
	}
	out := make([]selectorAlias, 0, len(k.ids))
	for _, keyID := range k.ids {
		mac := hmac.New(sha256.New, k.keys[keyID])
		_, _ = mac.Write([]byte("voice.chat.catalog.selector-cache.v2\x00"))
		for _, part := range [][]byte{[]byte(keyID), actor.AccountID[:], actor.ProfileID[:], capture.AccountGeneration[:], capture.ProfileGeneration[:], []byte(selection.Provider), []byte(selection.ProviderID)} {
			var size [8]byte
			binary.BigEndian.PutUint64(size[:], uint64(len(part)))
			_, _ = mac.Write(size[:])
			_, _ = mac.Write(part)
		}
		var digest [32]byte
		copy(digest[:], mac.Sum(nil))
		out = append(out, selectorAlias{keyID: keyID, digest: digest})
	}
	return out, nil
}
