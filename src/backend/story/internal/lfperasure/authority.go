// Package lfperasure defines the closed Story LFP erasure authority boundary.
// It implements no account owner, authenticated RPC adapter, snapshot source,
// negative-memory retention policy or physical-erasure authority.
package lfperasure

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUnavailable = errors.New("LFP erasure authority unavailable")
	ErrRejected    = errors.New("LFP erasure decision rejected")
	ErrErased      = errors.New("LFP evidence retired")
)

const Purpose = "STORY_LFP_ACCOUNT_ERASURE"

type State uint8

const (
	Recoverable State = iota + 1
	Canceled
	PurgeCommitted
)

// Binding is configuration from actual independently admitted trust. Matching
// payload strings alone cannot install this binding or authorize a participant.
type Binding struct {
	Environment, Installation, OwnerIncarnation uuid.UUID
}

func (b Binding) valid() bool {
	return b.Environment != uuid.Nil && b.Installation != uuid.Nil && b.OwnerIncarnation != uuid.Nil
}

type Request struct{ Cycle, Decision uuid.UUID }

// Observation is returned ONLY by an authenticated owner adapter. The adapter
// must check exact purpose/method/request/body/response, replay, admitted peer,
// owner incarnation, final revision and completeness of historical User scope.
// This data structure is not an authentication mechanism.
type Observation struct {
	Version                           uint32
	Purpose, Participant              string
	Binding                           Binding
	Revision                          uint64
	Cycle, Fence, Decision, Account   uuid.UUID
	Profiles                          []uuid.UUID
	CompleteHistoricalScope           bool
	ScopeSHA256                       [32]byte
	ConfirmedAt, PurgeAt, CommittedAt time.Time
	State                             State
}

type FinalAuthority interface {
	ResolveFinal(context.Context, Request) (Observation, error)
}

// FinalCut is immutable transient PERSONAL data. Never log it or retain its
// fields/hashes as an allegedly anonymous completion receipt after local scrub.
type FinalCut struct{ observation Observation }

func (c FinalCut) Account() uuid.UUID    { return c.observation.Account }
func (c FinalCut) Profiles() []uuid.UUID { return append([]uuid.UUID(nil), c.observation.Profiles...) }
func (c FinalCut) PurgeAt() time.Time    { return c.observation.PurgeAt }
func (c FinalCut) Binding() Binding      { return c.observation.Binding }
func (c FinalCut) Revision() uint64      { return c.observation.Revision }
func (c FinalCut) String() string        { return "[private Story LFP final cut]" }
func (c FinalCut) Valid() bool {
	return validateFinal(c.observation, c.observation.Binding, Request{c.observation.Cycle, c.observation.Decision}) == nil
}

// ResolveFinal does not derive finality from elapsed time, user.account_deleted,
// a floor, an absent profile or an archive worker. Nil/unknown stays closed.
func ResolveFinal(ctx context.Context, binding Binding, request Request, authority FinalAuthority) (FinalCut, error) {
	if authority == nil || !binding.valid() || request.Cycle == uuid.Nil || request.Decision == uuid.Nil {
		return FinalCut{}, ErrUnavailable
	}
	o, err := authority.ResolveFinal(ctx, request)
	if err != nil {
		return FinalCut{}, ErrUnavailable
	}
	if err := validateFinal(o, binding, request); err != nil {
		return FinalCut{}, err
	}
	o.Profiles = append([]uuid.UUID(nil), o.Profiles...)
	return FinalCut{observation: o}, nil
}

func validateFinal(o Observation, binding Binding, request Request) error {
	if !binding.valid() || o.Binding != binding || o.Version != 1 || o.Purpose != Purpose || o.Participant != "story" || o.Revision == 0 || o.State != PurgeCommitted || o.Cycle == uuid.Nil || o.Fence == uuid.Nil || o.Decision == uuid.Nil || o.Account == uuid.Nil || o.Cycle != request.Cycle || o.Decision != request.Decision || !o.CompleteHistoricalScope {
		return ErrRejected
	}
	// Calendar policy comes from the owner-confirmed cycle; Story never adds a
	// fresh recovery period at receipt/restore time. SQL must recheck due time
	// AFTER acquiring the mutation locks before deleting anything.
	if o.ConfirmedAt.IsZero() || o.PurgeAt.IsZero() || o.CommittedAt.IsZero() || !o.PurgeAt.Equal(o.ConfirmedAt.Add(30*24*time.Hour)) || o.CommittedAt.Before(o.PurgeAt) {
		return ErrRejected
	}
	h, err := ScopeHash(o.Account, o.Profiles)
	if err != nil || h != o.ScopeSHA256 {
		return ErrRejected
	}
	return nil
}

// ScopeHash authenticates no identity. It only checks exact canonical complete
// scope bytes supplied by the actual owner adapter. Its result is PERSONAL.
func ScopeHash(account uuid.UUID, profiles []uuid.UUID) ([32]byte, error) {
	if account == uuid.Nil || len(profiles) > 100000 {
		return [32]byte{}, ErrRejected
	}
	previous := ""
	b := append([]byte("voice.story.lfp-erasure-scope.v1\x00"), account[:]...)
	var count [4]byte
	binary.BigEndian.PutUint32(count[:], uint32(len(profiles)))
	b = append(b, count[:]...)
	for _, p := range profiles {
		if p == uuid.Nil || (previous != "" && p.String() <= previous) {
			return [32]byte{}, ErrRejected
		}
		previous = p.String()
		b = append(b, p[:]...)
	}
	return sha256.Sum256(b), nil
}

// CanonicalProfiles is a convenience for trusted adapters, not owner discovery.
func CanonicalProfiles(profiles []uuid.UUID) []uuid.UUID {
	p := append([]uuid.UUID(nil), profiles...)
	sort.Slice(p, func(i, j int) bool { return p[i].String() < p[j].String() })
	return p
}

type Verdict uint8

const (
	Unknown Verdict = iota
	LiveOrUnseen
	ErasedOrRetired
)

// References covers subject, object, operation and cohort domains. Empty or
// partial references never mean clean. No live owner/type lookup substitutes
// for the original verified identities or an old operation's negative state.
type References struct {
	Accounts, Profiles, Operations, Consents                 []uuid.UUID
	Offers                                                   []OfferReference
	CohortEnvironment, CohortInstallation, CohortIncarnation uuid.UUID
	DatabaseBinding                                          [32]byte
}
type OfferReference struct {
	Story, Incarnation uuid.UUID
	Revision           uint64
}

// View must be an immutable COMPLETE authenticated current negative snapshot.
// Check is local: no network calls inside SQL. The permit serializes cut/view
// transitions and is retained through actual terminal completion. A callback
// returning LiveOrUnseen is merely a fixture unless the missing installation,
// finite retention/disposal policy and backup/operation coverage are proved.
type View interface {
	Check(References) Verdict
	Release()
}
type NegativeAuthority interface {
	Enter(context.Context) (View, error)
}

func Enter(ctx context.Context, authority NegativeAuthority) (View, error) {
	if authority == nil {
		return nil, ErrUnavailable
	}
	v, err := authority.Enter(ctx)
	if err != nil || v == nil {
		if v != nil {
			v.Release()
		}
		return nil, ErrUnavailable
	}
	return v, nil
}

func Check(view View, refs References) error {
	if view == nil || refs.CohortEnvironment == uuid.Nil || refs.CohortInstallation == uuid.Nil || refs.CohortIncarnation == uuid.Nil || refs.DatabaseBinding == [32]byte{} || len(refs.Accounts)+len(refs.Profiles)+len(refs.Operations)+len(refs.Consents)+len(refs.Offers) == 0 {
		return ErrUnavailable
	}
	for _, ids := range [][]uuid.UUID{refs.Accounts, refs.Profiles, refs.Operations, refs.Consents} {
		for _, id := range ids {
			if id == uuid.Nil {
				return ErrUnavailable
			}
		}
	}
	for _, o := range refs.Offers {
		if o.Story == uuid.Nil || o.Incarnation == uuid.Nil || o.Revision == 0 {
			return ErrUnavailable
		}
	}
	switch view.Check(refs) {
	case LiveOrUnseen:
		return nil
	case ErasedOrRetired:
		return ErrErased
	default:
		return ErrUnavailable
	}
}

// SnapshotAuthority is deliberately only a required adapter boundary. No local
// watermark, empty page list, copied DB marker or readiness flag constructs it.
// A real implementation must authenticate complete pages, final/canceled/
// recoverable cycles and the CURRENT snapshot-to-open seal under cut admission.
type SnapshotAuthority interface {
	ReconcileAndSeal(context.Context, Binding) (SnapshotSeal, error)
}
type SnapshotSeal struct {
	Binding        Binding
	SnapshotID     uuid.UUID
	Revision       uint64
	ManifestSHA256 [32]byte
}

func ReconcileAndSeal(ctx context.Context, b Binding, a SnapshotAuthority) (SnapshotSeal, error) {
	if a == nil || !b.valid() {
		return SnapshotSeal{}, ErrUnavailable
	}
	s, e := a.ReconcileAndSeal(ctx, b)
	if e != nil || s.Binding != b || s.SnapshotID == uuid.Nil || s.Revision == 0 || s.ManifestSHA256 == [32]byte{} {
		return SnapshotSeal{}, ErrUnavailable
	}
	return s, nil
}
