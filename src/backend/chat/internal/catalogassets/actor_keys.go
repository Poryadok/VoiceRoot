package catalogassets

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
	"sort"
)

type ActorKeyring struct {
	active string
	keys   map[string][]byte
	ids    []string
}
type actorAlias struct {
	keyID  string
	kind   int16
	digest [32]byte
}

func NewActorKeyring(active string, keys map[string][]byte) (*ActorKeyring, error) {
	invalid := errors.New("invalid catalog actor keyring")
	if !validKeyID(active) || len(keys) == 0 || len(keys[active]) != 32 {
		return nil, invalid
	}
	ring := &ActorKeyring{active: active, keys: make(map[string][]byte, len(keys))}
	for id, key := range keys {
		if !validKeyID(id) || len(key) != 32 {
			return nil, invalid
		}
		ring.keys[id] = append([]byte(nil), key...)
		ring.ids = append(ring.ids, id)
	}
	sort.Strings(ring.ids)
	return ring, nil
}
func (k *ActorKeyring) aliases(kind int16, id uuid.UUID) ([]actorAlias, error) {
	if k == nil || id == uuid.Nil || (kind != 1 && kind != 2) {
		return nil, errors.New("invalid catalog actor key lookup")
	}
	purpose := "account"
	if kind == 2 {
		purpose = "profile"
	}
	return k.digests(kind, purpose, id), nil
}
func (k *ActorKeyring) eventAliases(id uuid.UUID) ([]actorAlias, error) {
	if k == nil || id == uuid.Nil {
		return nil, errors.New("invalid catalog event key lookup")
	}
	return k.digests(3, "account-delete-event", id), nil
}
func (k *ActorKeyring) digests(kind int16, purpose string, id uuid.UUID) []actorAlias {
	out := make([]actorAlias, 0, len(k.ids))
	for _, keyID := range k.ids {
		mac := hmac.New(sha256.New, k.keys[keyID])
		_, _ = mac.Write([]byte("voice.chat.catalog.actor.v1\x00" + purpose + "\x00" + keyID + "\x00"))
		_, _ = mac.Write(id[:])
		var digest [32]byte
		copy(digest[:], mac.Sum(nil))
		out = append(out, actorAlias{keyID: keyID, kind: kind, digest: digest})
	}
	return out
}
func (k *ActorKeyring) fingerprints() map[string][32]byte {
	out := make(map[string][32]byte, len(k.ids))
	for _, id := range k.ids {
		mac := hmac.New(sha256.New, k.keys[id])
		_, _ = mac.Write([]byte("voice.chat.catalog.actor-key-check.v1\x00" + id))
		var digest [32]byte
		copy(digest[:], mac.Sum(nil))
		out[id] = digest
	}
	return out
}
func validKeyID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for i, r := range id {
		alnum := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
		if !alnum && (i == 0 || r != '.' && r != '_' && r != '-') {
			return false
		}
	}
	return true
}
