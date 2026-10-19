// Package control defines the private MatchSquad owner-control boundary.
// References are resolved by protected authority/supervisor journals. These
// types and shape checks do not establish an installed enforcement capability.
package control

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
)

var (
	ErrUnavailable = errors.New("owner control capability unavailable")
	ErrConflict    = errors.New("owner control binding conflict")
	ErrPending     = errors.New("owner control obligation unresolved")
)

type ID string

func (id ID) Valid() bool {
	s := string(id)
	if len(s) != 36 || s == "00000000-0000-0000-0000-000000000000" {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

type Digest [32]byte

func Hash(domain string, value any) (Digest, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return Digest{}, err
	}
	return sha256.Sum256(append([]byte(domain+"\x00"), raw...)), nil
}

type Ref struct {
	Issuer, Record ID
	Hash           Digest
}

func (r Ref) Valid() bool { return r.Issuer.Valid() && r.Record.Valid() && r.Hash != (Digest{}) }

type OwnerBinding struct {
	Authority, Resource, Incarnation, LaunchOperation ID
	ControlRevision                                   uint64
	ImageDigest                                       Digest
}

func (b OwnerBinding) Valid() bool {
	return b.Authority.Valid() && b.Resource.Valid() && b.Incarnation.Valid() && b.LaunchOperation.Valid() && b.ControlRevision > 0 && b.ControlRevision <= 1<<53 && b.ImageDigest != (Digest{})
}

type LaunchIntent struct {
	Ref                 Ref
	Binding             OwnerBinding
	Supervisor, Profile ID
}

// CloseIntent is the immutable exhaustive authority census of every launch
// intent, including ambiguous spawn/enable outcomes. The protected reader must
// rederive closure from the retained native resource journal on EVERY read.
// A caller list, current route or a local worker census cannot supply it.
type CloseIntent struct {
	Ref                            Ref
	Authority, Resource, Operation ID
	SealingRevision                uint64
	Owners                         []LaunchIntent
	OwnerCensusHash                Digest
}

func (c CloseIntent) Validate() error {
	if !c.Ref.Valid() || c.Ref.Issuer != c.Authority || !c.Authority.Valid() || !c.Resource.Valid() || !c.Operation.Valid() || c.SealingRevision == 0 || c.SealingRevision > 1<<53 || c.Owners == nil {
		return ErrConflict
	}
	seen := map[ID]bool{}
	for _, o := range c.Owners {
		if !o.Ref.Valid() || o.Ref.Issuer != c.Authority || !o.Binding.Valid() || o.Binding.Authority != c.Authority || o.Binding.Resource != c.Resource || o.Binding.ControlRevision >= c.SealingRevision || !o.Supervisor.Valid() || !o.Profile.Valid() || seen[o.Binding.Incarnation] {
			return ErrConflict
		}
		seen[o.Binding.Incarnation] = true
	}
	hash, err := Hash("voice.matchsquad.owner-census.v1", c.Owners)
	if err != nil || hash != c.OwnerCensusHash {
		return ErrConflict
	}
	return nil
}

// RuntimeUID identifies a sandbox birth, never a recycled name/PID/label. The
// host boot identity and protected supervisor journal provide its provenance.
type RuntimeIdentity struct {
	Host, Boot                ID
	RuntimeUID, AttachmentUID string
	ImageDigest               Digest
}

func (r RuntimeIdentity) Valid() bool {
	return r.Host.Valid() && r.Boot.Valid() && r.RuntimeUID != "" && r.AttachmentUID != "" && strings.TrimSpace(r.RuntimeUID) == r.RuntimeUID && strings.TrimSpace(r.AttachmentUID) == r.AttachmentUID && r.ImageDigest != (Digest{})
}

type RuntimeFence struct {
	Runtime                                  RuntimeIdentity
	ForwardingFence, Termination, Quiescence Ref
}
type FenceReceipt struct {
	Ref     Ref
	Close   Ref
	Launch  Ref
	Binding OwnerBinding
	Profile ID
	// SealBarrier prevents every late spawn/enable, including across supervisor
	// restart. The supervisor's complete durable birth census owns this evidence.
	SealBarrier Ref
	// These references resolve the qualified current host/adapter profile and
	// complete durable birth registry; their hashes alone are not authority.
	ProfileWitness, BirthCensus Ref
	Runtimes                    []RuntimeFence
	RuntimeCensusHash           Digest
	CanonicalBytes              []byte
}

func (f FenceReceipt) Validate(c CloseIntent, o LaunchIntent) error {
	if !f.Ref.Valid() || f.Ref.Issuer != o.Supervisor || f.Close != c.Ref || f.Launch != o.Ref || f.Binding != o.Binding || f.Profile != o.Profile || !f.SealBarrier.Valid() || f.SealBarrier.Issuer != o.Supervisor || f.Runtimes == nil {
		return ErrConflict
	}
	for _, witness := range []Ref{f.ProfileWitness, f.BirthCensus} {
		if !witness.Valid() || witness.Issuer != o.Supervisor {
			return ErrConflict
		}
	}
	seen := map[string]bool{}
	for _, r := range f.Runtimes {
		if !r.Runtime.Valid() || r.Runtime.ImageDigest != o.Binding.ImageDigest || seen[r.Runtime.RuntimeUID] {
			return ErrConflict
		}
		seen[r.Runtime.RuntimeUID] = true
		for _, ref := range []Ref{r.ForwardingFence, r.Termination, r.Quiescence} {
			if !ref.Valid() || ref.Issuer != o.Supervisor {
				return ErrConflict
			}
		}
	}
	census, err := Hash("voice.matchsquad.runtime-census.v1", f.Runtimes)
	if err != nil || census != f.RuntimeCensusHash {
		return ErrConflict
	}
	saved := f.CanonicalBytes
	expected := f.Ref.Hash
	f.CanonicalBytes = nil
	f.Ref.Hash = Digest{}
	raw, err := json.Marshal(f)
	if err != nil || string(saved) != string(raw) {
		return ErrConflict
	}
	hash, _ := Hash("voice.matchsquad.fence-receipt.v1", f)
	if hash != expected {
		return ErrConflict
	}
	return nil
}

type CloseAcknowledgement struct {
	Ref, Close      Ref
	Resource        ID
	SealingRevision uint64
	OwnerCensusHash Digest
	WorkerFences    []Ref
	// Absence is a protected authority-owned aggregate over actual room,
	// participant, routes/store/bus and forwarding observations. No boolean or
	// current-owner NotFound supplies it. Native Voice final receipt follows it.
	Absence Ref
}
