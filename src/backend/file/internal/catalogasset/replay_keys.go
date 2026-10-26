package catalogasset

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"regexp"
	"sort"

	"github.com/google/uuid"
)

var ErrReplayKeys = errors.New("catalog lifecycle keys unavailable")
var replayKeyVersion = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,126}$`)

// ReplayDigest is pseudonymous sensitive lifecycle data, not anonymized data.
type ReplayDigest struct {
	KeyVersion string
	HMAC       []byte
}

// ReplayKeys retains old versions so permanent fences remain discoverable after
// rotation. A version must not be retired while its persisted fences/gates exist.
// These keys have a separate purpose from principal signing and file URL keys.
type ReplayKeys struct {
	versions []string
	keys     map[string][]byte
}

func (*ReplayKeys) String() string   { return "[catalog lifecycle keys]" }
func (*ReplayKeys) GoString() string { return "[catalog lifecycle keys]" }

func NewReplayKeys(active string, keys map[string][]byte) (*ReplayKeys, error) {
	if !replayKeyVersion.MatchString(active) || len(keys) == 0 || len(keys[active]) != 32 {
		return nil, ErrReplayKeys
	}
	ring := &ReplayKeys{keys: make(map[string][]byte, len(keys))}
	for version, key := range keys {
		if !replayKeyVersion.MatchString(version) || len(key) != 32 {
			return nil, ErrReplayKeys
		}
		ring.keys[version] = append([]byte(nil), key...)
		if version != active {
			ring.versions = append(ring.versions, version)
		}
	}
	sort.Strings(ring.versions)
	ring.versions = append([]string{active}, ring.versions...)
	return ring, nil
}

func (k *ReplayKeys) digests(purpose string, parts ...[]byte) ([]ReplayDigest, error) {
	if k == nil || len(k.versions) == 0 {
		return nil, ErrReplayKeys
	}
	result := make([]ReplayDigest, 0, len(k.versions))
	for _, version := range k.versions {
		mac := hmac.New(sha256.New, k.keys[version])
		_, _ = mac.Write([]byte("voice.file.catalog.lifecycle.v1\x00"))
		var length [8]byte
		for _, part := range append([][]byte{[]byte(purpose)}, parts...) {
			binary.BigEndian.PutUint64(length[:], uint64(len(part)))
			_, _ = mac.Write(length[:])
			_, _ = mac.Write(part)
		}
		result = append(result, ReplayDigest{KeyVersion: version, HMAC: mac.Sum(nil)})
	}
	return result, nil
}

func (k *ReplayKeys) Account(account uuid.UUID) ([]ReplayDigest, error) {
	if account == uuid.Nil {
		return nil, ErrActorRequest
	}
	return k.digests("actor-account", account[:])
}
func (k *ReplayKeys) Profile(actor ActorPair) ([]ReplayDigest, error) {
	if actor.AccountID == uuid.Nil || actor.ProfileID == uuid.Nil {
		return nil, ErrActorRequest
	}
	return k.digests("actor-profile", actor.AccountID[:], actor.ProfileID[:])
}
func (k *ReplayKeys) Operation(actor ActorPair, operation uuid.UUID) ([]ReplayDigest, error) {
	if actor.AccountID == uuid.Nil || actor.ProfileID == uuid.Nil || operation == uuid.Nil {
		return nil, ErrActorRequest
	}
	return k.digests("operation", actor.AccountID[:], actor.ProfileID[:], operation[:])
}
func (k *ReplayKeys) Source(actor ActorPair, source uuid.UUID) ([]ReplayDigest, error) {
	if actor.AccountID == uuid.Nil || actor.ProfileID == uuid.Nil || source == uuid.Nil {
		return nil, ErrActorRequest
	}
	return k.digests("source", actor.AccountID[:], actor.ProfileID[:], source[:])
}
func (k *ReplayKeys) Asset(actor ActorPair, source uuid.UUID, provider, providerID string, file uuid.UUID) ([]ReplayDigest, error) {
	if actor.AccountID == uuid.Nil || actor.ProfileID == uuid.Nil || source == uuid.Nil || file == uuid.Nil || provider == "" || providerID == "" {
		return nil, ErrActorRequest
	}
	return k.digests("asset", actor.AccountID[:], actor.ProfileID[:], source[:], []byte(provider), []byte(providerID), file[:])
}
