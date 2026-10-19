package store

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEffectiveEpisodeTransitions(t *testing.T) {
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	ordinary := effectiveSources{Ordinary: "o1"}
	community := effectiveSources{Community: "c1", CommunityUntil: now.Add(time.Minute)}
	newHead := func(s effectiveSources) effectiveHead {
		h, e := advanceEffectiveEpisode(effectiveHead{}, effectiveSources{}, s, now, now, uuid.New())
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	t.Run("overlap preserves entry across either source loss", func(t *testing.T) {
		h := newHead(ordinary)
		both := community
		both.Ordinary = "o1"
		a, e := advanceEffectiveEpisode(h, ordinary, both, now.Add(time.Second), now.Add(time.Second), uuid.New())
		if e != nil {
			t.Fatal(e)
		}
		b, e := advanceEffectiveEpisode(a, both, community, now.Add(2*time.Second), now.Add(2*time.Second), uuid.New())
		if e != nil || b.EpisodeID != h.EpisodeID || !b.Since.Equal(h.Since) || b.Generation != h.Generation {
			t.Fatalf("lost continuity: %+v %v", b, e)
		}
	})
	t.Run("expired member or owner cannot renew without sweeper", func(t *testing.T) {
		h := newHead(community)
		renewed := community
		renewed.CommunityUntil = now.Add(3 * time.Minute)
		for _, at := range []time.Time{community.CommunityUntil, community.CommunityUntil.Add(time.Second)} {
			expired := effectiveSources{}
			a, e := advanceEffectiveEpisode(h, expired, renewed, at, at, uuid.New())
			if e != nil || a.Generation != h.Generation+1 || a.EpisodeID == h.EpisodeID || !a.Since.Equal(at) || a.RetiredThrough != h.Generation {
				t.Fatalf("revived expired episode: %+v %v", a, e)
			}
		}
	})
	t.Run("semantic replacement requires exact bridge", func(t *testing.T) {
		h := newHead(community)
		next := community
		next.Community = "c2"
		a, e := advanceEffectiveEpisode(h, community, next, now, now, uuid.New())
		if e != nil || a.Generation != 2 || a.EpisodeID == h.EpisodeID {
			t.Fatalf("replacement not cut: %+v %v", a, e)
		}
		both := community
		both.Ordinary = "o1"
		h = newHead(both)
		next.Ordinary = "o1"
		a, e = advanceEffectiveEpisode(h, both, next, now, now, uuid.New())
		if e != nil || a.EpisodeID != h.EpisodeID {
			t.Fatalf("ordinary bridge lost: %+v %v", a, e)
		}
	})
	t.Run("final SQL wait cannot extend old bridge", func(t *testing.T) {
		h := newHead(community)
		next := community
		next.CommunityUntil = now.Add(3 * time.Minute)
		_, e := advanceEffectiveEpisode(h, community, next, now.Add(time.Second), community.CommunityUntil, uuid.New())
		if !errors.Is(e, ErrEffectiveEpisodeUnavailable) {
			t.Fatalf("stale continuity accepted: %v", e)
		}
	})
	t.Run("ordinary bridge survives expired community", func(t *testing.T) {
		both := community
		both.Ordinary = "o1"
		h := newHead(both)
		a, e := advanceEffectiveEpisode(h, ordinary, ordinary, now.Add(2*time.Minute), now.Add(2*time.Minute), uuid.New())
		if e != nil || a.EpisodeID != h.EpisodeID {
			t.Fatalf("ordinary cut: %+v %v", a, e)
		}
	})
	t.Run("legacy unknown cannot upgrade in overlap", func(t *testing.T) {
		legacy := ordinary
		legacy.Legacy = true
		h := newHead(legacy)
		both := community
		both.Ordinary = "o1"
		both.Legacy = true
		a, e := advanceEffectiveEpisode(h, legacy, both, now, now, uuid.New())
		if e != nil || a.Known || !a.Since.IsZero() {
			t.Fatalf("invented cutoff: %+v %v", a, e)
		}
		retired, e := advanceEffectiveEpisode(a, both, effectiveSources{}, now, now, uuid.New())
		if e != nil {
			t.Fatal(e)
		}
		b, e := advanceEffectiveEpisode(retired, effectiveSources{}, ordinary, now, now, uuid.New())
		if e != nil || !b.Known || b.Generation <= h.Generation {
			t.Fatalf("fresh join: %+v %v", b, e)
		}
	})
	t.Run("clock regression overflow and malformed witnesses fail", func(t *testing.T) {
		h := newHead(ordinary)
		if _, e := advanceEffectiveEpisode(h, ordinary, ordinary, now.Add(-time.Second), now, uuid.New()); e == nil {
			t.Fatal("clock regressed")
		}
		h.Revision = math.MaxInt64
		if _, e := advanceEffectiveEpisode(h, ordinary, ordinary, now, now, uuid.New()); e == nil {
			t.Fatal("revision wrapped")
		}
		h = newHead(ordinary)
		h.Generation = math.MaxInt64
		if _, e := advanceEffectiveEpisode(h, ordinary, effectiveSources{Ordinary: "o2"}, now, now, uuid.New()); e == nil {
			t.Fatal("generation wrapped")
		}
		h = newHead(ordinary)
		if _, e := advanceEffectiveEpisode(h, effectiveSources{Ordinary: "foreign"}, ordinary, now, now, uuid.New()); e == nil {
			t.Fatal("foreign old source")
		}
	})
	t.Run("final support must remain live", func(t *testing.T) {
		_, e := advanceEffectiveEpisode(effectiveHead{}, effectiveSources{}, community, now, community.CommunityUntil, uuid.New())
		if e == nil {
			t.Fatal("expired final support")
		}
	})
	t.Run("new entry uses owner publication completion and rejects future cutoff", func(t *testing.T) {
		finished := now.Add(time.Second)
		h, e := advanceEffectiveEpisode(effectiveHead{}, effectiveSources{}, ordinary, now, finished, uuid.New())
		if e != nil || !h.Since.Equal(finished) {
			t.Fatalf("entry backdated before owner publication: %+v %v", h, e)
		}
		h.Since = finished.Add(time.Second)
		if _, e = advanceEffectiveEpisode(h, ordinary, ordinary, finished, finished, uuid.New()); e == nil {
			t.Fatal("future cutoff accepted")
		}
	})
}

func TestEffectiveEpisodeCutoverNeverInventsHistory(t *testing.T) {
	at := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	for name, sources := range map[string]effectiveSources{
		"published ordinary": {Ordinary: "exact-native-positive-publication"},
		"explicit legacy":    {Ordinary: "exact-native-legacy-publication", Legacy: true},
		"community only":     {Community: "exact-native-community-episode", CommunityUntil: at.Add(time.Minute)},
	} {
		t.Run(name, func(t *testing.T) {
			h, err := cutoverEffectiveEpisode(sources, at, uuid.New())
			if err != nil || !h.Active || h.Known || !h.Since.IsZero() || h.Generation != 1 || h.Revision != 1 {
				t.Fatalf("cutover invented a boundary: %+v %v", h, err)
			}
			added := sources
			added.Ordinary = "new-published-overlap"
			next, err := advanceEffectiveEpisode(h, sources, added, at, at, uuid.New())
			if sources.Ordinary == "" && (err != nil || next.Known || next.EpisodeID != h.EpisodeID) {
				t.Fatalf("overlap upgraded UNKNOWN: %+v %v", next, err)
			}
		})
	}
	if _, err := cutoverEffectiveEpisode(effectiveSources{Community: "expired", CommunityUntil: at}, at, uuid.New()); err == nil {
		t.Fatal("expired source became active at cutover")
	}
	if _, err := cutoverEffectiveEpisode(effectiveSources{}, at, uuid.Nil); err != nil {
		t.Fatalf("known absent source must initialize an inert head: %v", err)
	}
}
