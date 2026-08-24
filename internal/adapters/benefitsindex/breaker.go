package benefitsindex

import (
	"sync"
	"time"
)

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

const (
	defaultFailureLimit = 3
	defaultCooldown     = 5 * time.Second
)

type CircuitBreaker struct {
	mu            sync.Mutex
	state         CircuitState
	failures      int
	trialInFlight bool
	openedAt      time.Time

	FailureLimit int
	Cooldown     time.Duration
}

func NewCircuitBreaker(failureLimit int, cooldown time.Duration) *CircuitBreaker {
	if failureLimit <= 0 {
		failureLimit = defaultFailureLimit
	}

	if cooldown <= 0 {
		cooldown = defaultCooldown
	}

	return &CircuitBreaker{
		state:        CircuitClosed,
		FailureLimit: failureLimit,
		Cooldown:     cooldown,
	}
}

func (b *CircuitBreaker) Allow() (allowed bool, isTrial bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case CircuitClosed:
		return true, false

	case CircuitOpen:
		if time.Since(b.openedAt) >= b.Cooldown {
			b.state = CircuitHalfOpen
			b.trialInFlight = true
			return true, true
		}
		return false, false

	case CircuitHalfOpen:
		return false, false
	}

	return false, false
}

func (b *CircuitBreaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = CircuitClosed
	b.failures = 0
	b.trialInFlight = false
}

func (b *CircuitBreaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.trialInFlight = false

	switch b.state {
	case CircuitHalfOpen:
		b.state = CircuitOpen
		b.openedAt = time.Now()
		b.failures = 0

	case CircuitClosed:
		b.failures++
		if b.failures >= b.FailureLimit {
			b.state = CircuitOpen
			b.openedAt = time.Now()
			b.failures = 0
		}
	}
}

func (b *CircuitBreaker) StateString() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half_open"
	default:
		return "closed"
	}
}
