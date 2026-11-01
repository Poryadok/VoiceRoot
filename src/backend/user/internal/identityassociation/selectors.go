// Package identityassociation is the single User-owned lifetime association and
// nonreuse authority. Selector material never leaves User purpose adapters.
package identityassociation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"github.com/google/uuid"
	"math"
	"sort"
)

var ErrKeysetIncomplete = errors.New("identity association keyset incomplete")
var ErrConflict = errors.New("identity association conflict")
var ErrIncomplete = errors.New("identity association evidence incomplete")

type KeysetSnapshot struct {
	Revision      uint64
	ActiveVersion uint32
	Versions      map[uint32][32]byte
}
type Keyset struct {
	snapshot KeysetSnapshot
	keys     map[uint32][]byte
}
type selectorAlias struct {
	version          uint32
	account, profile [32]byte
}
type PreparedSelectors struct {
	profileID, accountID uuid.UUID
	snapshot             KeysetSnapshot
	aliases              []selectorAlias
}

func NewKeyset(snapshot KeysetSnapshot, secrets map[uint32][]byte) (*Keyset, error) {
	if snapshot.Revision == 0 || snapshot.Revision > math.MaxInt64 || snapshot.ActiveVersion == 0 || len(snapshot.Versions) == 0 || len(snapshot.Versions) != len(secrets) {
		return nil, ErrKeysetIncomplete
	}
	if _, ok := snapshot.Versions[snapshot.ActiveVersion]; !ok {
		return nil, ErrKeysetIncomplete
	}
	k := &Keyset{snapshot: KeysetSnapshot{Revision: snapshot.Revision, ActiveVersion: snapshot.ActiveVersion, Versions: map[uint32][32]byte{}}, keys: map[uint32][]byte{}}
	for version, fingerprint := range snapshot.Versions {
		secret, ok := secrets[version]
		if version == 0 || !ok || len(secret) < 32 || sha256.Sum256(secret) != fingerprint {
			return nil, ErrKeysetIncomplete
		}
		k.snapshot.Versions[version] = fingerprint
		k.keys[version] = append([]byte(nil), secret...)
	}
	return k, nil
}

func selector(key []byte, kind string, id uuid.UUID) [32]byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte("voice.user.identity-association.v1\x00" + kind + "\x00"))
	h.Write(id[:])
	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result
}

func (k *Keyset) PrepareSelectors(profile, account uuid.UUID) (PreparedSelectors, error) {
	if k == nil || profile == uuid.Nil || account == uuid.Nil || len(k.keys) == 0 {
		return PreparedSelectors{}, ErrIncomplete
	}
	p := PreparedSelectors{profileID: profile, accountID: account, snapshot: k.Snapshot()}
	versions := make([]uint32, 0, len(k.keys))
	for version := range k.keys {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	for _, version := range versions {
		p.aliases = append(p.aliases, selectorAlias{version: version, account: selector(k.keys[version], "ACCOUNT", account), profile: selector(k.keys[version], "PROFILE", profile)})
	}
	return p, nil
}

func (k *Keyset) Snapshot() KeysetSnapshot {
	if k == nil {
		return KeysetSnapshot{}
	}
	s := KeysetSnapshot{Revision: k.snapshot.Revision, ActiveVersion: k.snapshot.ActiveVersion, Versions: map[uint32][32]byte{}}
	for v, f := range k.snapshot.Versions {
		s.Versions[v] = f
	}
	return s
}

// CustodyDigest commits the complete public keyset description, never secrets.
// Encoding: domain plus NUL, revision uint64, active version uint32, count
// uint32, then ascending (version uint32, SHA-256 fingerprint); integers are
// big endian. It binds a custody record to every retained selector key.
func (k *Keyset) CustodyDigest() ([32]byte, error) {
	if k == nil || k.snapshot.Revision == 0 || len(k.snapshot.Versions) == 0 {
		return [32]byte{}, ErrKeysetIncomplete
	}
	h := sha256.New()
	_, _ = h.Write([]byte("voice.user.identity-association.custody-keyset.v1\x00"))
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], k.snapshot.Revision)
	_, _ = h.Write(number[:])
	binary.BigEndian.PutUint32(number[:4], k.snapshot.ActiveVersion)
	_, _ = h.Write(number[:4])
	binary.BigEndian.PutUint32(number[:4], uint32(len(k.snapshot.Versions)))
	_, _ = h.Write(number[:4])
	versions := make([]uint32, 0, len(k.snapshot.Versions))
	for version := range k.snapshot.Versions {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	for _, version := range versions {
		binary.BigEndian.PutUint32(number[:4], version)
		_, _ = h.Write(number[:4])
		fingerprint := k.snapshot.Versions[version]
		_, _ = h.Write(fingerprint[:])
	}
	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result, nil
}

func (p PreparedSelectors) valid() bool {
	return p.profileID != uuid.Nil && p.accountID != uuid.Nil && p.snapshot.Revision > 0 && len(p.aliases) > 0 && len(p.aliases) == len(p.snapshot.Versions)
}

func (*Keyset) String() string             { return "User identity keyset [redacted]" }
func (*Keyset) GoString() string           { return "User identity keyset [redacted]" }
func (PreparedSelectors) String() string   { return "User identity prepared selectors [redacted]" }
func (PreparedSelectors) GoString() string { return "User identity prepared selectors [redacted]" }
