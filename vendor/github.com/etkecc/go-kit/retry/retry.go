// Package retry provides a simple retry mechanism for Go with linear backoff and jitter.
//
// # Usage (simple)
//
// Create a retrier with default settings and execute a function:
//
//	err := retry.New().Do(func() error {
//		return errors.New("this will be retried")
//	})
//	if err != nil {
//		fmt.Println("error:", err)
//	}
//
// # Usage (with options)
//
// Create a retrier with custom settings:
//
//	r := retry.New(
//		retry.WithMaxRetries(5),
//		retry.WithDelayStep(100*time.Millisecond),
//		retry.WithJitter(true),
//	)
//	err := r.Do(func() error {
//		return errors.New("this will be retried")
//	})
//	if err != nil {
//		fmt.Println("error:", err)
//	}
package retry

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"
)

var (
	// ErrNilRetry is returned by Do/DoCtx when called on a nil *Retry receiver.
	ErrNilRetry = errors.New("retry: nil retrier")
	// ErrNilContext is returned by DoCtx when a nil context is passed.
	ErrNilContext = errors.New("retry: nil context")
	// ErrNilFn is returned by Do/DoCtx when a nil function is passed.
	ErrNilFn = errors.New("retry: nil fn")
)

// DelayHinter is implemented by errors that carry their own retry delay, the way an HTTP
// 429 or 503 carries Retry-After. When DoCtx hits a retryable error whose chain satisfies
// DelayHinter (matched by errors.As), that duration replaces the linear backoff for the
// next wait, honored as a floor with up to 10% jitter so a fleet the server told to wait
// doesn't wake in lockstep and hammer it anyway.
//
// Implement it on a VALUE receiver. A pointer-receiver method is not in a value's method
// set, so a value-wrapped error slips past errors.As, the hint drops silently to default
// backoff, and that lockstep stampede hits after all. The failure passes every test that
// doesn't assert the delay.
type DelayHinter interface {
	SuggestedRetryDelay() time.Duration
}

// hintError is a bare delay carrier with no failure of its own, returned by After to
// feed a duration through DoCtx via DelayHinter. Its methods take a value receiver so
// errors.As matches it whether a caller wraps it by value or pointer.
type hintError struct {
	delay time.Duration
}

// Retry holds the configuration for executing a function with retry logic.
// It implements linear backoff with optional jitter to handle transient failures.
//
// The Retry struct is not safe for concurrent use across goroutines if options are mutated
// during Do execution. Do itself is stateless with respect to the struct fields.
type Retry struct {
	maxRetries int
	delayStep  time.Duration
	retryIf    func(error) bool
	jitter     bool
}

// New creates a new Retry instance with default settings, then applies the given options.
//
// Default settings applied by New:
//   - maxRetries: DefaultMaxRetries (3 total attempts)
//   - delayStep: DefaultDelayStep (1 second)
//   - jitter: DefaultJitter (true)
//   - retryIf: retry on any non-nil error
//
// Options are applied in the order they are passed. Nil options are ignored.
// Values of maxRetries less than 1 are clamped to 1, ensuring fn is invoked at least once.
func New(options ...Option) *Retry {
	r := &Retry{
		maxRetries: DefaultMaxRetries,
		delayStep:  DefaultDelayStep,
		jitter:     DefaultJitter,
		retryIf:    func(err error) bool { return err != nil },
	}

	for _, opt := range options {
		if opt != nil {
			opt(r)
		}
	}

	if r.maxRetries < 1 {
		r.maxRetries = 1
	}

	return r
}

// DoCtx executes fn repeatedly until it returns nil, the context is canceled,
// the retryIf predicate returns false, or the maximum number of attempts is exhausted.
//
// Returns ErrNilRetry, ErrNilContext, or ErrNilFn immediately if the receiver,
// ctx, or fn are nil respectively.
//
// The function implements linear backoff with optional jitter:
//   - Checks ctx.Err() before each attempt; returns ctx.Err() immediately if set
//   - Executes fn immediately (attempt 1)
//   - If fn returns nil, DoCtx returns nil immediately
//   - If retryIf returns false for the error, DoCtx returns the error immediately
//   - Between attempts, DoCtx sleeps for delayStep*(i+1); ctx cancellation interrupts the sleep
//   - If jitter is enabled, the delay is a random duration in [0, base), AWS full jitter
//   - If the error carries a DelayHinter (e.g. a Retry-After), its duration replaces
//     the step as a floor with up to 10% jitter on top
//   - After the final attempt, no sleep occurs
//   - If all attempts fail, DoCtx returns the error from the last attempt
func (r *Retry) DoCtx(ctx context.Context, fn func() error) error {
	if err := r.validate(ctx, fn); err != nil {
		return err
	}

	maxRetries := max(r.maxRetries, 1)

	var err error
	for i := range maxRetries {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err = fn(); err == nil {
			return nil
		}
		if !r.shouldRetry(err) {
			return err
		}
		if i == maxRetries-1 {
			break
		}
		if err := r.sleep(ctx, r.nextDelay(i, err)); err != nil {
			return err
		}
	}
	return err
}

// validate checks for nil receiver, ctx, and fn before the retry loop.
func (r *Retry) validate(ctx context.Context, fn func() error) error {
	if r == nil {
		return ErrNilRetry
	}
	if ctx == nil {
		return ErrNilContext
	}
	if fn == nil {
		return ErrNilFn
	}
	return nil
}

// shouldRetry reports whether err warrants another attempt.
func (r *Retry) shouldRetry(err error) bool {
	if r.retryIf == nil {
		return true
	}
	return r.retryIf(err)
}

// Do executes fn repeatedly until it returns nil or the maximum number of attempts is exhausted.
// It is equivalent to DoCtx(context.Background(), fn).
//
// See DoCtx for full documentation.
func (r *Retry) Do(fn func() error) error {
	return r.DoCtx(context.Background(), fn)
}

// sleep waits for delay or until ctx is done, whichever comes first.
func (r *Retry) sleep(ctx context.Context, delay time.Duration) error {
	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// calculateJitter returns a random delay in [0, base), AWS full jitter.
func (r *Retry) calculateJitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(delay.Nanoseconds())) //nolint:gosec // jitter does not require cryptographic randomness
}

// nextDelay returns the wait before attempt i+1's retry. A DelayHinter in err's chain
// (a server's Retry-After, say) overrides the linear step: its duration is a floor with
// up to 10% jitter, so a fleet handed the same interval doesn't wake in one wave. The
// hint is read first so the linear branch's jitter roll is skipped when a hint applies.
func (r *Retry) nextDelay(i int, err error) time.Duration {
	var dh DelayHinter
	if errors.As(err, &dh) {
		if hint := dh.SuggestedRetryDelay(); hint > 0 {
			if r.jitter {
				return hint + r.calculateJitter(hint/10)
			}
			return hint
		}
	}
	delay := r.delayStep * time.Duration(i+1)
	if r.jitter {
		delay = r.calculateJitter(delay)
	}
	return delay
}

// After returns an error that makes DoCtx wait d before the next attempt in place of the
// linear step, the safe way to honor a server's Retry-After without hand-rolling
// DelayHinter: skip it and a rate-limited fleet can wake in sync and slam the server again.
// The returned value satisfies the interface on a value receiver, so errors.As matches it
// whether a caller passes it by value or wraps a pointer to it. On its own it is a terminal
// error like any other; the delay only fires when it reaches DoCtx as a retryable error.
func After(d time.Duration) error {
	return hintError{delay: d}
}

func (e hintError) Error() string {
	return "retry: suggested delay " + e.delay.String()
}

func (e hintError) SuggestedRetryDelay() time.Duration {
	return e.delay
}
