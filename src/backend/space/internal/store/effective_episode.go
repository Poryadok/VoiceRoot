package store

import (
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
)

var ErrEffectiveEpisodeUnavailable = errors.New("effective membership episode unavailable")

// These private witnesses are supplied only by Space's validated native source
// reader, never by a request or Role admission binding. CommunityUntil is the
// minimum of the exact member and owner deadlines, not a roster receipt time.
type effectiveSources struct {
	Ordinary       string
	Legacy         bool
	Community      string
	CommunityUntil time.Time
}

type effectiveHead struct {
	Generation     int64
	RetiredThrough int64
	Revision       int64
	EpisodeID      uuid.UUID
	Active         bool
	Known          bool
	Since          time.Time
	ObservedAt     time.Time
	Sources        effectiveSources
	// Attempt-local old bridge deadline; never persisted as current support.
	bridgeUntil time.Time
}

// cutoverEffectiveEpisode is used only by the explicit owner-local cutover
// under the all-writer barrier, after native source validation. It is never a
// reader repair and never derives entry time from an existing active source.
func cutoverEffectiveEpisode(sources effectiveSources, sampled time.Time, freshID uuid.UUID) (effectiveHead, error) {
	if sampled.IsZero() || !sources.valid() || !sources.equal(sources.at(sampled)) {
		return effectiveHead{}, ErrEffectiveEpisodeUnavailable
	}
	head := effectiveHead{Revision: 1, ObservedAt: sampled, Sources: sources}
	if sources.active() {
		if freshID == uuid.Nil {
			return effectiveHead{}, ErrEffectiveEpisodeUnavailable
		}
		head.Generation, head.EpisodeID, head.Active = 1, freshID, true
	}
	if !head.valid() {
		return effectiveHead{}, ErrEffectiveEpisodeUnavailable
	}
	return head, nil
}

func (s effectiveSources) valid() bool {
	return (!s.Legacy || s.Ordinary != "") && ((s.Community == "") == s.CommunityUntil.IsZero())
}

func (s effectiveSources) at(t time.Time) effectiveSources {
	if s.Community != "" && !t.Before(s.CommunityUntil) {
		s.Community = ""
		s.CommunityUntil = time.Time{}
	}
	return s
}

func (s effectiveSources) active() bool { return s.Ordinary != "" || s.Community != "" }

func (s effectiveSources) equal(other effectiveSources) bool {
	return s.Ordinary == other.Ordinary && s.Legacy == other.Legacy && s.Community == other.Community && s.CommunityUntil.Equal(other.CommunityUntil)
}

func (h effectiveHead) valid() bool {
	if h.Generation < 0 || h.RetiredThrough < 0 || h.RetiredThrough > h.Generation || h.Revision < 0 || !h.Sources.valid() {
		return false
	}
	if h.Revision == 0 {
		return h.Generation == 0 && h.RetiredThrough == 0 && h.EpisodeID == uuid.Nil && !h.Active && !h.Known && h.Since.IsZero() && h.ObservedAt.IsZero() && !h.Sources.active()
	}
	if h.ObservedAt.IsZero() || h.Known == h.Since.IsZero() {
		return false
	}
	if h.Known && h.Since.After(h.ObservedAt) {
		return false
	}
	if h.Active {
		return h.Generation > h.RetiredThrough && h.EpisodeID != uuid.Nil && h.Sources.active()
	}
	return !h.Sources.active() && ((h.Generation == 0 && h.EpisodeID == uuid.Nil && !h.Known) || (h.Generation > 0 && h.RetiredThrough == h.Generation && h.EpisodeID != uuid.Nil))
}

// advanceEffectiveEpisode is the owner-local transition, not an authorization
// callback. Capture before under native locks BEFORE source writes, then supply
// the validated final source tuple and a fresh DB clock after SQL. Missing
// existing heads are never inferred from a current active source.
func advanceEffectiveEpisode(h effectiveHead, before, after effectiveSources, started, finished time.Time, freshID uuid.UUID) (effectiveHead, error) {
	fail := func() (effectiveHead, error) { return effectiveHead{}, ErrEffectiveEpisodeUnavailable }
	if !h.valid() || !before.valid() || !after.valid() || started.IsZero() || finished.Before(started) || started.Before(h.ObservedAt) || h.Revision == math.MaxInt64 {
		return fail()
	}
	before = before.at(started)
	if !after.equal(after.at(started)) || !after.equal(after.at(finished)) {
		return fail()
	}
	// Compare the admitted OLD witnesses, including the saved finite deadline.
	// Expiry is allowed without a revision bump, but unjournaled renewal is not.
	expected := h.Sources.at(started)
	if !before.equal(expected) {
		return fail()
	}
	continuous := h.Active && expected.active()
	ordinaryBridge := continuous && before.Ordinary != "" && before.Ordinary == after.Ordinary && before.Legacy == after.Legacy
	communityBridge := continuous && before.Community != "" && before.Community == after.Community
	if !ordinaryBridge && communityBridge && !finished.Before(before.CommunityUntil) {
		return fail()
	}
	preserve := ordinaryBridge || communityBridge
	next := h
	next.bridgeUntil = time.Time{}
	if preserve && !ordinaryBridge {
		next.bridgeUntil = before.CommunityUntil
	}
	if h.Active && !preserve {
		next.Active = false
		next.RetiredThrough = h.Generation
		next.Sources = effectiveSources{}
	}
	if after.active() && !preserve {
		if next.Generation == math.MaxInt64 || freshID == uuid.Nil || freshID == h.EpisodeID {
			return fail()
		}
		next.Generation++
		next.EpisodeID = freshID
		next.Active = true
		next.Known = !after.Legacy
		next.Since = time.Time{}
		if next.Known {
			next.Since = finished
		}
	}
	if after.active() {
		next.Sources = after
	} else {
		next.Sources = effectiveSources{}
	}
	next.Revision++
	next.ObservedAt = finished
	if !next.valid() {
		return fail()
	}
	return next, nil
}
