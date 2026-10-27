package catalogasset

import "github.com/google/uuid"

// Selection binds a watermark to both services' actor incarnations. Digests
// across the complete retained keyring discover one aggregate after rotation.
func (k *ReplayKeys) Selection(actor ActorPair, fileAccount, fileProfile, chatAccount, chatProfile uuid.UUID, provider, providerID string) ([]ReplayDigest, error) {
	if actor.AccountID == uuid.Nil || actor.ProfileID == uuid.Nil || fileAccount == uuid.Nil || fileProfile == uuid.Nil || chatAccount == uuid.Nil || chatProfile == uuid.Nil || provider == "" || providerID == "" {
		return nil, ErrActorRequest
	}
	return k.digests("selection-head", actor.AccountID[:], actor.ProfileID[:], fileAccount[:], fileProfile[:], chatAccount[:], chatProfile[:], []byte(provider), []byte(providerID))
}
