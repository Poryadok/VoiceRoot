//go:build spacebanintegration

package store

import (
	"math"
	"testing"
)

func TestNative50ProfileDeltaRejectsIncompleteCounters(t *testing.T) {
	good := native50ProfileStats{calls: 10, totalMS: 20, selfMS: 15, present: true}
	for name, pair := range map[string][2]native50ProfileStats{
		"missing before":     {{}, good},
		"missing after":      {good, {}},
		"calls rewind":       {good, {calls: 9, totalMS: 21, selfMS: 16, present: true}},
		"total rewind":       {good, {calls: 11, totalMS: 19, selfMS: 16, present: true}},
		"self rewind":        {good, {calls: 11, totalMS: 21, selfMS: 14, present: true}},
		"nan":                {good, {calls: 11, totalMS: math.NaN(), selfMS: 16, present: true}},
		"infinite":           {good, {calls: 11, totalMS: math.Inf(1), selfMS: 16, present: true}},
		"self exceeds total": {good, {calls: 11, totalMS: 21, selfMS: 22, present: true}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := native50ProfileDelta(pair[0], pair[1]); ok {
				t.Fatal("incomplete or decreasing counters accepted")
			}
		})
	}
	after := native50ProfileStats{calls: 13, totalMS: 26, selfMS: 18, present: true}
	got, ok := native50ProfileDelta(good, after)
	if !ok || got.calls != 3 || got.totalMS != 6 || got.selfMS != 3 {
		t.Fatalf("unexpected delta: %+v, valid=%v", got, ok)
	}
}
