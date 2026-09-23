package main

import (
	"context"
	"sync"
)

type realtimeConsumerReadiness struct {
	mu    sync.RWMutex
	bound map[string]bool
}

func newRealtimeConsumerReadiness(names ...string) *realtimeConsumerReadiness {
	r := &realtimeConsumerReadiness{bound: map[string]bool{}}
	for _, n := range names {
		r.bound[n] = false
	}
	return r
}
func (r *realtimeConsumerReadiness) set(name string, ready bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.bound[name] = ready
	r.mu.Unlock()
}
func (r *realtimeConsumerReadiness) ready() bool {
	if r == nil {
		return true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ok := range r.bound {
		if !ok {
			return false
		}
	}
	return true
}

type realtimeConsumerReadinessKey struct{}
type realtimeConsumerReadinessBinding struct {
	tracker *realtimeConsumerReadiness
	name    string
}

func withRealtimeConsumerReadiness(ctx context.Context, tracker *realtimeConsumerReadiness, name string) context.Context {
	return context.WithValue(ctx, realtimeConsumerReadinessKey{}, realtimeConsumerReadinessBinding{tracker, name})
}
func markRealtimeConsumerBound(ctx context.Context) {
	binding, _ := ctx.Value(realtimeConsumerReadinessKey{}).(realtimeConsumerReadinessBinding)
	binding.tracker.set(binding.name, true)
}
