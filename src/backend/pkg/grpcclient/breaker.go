package grpcclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BreakerConfig is copied when constructing a connection-scoped unary breaker.
type BreakerConfig struct {
	FailureThreshold int
	FailureWindow    time.Duration
	OpenTimeout      time.Duration
	MaxMethods       int
	IdleTimeout      time.Duration
	// Observe receives only fixed state/reason values and the full RPC method.
	// It runs outside the state mutex. Do not add request or authority material.
	Observe func(BreakerEvent)
}

type BreakerEvent struct{ Method, State, Reason string }

type breakerState uint8

const (
	breakerClosed breakerState = iota
	breakerOpen
	breakerHalfOpen
)

type methodCircuit struct {
	state        breakerState
	generation   uint64
	outstanding  int
	failures     int
	firstFailure time.Time
	openUntil    time.Time
	lastActivity time.Time
}

type breakerAdmission struct {
	method     string
	entry      *methodCircuit
	generation uint64
	state      breakerState
}

type breakerOutcome uint8

const (
	breakerNeutral breakerOutcome = iota
	breakerSuccess
	breakerFailure
)

// DefaultBreakerConfig returns the documented outbound unary breaker defaults.
func DefaultBreakerConfig() BreakerConfig {
	return BreakerConfig{FailureThreshold: 5, FailureWindow: 30 * time.Second,
		OpenTimeout: 10 * time.Second, MaxMethods: 1024, IdleTimeout: 10 * time.Minute}
}

// UnaryBreaker belongs to one ClientConn and never retries an invocation.
type UnaryBreaker struct {
	mu      sync.Mutex
	config  BreakerConfig
	now     func() time.Time
	methods map[string]*methodCircuit
}

func NewUnaryBreaker(config BreakerConfig) (*UnaryBreaker, error) {
	if config.FailureThreshold <= 0 || config.FailureThreshold > 100 ||
		config.FailureWindow <= 0 || config.FailureWindow > 5*time.Minute ||
		config.OpenTimeout <= 0 || config.OpenTimeout > time.Minute ||
		config.MaxMethods <= 0 || config.MaxMethods > 4096 ||
		config.IdleTimeout <= 0 || config.IdleTimeout > time.Hour {
		return nil, fmt.Errorf("grpc unary breaker configuration exceeds positive policy bounds")
	}
	return &UnaryBreaker{config: config, now: time.Now, methods: make(map[string]*methodCircuit)}, nil
}

// UnaryClientInterceptor returns the connection's unary admission interceptor.
func (b *UnaryBreaker) UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if err := ctx.Err(); err != nil {
			return status.FromContextError(err).Err()
		}
		if method == "/grpc.health.v1.Health/Check" || method == "/grpc.health.v1.Health/List" {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
		admission, event := b.admit(method)
		if admission.entry == nil {
			b.observe(event)
			return status.Error(codes.Unavailable, "upstream temporarily unavailable")
		}
		completed := false
		defer func() {
			if !completed {
				// Panic/Goexit unwinding releases the token without claiming recovery.
				// No recover is used: the original panic continues unchanged.
				b.complete(admission, breakerNeutral)
			}
		}()
		b.observe(event)
		err := invoker(ctx, method, req, reply, cc, opts...)
		outcome := breakerNeutral
		if ctx.Err() == nil {
			if err == nil {
				outcome = breakerSuccess
			} else if code := status.Code(err); code == codes.Unavailable || code == codes.DeadlineExceeded {
				outcome = breakerFailure
			}
		}
		completed = true
		b.complete(admission, outcome)
		return err
	}
}

// admit returns a token for exactly one invocation, or a fixed rejection event.
func (b *UnaryBreaker) admit(method string) (breakerAdmission, *BreakerEvent) {
	now := b.now()
	b.mu.Lock()
	defer b.mu.Unlock()
	entry := b.methods[method]
	if entry == nil {
		if len(b.methods) >= b.config.MaxMethods {
			for name, candidate := range b.methods {
				if candidate.state == breakerClosed && candidate.outstanding == 0 && now.Sub(candidate.lastActivity) >= b.config.IdleTimeout {
					delete(b.methods, name)
				}
			}
		}
		if len(b.methods) >= b.config.MaxMethods {
			return breakerAdmission{}, &BreakerEvent{Method: method, State: "closed", Reason: "capacity"}
		}
		entry = &methodCircuit{lastActivity: now}
		b.methods[method] = entry
	}
	var event *BreakerEvent
	switch entry.state {
	case breakerOpen:
		if now.Before(entry.openUntil) {
			return breakerAdmission{}, &BreakerEvent{Method: method, State: "open", Reason: "open"}
		}
		entry.state = breakerHalfOpen
		entry.generation++
		event = &BreakerEvent{Method: method, State: "half_open", Reason: "probe"}
	case breakerHalfOpen:
		return breakerAdmission{}, &BreakerEvent{Method: method, State: "half_open", Reason: "probe_busy"}
	}
	entry.outstanding++
	entry.lastActivity = now
	return breakerAdmission{method: method, entry: entry, generation: entry.generation, state: entry.state}, event
}

func (b *UnaryBreaker) complete(token breakerAdmission, outcome breakerOutcome) {
	now := b.now()
	b.mu.Lock()
	entry := token.entry
	entry.outstanding--
	if b.methods[token.method] != entry || entry.generation != token.generation || entry.state != token.state {
		b.mu.Unlock()
		return
	}
	entry.lastActivity = now
	var event *BreakerEvent
	if entry.state == breakerHalfOpen {
		entry.generation++
		if outcome == breakerSuccess {
			entry.state = breakerClosed
			entry.failures = 0
			entry.firstFailure = time.Time{}
			event = &BreakerEvent{Method: token.method, State: "closed", Reason: "recovered"}
		} else {
			entry.state = breakerOpen
			entry.openUntil = now.Add(b.config.OpenTimeout)
			reason := "probe_neutral"
			if outcome == breakerFailure {
				reason = "probe_failure"
			}
			event = &BreakerEvent{Method: token.method, State: "open", Reason: reason}
		}
	} else {
		switch outcome {
		case breakerSuccess:
			entry.failures = 0
			entry.firstFailure = time.Time{}
		case breakerFailure:
			if entry.failures == 0 || now.Sub(entry.firstFailure) >= b.config.FailureWindow {
				entry.failures = 0
				entry.firstFailure = now
			}
			entry.failures++
			if entry.failures >= b.config.FailureThreshold {
				entry.state = breakerOpen
				entry.generation++
				entry.openUntil = now.Add(b.config.OpenTimeout)
				event = &BreakerEvent{Method: token.method, State: "open", Reason: "failure_threshold"}
			}
		}
	}
	b.mu.Unlock()
	b.observe(event)
}

func (b *UnaryBreaker) observe(event *BreakerEvent) {
	if event != nil && b.config.Observe != nil {
		b.config.Observe(*event)
	}
}
