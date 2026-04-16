package lib

import (
	"errors"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type State int

const (
	StateClosed State = iota
	StateHalfOpen
	StateOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return "unknown"
	}
}

type CircuitBreakerOptions struct {
	MaxFailures int
	Timeout     time.Duration
	HalfOpenMax int
}

func DefaultCircuitBreakerOptions() *CircuitBreakerOptions {
	return &CircuitBreakerOptions{
		MaxFailures: 5,
		Timeout:     30 * time.Second,
		HalfOpenMax: 3,
	}
}

type CircuitBreaker struct {
	name              string
	opts              *CircuitBreakerOptions
	state             State
	failures          int
	successes         int
	lastFailure       time.Time
	halfOpenSuccesses int
	mu                sync.RWMutex
}

func NewCircuitBreaker(name string, opts *CircuitBreakerOptions) *CircuitBreaker {
	if opts == nil {
		opts = DefaultCircuitBreakerOptions()
	}
	return &CircuitBreaker{
		name:  name,
		opts:  opts,
		state: StateClosed,
	}
}

func (cb *CircuitBreaker) Name() string {
	return cb.name
}

func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func (cb *CircuitBreaker) Call(fn func() error) error {
	if !cb.allowRequest() {
		return ErrCircuitOpen
	}

	err := fn()
	if err != nil {
		cb.recordFailure()
		return err
	}

	cb.recordSuccess()
	return nil
}

func (cb *CircuitBreaker) CallWithFallback(fn func() error, fallback func(error) error) error {
	if !cb.allowRequest() {
		return fallback(ErrCircuitOpen)
	}

	err := fn()
	if err != nil {
		cb.recordFailure()
		return fallback(err)
	}

	cb.recordSuccess()
	return nil
}

func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateHalfOpen:
		return true
	case StateOpen:
		if time.Since(cb.lastFailure) >= cb.opts.Timeout {
			cb.state = StateHalfOpen
			cb.halfOpenSuccesses = 0
			return true
		}
		return false
	}
	return false
}

func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailure = time.Now()
	cb.failures++

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.opts.MaxFailures {
			cb.state = StateOpen
		}
	case StateHalfOpen:
		cb.state = StateOpen
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		cb.failures = 0
	case StateHalfOpen:
		cb.halfOpenSuccesses++
		if cb.halfOpenSuccesses >= cb.opts.HalfOpenMax {
			cb.state = StateClosed
			cb.failures = 0
			cb.halfOpenSuccesses = 0
		}
	}
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenSuccesses = 0
}

func (cb *CircuitBreaker) Stats() (failures int, state State) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures, cb.state
}

type CircuitBreakerGroup struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

func NewCircuitBreakerGroup() *CircuitBreakerGroup {
	return &CircuitBreakerGroup{
		breakers: make(map[string]*CircuitBreaker),
	}
}

func (g *CircuitBreakerGroup) Get(name string) (*CircuitBreaker, bool) {
	g.mu.RLock()
	cb, ok := g.breakers[name]
	g.mu.RUnlock()
	return cb, ok
}

func (g *CircuitBreakerGroup) GetOrCreate(name string, opts *CircuitBreakerOptions) *CircuitBreaker {
	g.mu.Lock()
	defer g.mu.Unlock()

	if cb, ok := g.breakers[name]; ok {
		return cb
	}

	cb := NewCircuitBreaker(name, opts)
	g.breakers[name] = cb
	return cb
}

func (g *CircuitBreakerGroup) Call(name string, fn func() error) error {
	cb := g.GetOrCreate(name, nil)
	return cb.Call(fn)
}

func (g *CircuitBreakerGroup) Reset(name string) {
	g.mu.RLock()
	cb, ok := g.breakers[name]
	g.mu.RUnlock()
	if ok {
		cb.Reset()
	}
}

func (g *CircuitBreakerGroup) ResetAll() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, cb := range g.breakers {
		cb.Reset()
	}
}

var globalCircuitBreakers = NewCircuitBreakerGroup()

func GlobalCircuitBreakers() *CircuitBreakerGroup {
	return globalCircuitBreakers
}
