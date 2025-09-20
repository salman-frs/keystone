package circuit

import (
	"context"
	"errors"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// Config holds circuit breaker configuration
type Config struct {
	FailureThreshold   int           // Number of failures to open circuit
	RecoveryTimeout    time.Duration // Time to wait before trying half-open
	SuccessThreshold   int           // Number of successes needed to close from half-open
	RequestTimeout     time.Duration // Timeout for individual requests
	MaxConcurrentCalls int           // Maximum concurrent calls in half-open state
}

// DefaultConfig returns a default circuit breaker configuration
func DefaultConfig() Config {
	return Config{
		FailureThreshold:   10,
		RecoveryTimeout:    5 * time.Minute,
		SuccessThreshold:   3,
		RequestTimeout:     30 * time.Second,
		MaxConcurrentCalls: 5,
	}
}

// Breaker implements a circuit breaker pattern for external service calls
type Breaker struct {
	config          Config
	state           State
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	mutex           sync.RWMutex
	activeCalls     int
}

// New creates a new circuit breaker with the given configuration
func New(config Config) *Breaker {
	return &Breaker{
		config: config,
		state:  StateClosed,
	}
}

// Errors
var (
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrTooManyCalls    = errors.New("too many concurrent calls")
	ErrRequestTimeout  = errors.New("request timeout")
)

// Call executes the given function with circuit breaker protection
func (b *Breaker) Call(ctx context.Context, fn func() error) error {
	state, err := b.beforeCall()
	if err != nil {
		return err
	}

	defer b.afterCall(state == StateHalfOpen)

	// Create context with timeout
	callCtx, cancel := context.WithTimeout(ctx, b.config.RequestTimeout)
	defer cancel()

	// Execute the function in a goroutine to handle timeouts
	errChan := make(chan error, 1)
	go func() {
		errChan <- fn()
	}()

	select {
	case err := <-errChan:
		b.onResult(err)
		return err
	case <-callCtx.Done():
		b.onResult(ErrRequestTimeout)
		return ErrRequestTimeout
	}
}

// beforeCall checks if the call should be allowed and updates state
func (b *Breaker) beforeCall() (State, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	now := time.Now()

	switch b.state {
	case StateClosed:
		// Allow call
		return StateClosed, nil

	case StateOpen:
		// Check if recovery timeout has passed
		if now.Sub(b.lastFailureTime) >= b.config.RecoveryTimeout {
			b.state = StateHalfOpen
			b.activeCalls = 0
			b.successCount = 0
			return StateHalfOpen, nil
		}
		return StateOpen, ErrCircuitOpen

	case StateHalfOpen:
		// Limit concurrent calls in half-open state
		if b.activeCalls >= b.config.MaxConcurrentCalls {
			return StateHalfOpen, ErrTooManyCalls
		}
		b.activeCalls++
		return StateHalfOpen, nil

	default:
		return StateClosed, nil
	}
}

// afterCall decrements active calls counter for half-open state
func (b *Breaker) afterCall(isHalfOpen bool) {
	if isHalfOpen {
		b.mutex.Lock()
		b.activeCalls--
		b.mutex.Unlock()
	}
}

// onResult processes the result of a call and updates circuit breaker state
func (b *Breaker) onResult(err error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if err != nil {
		b.onFailure()
	} else {
		b.onSuccess()
	}
}

// onFailure handles a failed call
func (b *Breaker) onFailure() {
	b.failureCount++
	b.lastFailureTime = time.Now()

	switch b.state {
	case StateClosed:
		if b.failureCount >= b.config.FailureThreshold {
			b.state = StateOpen
		}
	case StateHalfOpen:
		b.state = StateOpen
		b.successCount = 0
	}
}

// onSuccess handles a successful call
func (b *Breaker) onSuccess() {
	switch b.state {
	case StateClosed:
		b.failureCount = 0
	case StateHalfOpen:
		b.successCount++
		if b.successCount >= b.config.SuccessThreshold {
			b.state = StateClosed
			b.failureCount = 0
			b.successCount = 0
		}
	}
}

// State returns the current state of the circuit breaker
func (b *Breaker) State() State {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.state
}

// Stats returns statistics about the circuit breaker
type Stats struct {
	State        State
	FailureCount int
	SuccessCount int
	ActiveCalls  int
}

// Stats returns current circuit breaker statistics
func (b *Breaker) Stats() Stats {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return Stats{
		State:        b.state,
		FailureCount: b.failureCount,
		SuccessCount: b.successCount,
		ActiveCalls:  b.activeCalls,
	}
}

// Reset resets the circuit breaker to closed state
func (b *Breaker) Reset() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.state = StateClosed
	b.failureCount = 0
	b.successCount = 0
	b.activeCalls = 0
}